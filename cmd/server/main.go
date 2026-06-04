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

	"github.com/smatch/badminton-backend/internal/config"
	"github.com/smatch/badminton-backend/internal/handler"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/repository"
	"github.com/smatch/badminton-backend/internal/service"
	ws "github.com/smatch/badminton-backend/internal/websocket"
	fbpkg "github.com/smatch/badminton-backend/platform/firebase"
	pgpkg "github.com/smatch/badminton-backend/platform/postgres"
	redispkg "github.com/smatch/badminton-backend/platform/redis"
	s3pkg "github.com/smatch/badminton-backend/platform/s3"
	zalopkg "github.com/smatch/badminton-backend/platform/zalopay"
	"go.uber.org/zap"
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

	// ── Database ────────────────────────────────────────────────────────────
	pool, err := pgpkg.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("postgres connect", zap.Error(err))
	}
	defer pool.Close()
	logger.Info("postgres connected")

	// ── Redis ───────────────────────────────────────────────────────────────
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

	// ── Firebase ────────────────────────────────────────────────────────────
	if cfg.FirebaseCredentialsFile == "" {
		logger.Fatal("firebase credentials file not configured (FIREBASE_CREDENTIALS_FILE)")
	}
	fbClient, err := fbpkg.New(ctx, cfg.FirebaseCredentialsFile)
	if err != nil {
		logger.Fatal("firebase init failed", zap.Error(err))
	}
	logger.Info("firebase connected")

	// ── S3 ──────────────────────────────────────────────────────────────────
	s3Client, err := s3pkg.New(ctx, s3pkg.Config{
		Region:          cfg.AWS.Region,
		AccessKeyID:     cfg.AWS.AccessKeyID,
		SecretAccessKey: cfg.AWS.SecretAccessKey,
		Endpoint:        cfg.AWS.Endpoint,
		BucketProfile:   cfg.AWS.BucketProfile,
		BucketMatches:   cfg.AWS.BucketMatches,
	})
	if err != nil {
		logger.Warn("s3 unavailable", zap.Error(err))
	}
	if s3Client != nil {
		if err := s3Client.EnsureBuckets(ctx, cfg.AWS.BucketProfile, cfg.AWS.BucketMatches); err != nil {
			logger.Warn("s3 bucket init failed", zap.Error(err))
		}
	}

	// ── ZaloPay ─────────────────────────────────────────────────────────────
	zaloClient := zalopkg.New(zalopkg.Config{
		AppID:       intFromStr(cfg.ZaloPay.AppID),
		Key1:        cfg.ZaloPay.Key1,
		Key2:        cfg.ZaloPay.Key2,
		Endpoint:    cfg.ZaloPay.Endpoint,
		CallbackURL: cfg.ZaloPay.CallbackURL,
	})

	// ── Repositories ────────────────────────────────────────────────────────
	userRepo := repository.NewUserRepository(pool)
	courtRepo := repository.NewCourtRepository(pool)
	availRepo := repository.NewAvailabilityRepository(pool)
	paymentRepo := repository.NewPaymentRepository(pool)
	matchRepo := repository.NewMatchRepository(pool)
	searchRepo := repository.NewSearchRepository(pool)

	// ── Services ────────────────────────────────────────────────────────────
	var redisSvc *service.RedisService
	if redisClient != nil {
		redisSvc = service.NewRedisService(redisClient, service.PaymentValiditySeconds)
	}
	availSvc := service.NewAvailabilityService(availRepo, courtRepo)

	// ── WebSocket Hub ────────────────────────────────────────────────────────
	hub := ws.NewHub(logger)

	// ── Scheduler ───────────────────────────────────────────────────────────
	scheduler := service.NewSchedulerService(logger, availRepo, paymentRepo, matchRepo, hub, zaloClient, redisSvc)
	scheduler.Start()
	defer scheduler.Stop()

	// ── Auth Middleware ──────────────────────────────────────────────────────
	authMw := middleware.NewAuthMiddleware(fbClient, userRepo, cfg.AdminSecret)

	// ── Handlers ────────────────────────────────────────────────────────────
	authH := handler.NewAuthHandler(fbClient, userRepo, availRepo)
	courtH := handler.NewCourtHandler(courtRepo)
	availH := handler.NewAvailabilityHandler(availSvc, logger)
	matchH := handler.NewMatchHandler(matchRepo, redisSvc, hub)
	paymentH := handler.NewPaymentHandler(paymentRepo, availRepo, matchRepo, redisSvc, zaloClient, hub, logger,
		cfg.PaymentWSTicketTTLSec, cfg.Port, cfg.NodeEnv)
	searchH := handler.NewSearchHandler(redisSvc, searchRepo, courtRepo)
	proxyH := handler.NewProxyHandler(cfg.TileServerURL, cfg.TileLayerID)
	wsH := handler.NewWebSocketHandler(hub)
	loadTestH := handler.NewLoadTestHandler(cfg.LoadTestStressEnabled, cfg.AdminSecret)

	var uploadSvc *service.UploadService
	if s3Client != nil {
		uploadSvc = service.NewUploadService(s3Client, cfg.AWS.BucketMatches)
	}
	uploadH := handler.NewUploadHandler(uploadSvc)

	// ── Wire payment auto-cancel on WS disconnect ────────────────────────────
	hub.ValidatePaymentTicket = func(ctx context.Context, paymentID, ticket string) (bool, error) {
		if redisSvc == nil {
			return false, nil
		}
		return redisSvc.ConsumePaymentWSTicket(ctx, paymentID, ticket)
	}
	hub.PaymentStatusSnapshot = paymentH.PaymentStatusNotification
	hub.OnPaymentDisconnect = func(paymentID string) {
		bgCtx := context.Background()
		paymentH.CancelPaymentByID(bgCtx, paymentID)
	}

	// ── Router ──────────────────────────────────────────────────────────────
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestLogger(logger))
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestSize(10 * 1024 * 1024))
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "X-Admin-Secret"},
	}).Handler)
	if redisClient != nil {
		limitByIP := httprate.LimitByIP(100, time.Minute)
		r.Use(func(next http.Handler) http.Handler {
			limited := limitByIP(next)
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if loadTestStressRateLimitBypass(r, cfg.LoadTestStressEnabled, cfg.AdminSecret) {
					next.ServeHTTP(w, r)
					return
				}
				limited.ServeHTTP(w, r)
			})
		})
	}

	// Health / version
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})

	// Remove version path in prod
	r.Get("/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"1.0.0","lang":"go"}`)) //nolint:errcheck
	})

	// WebSocket
	r.Get("/ws/payments", wsH.ServePayments)
	r.Get("/ws/matches", wsH.ServeMatches)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Post("/load-test/stress", loadTestH.Stress)

		// ── Auth ─────────────────────────────────────────────────────────
		r.Route("/auth", func(r chi.Router) {
			r.With(httprate.LimitByIP(10, time.Minute)).Post("/verify", authH.Verify)
			r.Post("/anonymous", authH.Anonymous)
			r.With(authMw.RequireAuth).Get("/me", authH.Me)
			r.With(authMw.RequireRegisteredUser).Put("/me", authH.UpdateMe)
			r.With(authMw.RequireRegisteredUser).Delete("/me", authH.DeleteMe)
			r.With(authMw.RequireAuth).Get("/me/bookings", authH.MyBookings)
			r.With(authMw.RequireRegisteredUser).Post("/me/photo", authH.UploadPhoto)
			r.With(authMw.RequireAuth).Post("/fcm-token", authH.AddFCMToken)
			r.With(authMw.RequireAuth).Delete("/fcm-token/{token}", authH.RemoveFCMToken)
			r.Get("/username/check", authH.CheckUsername)
			r.Get("/username/lookup", authH.LookupUsername)
			r.With(authMw.RequireAnonymousUser).Post("/convert", authH.Convert)
			r.With(authMw.RequireRegisteredUser).Post("/link-bookings", authH.LinkBookings)
		})

		// ── Courts ───────────────────────────────────────────────────────
		r.Route("/courts", func(r chi.Router) {
			r.Get("/", courtH.List)
			r.Get("/nearby", courtH.Nearby)
			r.With(authMw.RequireAdmin).Post("/", courtH.Create)
			r.Get("/{id}", courtH.Get)
			r.With(authMw.RequireAdmin).Put("/{id}", courtH.Update)
			r.With(authMw.RequireAdmin).Delete("/{id}", courtH.Delete)
			r.Get("/{courtId}/availability", availH.GetAvailability)
		})

		// ── Bookings ──────────────────────────────────────────────────────
		r.Route("/bookings", func(r chi.Router) {
			r.With(authMw.OptionalAuth).Post("/", availH.CreateBooking)
			r.With(authMw.RequireAuth).Get("/{id}", availH.GetBooking)
			r.With(authMw.RequireAuth).Delete("/{id}", availH.CancelBooking)
			r.With(authMw.OptionalAuth).Get("/{id}/payment", paymentH.GetBookingPayment)
		})

		// ── Payments ──────────────────────────────────────────────────────
		r.Route("/payments", func(r chi.Router) {
			r.With(authMw.OptionalAuth).Post("/create", paymentH.CreatePayment)
			r.With(httprate.LimitByIP(10, time.Minute)).Post("/callback", paymentH.Callback)
			r.With(authMw.RequireAuth).Get("/{id}", paymentH.GetPayment)
			r.With(authMw.RequireAuth).Post("/{id}/cancel", paymentH.CancelPayment)
		})

		// ── Matches ───────────────────────────────────────────────────────
		r.Route("/matches", func(r chi.Router) {
			r.With(authMw.OptionalAuth).Get("/", matchH.GetAllMatches)
			r.With(authMw.RequireRegisteredUser).Get("/hosted", matchH.GetHostedMatches)
			r.With(authMw.RequireRegisteredUser).Get("/joined", matchH.GetJoinedMatches)
			r.With(authMw.RequireRegisteredUser).Post("/", matchH.CreateMatch)
			r.With(authMw.OptionalAuth).Get("/{id}", matchH.GetMatch)
			r.With(authMw.RequireRegisteredUser).Put("/{id}", matchH.UpdateMatch)
			r.With(authMw.RequireRegisteredUser).Delete("/{id}", matchH.CancelMatch)
			r.With(authMw.RequireRegisteredUser).Post("/{id}/join", matchH.JoinMatch)
			r.With(authMw.RequireRegisteredUser).Delete("/{id}/leave", matchH.LeaveMatch)
			r.With(authMw.RequireRegisteredUser).Get("/{id}/requests", matchH.GetJoinRequests)
			r.With(authMw.RequireRegisteredUser).Put("/{id}/requests/{playerId}/respond", matchH.RespondToJoinRequest)
			r.With(authMw.RequireRegisteredUser).Post("/{id}/payment", paymentH.CreateMatchPayment)
			r.With(authMw.RequireAuth).Get("/{matchId}/payment/{paymentId}/status", paymentH.GetMatchPaymentStatus)
		})

		// ── Search ────────────────────────────────────────────────────────
		r.Route("/search", func(r chi.Router) {
			r.Get("/autocomplete", searchH.Autocomplete)
			r.Get("/courts", searchH.SearchCourts)
			r.Get("/popular", searchH.Popular)
		})

		// ── Uploads ───────────────────────────────────────────────────────
		r.Route("/uploads", func(r chi.Router) {
			r.With(authMw.RequireRegisteredUser).Post("/match-image", uploadH.UploadMatchImage)
		})

		// ── Admin ─────────────────────────────────────────────────────────
		r.Route("/admin", func(r chi.Router) {
			r.With(authMw.RequireAdmin).Post("/search/reindex", searchH.Reindex)
			r.With(authMw.RequireAdmin).Get("/search/stats", searchH.Stats)
		})

		// ── Map Tiles ─────────────────────────────────────────────────────
		r.With(httprate.LimitByIP(1000, time.Minute)).
			Get("/map-tiles/{z}/{x}/{y}.pbf", proxyH.MapTile)
	})

	// ── HTTP Server ──────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("server starting", zap.Int("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}
	logger.Info("server stopped")
}

func intFromStr(s string) int {
	n := 0
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

func loadTestStressRateLimitBypass(r *http.Request, enabled bool, adminSecret string) bool {
	return enabled &&
		adminSecret != "" &&
		r.Method == http.MethodPost &&
		r.URL.Path == "/api/load-test/stress" &&
		r.Header.Get("X-Admin-Secret") == adminSecret
}
