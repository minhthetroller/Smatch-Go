package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoadTestStressDisabledReturnsNotFound(t *testing.T) {
	h := NewLoadTestHandler(false, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/load-test/stress", nil)
	req.Header.Set("X-Admin-Secret", "secret")
	rec := httptest.NewRecorder()

	h.Stress(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestLoadTestStressRequiresAdminSecret(t *testing.T) {
	h := NewLoadTestHandler(true, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/load-test/stress", nil)
	rec := httptest.NewRecorder()

	h.Stress(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestLoadTestStressRunsBoundedWork(t *testing.T) {
	h := NewLoadTestHandler(true, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/load-test/stress?duration_ms=1&workers=1", nil)
	req.Header.Set("X-Admin-Secret", "secret")
	rec := httptest.NewRecorder()

	h.Stress(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var env successEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !env.Success {
		t.Fatal("success = false, want true")
	}
}

func TestLoadTestStressRejectsInvalidBounds(t *testing.T) {
	h := NewLoadTestHandler(true, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/load-test/stress?duration_ms=6000", nil)
	req.Header.Set("X-Admin-Secret", "secret")
	rec := httptest.NewRecorder()

	h.Stress(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
