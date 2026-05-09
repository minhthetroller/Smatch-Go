package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/rs/cors"
	"go.uber.org/zap"

	"github.com/smatch/badminton-backend/internal/config"
	"github.com/smatch/badminton-backend/internal/handler"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/repository"
	"github.com/smatch/badminton-backend/internal/service"
	fbpkg "github.com/smatch/badminton-backend/platform/firebase"
	pgpkg "github.com/smatch/badminton-backend/platform/postgres"
	redispkg "github.com/smatch/badminton-backend/platform/redis"
	blobpkg "github.com/smatch/badminton-backend/platform/blob"
)

func main() {
	cfg := config.Load()

	// Logger
	var logger *zap.Logger
	var err error
	if cfg.NodeEnv == "production" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		panic(err)
	}
	defer logger.Sync() //nolint:errcheck

	ctx := context.Background()

	// Postgres
	pool, err := pgpkg.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("postgres connect", zap.Error(err))
	}
	defer pool.Close()
	logger.Info("postgres connected")

	// Redis
	redisClient, err := redispkg.NewClient(ctx, redispkg.Config{
		Host:       cfg.Redis.Host,
		Port:       cfg.Redis.Port,
		Password:   cfg.Redis.Password,
		TLSEnabled: cfg.Redis.TLSEnabled,
	})
	if err != nil {
		logger.Warn("redis unavailable (continuing without cache)", zap.Error(err))
	} else {
		logger.Info("redis connected")
	}

	// Firebase
	if cfg.FirebaseCredentialsFile == "" {
		logger.Fatal("firebase credentials file not configured")
	}
	fbClient, err := fbpkg.New(ctx, cfg.FirebaseCredentialsFile)
	if err != nil {
		logger.Fatal("firebase init failed", zap.Error(err))
	}
	logger.Info("firebase connected")

	// Blob Storage
	blobClient, err := blobpkg.New(ctx, blobpkg.Config{
		AccountName:           cfg.Blob.AccountName,
		AccountKey:            cfg.Blob.AccountKey,
		Endpoint:              cfg.Blob.Endpoint,
		ContainerProfile:      cfg.Blob.ContainerProfile,
		ContainerMatches:      cfg.Blob.ContainerMatches,
		ContainerBusinessDocs: cfg.Blob.ContainerBusinessDocs,
	})
	if err != nil {
		logger.Warn("blob storage unavailable", zap.Error(err))
	}

	// Repositories
	userRepo := repository.NewUserRepository(pool)
	bpRepo := repository.NewBusinessProfileRepository(pool)
	coRepo := repository.NewCourtOwnerRepository(pool)
	adminRepo := repository.NewAdminRepository(pool)
	courtRepo := repository.NewCourtRepository(pool)

	// Services
	uploadSvc := service.NewUploadService(blobClient, cfg.Blob.ContainerBusinessDocs)
	bpSvc := service.NewBusinessProfileService(bpRepo, userRepo, uploadSvc)
	coSvc := service.NewCourtOwnerService(coRepo, courtRepo)
	adminSvc := service.NewAdminService(bpRepo, userRepo, coRepo, adminRepo)

	// Auth Middleware
	authMw := middleware.NewAuthMiddleware(fbClient, userRepo, cfg.AdminSecret).WithBusinessProfileRepo(bpRepo)

	// Handlers
	bpH := handler.NewBusinessProfileHandler(bpSvc)
	coH := handler.NewCourtOwnerHandler(coSvc)
	adminH := handler.NewAdminHandler(adminSvc)

	// Router
	r := chi.NewRouter()

	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.RequestSize(10 * 1024 * 1024))
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(middleware.SecureHeaders)
	allowedOrigins := []string{cfg.AdminWebOrigin}
	if cfg.NodeEnv == "development" {
		allowedOrigins = append(allowedOrigins, "http://localhost:5173")
	}
	r.Use(cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Admin-Secret"},
		AllowCredentials: true,
		MaxAge:           300,
	}).Handler)
	if redisClient != nil {
		r.Use(httprate.LimitByIP(100, time.Minute))
	}

	// Health
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})
	r.Get("/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"1.0.0","service":"smatch-admin"}`)) //nolint:errcheck
	})

	// Auth handler (for verify endpoint)
	authH := handler.NewAuthHandler(fbClient, userRepo, nil)

	// Public auth route (no auth required)
	r.Post("/api/auth/verify", authH.Verify)

	// Auth routes (require valid Firebase token)
	r.Route("/api/auth", func(r chi.Router) {
		r.Use(authMw.RequireAuth)
		r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
			user := middleware.UserFromContext(r.Context())
			roles, _ := userRepo.GetRoles(r.Context(), user.ID)
			dto := handler.MapUserToDTO(user)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"id":          dto.ID,
					"firebaseUid": dto.FirebaseUID,
					"email":       dto.Email,
					"username":    dto.Username,
					"provider":    dto.Provider,
					"isAnonymous": dto.IsAnonymous,
					"firstName":   dto.FirstName,
					"lastName":    dto.LastName,
					"gender":      dto.Gender,
					"phoneNumber": dto.PhoneNumber,
					"photoUrl":    dto.PhotoURL,
					"address":     dto.Address,
					"roles":       roles,
					"createdAt":   dto.CreatedAt,
					"updatedAt":   dto.UpdatedAt,
				},
			})
		})
	})

	// Owner routes — business profile open to all authenticated users;
	// court management requires approved court_owner role.
	r.Route("/api/owner", func(r chi.Router) {
		r.Use(authMw.RequireAuth)
		r.Post("/business-profile", bpH.Submit)
		r.Get("/business-profile", bpH.GetMine)
		r.Put("/business-profile", bpH.Submit)
		r.Delete("/business-profile", bpH.DeleteMine)

		r.Group(func(r chi.Router) {
			r.Use(authMw.RequireCourtOwner)
			r.Get("/courts", coH.ListCourts)
			r.Get("/courts/{id}", coH.GetCourt)
			r.Get("/courts/{id}/stats", coH.GetCourtStats)
			r.Post("/courts/{id}/close", coH.CloseCourt)
			r.Post("/courts/{id}/open", coH.OpenCourt)
			r.Post("/courts/{id}/subcourts/{subId}/close", coH.CloseSubCourt)
			r.Post("/courts/{id}/subcourts/{subId}/open", coH.OpenSubCourt)
		})
	})

	// Admin routes
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(authMw.RequireAdmin)

		r.Get("/business-profiles", adminH.ListApplications)
		r.Get("/business-profiles/{id}", adminH.GetApplication)
		r.Post("/business-profiles/{id}/approve", adminH.ReviewApplication)
		r.Post("/business-profiles/{id}/reject", adminH.ReviewApplication)
		r.Post("/business-profiles/{id}/request-resubmit", adminH.ReviewApplication)
		r.Get("/stats", adminH.GetStats)
		r.Get("/stats/timeseries", adminH.GetTimeseriesStats)
	})

	// Server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.AdminPort),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("admin server starting", zap.Int("port", cfg.AdminPort))

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down admin server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}
