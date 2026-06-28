package middleware

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	requestLogWarnDuration  = time.Second
	requestLogErrorDuration = 5 * time.Second

	contextKeyRequestLog contextKey = "request_log"
)

type requestLogInfo struct {
	route           string
	outcome         string
	timeout         bool
	latencyBudgetMS int64
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *loggingResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

// RequestLogInfo returns request metadata populated by observability middleware.
func RequestLogInfo(ctx context.Context) *requestLogInfo {
	info, _ := ctx.Value(contextKeyRequestLog).(*requestLogInfo)
	return info
}

func withRequestLogInfo(r *http.Request, info *requestLogInfo) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), contextKeyRequestLog, info))
}

// RequestLogContext seeds mutable request metadata for later observability middleware.
func RequestLogContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestLogInfo(r.Context()) == nil {
			r = withRequestLogInfo(r, &requestLogInfo{
				route:   r.URL.Path,
				outcome: "success",
			})
		}
		next.ServeHTTP(w, r)
	})
}

// RequestLogger writes structured logs only for warning/error HTTP requests.
func RequestLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	return requestLogger(logger, time.Now)
}

func requestLogger(logger *zap.Logger, now func() time.Time) func(http.Handler) http.Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := now()

			lw := &loggingResponseWriter{ResponseWriter: w}
			next.ServeHTTP(lw, r)

			status := lw.status
			if status == 0 {
				status = http.StatusOK
			}
			duration := now().Sub(start)
			info := RequestLogInfo(r.Context())
			level := requestLogLevel(status, duration, info)
			if level == nil {
				return
			}

			fields := []zap.Field{
				zap.String("event", "http_request"),
				zap.String("client_ip", clientIP(r)),
				zap.String("endpoint", r.URL.Path),
				zap.String("method", r.Method),
				zap.Int("status", status),
				zap.String("status_class", statusClass(status)),
				zap.Int("response_bytes", lw.bytes),
				zap.Int64("duration_ms", duration.Milliseconds()),
				zap.Bool("query_present", r.URL.RawQuery != ""),
				zap.String("host", r.Host),
				zap.String("proto", r.Proto),
				zap.Int64("request_content_length", r.ContentLength),
			}
			if info != nil {
				outcome := info.outcome
				if !info.timeout && outcome == "success" {
					outcome = outcomeForStatus(status)
				}
				fields = append(fields,
					zap.String("route", info.route),
					zap.String("outcome", outcome),
					zap.Bool("timeout", info.timeout),
					zap.Int64("latency_budget_ms", info.latencyBudgetMS),
				)
			} else {
				fields = append(fields,
					zap.String("route", r.URL.Path),
					zap.String("outcome", outcomeForStatus(status)),
					zap.Bool("timeout", false),
					zap.Int64("latency_budget_ms", 0),
				)
			}
			if reqID := chimiddleware.GetReqID(r.Context()); reqID != "" {
				fields = append(fields, zap.String("request_id", reqID))
			}

			switch *level {
			case zapcore.ErrorLevel:
				logger.Error("http request completed", fields...)
			case zapcore.WarnLevel:
				logger.Warn("http request completed", fields...)
			}
		})
	}
}

func requestLogLevel(status int, duration time.Duration, info *requestLogInfo) *zapcore.Level {
	switch {
	case status >= http.StatusInternalServerError || duration >= requestLogErrorDuration || (info != nil && info.timeout):
		level := zapcore.ErrorLevel
		return &level
	case status >= http.StatusBadRequest || duration >= requestLogWarnDuration:
		level := zapcore.WarnLevel
		return &level
	default:
		return nil
	}
}

