package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestLoggerLogsRequiredFieldsAndRestoresBody(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	var decoded map[string]string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&decoded); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodPost, "/api/payments/create?debug=true", strings.NewReader(`{"bookingId":"b-1","secret":"hidden"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:45678"
	rec := httptest.NewRecorder()

	RequestLogger(logger)(next).ServeHTTP(rec, req)

	if decoded["bookingId"] != "b-1" {
		t.Fatalf("handler body bookingId = %q, want b-1", decoded["bookingId"])
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if logs.Len() != 1 {
		t.Fatalf("logs len = %d, want 1", logs.Len())
	}

	entry := logs.All()[0]
	fields := entry.ContextMap()
	if got := fields["client_ip"]; got != "203.0.113.10" {
		t.Fatalf("client_ip = %v, want 203.0.113.10", got)
	}
	if got := fields["endpoint"]; got != "/api/payments/create" {
		t.Fatalf("endpoint = %v, want /api/payments/create", got)
	}
	if got := fields["method"]; got != http.MethodPost {
		t.Fatalf("method = %v, want POST", got)
	}
	if got := fields["content_type"]; got != "application/json" {
		t.Fatalf("content_type = %v, want application/json", got)
	}
	if got := fields["status"]; got != int64(http.StatusCreated) {
		t.Fatalf("status = %v, want %d", got, http.StatusCreated)
	}
	summary, _ := fields["payload_summary"].(string)
	if !strings.Contains(summary, "json_object") || !strings.Contains(summary, "bookingId") || !strings.Contains(summary, "secret") {
		t.Fatalf("payload_summary = %q, want json key summary", summary)
	}
	if strings.Contains(summary, "hidden") || strings.Contains(summary, "b-1") {
		t.Fatalf("payload_summary leaked raw values: %q", summary)
	}
}

func TestRequestLoggerWarnsForClientErrors(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad", http.StatusBadRequest)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/courts", nil)

	RequestLogger(logger)(next).ServeHTTP(httptest.NewRecorder(), req)

	if logs.Len() != 1 {
		t.Fatalf("logs len = %d, want 1", logs.Len())
	}
	if level := logs.All()[0].Level; level != zapcore.WarnLevel {
		t.Fatalf("level = %s, want warn", level)
	}
}
