package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/repository"
	"github.com/smatch/badminton-backend/internal/service"
	zalopay "github.com/smatch/badminton-backend/platform/zalopay"
)

type fakePaymentStore struct {
	latest        *domain.Payment
	findByID      *domain.Payment
	createErr     error
	updateOrderID string
	updateStatus  domain.PaymentStatus
}

func (f *fakePaymentStore) FindByID(context.Context, string) (*domain.Payment, error) {
	return f.findByID, nil
}

func (f *fakePaymentStore) FindByAppTransID(context.Context, string) (*domain.Payment, error) {
	return nil, nil
}

func (f *fakePaymentStore) FindLatestPendingByBookingID(context.Context, string) (*domain.Payment, error) {
	return f.latest, nil
}

func (f *fakePaymentStore) Create(_ context.Context, bookingID *string, _ *string, paymentType domain.PaymentType, appTransID string, amount int) (*domain.Payment, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &domain.Payment{
		ID:          "payment-1",
		BookingID:   bookingID,
		PaymentType: paymentType,
		AppTransID:  appTransID,
		Amount:      amount,
		Status:      domain.PaymentStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (f *fakePaymentStore) UpdateOrderURL(_ context.Context, id, _ string, _ string) error {
	f.updateOrderID = id
	return nil
}

func (f *fakePaymentStore) UpdateStatus(_ context.Context, _ string, status domain.PaymentStatus, _ *string, _ json.RawMessage) error {
	f.updateStatus = status
	return nil
}

func (f *fakePaymentStore) UpdateStatusByAppTransID(context.Context, string, domain.PaymentStatus, *string, json.RawMessage) (*domain.Payment, error) {
	return nil, nil
}

type fakeAvailabilityStore struct {
	booking *repository.BookingRow
	group   []*repository.RawBooking
}

type fakePaymentRedis struct {
	ticket string
	err    error
}

func (f fakePaymentRedis) AcquireSlotLocks(context.Context, []service.SlotLockSpec) (bool, error) {
	return true, nil
}

func (fakePaymentRedis) ReleaseSlotLocks(context.Context, []service.SlotLockSpec) {}

func (f fakePaymentRedis) CreatePaymentWSTicket(context.Context, string, time.Duration) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.ticket, nil
}

func (f fakeAvailabilityStore) GetBookingByID(context.Context, string) (*repository.BookingRow, error) {
	return f.booking, nil
}

func (f fakeAvailabilityStore) GetBookingsByGroupID(context.Context, string) ([]*repository.RawBooking, error) {
	return f.group, nil
}

func (f fakeAvailabilityStore) UpdateBookingStatus(context.Context, string, string) error {
	return nil
}

type fakeMatchPaymentStore struct{}

func (fakeMatchPaymentStore) FindByID(context.Context, string) (*repository.MatchRow, error) {
	return nil, nil
}

func (fakeMatchPaymentStore) FindPlayerByID(context.Context, string) (*repository.MatchPlayerRow, error) {
	return nil, nil
}

func (fakeMatchPaymentStore) GetNextPosition(context.Context, string) (int, error) {
	return 0, nil
}

func (fakeMatchPaymentStore) UpdatePlayerStatus(context.Context, string, domain.MatchPlayerStatus, *int) (*repository.MatchPlayerRow, error) {
	return nil, nil
}

func (fakeMatchPaymentStore) CountAcceptedPlayers(context.Context, string) (int, error) {
	return 0, nil
}

func (fakeMatchPaymentStore) UpdateStatus(context.Context, string, domain.MatchStatus) error {
	return nil
}

type fakePaymentGateway struct {
	createErr error
}

func (fakePaymentGateway) GenerateAppTransID(string) string {
	return "260520_test001"
}

func (f fakePaymentGateway) CreateOrder(string, string, string, string, string, int, zalopay.EmbedData) (*zalopay.CreateOrderResponse, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &zalopay.CreateOrderResponse{
		ReturnCode:   1,
		OrderURL:     "https://example.test/pay",
		ZPTransToken: "zp-token",
	}, nil
}

func (fakePaymentGateway) VerifyCallback(string, string) (*zalopay.CallbackData, bool) {
	return nil, false
}

func TestCreatePayment_ReturnsWebSocketTicketURL(t *testing.T) {
	bookingID := "12345678-1234-1234-1234-123456789abc"
	orderURL := "https://example.test/pay"
	zpToken := "zp-token"
	now := time.Now()
	paymentStore := &fakePaymentStore{
		findByID: &domain.Payment{
			ID:           "payment-1",
			BookingID:    &bookingID,
			PaymentType:  domain.PaymentTypeBooking,
			AppTransID:   "260520_test001",
			ZPTransToken: &zpToken,
			Amount:       120000,
			Status:       domain.PaymentStatusPending,
			OrderURL:     &orderURL,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
	h := &PaymentHandler{
		paymentRepo: paymentStore,
		availRepo: fakeAvailabilityStore{booking: &repository.BookingRow{
			ID:         bookingID,
			SubCourtID: "subcourt-1",
			CourtName:  "Court A",
			GuestName:  "Guest",
			GuestPhone: "0900000000",
			Date:       "2026-05-20",
			StartTime:  "09:00",
			EndTime:    "10:00",
			TotalPrice: 120000,
			Status:     "pending",
		}},
		matchRepo:          fakeMatchPaymentStore{},
		redis:              fakePaymentRedis{ticket: "ticket-1"},
		zalopay:            fakePaymentGateway{},
		slotLockTTL:        900,
		paymentWSTicketTTL: 60,
		port:               3000,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/payments/create", bytes.NewBufferString(`{"bookingId":"`+bookingID+`"}`))

	h.CreatePayment(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if paymentStore.updateOrderID != "payment-1" {
		t.Fatalf("UpdateOrderURL id = %q, want payment-1", paymentStore.updateOrderID)
	}
	var env struct {
		Success bool `json:"success"`
		Data    struct {
			WsSubscribeURL string `json:"wsSubscribeUrl"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("response json: %v", err)
	}
	if env.Data.WsSubscribeURL != "ws://example.com/ws/payments?paymentId=payment-1&ticket=ticket-1" {
		t.Fatalf("wsSubscribeUrl = %q", env.Data.WsSubscribeURL)
	}
}

func TestCreatePayment_GatewayFailureDoesNotPanic(t *testing.T) {
	bookingID := "12345678-1234-1234-1234-123456789abc"
	h := &PaymentHandler{
		paymentRepo: &fakePaymentStore{},
		availRepo: fakeAvailabilityStore{booking: &repository.BookingRow{
			ID:         bookingID,
			SubCourtID: "subcourt-1",
			CourtName:  "Court A",
			GuestName:  "Guest",
			GuestPhone: "0900000000",
			Date:       "2026-05-20",
			StartTime:  "09:00",
			EndTime:    "10:00",
			TotalPrice: 120000,
			Status:     "pending",
		}},
		matchRepo:          fakeMatchPaymentStore{},
		redis:              fakePaymentRedis{ticket: "ticket-1"},
		zalopay:            fakePaymentGateway{createErr: errors.New("gateway down")},
		slotLockTTL:        900,
		paymentWSTicketTTL: 60,
		port:               3000,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/payments/create", bytes.NewBufferString(`{"bookingId":"`+bookingID+`"}`))

	h.CreatePayment(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
}
