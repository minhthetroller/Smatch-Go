package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/repository"
	zalopay "github.com/smatch/badminton-backend/platform/zalopay"
	"go.uber.org/zap"
)

// Orchestration-capable fakes (richer than the settlement-only ones in payment_service_test.go).

type paymentOrchestrationStore struct {
	latest        *domain.Payment
	findByID      *domain.Payment
	createErr     error
	updateOrderID string
	updateStatus  domain.PaymentStatus
}

func (f *paymentOrchestrationStore) FindByID(context.Context, string) (*domain.Payment, error) {
	return f.findByID, nil
}
func (f *paymentOrchestrationStore) FindByAppTransID(context.Context, string) (*domain.Payment, error) {
	return nil, nil
}
func (f *paymentOrchestrationStore) FindLatestPendingByBookingID(context.Context, string) (*domain.Payment, error) {
	return f.latest, nil
}
func (f *paymentOrchestrationStore) Create(_ context.Context, bookingID *string, _ *string, paymentType domain.PaymentType, appTransID string, amount int) (*domain.Payment, error) {
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
func (f *paymentOrchestrationStore) UpdateOrderURL(_ context.Context, id, _, _ string) error {
	f.updateOrderID = id
	return nil
}
func (f *paymentOrchestrationStore) UpdatePendingStatus(_ context.Context, id string, status domain.PaymentStatus, _ *string, _ json.RawMessage) (*domain.Payment, error) {
	f.updateStatus = status
	if f.findByID != nil && f.findByID.ID == id {
		f.findByID.Status = status
		return f.findByID, nil
	}
	return nil, nil
}
func (f *paymentOrchestrationStore) UpdatePendingStatusByAppTransID(_ context.Context, appTransID string, status domain.PaymentStatus, zpTransID *string, _ json.RawMessage) (*domain.Payment, error) {
	f.updateStatus = status
	if f.findByID != nil && f.findByID.AppTransID == appTransID {
		f.findByID.Status = status
		f.findByID.ZPTransID = zpTransID
		return f.findByID, nil
	}
	if f.latest != nil && f.latest.AppTransID == appTransID {
		f.latest.Status = status
		f.latest.ZPTransID = zpTransID
		return f.latest, nil
	}
	return nil, nil
}
func (f *paymentOrchestrationStore) FindPendingPayments(context.Context) ([]*domain.Payment, error) {
	return nil, nil
}

type paymentOrchestrationAvailability struct {
	booking *repository.BookingRow
	group   []*repository.RawBooking
}

func (f paymentOrchestrationAvailability) GetBookingByID(context.Context, string) (*repository.BookingRow, error) {
	return f.booking, nil
}
func (f paymentOrchestrationAvailability) GetBookingsByGroupID(context.Context, string) ([]*repository.RawBooking, error) {
	return f.group, nil
}
func (f paymentOrchestrationAvailability) UpdateBookingStatus(context.Context, string, string) error {
	return nil
}
func (f paymentOrchestrationAvailability) MarkExpiredPendingBookings(context.Context, int) error {
	return nil
}

type paymentOrchestrationMatch struct{}

func (paymentOrchestrationMatch) FindByID(context.Context, string) (*repository.MatchRow, error) {
	return nil, nil
}
func (paymentOrchestrationMatch) FindPlayerByID(context.Context, string) (*repository.MatchPlayerRow, error) {
	return nil, nil
}
func (paymentOrchestrationMatch) GetNextPosition(context.Context, string) (int, error) { return 0, nil }
func (paymentOrchestrationMatch) UpdatePlayerStatus(context.Context, string, domain.MatchPlayerStatus, *int) (*repository.MatchPlayerRow, error) {
	return nil, nil
}
func (paymentOrchestrationMatch) CountAcceptedPlayers(context.Context, string) (int, error) {
	return 0, nil
}
func (paymentOrchestrationMatch) UpdateStatus(context.Context, string, domain.MatchStatus) error {
	return nil
}
func (paymentOrchestrationMatch) MarkExpiredMatchPlayers(context.Context, []string) error { return nil }

type paymentOrchestrationRedis struct {
	ticket string
	err    error
}

func (f paymentOrchestrationRedis) AcquireSlotLocks(context.Context, []SlotLockSpec) (bool, error) {
	return true, nil
}
func (paymentOrchestrationRedis) ReleaseSlotLocks(context.Context, []SlotLockSpec) {}
func (f paymentOrchestrationRedis) CreatePaymentWSTicket(context.Context, string, time.Duration) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.ticket, nil
}

type paymentOrchestrationGateway struct {
	createErr error
}

