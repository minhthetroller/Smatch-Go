package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestLoggerSuppressesFastSuccessAndDoesNotReadBody(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	now := fixedClock(time.Unix(100, 0), time.Unix(100, 200*int64(time.Millisecond)))

	var decoded map[string]string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&decoded); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodPost, "/api/payments/create?debug=true", strings.NewReader(`{"bookingId":"b-1"}`))
	rec := httptest.NewRecorder()

	requestLogger(logger, now)(next).ServeHTTP(rec, req)

	if decoded["bookingId"] != "b-1" {
		t.Fatalf("handler body bookingId = %q, want b-1", decoded["bookingId"])
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if logs.Len() != 0 {
		t.Fatalf("logs len = %d, want 0", logs.Len())
	}
}

func TestRequestLoggerWarnsForClientErrors(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	now := fixedClock(time.Unix(100, 0), time.Unix(100, 100*int64(time.Millisecond)))

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad", http.StatusBadRequest)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/courts", nil)
	req.RemoteAddr = "203.0.113.10:45678"

	requestLogger(logger, now)(next).ServeHTTP(httptest.NewRecorder(), req)

	if logs.Len() != 1 {
		t.Fatalf("logs len = %d, want 1", logs.Len())
	}
	entry := logs.All()[0]
	if entry.Level != zapcore.WarnLevel {
		t.Fatalf("level = %s, want warn", entry.Level)
	}
	fields := entry.ContextMap()
	if got := fields["client_ip"]; got != "203.0.113.10" {
		t.Fatalf("client_ip = %v, want 203.0.113.10", got)
	}
	if got := fields["endpoint"]; got != "/api/courts" {
		t.Fatalf("endpoint = %v, want /api/courts", got)
	}
	if got := fields["method"]; got != http.MethodGet {
		t.Fatalf("method = %v, want GET", got)
	}
	if got := fields["status"]; got != int64(http.StatusBadRequest) {
		t.Fatalf("status = %v, want %d", got, http.StatusBadRequest)
	}
	if _, ok := fields["payload_summary"]; ok {
		t.Fatal("payload_summary was logged, want omitted")
	}
}

func TestRequestLoggerErrorsForServerErrors(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	now := fixedClock(time.Unix(100, 0), time.Unix(100, 100*int64(time.Millisecond)))

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad", http.StatusInternalServerError)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/courts", nil)

	requestLogger(logger, now)(next).ServeHTTP(httptest.NewRecorder(), req)

	if logs.Len() != 1 {
		t.Fatalf("logs len = %d, want 1", logs.Len())
	}
	if level := logs.All()[0].Level; level != zapcore.ErrorLevel {
		t.Fatalf("level = %s, want error", level)
	}
}

func TestRequestLoggerUsesDurationThresholds(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     zapcore.Level
	}{
		{name: "warn threshold", duration: time.Second, want: zapcore.WarnLevel},
		{name: "error threshold", duration: 5 * time.Second, want: zapcore.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)
			logger := zap.New(core)
			start := time.Unix(100, 0)
			now := fixedClock(start, start.Add(tt.duration))

			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodGet, "/health", nil)

			requestLogger(logger, now)(next).ServeHTTP(httptest.NewRecorder(), req)

			if logs.Len() != 1 {
				t.Fatalf("logs len = %d, want 1", logs.Len())
			}
			if level := logs.All()[0].Level; level != tt.want {
				t.Fatalf("level = %s, want %s", level, tt.want)
			}
		})
	}
}

func fixedClock(times ...time.Time) func() time.Time {
	i := 0
	return func() time.Time {
		if i >= len(times) {
			return times[len(times)-1]
		}
		t := times[i]
		i++
		return t
	}
}
