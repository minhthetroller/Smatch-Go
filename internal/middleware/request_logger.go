package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	requestLogWarnDuration  = time.Second
	requestLogErrorDuration = 5 * time.Second
)

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
			level := requestLogLevel(status, duration)
			if level == nil {
				return
			}

			fields := []zap.Field{
				zap.String("client_ip", clientIP(r)),
				zap.String("endpoint", r.URL.Path),
				zap.String("method", r.Method),
				zap.Int("status", status),
				zap.Int("response_bytes", lw.bytes),
				zap.Int64("duration_ms", duration.Milliseconds()),
				zap.Bool("query_present", r.URL.RawQuery != ""),
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

func requestLogLevel(status int, duration time.Duration) *zapcore.Level {
	switch {
	case status >= http.StatusInternalServerError || duration >= requestLogErrorDuration:
		level := zapcore.ErrorLevel
		return &level
	case status >= http.StatusBadRequest || duration >= requestLogWarnDuration:
		level := zapcore.WarnLevel
		return &level
	default:
		return nil
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
