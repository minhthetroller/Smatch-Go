package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/repository"
	zalopay "github.com/smatch/badminton-backend/platform/zalopay"
)

type fakePaymentServiceRepo struct {
	payment *domain.Payment
}

func (f *fakePaymentServiceRepo) FindByID(context.Context, string) (*domain.Payment, error) {
	return f.payment, nil
}

func (f *fakePaymentServiceRepo) FindByAppTransID(context.Context, string) (*domain.Payment, error) {
	return f.payment, nil
}

func (f *fakePaymentServiceRepo) FindPendingPayments(context.Context) ([]*domain.Payment, error) {
	if f.payment != nil && f.payment.Status == domain.PaymentStatusPending {
		return []*domain.Payment{f.payment}, nil
	}
	return nil, nil
}

func (f *fakePaymentServiceRepo) UpdatePendingStatus(_ context.Context, _ string, status domain.PaymentStatus, zpTransID *string, callbackData json.RawMessage) (*domain.Payment, error) {
	if f.payment == nil || f.payment.Status != domain.PaymentStatusPending {
		return nil, nil
	}
	f.payment.Status = status
	f.payment.ZPTransID = zpTransID
	f.payment.CallbackData = callbackData
	return f.payment, nil
}

func (f *fakePaymentServiceRepo) FindLatestPendingByBookingID(context.Context, string) (*domain.Payment, error) {
	return nil, nil
}

func (f *fakePaymentServiceRepo) Create(context.Context, *string, *string, domain.PaymentType, string, int) (*domain.Payment, error) {
	return nil, nil
}

func (f *fakePaymentServiceRepo) UpdateOrderURL(context.Context, string, string, string) error {
	return nil
}

func (f *fakePaymentServiceRepo) UpdatePendingStatusByAppTransID(_ context.Context, _ string, status domain.PaymentStatus, zpTransID *string, callbackData json.RawMessage) (*domain.Payment, error) {
	return f.UpdatePendingStatus(context.Background(), "", status, zpTransID, callbackData)
}

type fakePaymentServiceAvailability struct {
	booking       *repository.BookingRow
	bookingStatus string
}

func (f *fakePaymentServiceAvailability) GetBookingByID(context.Context, string) (*repository.BookingRow, error) {
	return f.booking, nil
}

func (f *fakePaymentServiceAvailability) GetBookingsByGroupID(context.Context, string) ([]*repository.RawBooking, error) {
	return nil, nil
}

func (f *fakePaymentServiceAvailability) UpdateBookingStatus(_ context.Context, _ string, status string) error {
	f.bookingStatus = status
	return nil
}

func (f *fakePaymentServiceAvailability) MarkExpiredPendingBookings(context.Context, int) error {
	return nil
}

type fakePaymentServiceMatch struct {
	player       *repository.MatchPlayerRow
	match        *repository.MatchRow
	playerStatus domain.MatchPlayerStatus
	matchStatus  domain.MatchStatus
}

func (f *fakePaymentServiceMatch) FindByID(context.Context, string) (*repository.MatchRow, error) {
	return f.match, nil
}

func (f *fakePaymentServiceMatch) FindPlayerByID(context.Context, string) (*repository.MatchPlayerRow, error) {
	return f.player, nil
}

func (f *fakePaymentServiceMatch) GetNextPosition(context.Context, string) (int, error) {
	return 2, nil
}

func (f *fakePaymentServiceMatch) UpdatePlayerStatus(_ context.Context, _ string, status domain.MatchPlayerStatus, position *int) (*repository.MatchPlayerRow, error) {
	f.playerStatus = status
	if f.player != nil {
		f.player.Status = status
		f.player.Position = position
	}
	return f.player, nil
}

func (f *fakePaymentServiceMatch) CountAcceptedPlayers(context.Context, string) (int, error) {
	return f.match.SlotsNeeded, nil
}

func (f *fakePaymentServiceMatch) UpdateStatus(_ context.Context, _ string, status domain.MatchStatus) error {
	f.matchStatus = status
	return nil
}

func (f *fakePaymentServiceMatch) MarkExpiredMatchPlayers(context.Context, []string) error {
	f.playerStatus = domain.MatchPlayerStatusExpired
	return nil
}

type fakePaymentServiceGateway struct {
	response *zalopay.QueryOrderResponse
	err      error
}

