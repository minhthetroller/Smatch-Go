package service

import (
	"context"

	"github.com/robfig/cron/v3"
	"github.com/smatch/badminton-backend/internal/repository"
	ws "github.com/smatch/badminton-backend/internal/websocket"
	"go.uber.org/zap"
)

type SchedulerService struct {
	logger      *zap.Logger
	availRepo   *repository.AvailabilityRepository
	paymentRepo *repository.PaymentRepository
	matchRepo   *repository.MatchRepository
	hub         *ws.Hub
	cron        *cron.Cron
	timeoutSec  int
}

func NewSchedulerService(
	logger *zap.Logger,
	availRepo *repository.AvailabilityRepository,
	paymentRepo *repository.PaymentRepository,
	matchRepo *repository.MatchRepository,
	hub *ws.Hub,
	slotLockTTL int,
) *SchedulerService {
	return &SchedulerService{
		logger:      logger,
		availRepo:   availRepo,
		paymentRepo: paymentRepo,
		matchRepo:   matchRepo,
		hub:         hub,
		timeoutSec:  slotLockTTL + 300, // 5-min buffer
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

	// Expire pending bookings every 2 minutes
	s.cron.AddFunc("*/2 * * * *", func() { //nolint:errcheck
		ctx := context.Background()
		if err := s.availRepo.MarkExpiredPendingBookings(ctx, s.timeoutSec); err != nil {
			s.logger.Error("mark expired pending bookings", zap.Error(err))
		}
	})

	// Expire pending match payments every 2 minutes
	s.cron.AddFunc("*/2 * * * *", func() { //nolint:errcheck
		ctx := context.Background()
		playerIDs, err := s.paymentRepo.MarkExpiredMatchPayments(ctx, s.timeoutSec)
		if err != nil {
			s.logger.Error("mark expired match payments", zap.Error(err))
			return
		}
		if len(playerIDs) == 0 {
			return
		}
		// Update match_players to EXPIRED
		if err := s.matchRepo.MarkExpiredMatchPlayers(ctx, playerIDs); err != nil {
			s.logger.Error("mark expired match players", zap.Error(err))
		}
		// Notify via WebSocket
		for _, pid := range playerIDs {
			pid := pid
			s.hub.NotifyPaymentStatus(ws.PaymentNotification{
				Type:          "payment_status",
				PaymentID:     pid,
				Status:        "expired",
				MatchPlayerID: &pid,
				Message:       "Payment expired. Please try again.",
			})
		}
		s.logger.Info("marked match payments expired", zap.Int("count", len(playerIDs)))
	})

	s.cron.Start()
	s.logger.Info("scheduler started")
}

func (s *SchedulerService) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
}
