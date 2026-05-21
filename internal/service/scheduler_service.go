package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/robfig/cron/v3"
	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/repository"
	ws "github.com/smatch/badminton-backend/internal/websocket"
	zalopay "github.com/smatch/badminton-backend/platform/zalopay"
	"go.uber.org/zap"
)

type SchedulerService struct {
	logger      *zap.Logger
	availRepo   *repository.AvailabilityRepository
	paymentRepo *repository.PaymentRepository
	matchRepo   *repository.MatchRepository
	hub         *ws.Hub
	zalo        *zalopay.Client
	redis       *RedisService
	cron        *cron.Cron
	timeoutSec  int
}

func NewSchedulerService(
	logger *zap.Logger,
	availRepo *repository.AvailabilityRepository,
	paymentRepo *repository.PaymentRepository,
	matchRepo *repository.MatchRepository,
	hub *ws.Hub,
	zalo *zalopay.Client,
	redis *RedisService,
	slotLockTTL int,
) *SchedulerService {
	return &SchedulerService{
		logger:      logger,
		availRepo:   availRepo,
		paymentRepo: paymentRepo,
		matchRepo:   matchRepo,
		hub:         hub,
		zalo:        zalo,
		redis:       redis,
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

	// Reconcile stale pending booking payments with ZaloPay every 1 minute
	s.cron.AddFunc("* * * * *", func() { //nolint:errcheck
		ctx := context.Background()
		s.reconcilePayments(ctx)
	})

	s.cron.Start()
	s.logger.Info("scheduler started")
}

// reconcilePayments queries ZaloPay for stale pending BOOKING payments
// and updates their status if the payment has already succeeded.
func (s *SchedulerService) reconcilePayments(ctx context.Context) {
	minAgeSec := 60 // give callback 1 min to arrive
	maxAgeSec := s.timeoutSec

	payments, err := s.paymentRepo.FindStalePendingBookingPayments(ctx, minAgeSec, maxAgeSec)
	if err != nil {
		s.logger.Error("reconcile: find stale pending payments", zap.Error(err))
		return
	}
	if len(payments) == 0 {
		return
	}

	for _, p := range payments {
		p := p
		if p.AppTransID == "" {
			continue
		}

		resp, err := s.zalo.QueryOrder(p.AppTransID)
		if err != nil {
			s.logger.Warn("reconcile: query order failed",
				zap.String("payment_id", p.ID),
				zap.String("app_trans_id", p.AppTransID),
				zap.Error(err))
			continue
		}

		fields := []zap.Field{
			zap.String("payment_id", p.ID),
			zap.String("app_trans_id", p.AppTransID),
			zap.Int("return_code", resp.ReturnCode),
			zap.String("return_message", resp.ReturnMessage),
		}

		switch resp.ReturnCode {
		case 1: // success
			zpTransID := fmt.Sprintf("%d", resp.ZPTransID)
			rawData, _ := json.Marshal(resp)
			updated, err := s.paymentRepo.UpdateStatusByAppTransID(
				ctx, p.AppTransID,
				domain.PaymentStatusSuccess,
				&zpTransID,
				json.RawMessage(rawData),
			)
			if err != nil || updated == nil {
				s.logger.Error("reconcile: update payment status failed",
					append(fields, zap.Error(err))...)
				continue
			}

			// Confirm booking if linked
			if updated.BookingID != nil {
				booking, err := s.availRepo.GetBookingByID(ctx, *updated.BookingID)
				if err == nil && booking != nil {
					_ = s.availRepo.UpdateBookingStatus(ctx, *updated.BookingID, "confirmed")

					// Release slot locks
					if s.redis != nil {
						s.redis.ReleaseSlotLocks(ctx, []SlotLockSpec{{
							SubCourtID: booking.SubCourtID,
							Date:       booking.Date,
							StartTime:  booking.StartTime,
							EndTime:    booking.EndTime,
							BookingID:  *updated.BookingID,
						}})
					}

					// Release group booking locks if any
					if booking.GroupID != nil {
						groupBookings, err := s.availRepo.GetBookingsByGroupID(ctx, *booking.GroupID)
						if err == nil {
							for _, gb := range groupBookings {
								if gb.ID != *updated.BookingID {
									_ = s.availRepo.UpdateBookingStatus(ctx, gb.ID, "confirmed")
									if s.redis != nil {
										s.redis.ReleaseSlotLocks(ctx, []SlotLockSpec{{
											SubCourtID: gb.SubCourtID,
											Date:       gb.Date,
											StartTime:  gb.StartTime,
											EndTime:    gb.EndTime,
											BookingID:  gb.ID,
										}})
									}
								}
							}
						}
					}
				}
			}

			s.hub.NotifyPaymentStatus(ws.PaymentNotification{
				Type:      "payment_success",
				PaymentID: updated.ID,
				Status:    string(domain.PaymentStatusSuccess),
				BookingID: updated.BookingID,
				ZPTransID: &zpTransID,
				Message:   "Payment successful",
			})
			s.logger.Info("reconcile: payment marked success", fields...)

		case 2: // processing
			s.logger.Debug("reconcile: payment still processing", fields...)

		default: // failed or not found
			s.logger.Info("reconcile: payment failed or not found on gateway", fields...)
			// Let the existing expiry job handle cancellation
		}
	}
}

func (s *SchedulerService) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
}