func statusClass(status int) string {
	switch {
	case status >= 100 && status < 200:
		return "1xx"
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}

func outcomeForStatus(status int) string {
	switch {
	case status >= http.StatusInternalServerError:
		return "server_error"
	case status >= http.StatusBadRequest:
		return "client_error"
	default:
		return "success"
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

type TimeoutConfig struct {
	Fast    time.Duration
	Default time.Duration
	Payment time.Duration
}

func (c TimeoutConfig) withDefaults() TimeoutConfig {
	if c.Fast <= 0 {
		c.Fast = 500 * time.Millisecond
	}
	if c.Default <= 0 {
		c.Default = 5 * time.Second
	}
	if c.Payment <= 0 {
		c.Payment = 10 * time.Second
	}
	return c
}

// RequestTimeout enforces latency budgets for normal HTTP requests.
func RequestTimeout(cfg TimeoutConfig, logger *zap.Logger) func(http.Handler) http.Handler {
	cfg = cfg.withDefaults()
	_ = logger
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info := RequestLogInfo(r.Context())
			if info == nil {
				info = &requestLogInfo{route: requestRoute(r), outcome: "success"}
				r = withRequestLogInfo(r, info)
			}
			if isWebSocketRequest(r) {
				info.route = requestRoute(r)
				next.ServeHTTP(w, r)
				return
			}

			budget := latencyBudget(r, cfg)
			info.route = requestRoute(r)
			info.outcome = "success"
			info.latencyBudgetMS = budget.Milliseconds()
			ctx, cancel := context.WithTimeout(r.Context(), budget)
			defer cancel()
			r = r.WithContext(ctx)

			bw := newBufferedResponseWriter(w)
			done := make(chan struct{})
			panicCh := make(chan any, 1)

			go func() {
				defer close(done)
				defer func() {
					if recovered := recover(); recovered != nil {
						panicCh <- recovered
					}
				}()
				next.ServeHTTP(bw, r)
			}()

			select {
			case <-done:
				select {
				case recovered := <-panicCh:
					panic(recovered)
				default:
				}
				bw.commit()
				info.route = requestRoute(r)
				info.outcome = outcomeForStatus(bw.statusCode())
			case <-ctx.Done():
				info.timeout = true
				info.outcome = "timeout"
				info.route = requestRoute(r)
				bw.discard()
				writeJSONError(w, http.StatusGatewayTimeout, "Request took too long", "REQUEST_TIMEOUT")
			}
		})
	}
}

// Recoverer writes compact structured panic logs and a JSON 500 response.
func Recoverer(logger *zap.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				if recovered == http.ErrAbortHandler {
					panic(recovered)
				}
				info := RequestLogInfo(r.Context())
				if info != nil {
					info.outcome = "panic"
				}
				fields := []zap.Field{
					zap.String("event", "http_panic"),
					zap.String("method", r.Method),
					zap.String("endpoint", r.URL.Path),
					zap.String("route", requestRoute(r)),
					zap.Any("panic", recovered),
				}
				if reqID := chimiddleware.GetReqID(r.Context()); reqID != "" {
					fields = append(fields, zap.String("request_id", reqID))
				}
				logger.Error("http request panic", fields...)
				if !isWebSocketRequest(r) {
					writeJSONError(w, http.StatusInternalServerError, "Internal server error", "INTERNAL_ERROR")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func latencyBudget(r *http.Request, cfg TimeoutConfig) time.Duration {
	path := r.URL.Path
	if path == "/health" || path == "/version" {
		return cfg.Fast
	}
	if isPaymentRoute(path) {
		return cfg.Payment
	}
	return cfg.Default
}

func isPaymentRoute(path string) bool {
	return strings.HasPrefix(path, "/api/payments") ||
		strings.Contains(path, "/payment") ||
		strings.HasSuffix(path, "/payment")
}

func isWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Connection"), "Upgrade") ||
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
		strings.HasPrefix(r.URL.Path, "/ws/")
}

func requestRoute(r *http.Request) string {
	if route := chi.RouteContext(r.Context()); route != nil {
		if pattern := route.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return r.URL.Path
}

func writeJSONError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error": map[string]string{
			"message": message,
			"code":    code,
		},
	})
}

type bufferedResponseWriter struct {
	dst     http.ResponseWriter
	header  http.Header
	body    bytes.Buffer
	status  int
	bytes   int
	mu      sync.Mutex
	closed  bool
	flushed bool
}

func newBufferedResponseWriter(dst http.ResponseWriter) *bufferedResponseWriter {
	return &bufferedResponseWriter{
		dst:    dst,
		header: make(http.Header),
	}
}

func (w *bufferedResponseWriter) Header() http.Header {
	return w.header
}

func (w *bufferedResponseWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.status != 0 {
		return
	}
	w.status = status
}

func (w *bufferedResponseWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return len(b), nil
	}
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.body.Write(b)
	w.bytes += n
	return n, err
}

func (w *bufferedResponseWriter) Flush() {
	w.flushed = true
}

func (w *bufferedResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.dst.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *bufferedResponseWriter) commit() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	w.closed = true
	copyHeader(w.dst.Header(), w.header)
	status := w.statusCodeLocked()
	w.dst.WriteHeader(status)
	_, _ = w.dst.Write(w.body.Bytes())
	if w.flushed {
		if flusher, ok := w.dst.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

func (w *bufferedResponseWriter) discard() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
}

func (w *bufferedResponseWriter) statusCode() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.statusCodeLocked()
}

func (w *bufferedResponseWriter) statusCodeLocked() int {
	if w.status != 0 {
		return w.status
	}
	return http.StatusOK
}

func copyHeader(dst, src http.Header) {
	for key, values := range src {
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
