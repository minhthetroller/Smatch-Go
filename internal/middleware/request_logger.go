package middleware

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

const requestLogBodySampleLimit = 64 * 1024

type bodyReadCloser struct {
	io.Reader
	io.Closer
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

// RequestLogger writes a single structured log entry for every HTTP request.
func RequestLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
			payloadSummary := summarizeAndRestoreBody(r, contentType)

			lw := &loggingResponseWriter{ResponseWriter: w}
			next.ServeHTTP(lw, r)

			status := lw.status
			if status == 0 {
				status = http.StatusOK
			}

			fields := []zap.Field{
				zap.String("client_ip", clientIP(r)),
				zap.String("endpoint", r.URL.Path),
				zap.String("method", r.Method),
				zap.String("content_type", contentType),
				zap.String("payload_summary", payloadSummary),
				zap.Int("status", status),
				zap.Int("response_bytes", lw.bytes),
				zap.Int64("duration_ms", time.Since(start).Milliseconds()),
				zap.Bool("query_present", r.URL.RawQuery != ""),
			}
			if reqID := chimiddleware.GetReqID(r.Context()); reqID != "" {
				fields = append(fields, zap.String("request_id", reqID))
			}

			switch {
			case status >= http.StatusInternalServerError:
				logger.Error("http request completed", fields...)
			case status >= http.StatusBadRequest:
				logger.Warn("http request completed", fields...)
			default:
				logger.Info("http request completed", fields...)
			}
		})
	}
}

func summarizeAndRestoreBody(r *http.Request, contentType string) string {
	if r.Body == nil || r.Body == http.NoBody {
		return ""
	}

	original := r.Body
	consumed, err := io.ReadAll(io.LimitReader(original, requestLogBodySampleLimit+1))
	if err != nil {
		r.Body = original
		return "body_read_error"
	}

	sample := consumed
	truncated := len(consumed) > requestLogBodySampleLimit
	if truncated {
		sample = consumed[:requestLogBodySampleLimit]
	}
	r.Body = bodyReadCloser{
		Reader: io.MultiReader(bytes.NewReader(consumed), original),
		Closer: original,
	}

	return payloadSummary(contentType, sample, truncated, r.ContentLength)
}

func payloadSummary(contentType string, sample []byte, truncated bool, contentLength int64) string {
	baseType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch baseType {
	case "application/json", "text/json":
		return jsonPayloadSummary(sample, truncated)
	case "application/x-www-form-urlencoded":
		return formPayloadSummary(sample, truncated)
	case "multipart/form-data":
		return lengthSummary("multipart_form", len(sample), truncated, contentLength)
	case "":
		if len(bytes.TrimSpace(sample)) == 0 {
			return ""
		}
		return lengthSummary("body", len(sample), truncated, contentLength)
	default:
		return lengthSummary(safeContentTypeLabel(baseType), len(sample), truncated, contentLength)
	}
}

func jsonPayloadSummary(sample []byte, truncated bool) string {
	if truncated {
		return "json_body_truncated bytes_sampled=" + strconv.Itoa(len(sample))
	}

	var v interface{}
	if err := json.Unmarshal(sample, &v); err != nil {
		return "json_invalid bytes=" + strconv.Itoa(len(sample))
	}

	switch typed := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return "json_object keys=" + strings.Join(limitStrings(keys, 12), ",") + " bytes=" + strconv.Itoa(len(sample))
	case []interface{}:
		return "json_array length=" + strconv.Itoa(len(typed)) + " bytes=" + strconv.Itoa(len(sample))
	default:
		return "json_" + fmt.Sprintf("%T", typed) + " bytes=" + strconv.Itoa(len(sample))
	}
}

func formPayloadSummary(sample []byte, truncated bool) string {
	if truncated {
		return "form_urlencoded_truncated bytes_sampled=" + strconv.Itoa(len(sample))
	}

	values, err := url.ParseQuery(string(sample))
	if err != nil {
		return "form_urlencoded_invalid bytes=" + strconv.Itoa(len(sample))
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return "form_urlencoded keys=" + strings.Join(limitStrings(keys, 12), ",") + " bytes=" + strconv.Itoa(len(sample))
}

func lengthSummary(label string, sampled int, truncated bool, contentLength int64) string {
	parts := []string{label, "bytes_sampled=" + strconv.Itoa(sampled)}
	if contentLength >= 0 {
		parts = append(parts, "content_length="+strconv.FormatInt(contentLength, 10))
	}
	if truncated {
		parts = append(parts, "truncated=true")
	}
	return strings.Join(parts, " ")
}

func safeContentTypeLabel(contentType string) string {
	if contentType == "" {
		return "body"
	}
	return strings.NewReplacer("/", "_", "+", "_", ".", "_").Replace(contentType)
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	limited := append([]string{}, values[:limit]...)
	limited = append(limited, "...")
	return limited
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
