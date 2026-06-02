package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/repository"
	ws "github.com/smatch/badminton-backend/internal/websocket"
	zalopay "github.com/smatch/badminton-backend/platform/zalopay"
	"go.uber.org/zap"
)

const (
	PaymentValidity        = 5 * time.Minute
	PaymentValiditySeconds = int(PaymentValidity / time.Second)
	paymentQueryTimeout    = 3 * time.Second
)

type paymentRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Payment, error)
	FindByAppTransID(ctx context.Context, appTransID string) (*domain.Payment, error)
	FindPendingPayments(ctx context.Context) ([]*domain.Payment, error)
	UpdatePendingStatus(ctx context.Context, id string, status domain.PaymentStatus, zpTransID *string, callbackData json.RawMessage) (*domain.Payment, error)
	UpdatePendingStatusByAppTransID(ctx context.Context, appTransID string, status domain.PaymentStatus, zpTransID *string, callbackData json.RawMessage) (*domain.Payment, error)
}

type paymentAvailabilityRepository interface {
	GetBookingByID(ctx context.Context, id string) (*repository.BookingRow, error)
	GetBookingsByGroupID(ctx context.Context, groupID string) ([]*repository.RawBooking, error)
	UpdateBookingStatus(ctx context.Context, id, status string) error
	MarkExpiredPendingBookings(ctx context.Context, timeoutSec int) error
}

type paymentMatchRepository interface {
	FindByID(ctx context.Context, id string) (*repository.MatchRow, error)
	FindPlayerByID(ctx context.Context, playerID string) (*repository.MatchPlayerRow, error)
	GetNextPosition(ctx context.Context, matchID string) (int, error)
	UpdatePlayerStatus(ctx context.Context, playerID string, status domain.MatchPlayerStatus, position *int) (*repository.MatchPlayerRow, error)
	CountAcceptedPlayers(ctx context.Context, matchID string) (int, error)
	UpdateStatus(ctx context.Context, id string, status domain.MatchStatus) error
	MarkExpiredMatchPlayers(ctx context.Context, playerIDs []string) error
}

type paymentLockReleaser interface {
	ReleaseSlotLocks(ctx context.Context, slots []SlotLockSpec)
}

type paymentOrderQuerier interface {
	QueryOrder(ctx context.Context, appTransID string) (*zalopay.QueryOrderResponse, error)
}

// PaymentService centralizes payment settlement, expiry, side effects, and notifications.
type PaymentService struct {
	paymentRepo paymentRepository
	availRepo   paymentAvailabilityRepository
	matchRepo   paymentMatchRepository
	redis       paymentLockReleaser
	zalo        paymentOrderQuerier
	hub         *ws.Hub
	logger      *zap.Logger
	now         func() time.Time
}

func NewPaymentService(
	paymentRepo paymentRepository,
	availRepo paymentAvailabilityRepository,
	matchRepo paymentMatchRepository,
	redis paymentLockReleaser,
	zalo paymentOrderQuerier,
	hub *ws.Hub,
	logger *zap.Logger,
) *PaymentService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PaymentService{
		paymentRepo: paymentRepo,
		availRepo:   availRepo,
		matchRepo:   matchRepo,
		redis:       redis,
		zalo:        zalo,
		hub:         hub,
		logger:      logger,
		now:         time.Now,
	}
}

func (s *PaymentService) ExpireAt(p *domain.Payment) time.Time {
	if p == nil {
		return time.Time{}
	}
	return p.CreatedAt.Add(PaymentValidity)
}

func (s *PaymentService) RemainingValidity(p *domain.Payment) time.Duration {
	if p == nil {
		return 0
	}
	return s.ExpireAt(p).Sub(s.now())
}

func (s *PaymentService) EnsureCurrentStatus(ctx context.Context, paymentID string) (*domain.Payment, error) {
	payment, err := s.paymentRepo.FindByID(ctx, paymentID)
	if err != nil || payment == nil {
		return payment, err
	}
	return s.EnsurePaymentCurrent(ctx, payment)
}