func (f fakePaymentServiceGateway) QueryOrder(context.Context, string) (*zalopay.QueryOrderResponse, error) {
	return f.response, f.err
}

func (fakePaymentServiceGateway) GenerateAppTransID(string) string {
	return "260520_test001"
}

func (fakePaymentServiceGateway) CreateOrder(context.Context, zalopay.CreateOrderInput) (*zalopay.CreateOrderResponse, error) {
	return &zalopay.CreateOrderResponse{ReturnCode: 1, OrderURL: "https://example.test/pay", ZPTransToken: "zp-token"}, nil
}

func (fakePaymentServiceGateway) VerifyCallback(string, string) (*zalopay.CallbackData, bool) {
	return nil, false
}

func TestPaymentServiceEnsureCurrentStatusSettlesMatchPaymentFromZaloPay(t *testing.T) {
	now := time.Now()
	matchPlayerID := "player-1"
	payment := &domain.Payment{
		ID:            "payment-1",
		MatchPlayerID: &matchPlayerID,
		PaymentType:   domain.PaymentTypeMatchJoin,
		AppTransID:    "260520_test001",
		Amount:        120000,
		Status:        domain.PaymentStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	paymentRepo := &fakePaymentServiceRepo{payment: payment}
	matchRepo := &fakePaymentServiceMatch{
		player: &repository.MatchPlayerRow{MatchPlayer: domain.MatchPlayer{
			ID:      matchPlayerID,
			MatchID: "match-1",
			Status:  domain.MatchPlayerStatusPendingPayment,
		}},
		match: &repository.MatchRow{Match: domain.Match{
			ID:          "match-1",
			SlotsNeeded: 2,
			Status:      domain.MatchStatusOpen,
		}},
	}
	svc := &PaymentService{
		paymentRepo: paymentRepo,
		matchRepo:   matchRepo,
		zalo: fakePaymentServiceGateway{
			response: &zalopay.QueryOrderResponse{
				ReturnCode: 1,
				ZPTransID:  12345,
				ServerTime: now.Add(time.Minute).UnixMilli(),
			},
		},
		now: time.Now,
	}

	updated, err := svc.EnsureCurrentStatus(context.Background(), payment.ID)
	if err != nil {
		t.Fatalf("EnsureCurrentStatus error: %v", err)
	}
	if updated.Status != domain.PaymentStatusSuccess {
		t.Fatalf("payment status = %q, want success", updated.Status)
	}
	if matchRepo.playerStatus != domain.MatchPlayerStatusAccepted {
		t.Fatalf("player status = %q, want ACCEPTED", matchRepo.playerStatus)
	}
	if matchRepo.matchStatus != domain.MatchStatusFull {
		t.Fatalf("match status = %q, want FULL", matchRepo.matchStatus)
	}
}

func TestPaymentServiceEnsureCurrentStatusExpiresAfterFiveMinutes(t *testing.T) {
	bookingID := "booking-1"
	payment := &domain.Payment{
		ID:          "payment-1",
		BookingID:   &bookingID,
		PaymentType: domain.PaymentTypeBooking,
		AppTransID:  "260520_test001",
		Amount:      120000,
		Status:      domain.PaymentStatusPending,
		CreatedAt:   time.Now().Add(-PaymentValidity - time.Second),
		UpdatedAt:   time.Now().Add(-PaymentValidity - time.Second),
	}
	paymentRepo := &fakePaymentServiceRepo{payment: payment}
	availability := &fakePaymentServiceAvailability{booking: &repository.BookingRow{
		ID:         bookingID,
		SubCourtID: "subcourt-1",
		Date:       "2026-05-20",
		StartTime:  "09:00",
		EndTime:    "10:00",
		Status:     "pending",
	}}
	svc := &PaymentService{
		paymentRepo: paymentRepo,
		availRepo:   availability,
		zalo: fakePaymentServiceGateway{
			response: &zalopay.QueryOrderResponse{ReturnCode: 2, IsProcessing: true},
		},
		now: time.Now,
	}

	updated, err := svc.EnsureCurrentStatus(context.Background(), payment.ID)
	if err != nil {
		t.Fatalf("EnsureCurrentStatus error: %v", err)
	}
	if updated.Status != domain.PaymentStatusExpired {
		t.Fatalf("payment status = %q, want expired", updated.Status)
	}
	if availability.bookingStatus != "cancelled" {
		t.Fatalf("booking status = %q, want cancelled", availability.bookingStatus)
	}
}
