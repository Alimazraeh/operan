package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type testHandler struct {
	statusCode int
	body       []byte
	delay      time.Duration
}

func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.delay > 0 {
		time.Sleep(h.delay)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.statusCode)
	w.Write(h.body)
}

func TestLoggingJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := LoggingConfig{Logger: logger}

	handler := Logging(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tenants", nil)
	req.RemoteAddr = "192.168.1.1:54321"

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handler returned status = %v, want %v", w.Code, http.StatusOK)
	}

	// Parse the log line.
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log JSON: %v", err)
	}

	// Check required fields.
	if logEntry["level"] != "info" {
		t.Errorf("log level = %v, want info", logEntry["level"])
	}

	if logEntry["method"] != "GET" {
		t.Errorf("method = %v, want GET", logEntry["method"])
	}

	if logEntry["path"] != "/api/tenants" {
		t.Errorf("path = %v, want /api/tenants", logEntry["path"])
	}

	if logEntry["remote_addr"] != "192.168.1.1" {
		t.Errorf("remote_addr = %v, want 192.168.1.1", logEntry["remote_addr"])
	}

	statusCode, ok := logEntry["status_code"].(float64)
	if !ok {
		t.Fatal("status_code not a number in log entry")
	}
	if int(statusCode) != http.StatusOK {
		t.Errorf("status_code = %v, want %v", statusCode, http.StatusOK)
	}
}

func TestLoggingCorrectStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantLevel  string
	}{
		{"200 OK", http.StatusOK, "info"},
		{"201 Created", http.StatusCreated, "info"},
		{"301 Moved", http.StatusMovedPermanently, "info"},
		{"400 Bad Request", http.StatusBadRequest, "warn"},
		{"401 Unauthorized", http.StatusUnauthorized, "warn"},
		{"403 Forbidden", http.StatusForbidden, "warn"},
		{"404 Not Found", http.StatusNotFound, "warn"},
		{"429 Too Many", http.StatusTooManyRequests, "warn"},
		{"500 Internal", http.StatusInternalServerError, "error"},
		{"502 Bad Gateway", http.StatusBadGateway, "error"},
		{"503 Unavailable", http.StatusServiceUnavailable, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			cfg := LoggingConfig{Logger: logger}

			handler := Logging(cfg)(&testHandler{statusCode: tt.statusCode, body: []byte(`{"ok":true}`)})

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.RemoteAddr = "10.0.0.1:12345"

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			var logEntry map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
				t.Fatalf("failed to parse log: %v", err)
			}

			if logEntry["level"] != tt.wantLevel {
				t.Errorf("level = %v, want %v", logEntry["level"], tt.wantLevel)
			}

			code, ok := logEntry["status_code"].(float64)
			if !ok {
				t.Fatal("status_code not a number")
			}
			if int(code) != tt.statusCode {
				t.Errorf("status_code = %v, want %v", code, tt.statusCode)
			}
		})
	}
}

func TestLoggingIncludesRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := LoggingConfig{Logger: logger}

	handler := Logging(cfg)(&testHandler{statusCode: http.StatusOK, body: []byte(`{"ok":true}`)})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req = req.WithContext(SetRequestID(req.Context(), "req-abc-123"))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	requestID, ok := logEntry["request_id"]
	if !ok {
		t.Fatal("request_id not present in log entry")
	}
	if requestID != "req-abc-123" {
		t.Errorf("request_id = %v, want req-abc-123", requestID)
	}
}

func TestLoggingWithoutRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := LoggingConfig{Logger: logger}

	handler := Logging(cfg)(&testHandler{statusCode: http.StatusOK, body: []byte(`{"ok":true}`)})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	// No request ID set

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	// request_id should NOT be present.
	if _, ok := logEntry["request_id"]; ok {
		t.Error("request_id should not be present when not set in context")
	}
}

func TestLoggingDuration(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := LoggingConfig{Logger: logger}

	handler := Logging(cfg)(&testHandler{statusCode: http.StatusOK, body: []byte(`ok`), delay: 100 * time.Millisecond})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	duration, ok := logEntry["duration"]
	if !ok {
		t.Fatal("duration not present in log entry")
	}

	durationStr, ok := duration.(string)
	if !ok {
		t.Fatalf("duration is not a string: %v", duration)
	}

	// Parse duration string (format: "X.XXXs").
	d, err := time.ParseDuration(durationStr)
	if err != nil {
		t.Fatalf("failed to parse duration %q: %v", durationStr, err)
	}

	if d < 50*time.Millisecond {
		t.Errorf("duration %v seems too short for a 100ms handler", d)
	}
}

func TestLoggingCustomLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	cfg := LoggingConfig{Logger: logger}

	handler := Logging(cfg)(&testHandler{statusCode: http.StatusOK, body: []byte(`{"ok":true}`)})

	req := httptest.NewRequest(http.MethodPost, "/api/tenants", nil)
	req.RemoteAddr = "172.16.0.1:55555"

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	output := buf.String()
	if !strings.Contains(output, "POST") {
		t.Error("log should contain method POST")
	}
	if !strings.Contains(output, "/api/tenants") {
		t.Error("log should contain path /api/tenants")
	}
	if !strings.Contains(output, "172.16.0.1") {
		t.Error("log should contain stripped remote IP 172.16.0.1")
	}
}

func TestLoggingResponseSize(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := LoggingConfig{Logger: logger}

	body := []byte(`{"id":"test","name":"Test Tenant","status":"active"}`)
	handler := Logging(cfg)(&testHandler{statusCode: http.StatusOK, body: body})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	responseSize, ok := logEntry["response_size"].(float64)
	if !ok {
		t.Fatal("response_size not a number")
	}

	if int(responseSize) != len(body) {
		t.Errorf("response_size = %v, want %v", responseSize, len(body))
	}
}

func TestLoggingStripPort(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"192.168.1.1:8080", "192.168.1.1"},
		{"10.0.0.1:54321", "10.0.0.1"},
		{"[::1]:1234", "::1"},
		{"localhost", "localhost"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripPort(tt.input)
			if got != tt.expected {
				t.Errorf("stripPort(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLoggingSanitizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"clean path", "/api/tenants", "/api/tenants"},
		{"with query", "/api/tenants?page=1&limit=10", "/api/tenants"},
		{"with fragment", "/api/tenants#section", "/api/tenants"},
		{"query and fragment", "/api/tenants?page=1#section", "/api/tenants"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizePath(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLoggingWithDefaultLogger(t *testing.T) {
	// Using nil logger should fall back to slog.Default() — this should not panic.
	cfg := LoggingConfig{Logger: nil}

	handler := Logging(cfg)(&testHandler{statusCode: http.StatusOK, body: []byte(`ok`)})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	rr := httptest.NewRecorder()

	// Should not panic.
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusOK)
	}
}

func TestLoggingHandlesConcurrentRequests(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := LoggingConfig{Logger: logger}

	handler := Logging(cfg)(&testHandler{statusCode: http.StatusOK, body: []byte(`ok`)})

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.RemoteAddr = "10.0.0.1"
			req = req.WithContext(SetRequestID(req.Context(), "req-"+string(rune('A'+id))))

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("concurrent request %d: status = %v, want %v", id, rr.Code, http.StatusOK)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
