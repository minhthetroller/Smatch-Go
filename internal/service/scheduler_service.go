package service

import (
	"context"

	"github.com/robfig/cron/v3"
	"github.com/smatch/badminton-backend/internal/repository"
	ws "github.com/smatch/badminton-backend/internal/websocket"
	zalopay "github.com/smatch/badminton-backend/platform/zalopay"
	"go.uber.org/zap"
)

type SchedulerService struct {
	logger    *zap.Logger
	availRepo *repository.AvailabilityRepository
	payments  *PaymentService
	cron      *cron.Cron
}

func NewSchedulerService(
	logger *zap.Logger,
	availRepo *repository.AvailabilityRepository,
	paymentRepo *repository.PaymentRepository,
	matchRepo *repository.MatchRepository,
	hub *ws.Hub,
	zalo *zalopay.Client,
	redis *RedisService,
) *SchedulerService {
	payments := NewPaymentService(paymentRepo, availRepo, matchRepo, redis, zalo, hub, logger)
	return &SchedulerService{
		logger:    logger,
		availRepo: availRepo,
		payments:  payments,
	}
}

func (s *SchedulerService) Start() {
	s.cron = cron.New()

	// Mark completed bookings every 5 minutes
	s.cron.AddFunc("*/5 * * * *", func() { //nolint:errcheck
		ctx := context.Background()
		n, err := s.availRepo.MarkCompletedBookings(ctx)
		if err != nil {
			s.logger.Error("mark completed bookings", zap.Error(err))
		} else if n > 0 {
			s.logger.Info("marked bookings completed", zap.Int64("count", n))
		}
	})

	// Reconcile pending BOOKING and MATCH_JOIN payments frequently.
	s.cron.AddFunc("@every 10s", func() { //nolint:errcheck
		ctx := context.Background()
		if err := s.payments.ReconcilePendingPayments(ctx); err != nil {
			s.logger.Error("reconcile pending payments", zap.Error(err))
		}
	})

	s.cron.Start()
	s.logger.Info("scheduler started")
}

func (s *SchedulerService) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
}
