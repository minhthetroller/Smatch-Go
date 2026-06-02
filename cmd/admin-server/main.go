package main

import (
	"context"
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
	s3pkg "github.com/smatch/badminton-backend/platform/s3"
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

	// S3
	s3Client, err := s3pkg.New(ctx, s3pkg.Config{
		Region:             cfg.AWS.Region,
		AccessKeyID:        cfg.AWS.AccessKeyID,
		SecretAccessKey:    cfg.AWS.SecretAccessKey,
		Endpoint:           cfg.AWS.Endpoint,
		BucketProfile:      cfg.AWS.BucketProfile,
		BucketMatches:      cfg.AWS.BucketMatches,
		BucketBusinessDocs: cfg.AWS.BucketBusinessDocs,
	})
	if err != nil {
		logger.Warn("s3 unavailable", zap.Error(err))
	}
	if s3Client != nil {
		if err := s3Client.EnsureBuckets(ctx, cfg.AWS.BucketProfile, cfg.AWS.BucketMatches, cfg.AWS.BucketBusinessDocs); err != nil {
			logger.Warn("s3 bucket init failed", zap.Error(err))
		}
	}

	// Repositories
	userRepo := repository.NewUserRepository(pool)
	bpRepo := repository.NewBusinessProfileRepository(pool)
	coRepo := repository.NewCourtOwnerRepository(pool)
	adminRepo := repository.NewAdminRepository(pool)
	courtRepo := repository.NewCourtRepository(pool)

	// Services
	uploadSvc := service.NewUploadService(s3Client, cfg.AWS.BucketBusinessDocs)
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

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestLogger(logger))
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestSize(10 * 1024 * 1024))
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(middleware.SecureHeaders)
	r.Use(cors.New(cors.Options{
		AllowedOrigins:   []string{cfg.AdminWebOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Admin-Secret", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}).Handler)
	r.Use(middleware.CSRFProtection) // security gate before rate limiting
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

	// Owner routes
	r.Route("/api/owner", func(r chi.Router) {
		r.Use(authMw.RequireAuth)
		r.Use(authMw.RequireCourtOwner)

		r.Post("/business-profile", bpH.Submit)
		r.Get("/business-profile", bpH.GetMine)
		r.Put("/business-profile", bpH.Submit)

		r.Get("/courts", coH.ListCourts)
		r.Get("/courts/{id}/stats", coH.GetCourtStats)
		r.Post("/courts/{id}/close", coH.CloseCourt)
		r.Post("/courts/{id}/open", coH.OpenCourt)
		r.Post("/courts/{id}/subcourts/{subId}/close", coH.CloseSubCourt)
		r.Post("/courts/{id}/subcourts/{subId}/open", coH.OpenSubCourt)
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
