package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smatch/badminton-backend/internal/repository"
	"github.com/smatch/badminton-backend/internal/service"
)

// These tests exercise HTTP request-decoding boundaries only; the orchestration
// logic is covered by service-level tests (see service/payment_orchestration_test.go).
func newPaymentHandlerBoundary(t *testing.T) *PaymentHandler {
	t.Helper()
	svc := service.NewPaymentService(
		(*repository.PaymentRepository)(nil),
		(*repository.AvailabilityRepository)(nil),
		(*repository.MatchRepository)(nil),
		nil,
		nil,
		nil,
		nil,
	)
	return NewPaymentHandler(svc, nil, 60, 3000, "test")
}

func TestPayment_CreatePayment_InvalidBody(t *testing.T) {
	h := newPaymentHandlerBoundary(t)
	req := httptest.NewRequest(http.MethodPost, "/api/payments/create", bytes.NewBufferString("{nope"))
	rec := httptest.NewRecorder()
	h.CreatePayment(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestPayment_Callback_InvalidBody(t *testing.T) {
	h := newPaymentHandlerBoundary(t)
	req := httptest.NewRequest(http.MethodPost, "/api/payments/callback", bytes.NewBufferString("x"))
	rec := httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("callback always responds 200 (ZaloPay contract); got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"return_code":-1`)) {
		t.Errorf("expected return_code -1 for invalid body, got %s", rec.Body.String())
	}
}

func TestPayment_CreateMatchPayment_InvalidBody(t *testing.T) {
	h := newPaymentHandlerBoundary(t)
	req := httptest.NewRequest(http.MethodPost, "/api/matches/m1/payment", bytes.NewBufferString("x"))
	rec := httptest.NewRecorder()
	h.CreateMatchPayment(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