func (paymentOrchestrationGateway) GenerateAppTransID(string) string { return "260520_test001" }
func (g paymentOrchestrationGateway) CreateOrder(context.Context, zalopay.CreateOrderInput) (*zalopay.CreateOrderResponse, error) {
	if g.createErr != nil {
		return nil, g.createErr
	}
	return &zalopay.CreateOrderResponse{
		ReturnCode:   1,
		OrderURL:     "https://example.test/pay",
		ZPTransToken: "zp-token",
	}, nil
}
func (paymentOrchestrationGateway) QueryOrder(context.Context, string) (*zalopay.QueryOrderResponse, error) {
	return &zalopay.QueryOrderResponse{ReturnCode: 2, IsProcessing: true}, nil
}
func (paymentOrchestrationGateway) VerifyCallback(string, string) (*zalopay.CallbackData, bool) {
	return nil, false
}

func newPaymentOrchestrationSvc(store *paymentOrchestrationStore, avail paymentOrchestrationAvailability, gateway paymentOrchestrationGateway, redis paymentOrchestrationRedis) *PaymentService {
	return &PaymentService{
		paymentRepo: store,
		availRepo:   avail,
		matchRepo:   paymentOrchestrationMatch{},
		zalo:        gateway,
		redis:       redis,
		now:         time.Now,
		logger:      zap.NewNop(),
	}
}

func TestPaymentService_CreatePayment_ReturnsWebSocketTicketURL(t *testing.T) {
	bookingID := "12345678-1234-1234-1234-123456789abc"
	orderURL := "https://example.test/pay"
	zpToken := "zp-token"
	now := time.Now()
	store := &paymentOrchestrationStore{
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
	avail := paymentOrchestrationAvailability{booking: &repository.BookingRow{
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
	}}
	svc := newPaymentOrchestrationSvc(store, avail, paymentOrchestrationGateway{}, paymentOrchestrationRedis{ticket: "ticket-1"})

	resp, err := svc.CreatePayment(context.Background(), bookingID, WSURLParams{Host: "example.com", Port: 3000})
	if err != nil {
		t.Fatalf("CreatePayment error: %v", err)
	}
	if store.updateOrderID != "payment-1" {
		t.Fatalf("UpdateOrderURL id = %q, want payment-1", store.updateOrderID)
	}
	if resp.WsSubscribeURL != "ws://example.com/ws/payments?paymentId=payment-1&ticket=ticket-1" {
		t.Fatalf("wsSubscribeUrl = %q", resp.WsSubscribeURL)
	}
	wantExpireAt := now.Add(PaymentValidity).Format("2006-01-02T15:04:05.000Z")
	if resp.ExpireAt != wantExpireAt {
		t.Fatalf("expireAt = %q, want %q", resp.ExpireAt, wantExpireAt)
	}
}

func TestPaymentService_CreatePayment_DoesNotReturnExpiredExistingPendingPayment(t *testing.T) {
	bookingID := "12345678-1234-1234-1234-123456789abc"
	orderURL := "https://example.test/pay"
	store := &paymentOrchestrationStore{
		latest: &domain.Payment{
			ID:          "payment-1",
			BookingID:   &bookingID,
			PaymentType: domain.PaymentTypeBooking,
			AppTransID:  "260520_test001",
			Amount:      120000,
			Status:      domain.PaymentStatusPending,
			OrderURL:    &orderURL,
			CreatedAt:   time.Now().Add(-PaymentValidity - time.Second),
			UpdatedAt:   time.Now().Add(-PaymentValidity - time.Second),
		},
	}
	avail := paymentOrchestrationAvailability{booking: &repository.BookingRow{
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
	}}
	svc := newPaymentOrchestrationSvc(store, avail, paymentOrchestrationGateway{}, paymentOrchestrationRedis{ticket: "ticket-1"})

	_, err := svc.CreatePayment(context.Background(), bookingID, WSURLParams{Host: "example.com", Port: 3000})
	if err == nil {
		t.Fatal("expected PAYMENT_NOT_PENDING error")
	}
	appErr := AppErrorFromPaymentErr(err)
	if appErr.Status != 409 {
		t.Fatalf("status = %d, want %d", appErr.Status, 409)
	}
	if store.updateStatus != domain.PaymentStatusExpired {
		t.Fatalf("updateStatus = %q, want expired", store.updateStatus)
	}
}

func TestPaymentService_CreatePayment_GatewayFailureDoesNotPanic(t *testing.T) {
	bookingID := "12345678-1234-1234-1234-123456789abc"
	store := &paymentOrchestrationStore{}
	avail := paymentOrchestrationAvailability{booking: &repository.BookingRow{
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
	}}
	svc := newPaymentOrchestrationSvc(store, avail, paymentOrchestrationGateway{createErr: errors.New("gateway down")}, paymentOrchestrationRedis{ticket: "ticket-1"})

	_, err := svc.CreatePayment(context.Background(), bookingID, WSURLParams{Host: "example.com", Port: 3000})
	if err == nil {
		t.Fatal("expected gateway error")
	}
	appErr := AppErrorFromPaymentErr(err)
	if appErr.Status != 502 {
		t.Fatalf("status = %d, want %d (502 Bad Gateway)", appErr.Status, 502)
	}
}
