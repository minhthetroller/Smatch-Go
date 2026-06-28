package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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
	if got := fields["event"]; got != "http_request" {
		t.Fatalf("event = %v, want http_request", got)
	}
	if got := fields["status_class"]; got != "4xx" {
		t.Fatalf("status_class = %v, want 4xx", got)
	}
	if got := fields["outcome"]; got != "client_error" {
		t.Fatalf("outcome = %v, want client_error", got)
	}
	if got := fields["timeout"]; got != false {
		t.Fatalf("timeout = %v, want false", got)
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

func TestRequestTimeoutPassesUnderBudgetResponses(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Test", "ok")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})

	req := httptest.NewRequest(http.MethodPost, "/api/courts", nil)
	rec := httptest.NewRecorder()

	RequestTimeout(TimeoutConfig{Default: time.Second}, zap.NewNop())(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Body.String(); got != "created" {
		t.Fatalf("body = %q, want created", got)
	}
	if got := rec.Header().Get("X-Test"); got != "ok" {
		t.Fatalf("X-Test = %q, want ok", got)
	}
}

func TestRequestTimeoutReturnsGatewayTimeoutAndSuppressesLateWrite(t *testing.T) {
	release := make(chan struct{})
	var sawCancelled atomic.Bool

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		sawCancelled.Store(true)
		<-release
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("late success"))
	})
	defer close(release)

	req := httptest.NewRequest(http.MethodGet, "/api/courts", nil)
	rec := httptest.NewRecorder()

	RequestTimeout(TimeoutConfig{Default: time.Millisecond}, zap.NewNop())(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
	if !strings.Contains(rec.Body.String(), `"code":"REQUEST_TIMEOUT"`) {
		t.Fatalf("body = %s, want REQUEST_TIMEOUT", rec.Body.String())
	}
	if !sawCancelled.Load() {
		t.Fatal("handler context was not cancelled")
	}
}

func TestRequestTimeoutUsesTieredBudgets(t *testing.T) {
	cfg := TimeoutConfig{
		Fast:    10 * time.Millisecond,
		Default: 20 * time.Millisecond,
		Payment: 30 * time.Millisecond,
	}

	tests := []struct {
		path string
		want int64
	}{
		{path: "/health", want: 10},
		{path: "/version", want: 10},
		{path: "/api/courts", want: 20},
		{path: "/api/payments/create", want: 30},
		{path: "/api/bookings/b-1/payment", want: 30},
		{path: "/api/matches/m-1/payment", want: 30},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				info := RequestLogInfo(r.Context())
				if info == nil {
					t.Fatal("request log info missing")
				}
				if info.latencyBudgetMS != tt.want {
					t.Fatalf("latencyBudgetMS = %d, want %d", info.latencyBudgetMS, tt.want)
				}
			})
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			RequestTimeout(cfg, zap.NewNop())(next).ServeHTTP(httptest.NewRecorder(), req)
		})
	}
}

func TestRequestTimeoutExemptsWebSockets(t *testing.T) {
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		info := RequestLogInfo(r.Context())
		if info == nil {
			t.Fatal("request log info missing")
		}
		if info.latencyBudgetMS != 0 {
			t.Fatalf("latencyBudgetMS = %d, want 0", info.latencyBudgetMS)
		}
		if _, ok := r.Context().Deadline(); ok {
			t.Fatal("websocket request should not receive timeout deadline")
		}
	})
	req := httptest.NewRequest(http.MethodGet, "/ws/payments", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	RequestTimeout(TimeoutConfig{Default: time.Millisecond}, zap.NewNop())(next).ServeHTTP(httptest.NewRecorder(), req)
}

func TestRecovererLogsPanicAndWritesJSONError(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	req := httptest.NewRequest(http.MethodGet, "/api/courts", nil)
	rec := httptest.NewRecorder()

	Recoverer(logger)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), `"code":"INTERNAL_ERROR"`) {
		t.Fatalf("body = %s, want INTERNAL_ERROR", rec.Body.String())
	}
	if logs.Len() != 1 {
		t.Fatalf("logs len = %d, want 1", logs.Len())
	}
	fields := logs.All()[0].ContextMap()
	if got := fields["event"]; got != "http_panic" {
		t.Fatalf("event = %v, want http_panic", got)
	}
	if _, ok := fields["stack"]; ok {
		t.Fatal("panic log should not include stack payload")
	}
	if got := fields["panic"]; got != "boom" {
		t.Fatalf("panic = %v, want boom", got)
	}
}

func TestRequestTimeoutDoesNotEmitDuplicateLog(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(20 * time.Millisecond)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/courts", nil)

	RequestTimeout(TimeoutConfig{Default: time.Millisecond}, logger)(next).ServeHTTP(httptest.NewRecorder(), req)

	if logs.Len() != 0 {
		t.Fatalf("logs len = %d, want 0; timeout should be logged once by RequestLogger", logs.Len())
	}
}

func TestRequestLoggerMarksTimeoutAsError(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	now := fixedClock(time.Unix(100, 0), time.Unix(100, 2*int64(time.Millisecond)))

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := RequestLogInfo(r.Context())
		info.timeout = true
		info.outcome = "timeout"
		info.latencyBudgetMS = 1
		w.WriteHeader(http.StatusGatewayTimeout)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/courts", nil)
	req = withRequestLogInfo(req.WithContext(context.Background()), &requestLogInfo{route: "/api/courts"})

	requestLogger(logger, now)(next).ServeHTTP(httptest.NewRecorder(), req)

	if logs.Len() != 1 {
		t.Fatalf("logs len = %d, want 1", logs.Len())
	}
	entry := logs.All()[0]
	if entry.Level != zapcore.ErrorLevel {
		t.Fatalf("level = %s, want error", entry.Level)
	}
	fields := entry.ContextMap()
	if got := fields["timeout"]; got != true {
		t.Fatalf("timeout = %v, want true", got)
	}
	if got := fields["latency_budget_ms"]; got != int64(1) {
		t.Fatalf("latency_budget_ms = %v, want 1", got)
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
