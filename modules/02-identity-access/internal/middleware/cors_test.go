package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSAllowedOrigin(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"https://app.operan.internal"}

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://app.operan.internal")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusOK)
	}

	allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "https://app.operan.internal" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", allowOrigin, "https://app.operan.internal")
	}
}

func TestCORSDisallowedOrigin(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"https://app.operan.internal"}

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://evil.attacker.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusOK)
	}

	// Disallowed origin should NOT have CORS headers.
	allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "" {
		t.Errorf("Access-Control-Allow-Origin should be empty for disallowed origin, got %q", allowOrigin)
	}

	allowCredentials := rr.Header().Get("Access-Control-Allow-Credentials")
	if allowCredentials != "" {
		t.Errorf("Access-Control-Allow-Credentials should be empty for disallowed origin, got %q", allowCredentials)
	}
}

func TestCORSWildcardOriginWithCredentials(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"*"}
	cfg.AllowCredentials = true

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://app.operan.internal")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusOK)
	}

	allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
	// Wildcard with credentials: echo the actual origin (not "*") per CORS spec.
	if allowOrigin != "https://app.operan.internal" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", allowOrigin, "https://app.operan.internal")
	}

	allowCredentials := rr.Header().Get("Access-Control-Allow-Credentials")
	if allowCredentials != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want %q", allowCredentials, "true")
	}
}

func TestCORSNoOriginHeader(t *testing.T) {
	cfg := DefaultCORSConfig()

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	// No Origin header set

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusOK)
	}

	// No Origin header means no CORS headers.
	allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "" {
		t.Errorf("Access-Control-Allow-Origin should be empty when no Origin header, got %q", allowOrigin)
	}
}

func TestCORSOPTIONSValidPreflight(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"https://app.operan.internal", "https://admin.operan.internal"}

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "https://app.operan.internal")
	req.Header.Set("Access-Control-Request-Method", "DELETE")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, X-Custom-Header")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusNoContent)
	}

	if rr.Header().Get("Access-Control-Allow-Origin") != "https://app.operan.internal" {
		t.Error("Access-Control-Allow-Origin not set correctly")
	}

	if !strings.Contains(rr.Header().Get("Access-Control-Allow-Methods"), "DELETE") {
		t.Error("Access-Control-Allow-Methods does not contain DELETE")
	}

	// Default config has AllowedHeaders = ["*"], so the middleware echoes "*"
	// to indicate all requested headers are permitted.
	allowHeaders := rr.Header().Get("Access-Control-Allow-Headers")
	if allowHeaders != "*" {
		t.Errorf("Access-Control-Allow-Headers = %q, want %q", allowHeaders, "*")
	}

	maxAge := rr.Header().Get("Access-Control-Max-Age")
	if maxAge != "86400" {
		t.Errorf("Access-Control-Max-Age = %q, want %q", maxAge, "86400")
	}
}

func TestCROPTIONSInvalidOrigin(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"https://app.operan.internal"}

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "https://evil.attacker.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusForbidden)
	}
}

func TestCROPTIONSInvalidMethod(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"*"}
	cfg.AllowedMethods = []string{"GET", "POST"}

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "https://app.operan.internal")
	req.Header.Set("Access-Control-Request-Method", "TRACE")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusForbidden)
	}
}

func TestCROPTIONSInvalidHeader(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"*"}
	cfg.AllowedHeaders = []string{"Content-Type", "Authorization"}

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "https://app.operan.internal")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, X-Unauthorized-Header")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusForbidden)
	}
}

func TestCROPTIONSWildcardHeaders(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"*"}
	cfg.AllowedHeaders = []string{"*"}

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "https://app.operan.internal")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "X-Any-Custom-Header, X-Another-Header")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusNoContent)
	}
}

func TestCORSCustomAllowedOrigins(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{
		"https://app.operan.internal",
		"https://admin.operan.internal",
		"https://staging.operan.internal",
	}
	cfg.AllowCredentials = true
	cfg.MaxAge = 3600

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name          string
		origin        string
		wantAllow     bool
	}{
		{"allowed origin 1", "https://app.operan.internal", true},
		{"allowed origin 2", "https://admin.operan.internal", true},
		{"allowed origin 3", "https://staging.operan.internal", true},
		{"disallowed origin", "https://evil.attacker.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.Header.Set("Origin", tt.origin)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
			if tt.wantAllow && allowOrigin != tt.origin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", allowOrigin, tt.origin)
			}
			if !tt.wantAllow && allowOrigin != "" {
				t.Errorf("Access-Control-Allow-Origin should be empty for disallowed origin, got %q", allowOrigin)
			}
		})
	}
}

func TestCORSDefaultConfig(t *testing.T) {
	cfg := DefaultCORSConfig()

	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "*" {
		t.Errorf("default AllowedOrigins = %v, want [\"*\"]", cfg.AllowedOrigins)
	}

	expectedMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	if len(cfg.AllowedMethods) != len(expectedMethods) {
		t.Errorf("default AllowedMethods length = %d, want %d", len(cfg.AllowedMethods), len(expectedMethods))
	}

	if len(cfg.AllowedHeaders) != 1 || cfg.AllowedHeaders[0] != "*" {
		t.Errorf("default AllowedHeaders = %v, want [\"*\"]", cfg.AllowedHeaders)
	}

	if cfg.AllowCredentials != false {
		t.Errorf("default AllowCredentials = %v, want false", cfg.AllowCredentials)
	}

	if cfg.MaxAge != 86400 {
		t.Errorf("default MaxAge = %d, want 86400", cfg.MaxAge)
	}
}

func TestCORSPassthroughForNonOPTIONS(t *testing.T) {
	cfg := DefaultCORSConfig()

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "custom-value")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.Header.Set("Origin", "https://app.operan.internal")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusOK)
	}

	if rr.Header().Get("X-Custom-Header") != "custom-value" {
		t.Error("Downstream headers should pass through")
	}

	// Body should be correct.
	if rr.Body.String() != `{"status":"ok"}` {
		t.Errorf("body = %q, want %q", rr.Body.String(), `{"status":"ok"}`)
	}
}

func TestCORSPreflightWithCredentials(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"https://app.operan.internal"}
	cfg.AllowCredentials = true

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "https://app.operan.internal")
	req.Header.Set("Access-Control-Request-Method", "GET")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusNoContent)
	}

	allowCredentials := rr.Header().Get("Access-Control-Allow-Credentials")
	if allowCredentials != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want %q", allowCredentials, "true")
	}
}

func TestCORSWildcardOriginNoCredentials(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"*"}
	cfg.AllowCredentials = false

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://app.operan.internal")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", allowOrigin, "*")
	}

	allowCredentials := rr.Header().Get("Access-Control-Allow-Credentials")
	if allowCredentials != "" {
		t.Errorf("Access-Control-Allow-Credentials should be empty with credentials disabled, got %q", allowCredentials)
	}
}
