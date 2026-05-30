package middleware

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// LoggingConfig holds configuration for the request/response logging middleware.
type LoggingConfig struct {
	Logger *slog.Logger // Custom logger; if nil, defaults to slog.Default() writing to stderr
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// RequestLog represents the structured fields logged per request.
type RequestLog struct {
	Timestamp    time.Time `json:"timestamp"`
	Level        string    `json:"level"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	RemoteAddr   string    `json:"remote_addr"`
	RequestID    string    `json:"request_id,omitempty"`
	Duration     string    `json:"duration"`
	StatusCode   int       `json:"status_code"`
	ResponseSize int64     `json:"response_size"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// Logging returns middleware that logs every request/response as structured JSON.
//
// Log levels:
//   - info:  2xx and 3xx status codes
//   - warn:  4xx status codes
//   - error: 5xx status codes
//
// Each log line contains: method, path, remote_addr, request_id, timestamp,
// duration, status_code, and response_size.
func Logging(cfg LoggingConfig) func(http.Handler) http.Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap ResponseWriter to capture status code and body size.
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Proceed with the request.
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			// Determine log level based on status code.
			var level string
			switch {
			case wrapped.statusCode >= 500:
				level = "error"
			case wrapped.statusCode >= 400:
				level = "warn"
			default:
				level = "info"
			}

			// Extract request ID from context.
			requestID := GetRequestID(r.Context())

			// Strip port from remote address.
			remoteAddr := stripPort(r.RemoteAddr)

			// Sanitize path (remove query string and fragment).
			logPath := sanitizePath(r.URL.Path)

			// Build log attributes.
			attrs := []any{
				"timestamp", start.Format(time.RFC3339),
				"level", level,
				"method", r.Method,
				"path", logPath,
				"remote_addr", remoteAddr,
				"duration", fmt.Sprintf("%.3fs", duration.Seconds()),
				"status_code", wrapped.statusCode,
				"response_size", wrapped.written,
			}
			if requestID != "" {
				attrs = append(attrs, "request_id", requestID)
			}

			// Log the request.
			switch level {
			case "error":
				logger.Error("request completed", attrs...)
			case "warn":
				logger.Warn("request completed", attrs...)
			default:
				logger.Info("request completed", attrs...)
			}
		})
	}
}

// stripPort removes the port from a host:port address, returning just the host.
func stripPort(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No port to strip — return as-is.
		return addr
	}
	return host
}

// Logger creates a file-based slog.Logger configured with JSON handler.
// Useful for testing or production log rotation.
func Logger(path string) *slog.Logger {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Default().Error("failed to open log file", "path", path, "error", err)
		return slog.Default()
	}
	return slog.New(slog.NewJSONHandler(f, nil))
}

// sanitizePath removes query string and fragment from the path for cleaner logs.
func sanitizePath(path string) string {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	if i := strings.IndexByte(path, '#'); i >= 0 {
		path = path[:i]
	}
	return path
}
