package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/ctxkeys"
)

func TestRequestID(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test without existing RequestID
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	RequestID(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	reqID := rec.Header().Get("X-Request-ID")
	if reqID == "" {
		t.Error("expected X-Request-ID header to be set")
	}

	// RequestID middleware always generates a new ID (replaces existing)
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "custom-id-123")
	rec = httptest.NewRecorder()

	RequestID(next).ServeHTTP(rec, req)

	reqID = rec.Header().Get("X-Request-ID")
	// The middleware generates a new ID regardless
	if reqID == "" {
		t.Error("expected X-Request-ID header to be set even when existing")
	}
	if reqID == "custom-id-123" {
		t.Error("RequestID middleware should generate a new ID, not preserve the existing one")
	}
}

func TestTraceID(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test without existing TraceID
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	TraceID(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	traceID := rec.Header().Get("X-Trace-Id")
	if traceID == "" {
		t.Error("expected X-Trace-Id header to be set")
	}

	// Test with existing TraceID
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Trace-Id", "custom-trace-456")
	rec = httptest.NewRecorder()

	TraceID(next).ServeHTTP(rec, req)

	traceID = rec.Header().Get("X-Trace-Id")
	if traceID != "custom-trace-456" {
		t.Errorf("expected X-Trace-Id 'custom-trace-456', got %s", traceID)
	}
}

func TestTenantContext(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantIDFromContext(r.Context())
		if tenantID != "test-tenant-789" {
			t.Errorf("expected tenant ID 'test-tenant-789', got %s", tenantID)
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Tenant-ID", "test-tenant-789")
	rec := httptest.NewRecorder()

	TenantContext(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestTenantContext_NoHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	TenantContext(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when no tenant ID header, got %d", rec.Code)
	}
}

func TestJWTAuth(t *testing.T) {
	secret := "test-secret-key-for-hmac-s256"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		if userID != "user-123" {
			t.Errorf("expected user ID 'user-123', got %s", userID)
		}
		w.WriteHeader(http.StatusOK)
	})

	// Create a valid token
	token, err := createTestToken("user-123")
	if err != nil {
		t.Fatalf("createTestToken failed: %v", err)
	}

	// Test with valid token
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	JWTAuth(secret, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestJWTAuth_NoToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	JWTAuth("test-secret-key-for-hmac-s256", next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	secret := "test-secret-key-for-hmac-s256"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	JWTAuth(secret, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestJWTAuth_BadScheme(t *testing.T) {
	secret := "test-secret-key-for-hmac-s256"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic token")
	rec := httptest.NewRecorder()

	JWTAuth(secret, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for bad scheme, got %d", rec.Code)
	}
}

func TestJWTAuth_WithRoles(t *testing.T) {
	secret := "test-secret-key-for-hmac-s256"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roles := UserRolesFromContext(r.Context())
		if len(roles) != 2 {
			t.Errorf("expected 2 roles, got %d", len(roles))
		}
		found := false
		for _, role := range roles {
			if role == "admin" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected 'admin' role in roles")
		}
		w.WriteHeader(http.StatusOK)
	})

	token, err := createTestTokenWithRoles("user-456", []string{"admin", "editor"})
	if err != nil {
		t.Fatalf("createTestTokenWithRoles failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	JWTAuth(secret, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestJWTAuth_WrongSecret(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	token, err := createTestToken("user-123")
	if err != nil {
		t.Fatalf("createTestToken failed: %v", err)
	}

	// Try with wrong secret
	wrongSecret := "wrong-secret-key"

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	JWTAuth(wrongSecret, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with wrong secret, got %d", rec.Code)
	}
}

func TestContextAccessFunctions(t *testing.T) {
	ctx := context.Background()

	if RequestIDFromContext(ctx) != "" {
		t.Error("expected empty RequestID from empty context")
	}

	if TenantIDFromContext(ctx) != "" {
		t.Error("expected empty TenantID from empty context")
	}

	if UserIDFromContext(ctx) != "" {
		t.Error("expected empty UserID from empty context")
	}

	if UserRolesFromContext(ctx) != nil {
		t.Error("expected nil UserRoles from empty context")
	}
}

func TestLogger(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	rec := httptest.NewRecorder()

	// Wrap with RequestID first so logger has a request ID
	mw := RequestID(Logger(next))
	mw.ServeHTTP(rec, req)

	if !called {
		t.Error("expected next handler to be called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestMiddlewareChain(t *testing.T) {
	secret := "test-secret-key-for-hmac-s256"

	handlerCalled := false
	handlerUserID := ""
	handlerTenantID := ""
	handlerRequestID := ""

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		handlerUserID = UserIDFromContext(r.Context())
		handlerTenantID = TenantIDFromContext(r.Context())
		handlerRequestID = RequestIDFromContext(r.Context())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"success"}`))
	})

	token, err := createTestToken("user-789")
	if err != nil {
		t.Fatalf("createTestToken failed: %v", err)
	}

	// Build the middleware chain as in main.go
	mw := Logger(
		RequestID(
			TraceID(
				JWTAuth(secret,
					TenantContext(next),
				),
			),
		),
	)

	req := httptest.NewRequest("GET", "/api/v1/templates", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("expected handler to be called")
	}

	if handlerUserID != "user-789" {
		t.Errorf("expected handler UserID 'user-789', got %s", handlerUserID)
	}

	if handlerTenantID != "tenant-abc" {
		t.Errorf("expected handler TenantID 'tenant-abc', got %s", handlerTenantID)
	}

	if handlerRequestID == "" {
		t.Error("expected handler RequestID to be set")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ─── Helper functions ────────────────────────────────────────────────────────

func createTestToken(userID string) (string, error) {
	return createTestTokenWithRoles(userID, nil)
}

func createTestTokenWithRoles(userID string, roles []string) (string, error) {
	// Create a proper JWT token that passes validation

	header := `{"alg":"HS256","typ":"JWT"}`
	payload := `{"sub":"` + userID + `"}`
	if len(roles) > 0 {
		rolesStr := "["
		for i, r := range roles {
			if i > 0 {
				rolesStr += ","
			}
			rolesStr += `"` + r + `"`
		}
		rolesStr += "]"
		payload = `{"sub":"` + userID + `","roles":` + rolesStr + `}`
	}

	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(header))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))

	signingInput := string(headerB64) + "." + string(payloadB64)

	// Compute HMAC-S256 signature with the test secret
	secret := []byte("test-secret-key-for-hmac-s256")
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	signature := mac.Sum(nil)
	sigB64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + sigB64, nil
}

// ─── Rate Limiting Tests ─────────────────────────────────────────────────────

func TestRateLimit_Allow(t *testing.T) {
	rl := newRateLimiter(5, 1*time.Minute)

	// Should allow first 5 requests
	for i := 0; i < 5; i++ {
		if !rl.Allow("tenant-1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 6th request should be rejected
	if rl.Allow("tenant-1") {
		t.Error("6th request should be rejected")
	}
}

func TestRateLimit_DifferentTenants(t *testing.T) {
	rl := newRateLimiter(3, 1*time.Minute)

	// Fill up tenant-1
	for i := 0; i < 3; i++ {
		rl.Allow("tenant-1")
	}

	// tenant-2 should still be allowed
	if !rl.Allow("tenant-2") {
		t.Error("tenant-2 should be allowed independently")
	}
}

func TestRateLimit_Middleware(t *testing.T) {
	rateLimitMW := RateLimit(2, 1*time.Second)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with rate limit middleware
	handler := rateLimitMW(next)

	// Create request with tenant context
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-Tenant-ID", "tenant-1")
	req1 = req1.WithContext(context.WithValue(req1.Context(), ctxkeys.TenantID, "tenant-1"))

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", rec1.Code)
	}

	// Second request should also succeed
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Tenant-ID", "tenant-1")
	req2 = req2.WithContext(context.WithValue(req2.Context(), ctxkeys.TenantID, "tenant-1"))

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("second request: expected 200, got %d", rec2.Code)
	}

	// Third request should be rate-limited
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.Header.Set("X-Tenant-ID", "tenant-1")
	req3 = req3.WithContext(context.WithValue(req3.Context(), ctxkeys.TenantID, "tenant-1"))

	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusTooManyRequests {
		t.Errorf("third request: expected 429, got %d: %s", rec3.Code, rec3.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec3.Body.Bytes(), &resp)
	if resp["status"] != float64(http.StatusTooManyRequests) {
		t.Errorf("expected status 429 in response, got %v", resp["status"])
	}
}