func (s *PaymentService) EnsurePaymentCurrent(ctx context.Context, payment *domain.Payment) (*domain.Payment, error) {
	if payment == nil || payment.Status != domain.PaymentStatusPending {
		return payment, nil
	}

	settled, ok, err := s.queryAndSettleSuccess(ctx, payment)
	if err != nil {
		s.logger.Warn("payment proactive query failed",
			zap.String("payment_id", payment.ID),
			zap.String("app_trans_id", payment.AppTransID),
			zap.Error(err))
	}
	if ok {
		return settled, nil
	}
	if s.now().Before(s.ExpireAt(payment)) {
		return payment, nil
	}
	return s.expirePayment(ctx, payment)
}

func (s *PaymentService) SettleCallback(ctx context.Context, cb *zalopay.CallbackData) (*domain.Payment, error) {
	if cb == nil {
		return nil, domain.BadRequest("Invalid callback data")
	}
	payment, err := s.paymentRepo.FindByAppTransID(ctx, cb.AppTransID)
	if err != nil || payment == nil {
		return payment, err
	}
	if payment.Status != domain.PaymentStatusPending {
		return payment, nil
	}

	successAt := s.callbackSuccessTime(cb)
	if successAt.After(s.ExpireAt(payment)) {
		return s.expirePayment(ctx, payment)
	}

	zpTransID := fmt.Sprintf("%d", cb.ZPTransID)
	rawData, _ := json.Marshal(cb)
	updated, err := s.paymentRepo.UpdatePendingStatusByAppTransID(
		ctx,
		cb.AppTransID,
		domain.PaymentStatusSuccess,
		&zpTransID,
		json.RawMessage(rawData),
	)
	if err != nil || updated == nil {
		return s.refetchPayment(ctx, payment.ID, err)
	}
	s.applySuccessSideEffects(ctx, updated)
	s.notify(updated, "Payment successful")
	return updated, nil
}

func (s *PaymentService) CancelPayment(ctx context.Context, paymentID, message string) (*domain.Payment, error) {
	payment, err := s.EnsureCurrentStatus(ctx, paymentID)
	if err != nil || payment == nil {
		return payment, err
	}
	if payment.Status != domain.PaymentStatusPending {
		return payment, nil
	}

	updated, err := s.paymentRepo.UpdatePendingStatus(ctx, payment.ID, domain.PaymentStatusFailed, nil, nil)
	if err != nil || updated == nil {
		return s.refetchPayment(ctx, payment.ID, err)
	}
	s.applyCancelSideEffects(ctx, updated)
	if message == "" {
		message = "Payment was cancelled"
	}
	s.notify(updated, message)
	return updated, nil
}

func (s *PaymentService) ReconcilePendingPayments(ctx context.Context) error {
	payments, err := s.paymentRepo.FindPendingPayments(ctx)
	if err != nil {
		return err
	}
	for _, payment := range payments {
		if _, err := s.EnsurePaymentCurrent(ctx, payment); err != nil {
			s.logger.Warn("payment reconciliation failed",
				zap.String("payment_id", payment.ID),
				zap.String("app_trans_id", payment.AppTransID),
				zap.Error(err))
		}
	}
	if s.availRepo != nil {
		if err := s.availRepo.MarkExpiredPendingBookings(ctx, PaymentValiditySeconds); err != nil {
			s.logger.Error("mark expired pending bookings", zap.Error(err))
		}
	}
	return nil
}

func (s *PaymentService) queryAndSettleSuccess(ctx context.Context, payment *domain.Payment) (*domain.Payment, bool, error) {
	if s.zalo == nil || payment.AppTransID == "" {
		return nil, false, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, paymentQueryTimeout)
	defer cancel()
	resp, err := s.zalo.QueryOrder(queryCtx, payment.AppTransID)
	if err != nil || resp == nil {
		return nil, false, err
	}
	if resp.ReturnCode != 1 {
		return nil, false, nil
	}

	successAt := s.querySuccessTime(resp)
	if successAt.After(s.ExpireAt(payment)) {
		return nil, false, nil
	}

	zpTransID := fmt.Sprintf("%d", resp.ZPTransID)
	rawData, _ := json.Marshal(resp)
	updated, err := s.paymentRepo.UpdatePendingStatusByAppTransID(
		ctx,
		payment.AppTransID,
		domain.PaymentStatusSuccess,
		&zpTransID,
		json.RawMessage(rawData),
	)
	if err != nil || updated == nil {
		refetched, refetchErr := s.refetchPayment(ctx, payment.ID, err)
		return refetched, refetched != nil && refetched.Status == domain.PaymentStatusSuccess, refetchErr
	}
	s.applySuccessSideEffects(ctx, updated)
	s.notify(updated, "Payment successful")
	return updated, true, nil
}

