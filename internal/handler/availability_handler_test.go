package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smatch/badminton-backend/internal/service"
)

func TestCreateBooking_InvalidSubCourtIDReturnsBadRequest(t *testing.T) {
	h := NewAvailabilityHandler(service.NewAvailabilityService(nil, nil))
	req := httptest.NewRequest(http.MethodPost, "/api/bookings", strings.NewReader(`{
		"subCourtId":"subcourt-1",
		"date":"2026-05-20",
		"startTime":"09:00",
		"endTime":"10:00",
		"guestName":"Guest",
		"guestPhone":"0900000000"
	}`))
	rec := httptest.NewRecorder()

	h.CreateBooking(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var resp errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error.Code != "BAD_REQUEST" {
		t.Fatalf("error code = %q, want BAD_REQUEST", resp.Error.Code)
	}
	if resp.Error.Message != "Invalid subCourtId: must be a UUID" {
		t.Fatalf("message = %q, want invalid subCourtId message", resp.Error.Message)
	}
}