func (s *PaymentService) expirePayment(ctx context.Context, payment *domain.Payment) (*domain.Payment, error) {
	updated, err := s.paymentRepo.UpdatePendingStatus(ctx, payment.ID, domain.PaymentStatusExpired, nil, nil)
	if err != nil || updated == nil {
		return s.refetchPayment(ctx, payment.ID, err)
	}
	s.applyExpireSideEffects(ctx, updated)
	s.notify(updated, "Payment expired. Please try again.")
	return updated, nil
}

func (s *PaymentService) refetchPayment(ctx context.Context, paymentID string, err error) (*domain.Payment, error) {
	refetched, refetchErr := s.paymentRepo.FindByID(ctx, paymentID)
	if err != nil {
		return refetched, err
	}
	return refetched, refetchErr
}

func (s *PaymentService) applySuccessSideEffects(ctx context.Context, payment *domain.Payment) {
	s.confirmBookingPayment(ctx, payment)
	s.acceptMatchPayment(ctx, payment)
}

func (s *PaymentService) applyExpireSideEffects(ctx context.Context, payment *domain.Payment) {
	s.cancelBookingPayment(ctx, payment)
	if payment.MatchPlayerID != nil && s.matchRepo != nil {
		if err := s.matchRepo.MarkExpiredMatchPlayers(ctx, []string{*payment.MatchPlayerID}); err != nil {
			s.logger.Warn("expire match payment: mark player expired failed",
				zap.String("payment_id", payment.ID),
				zap.String("match_player_id", *payment.MatchPlayerID),
				zap.Error(err))
		}
	}
}

func (s *PaymentService) applyCancelSideEffects(ctx context.Context, payment *domain.Payment) {
	s.cancelBookingPayment(ctx, payment)
}

func (s *PaymentService) confirmBookingPayment(ctx context.Context, payment *domain.Payment) {
	if payment.BookingID == nil || s.availRepo == nil {
		return
	}
	booking, err := s.availRepo.GetBookingByID(ctx, *payment.BookingID)
	if err != nil || booking == nil {
		if err != nil {
			s.logger.Warn("confirm booking payment: booking lookup failed",
				zap.String("payment_id", payment.ID),
				zap.String("booking_id", *payment.BookingID),
				zap.Error(err))
		}
		return
	}

	_ = s.availRepo.UpdateBookingStatus(ctx, booking.ID, "confirmed")
	s.releaseBookingLock(ctx, booking.ID, booking.SubCourtID, booking.Date, booking.StartTime, booking.EndTime)

	if booking.GroupID == nil {
		return
	}
	groupBookings, err := s.availRepo.GetBookingsByGroupID(ctx, *booking.GroupID)
	if err != nil {
		s.logger.Warn("confirm booking payment: group bookings lookup failed",
			zap.String("payment_id", payment.ID),
			zap.String("group_id", *booking.GroupID),
			zap.Error(err))
		return
	}
	for _, gb := range groupBookings {
		if gb.ID == booking.ID {
			continue
		}
		_ = s.availRepo.UpdateBookingStatus(ctx, gb.ID, "confirmed")
		s.releaseBookingLock(ctx, gb.ID, gb.SubCourtID, gb.Date, gb.StartTime, gb.EndTime)
	}
}

func (s *PaymentService) cancelBookingPayment(ctx context.Context, payment *domain.Payment) {
	if payment.BookingID == nil || s.availRepo == nil {
		return
	}
	booking, err := s.availRepo.GetBookingByID(ctx, *payment.BookingID)
	if err != nil || booking == nil {
		if err != nil {
			s.logger.Warn("cancel booking payment: booking lookup failed",
				zap.String("payment_id", payment.ID),
				zap.String("booking_id", *payment.BookingID),
				zap.Error(err))
		}
		return
	}

	_ = s.availRepo.UpdateBookingStatus(ctx, booking.ID, "cancelled")
	s.releaseBookingLock(ctx, booking.ID, booking.SubCourtID, booking.Date, booking.StartTime, booking.EndTime)

	if booking.GroupID == nil {
		return
	}
	groupBookings, err := s.availRepo.GetBookingsByGroupID(ctx, *booking.GroupID)
	if err != nil {
		s.logger.Warn("cancel booking payment: group bookings lookup failed",
			zap.String("payment_id", payment.ID),
			zap.String("group_id", *booking.GroupID),
			zap.Error(err))
		return
	}
	for _, gb := range groupBookings {
		if gb.ID == booking.ID {
			continue
		}
		_ = s.availRepo.UpdateBookingStatus(ctx, gb.ID, "cancelled")
		s.releaseBookingLock(ctx, gb.ID, gb.SubCourtID, gb.Date, gb.StartTime, gb.EndTime)
	}
}

func (s *PaymentService) acceptMatchPayment(ctx context.Context, payment *domain.Payment) {
	if payment.MatchPlayerID == nil || s.matchRepo == nil {
		return
	}
	player, err := s.matchRepo.FindPlayerByID(ctx, *payment.MatchPlayerID)
	if err != nil || player == nil {
		if err != nil {
			s.logger.Warn("accept match payment: player lookup failed",
				zap.String("payment_id", payment.ID),
				zap.String("match_player_id", *payment.MatchPlayerID),
				zap.Error(err))
		}
		return
	}
	pos, err := s.matchRepo.GetNextPosition(ctx, player.MatchID)
	if err != nil {
		s.logger.Warn("accept match payment: next position failed",
			zap.String("payment_id", payment.ID),
			zap.String("match_player_id", player.ID),
			zap.Error(err))
		return
	}
	if _, err := s.matchRepo.UpdatePlayerStatus(ctx, player.ID, domain.MatchPlayerStatusAccepted, &pos); err != nil {
		s.logger.Warn("accept match payment: update player failed",
			zap.String("payment_id", payment.ID),
			zap.String("match_player_id", player.ID),
			zap.Error(err))
		return
	}
	match, err := s.matchRepo.FindByID(ctx, player.MatchID)
	if err != nil || match == nil {
		return
	}
	acceptedCount, err := s.matchRepo.CountAcceptedPlayers(ctx, player.MatchID)
	if err != nil {
		return
	}
	if acceptedCount >= match.SlotsNeeded {
		_ = s.matchRepo.UpdateStatus(ctx, player.MatchID, domain.MatchStatusFull)
	}
}

func (s *PaymentService) releaseBookingLock(ctx context.Context, bookingID, subCourtID, date, startTime, endTime string) {
	if s.redis == nil {
		return
	}
	s.redis.ReleaseSlotLocks(ctx, []SlotLockSpec{{
		SubCourtID: subCourtID,
		Date:       date,
		StartTime:  startTime,
		EndTime:    endTime,
		BookingID:  bookingID,
	}})
}

func (s *PaymentService) notify(payment *domain.Payment, message string) {
	if payment == nil || s.hub == nil {
		return
	}
	s.hub.NotifyPaymentStatus(s.Notification(payment, message))
}

func (s *PaymentService) Notification(payment *domain.Payment, message string) ws.PaymentNotification {
	if message == "" {
		message = PaymentStatusMessage(payment.Status)
	}
	return ws.PaymentNotification{
		Type:          "payment_status",
		PaymentID:     payment.ID,
		Status:        string(payment.Status),
		BookingID:     payment.BookingID,
		MatchPlayerID: payment.MatchPlayerID,
		ZPTransID:     payment.ZPTransID,
		Message:       message,
	}
}

func PaymentStatusMessage(status domain.PaymentStatus) string {
	switch status {
	case domain.PaymentStatusSuccess:
		return "Payment successful"
	case domain.PaymentStatusFailed:
		return "Payment failed"
	case domain.PaymentStatusExpired:
		return "Payment expired. Please try again."
	default:
		return "Payment is pending"
	}
}

func (s *PaymentService) querySuccessTime(resp *zalopay.QueryOrderResponse) time.Time {
	if resp != nil && resp.ServerTime > 0 {
		return time.UnixMilli(resp.ServerTime)
	}
	return s.now()
}

func (s *PaymentService) callbackSuccessTime(cb *zalopay.CallbackData) time.Time {
	if cb != nil && cb.ServerTime > 0 {
		return time.UnixMilli(cb.ServerTime)
	}
	return s.now()
}
