package middleware

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/operan/modules/04-agent-registry/internal/ctxkeys"
)

func uid() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(i + 1)
	}
	return hex.EncodeToString(b)
}

func makeValidJWT(t *testing.T, secret string, claims map[string]interface{}) string {
	t.Helper()

	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))

	payloadMap := map[string]interface{}{
		"sub":         "user-1",
		"tenant_id":   "tenant-100",
		"tenantId":    "tenant-100",
		"role":        "admin",
		"iat":         time.Now().Unix(),
		"exp":         time.Now().Add(24 * time.Hour).Unix(),
	}
	for k, v := range claims {
		payloadMap[k] = v
	}

	// Simpler approach - just encode manually
	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)

	sigBytes := computeHMAC(secret, header+"."+payloadB64)
	signature := base64URLEncode(sigBytes)
	return header + "." + payloadB64 + "." + signature
}

func TestTenantIDFromContext(t *testing.T) {
	ctx := context.Background()
	if TenantIDFromContext(ctx) != "" {
		t.Error("expected empty tenant ID")
	}

	ctx = SetTenantIDToContext(ctx, "tenant-123")
	if TenantIDFromContext(ctx) != "tenant-123" {
		t.Errorf("expected 'tenant-123', got %q", TenantIDFromContext(ctx))
	}
}

func TestUserIDFromContext(t *testing.T) {
	ctx := context.Background()
	if UserIDFromContext(ctx) != "" {
		t.Error("expected empty user ID")
	}

	ctx = SetUserIDToContext(ctx, "user-456")
	if UserIDFromContext(ctx) != "user-456" {
		t.Errorf("expected 'user-456', got %q", UserIDFromContext(ctx))
	}
}

func TestExtractTenant_Success(t *testing.T) {
	nextCalled := false
	mw := ExtractTenant(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		tenantID := TenantIDFromContext(r.Context())
		if tenantID != "tenant-1" {
			t.Errorf("expected 'tenant-1', got %q", tenantID)
		}
	})

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	w := httptest.NewRecorder()

	mw(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
}

func TestExtractTenant_FallbackHeaders(t *testing.T) {
	nextCalled := false
	mw := ExtractTenant(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		tenantID := TenantIDFromContext(r.Context())
		if tenantID != "tenant-2" {
			t.Errorf("expected 'tenant-2', got %q", tenantID)
		}
	})

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("tenant-id", "tenant-2")
	w := httptest.NewRecorder()

	mw(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
}

func TestExtractTenant_QueryParam(t *testing.T) {
	nextCalled := false
	mw := ExtractTenant(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		tenantID := TenantIDFromContext(r.Context())
		if tenantID != "tenant-3" {
			t.Errorf("expected 'tenant-3', got %q", tenantID)
		}
	})

	req := httptest.NewRequest("GET", "/registry/agents?tenant_id=tenant-3", nil)
	w := httptest.NewRecorder()

	mw(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
}

func TestExtractTenant_Missing(t *testing.T) {
	mw := ExtractTenant(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	w := httptest.NewRecorder()

	mw(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Type != "missing_tenant_id" {
		t.Errorf("expected error type 'missing_tenant_id', got %q", errResp.Type)
	}
}

func TestRequestID_Propagate(t *testing.T) {
	var gotRequestID string
	mw := RequestID(func(w http.ResponseWriter, r *http.Request) {
		gotRequestID = RequestIDFromContext(r.Context())
	})

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("X-Request-Id", "test-req-id")
	w := httptest.NewRecorder()

	mw(w, req)

	if gotRequestID != "test-req-id" {
		t.Errorf("expected 'test-req-id', got %q", gotRequestID)
	}
	if w.Header().Get("X-Request-Id") != "test-req-id" {
		t.Errorf("expected X-Request-Id header, got %q", w.Header().Get("X-Request-Id"))
	}
}

func TestRequestID_Generate(t *testing.T) {
	var gotRequestID string
	mw := RequestID(func(w http.ResponseWriter, r *http.Request) {
		gotRequestID = RequestIDFromContext(r.Context())
	})

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	w := httptest.NewRecorder()

	mw(w, req)

	if gotRequestID == "" {
		t.Error("expected generated request ID")
	}
}

func TestTraceID_Propagate(t *testing.T) {
	var gotTraceID string
	mw := TraceID(func(w http.ResponseWriter, r *http.Request) {
		gotTraceID = TraceIDFromContext(r.Context())
	})

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("X-Trace-Id", "test-trace-123")
	w := httptest.NewRecorder()

	mw(w, req)

	if gotTraceID != "test-trace-123" {
		t.Errorf("expected 'test-trace-123', got %q", gotTraceID)
	}
	if w.Header().Get("X-Trace-Id") != "test-trace-123" {
		t.Errorf("expected X-Trace-Id header, got %q", w.Header().Get("X-Trace-Id"))
	}
}

func TestTraceID_Generate(t *testing.T) {
	var gotTraceID string
	mw := TraceID(func(w http.ResponseWriter, r *http.Request) {
		gotTraceID = TraceIDFromContext(r.Context())
	})

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	w := httptest.NewRecorder()

	mw(w, req)

	if gotTraceID == "" {
		t.Error("expected generated trace ID")
	}
}

func TestJWTAuth_ValidToken(t *testing.T) {
	secret := "test-jwt-secret"
	token := makeValidJWT(t, secret, map[string]interface{}{})

	nextCalled := false
	jwtMW := JWTAuth(secret)
	handler := jwtMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		if UserIDFromContext(r.Context()) != "user-1" {
			t.Errorf("expected user-1, got %q", UserIDFromContext(r.Context()))
		}
		if TenantIDFromContext(r.Context()) != "tenant-100" {
			t.Errorf("expected tenant-100, got %q", TenantIDFromContext(r.Context()))
		}
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	jwtMW := JWTAuth("JWT_SECRET")
	handler := jwtMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuth_InvalidScheme(t *testing.T) {
	jwtMW := JWTAuth("JWT_SECRET")
	handler := jwtMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuth_InvalidSignature(t *testing.T) {
	token := "header.payload.invalidsignature"

	jwtMW := JWTAuth("JWT_SECRET")
	handler := jwtMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	secret := "test-jwt-secret"

	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadMap := map[string]interface{}{
		"sub":   "user-1",
		"tenant_id": "tenant-100",
		"iat":   time.Now().Add(-2 * time.Hour).Unix(),
		"exp":   time.Now().Add(-1 * time.Hour).Unix(), // expired 1 hour ago
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)
	sigBytes := computeHMAC(secret, header+"."+payloadB64)
	signature := base64URLEncode(sigBytes)
	token := header + "." + payloadB64 + "." + signature

	jwtMW := JWTAuth(secret)
	handler := jwtMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusOK, map[string]string{"key": "value"})

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", w.Header().Get("Content-Type"))
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["key"] != "value" {
		t.Errorf("expected value, got %q", resp["key"])
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusBadRequest, "bad_request", "Bad Request", "Invalid input", "/test")

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)

	if errResp.Type != "bad_request" {
		t.Errorf("expected type 'bad_request', got %q", errResp.Type)
	}
	if errResp.Title != "Bad Request" {
		t.Errorf("expected title 'Bad Request', got %q", errResp.Title)
	}
}

func TestChain(t *testing.T) {
	values := []string{}

	mw := Chain(
		func(w http.ResponseWriter, r *http.Request) {
			values = append(values, "last")
		},
		func(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
			return func(w http.ResponseWriter, r *http.Request) {
				values = append(values, "first")
				next(w, r)
			}
		},
		func(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
			return func(w http.ResponseWriter, r *http.Request) {
				values = append(values, "second")
				next(w, r)
			}
		},
	)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	mw(w, req)

	if len(values) != 3 || values[0] != "first" || values[1] != "second" || values[2] != "last" {
		t.Errorf("expected [first second last], got %v", values)
	}
}

func TestSecureCompare(t *testing.T) {
	if !secureCompare("test", "test") {
		t.Error("expected true for equal strings")
	}
	if secureCompare("test", "other") {
		t.Error("expected false for different strings")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if len(id1) == 0 {
		t.Error("expected non-empty ID")
	}
	if id1 == id2 {
		t.Error("expected different IDs")
	}
}

func TestLoggerMiddleware(t *testing.T) {
	mw := Logger(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req = req.WithContext(SetRequestIDToContext(req.Context(), "test-req"))
	req = req.WithContext(SetTraceIDToContext(req.Context(), "test-trace"))
	req = req.WithContext(SetTenantIDToContext(req.Context(), "tenant-1"))
	w := httptest.NewRecorder()

	mw(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestJWTAuth_TenantIdClaim(t *testing.T) {
	secret := "test-jwt-secret"

	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadMap := map[string]interface{}{
		"sub":      "user-1",
		"tenantId": "tenant-alt", // camelCase variant
		"role":     "admin",
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)
	sigBytes := computeHMAC(secret, header+"."+payloadB64)
	signature := base64URLEncode(sigBytes)
	token := header + "." + payloadB64 + "." + signature

	nextCalled := false
	jwtMW := JWTAuth(secret)
	handler := jwtMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		if TenantIDFromContext(r.Context()) != "tenant-alt" {
			t.Errorf("expected tenant-alt, got %q", TenantIDFromContext(r.Context()))
		}
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
}

func TestJWTAuth_RoleClaim(t *testing.T) {
	secret := "test-jwt-secret"

	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadMap := map[string]interface{}{
		"sub":       "user-1",
		"tenant_id": "tenant-100",
		"role":      "editor",
		"iat":       time.Now().Unix(),
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)
	sigBytes := computeHMAC(secret, header+"."+payloadB64)
	signature := base64URLEncode(sigBytes)
	token := header + "." + payloadB64 + "." + signature

	var gotRole string
	jwtMW := JWTAuth(secret)
	handler := jwtMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRole = r.Context().Value(ctxkeys.UserRole).(string)
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if gotRole != "editor" {
		t.Errorf("expected role 'editor', got %q", gotRole)
	}
}

// ─── ChainJWTAuth Tests ─────────────────────────────────────────────────────

func TestChainJWTAuth_ValidToken(t *testing.T) {
	secret := "test-secret"
	token := makeValidJWT(t, secret, nil)

	nextCalled := false
	var gotUserID, gotTenantID string
	chainMW := ChainJWTAuth(secret)
	handler := chainMW(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		gotUserID = UserIDFromContext(r.Context())
		gotTenantID = TenantIDFromContext(r.Context())
	})

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !nextCalled {
		t.Error("next handler should be called")
	}
	if gotUserID != "user-1" {
		t.Errorf("expected user-1, got %q", gotUserID)
	}
	if gotTenantID != "tenant-100" {
		t.Errorf("expected tenant-100, got %q", gotTenantID)
	}
}

func TestChainJWTAuth_MissingHeader(t *testing.T) {
	chainMW := ChainJWTAuth("secret")
	handler := chainMW(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestChainJWTAuth_InvalidToken(t *testing.T) {
	chainMW := ChainJWTAuth("secret")
	handler := chainMW(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// ─── ExtractTenant JWT Context Fallback Tests ────────────────────────────────

func TestExtractTenant_JWTContextFallback(t *testing.T) {
	nextCalled := false
	var gotTenantID string
	mw := ExtractTenant(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		gotTenantID = TenantIDFromContext(r.Context())
	})

	// Create request with JWT-extracted tenant in context
	token := makeValidJWT(t, "test-secret", nil)
	sigParts := strings.Split(token, ".")
	sig := base64URLEncode(computeHMAC("test-secret", sigParts[0]+"."+sigParts[1]))
	validToken := sigParts[0] + "." + sigParts[1] + "." + sig

	jwtMW := JWTAuthWithSecret("test-secret")
	wrapped := jwtMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Apply ExtractTenant middleware
		mw(w, r)
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called after JWT validation and ExtractTenant")
	}
	if gotTenantID != "tenant-100" {
		t.Errorf("expected tenant-100 from JWT, got %q", gotTenantID)
	}
}

func TestExtractTenant_ContextPrecedenceOverHeader(t *testing.T) {
	nextCalled := false
	var gotTenantID string
	mw := ExtractTenant(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		gotTenantID = TenantIDFromContext(r.Context())
	})

	// Create request with JWT-extracted tenant that differs from header
	token := makeValidJWT(t, "test-secret", nil)
	sigParts := strings.Split(token, ".")
	sig := base64URLEncode(computeHMAC("test-secret", sigParts[0]+"."+sigParts[1]))
	validToken := sigParts[0] + "." + sigParts[1] + "." + sig

	jwtMW := JWTAuthWithSecret("test-secret")
	wrapped := jwtMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mw(w, r)
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)
	req.Header.Set("X-Tenant-ID", "header-tenant") // Different from JWT tenant
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
	// JWT tenant should take precedence
	if gotTenantID != "tenant-100" {
		t.Errorf("expected tenant-100 from JWT (not header), got %q", gotTenantID)
	}
}

// ─── RequireRole Tests ───────────────────────────────────────────────────────

func TestRequireRole_ValidRole(t *testing.T) {
	nextCalled := false
	mw := RequireRole("admin", "editor")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}))

	// Create request with admin role in context
	token := makeValidJWT(t, "test-secret", map[string]interface{}{
		"sub": "user-1",
		"role": "admin",
		"tenant_id": "tenant-1",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	// JWTAuth must run first to populate context
	jwtMW := JWTAuthWithSecret("test-secret")
	jwtMW(handler).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !nextCalled {
		t.Error("next handler should be called for valid role")
	}
}

func TestRequireRole_InvalidRole(t *testing.T) {
	mw := RequireRole("admin", "editor")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called for invalid role")
	}))

	// Create request with viewer role (not in allowed list)
	token := makeValidJWT(t, "test-secret", map[string]interface{}{
		"sub": "user-1",
		"role": "viewer",
		"tenant_id": "tenant-1",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	// JWTAuth must run first to populate context
	jwtMW := JWTAuthWithSecret("test-secret")
	jwtMW(handler).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRequireRole_MissingRole(t *testing.T) {
	mw := RequireRole("admin", "editor")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called when role is missing")
	}))

	// Create request with no role claim (override default admin)
	token := makeValidJWT(t, "test-secret", map[string]interface{}{
		"sub": "user-1",
		"role": nil, // Explicitly set role to nil to remove it
		"tenant_id": "tenant-1",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	// JWTAuth must run first to populate context
	jwtMW := JWTAuthWithSecret("test-secret")
	jwtMW(handler).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// ─── RequireAdmin Tests ──────────────────────────────────────────────────────

func TestRequireAdmin_WithAdminRole(t *testing.T) {
	nextCalled := false
	mw := RequireAdmin
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}))

	// Create request with admin role
	token := makeValidJWT(t, "test-secret", map[string]interface{}{
		"sub": "user-1",
		"role": "admin",
		"tenant_id": "tenant-1",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	// JWTAuth must run first to populate context
	jwtMW := JWTAuthWithSecret("test-secret")
	jwtMW(handler).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !nextCalled {
		t.Error("next handler should be called for admin role")
	}
}

func TestRequireAdmin_WithNonAdminRole(t *testing.T) {
	mw := RequireAdmin
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called for non-admin role")
	}))

	// Create request with editor role
	token := makeValidJWT(t, "test-secret", map[string]interface{}{
		"sub": "user-1",
		"role": "editor",
		"tenant_id": "tenant-1",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	// JWTAuth must run first to populate context
	jwtMW := JWTAuthWithSecret("test-secret")
	jwtMW(handler).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// ─── JWKSClient Tests ────────────────────────────────────────────────────────

func TestNewJWKSClient(t *testing.T) {
	client := NewJWKSClient("https://example.com/.well-known/jwks.json")

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.url != "https://example.com/.well-known/jwks.json" {
		t.Errorf("expected url 'https://example.com/.well-known/jwks.json', got %q", client.url)
	}
	if len(client.keys) != 0 {
		t.Error("expected empty keys map on creation")
	}
}

func TestJWKSClient_GetKey(t *testing.T) {
	client := NewJWKSClient("https://example.com/.well-known/jwks.json")

	// Set a key directly (simulating successful fetch)
	client.keys["key-1"] = map[string]string{
		"kty": "RSA",
		"kid": "key-1",
	}

	// Test key found
	key := client.GetKey("key-1")
	if key == nil {
		t.Fatal("expected key to be found")
	}
	if keyMap, ok := key.(map[string]string); !ok || keyMap["kid"] != "key-1" {
		t.Errorf("expected key-1, got %v", key)
	}

	// Test key not found
	missingKey := client.GetKey("missing-key")
	if missingKey != nil {
		t.Errorf("expected nil for missing key, got %v", missingKey)
	}
}

func TestJWKSClient_FetchKeys_Error(t *testing.T) {
	// Use an invalid URL that will fail HTTP request
	client := NewJWKSClient("http://invalid.invalid:99999/.well-known/jwks.json")
	err := client.FetchKeys()
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// ─── ValidateJWKSJWT Tests ──────────────────────────────────────────────────

func TestValidateJWKSJWT_MissingKid(t *testing.T) {
	// Create a valid JWT without kid in header
	secret := "test-secret"
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadMap := map[string]interface{}{
		"sub": "user-1",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)
	sigBytes := computeHMAC(secret, header+"."+payloadB64)
	signature := base64URLEncode(sigBytes)
	token := header + "." + payloadB64 + "." + signature

	_, err := ValidateJWKSJWT("https://example.com/jwks", token)
	if err == nil {
		t.Error("expected error for JWT without kid")
	}
}

func TestValidateJWKSJWT_InvalidHeader(t *testing.T) {
	// Create a token with invalid header encoding
	token := "!!!invalid!!!.payload.signature"

	_, err := ValidateJWKSJWT("https://example.com/jwks", token)
	if err == nil {
		t.Error("expected error for invalid header encoding")
	}
}

func TestValidateJWKSJWT_MissingKey(t *testing.T) {
	// Create a valid JWT with kid
	secret := "test-secret"
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT","kid":"key-1"}`))
	payloadMap := map[string]interface{}{
		"sub": "user-1",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)
	sigBytes := computeHMAC(secret, header+"."+payloadB64)
	signature := base64URLEncode(sigBytes)
	token := header + "." + payloadB64 + "." + signature

	// JWKS endpoint is invalid, so it should fail when trying to fetch keys
	_, err := ValidateJWKSJWT("http://invalid.invalid:99999/jwks", token)
	if err == nil {
		t.Error("expected error for missing JWKS key")
	}
}

// ─── JWKSAuth Tests ──────────────────────────────────────────────────────────

func TestJWKSAuth_HMACFallback(t *testing.T) {
	secret := "test-jwt-secret"
	token := makeValidJWT(t, secret, map[string]interface{}{
		"sub": "user-1",
		"tenant_id": "tenant-100",
		"role": "admin",
	})

	nextCalled := false
	var gotUserID, gotTenantID, gotRole string
	middleware := JWKSAuth("", secret) // Empty JWKS URL, use HMAC fallback
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		gotUserID = UserIDFromContext(r.Context())
		gotTenantID = TenantIDFromContext(r.Context())
		gotRole = RoleFromContext(r.Context())
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !nextCalled {
		t.Error("next handler should be called")
	}
	if gotUserID != "user-1" {
		t.Errorf("expected user-1, got %q", gotUserID)
	}
	if gotTenantID != "tenant-100" {
		t.Errorf("expected tenant-100, got %q", gotTenantID)
	}
	if gotRole != "admin" {
		t.Errorf("expected admin, got %q", gotRole)
	}
}

func TestJWKSAuth_MissingAuthHeader(t *testing.T) {
	middleware := JWKSAuth("", "secret")
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWKSAuth_InvalidToken(t *testing.T) {
	middleware := JWKSAuth("", "secret")
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// ─── RoleFromContext Test ────────────────────────────────────────────────────

func TestRoleFromContext(t *testing.T) {
	ctx := context.Background()
	if RoleFromContext(ctx) != "" {
		t.Error("expected empty role")
	}

	// Set role in context
	ctx = context.WithValue(ctx, ctxkeys.UserRole, "admin")
	if RoleFromContext(ctx) != "admin" {
		t.Errorf("expected 'admin', got %q", RoleFromContext(ctx))
	}
}

// ─── base64URLDecode Error Path Test ─────────────────────────────────────────

func TestBase64URLDecode_ErrorPath(t *testing.T) {
	// Invalid base64url input
	_, err := base64URLDecode("!!!not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64url input")
	}
}

// ─── FetchKeys HTTP error path test ──────────────────────────────────────────

func TestFetchKeys_HTTPError(t *testing.T) {
	client := NewJWKSClient("http://invalid.invalid:99999/.well-known/jwks.json")
	err := client.FetchKeys()
	if err == nil {
		t.Error("expected error for invalid HTTP endpoint")
	}
}

// ─── ValidateJWKSJWT invalid key test ────────────────────────────────────────

func TestValidateJWKSJWT_InvalidKeyType(t *testing.T) {
	// Create a JWT with a valid kid but the JWKS will fail to fetch
	secret := "test-secret"
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT","kid":"key-1"}`))
	payloadMap := map[string]interface{}{
		"sub": "user-1",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)
	sigBytes := computeHMAC(secret, header+"."+payloadB64)
	signature := base64URLEncode(sigBytes)
	token := header + "." + payloadB64 + "." + signature

	_, err := ValidateJWKSJWT("http://invalid.invalid:99999/jwks", token)
	if err == nil {
		t.Error("expected error for invalid JWKS endpoint")
	}
}

// ─── JWTAuthWithSecret Bearer prefix validation ──────────────────────────────

func TestJWTAuthWithSecret_InvalidBearerPrefix(t *testing.T) {
	jwtMW := JWTAuthWithSecret("secret")
	handler := jwtMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// ─── validateHMACJWT expiry validation ───────────────────────────────────────

func TestValidateHMACJWT_Expiry(t *testing.T) {
	secret := "test-secret"

	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadMap := map[string]interface{}{
		"sub":   "user-1",
		"tenant_id": "tenant-100",
		"iat":   time.Now().Add(-2 * time.Hour).Unix(),
		"exp":   time.Now().Add(-1 * time.Hour).Unix(), // expired 1 hour ago
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)
	sigBytes := computeHMAC(secret, header+"."+payloadB64)
	signature := base64URLEncode(sigBytes)
	token := header + "." + payloadB64 + "." + signature

	_, err := validateHMACJWT(secret, token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

// ─── validateHMACJWT claim validation ────────────────────────────────────────

func TestValidateHMACJWT_MalformedPayload(t *testing.T) {
	// Create a token with invalid JSON in payload
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadB64 := base64URLEncode([]byte("not valid json"))
	sigBytes := computeHMAC("secret", header+"."+payloadB64)
	signature := base64URLEncode(sigBytes)
	token := header + "." + payloadB64 + "." + signature

	_, err := validateHMACJWT("secret", token)
	if err == nil {
		t.Error("expected error for malformed payload")
	}
}

// ─── FetchKeys success path ──────────────────────────────────────────────────

func TestFetchKeys_Success(t *testing.T) {
	// Create a local HTTP server that returns a valid JWKS
	jwksPayload := `{"keys":[{"kty":"RSA","use":"sig","kid":"test-key-1","n":"testn","e":"teste"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jwksPayload))
	}))
	defer server.Close()

	client := NewJWKSClient(server.URL)
	err := client.FetchKeys()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	key := client.GetKey("test-key-1")
	if key == nil {
		t.Fatal("expected key 'test-key-1' to be stored")
	}
}

func TestFetchKeys_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	client := NewJWKSClient(server.URL)
	err := client.FetchKeys()
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestFetchKeys_HTTPError404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewJWKSClient(server.URL)
	err := client.FetchKeys()
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

func TestFetchKeys_HTTPError500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewJWKSClient(server.URL)
	err := client.FetchKeys()
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestFetchKeys_EmptyKeysArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"keys":[]}`))
	}))
	defer server.Close()

	client := NewJWKSClient(server.URL)
	err := client.FetchKeys()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Keys map should be empty
	if len(client.keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(client.keys))
	}
}

func TestFetchKeys_KeyWithNoKid(t *testing.T) {
	// A JWKS with keys that have no kid should not store them
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"keys":[{"kty":"RSA","use":"sig","n":"testn","e":"teste"}]}`))
	}))
	defer server.Close()

	client := NewJWKSClient(server.URL)
	err := client.FetchKeys()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(client.keys) != 0 {
		t.Errorf("expected 0 keys (no kid), got %d", len(client.keys))
	}
}

// ─── ValidateJWKSJWT success path ────────────────────────────────────────────

func TestValidateJWKSJWT_Success(t *testing.T) {
	// Create a local HTTP server that returns a valid JWKS
	jwksPayload := `{"keys":[{"kty":"RSA","use":"sig","kid":"key-1","n":"testn","e":"teste"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jwksPayload))
	}))
	defer server.Close()

	// Create a JWT with kid in header
	secret := "test-secret"
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT","kid":"key-1"}`))
	payloadMap := map[string]interface{}{
		"sub":  "user-1",
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
		"role": "admin",
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)
	sigBytes := computeHMAC(secret, header+"."+payloadB64)
	signature := base64URLEncode(sigBytes)
	token := header + "." + payloadB64 + "." + signature

	claims, err := ValidateJWKSJWT(server.URL, token)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if claims["sub"] != "user-1" {
		t.Errorf("expected sub 'user-1', got %v", claims["sub"])
	}
}

func TestValidateJWKSJWT_NoMatchingKey(t *testing.T) {
	// JWKS returns keys but none match the JWT's kid
	jwksPayload := `{"keys":[{"kty":"RSA","use":"sig","kid":"other-key","n":"testn","e":"teste"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jwksPayload))
	}))
	defer server.Close()

	secret := "test-secret"
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT","kid":"key-1"}`))
	payloadMap := map[string]interface{}{
		"sub": "user-1",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)
	sigBytes := computeHMAC(secret, header+"."+payloadB64)
	signature := base64URLEncode(sigBytes)
	token := header + "." + payloadB64 + "." + signature

	_, err := ValidateJWKSJWT(server.URL, token)
	if err == nil {
		t.Error("expected error for no matching key")
	}
}

func TestValidateJWKSJWT_EmptySignature(t *testing.T) {
	// JWKS returns keys matching the kid
	jwksPayload := `{"keys":[{"kty":"RSA","use":"sig","kid":"key-1","n":"testn","e":"teste"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jwksPayload))
	}))
	defer server.Close()

	// Create a JWT with empty signature
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT","kid":"key-1"}`))
	payloadMap := map[string]interface{}{
		"sub": "user-1",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)
	token := header + "." + payloadB64 + "."

	_, err := ValidateJWKSJWT(server.URL, token)
	if err == nil {
		t.Error("expected error for empty signature")
	}
}

func TestValidateJWKSJWT_MalformedPayload(t *testing.T) {
	// JWKS returns keys matching the kid
	jwksPayload := `{"keys":[{"kty":"RSA","use":"sig","kid":"key-1","n":"testn","e":"teste"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jwksPayload))
	}))
	defer server.Close()

	// Create a JWT with non-base64url payload
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT","kid":"key-1"}`))
	invalidPayloadB64 := "!!!invalid!!!invalid!!!invalid!!!invalid!!!"
	signature := base64URLEncode(computeHMAC("secret", header+"."+invalidPayloadB64))
	token := header + "." + invalidPayloadB64 + "." + signature

	_, err := ValidateJWKSJWT(server.URL, token)
	if err == nil {
		t.Error("expected error for malformed payload")
	}
}

// ─── JWKSAuth success path (JWKS validation, not HMAC fallback) ──────────────

func TestJWKSAuth_JWKSSuccess(t *testing.T) {
	// Create a local HTTP server that returns a valid JWKS
	jwksPayload := `{"keys":[{"kty":"RSA","use":"sig","kid":"key-1","n":"testn","e":"teste"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jwksPayload))
	}))
	defer server.Close()

	// Create a JWT with kid in header
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT","kid":"key-1"}`))
	payloadMap := map[string]interface{}{
		"sub":       "user-jwks",
		"tenant_id": "tenant-jwks",
		"role":      "editor",
		"iat":       time.Now().Unix(),
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)
	// Use HMAC to create a valid signature (the code uses HMAC for simplicity)
	sigBytes := computeHMAC("test-secret", header+"."+payloadB64)
	signature := base64URLEncode(sigBytes)
	token := header + "." + payloadB64 + "." + signature

	nextCalled := false
	var gotUserID, gotTenantID, gotRole string
	middleware := JWKSAuth(server.URL, "unused-secret")
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		gotUserID = UserIDFromContext(r.Context())
		gotTenantID = TenantIDFromContext(r.Context())
		gotRole = RoleFromContext(r.Context())
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !nextCalled {
		t.Error("next handler should be called for JWKS-validated token")
	}
	if gotUserID != "user-jwks" {
		t.Errorf("expected user-jwks, got %q", gotUserID)
	}
	if gotTenantID != "tenant-jwks" {
		t.Errorf("expected tenant-jwks, got %q", gotTenantID)
	}
	if gotRole != "editor" {
		t.Errorf("expected editor, got %q", gotRole)
	}
}

func TestJWKSAuth_JWKSFetchFailHMACFallback(t *testing.T) {
	// JWKS URL is invalid so FetchKeys fails, should fall back to HMAC
	secret := "test-secret"
	token := makeValidJWT(t, secret, map[string]interface{}{
		"sub":       "user-hmac",
		"tenant_id": "tenant-hmac",
		"role":      "viewer",
	})

	nextCalled := false
	middleware := JWKSAuth("http://invalid.invalid:99999/jwks", secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (HMAC fallback), got %d", w.Code)
	}
	if !nextCalled {
		t.Error("next handler should be called after HMAC fallback")
	}
}

func TestJWKSAuth_JWKSFetchFailAndHMACFail(t *testing.T) {
	// JWKS URL is invalid AND the token is not valid HMAC
	middleware := JWKSAuth("http://invalid.invalid:99999/jwks", "wrong-secret")
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	// Create a JWT signed with a different secret
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT","kid":"key-1"}`))
	payloadMap := map[string]interface{}{
		"sub":       "user-hmac",
		"tenant_id": "tenant-hmac",
		"role":      "viewer",
		"iat":       time.Now().Unix(),
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)
	sigBytes := computeHMAC("different-secret", header+"."+payloadB64)
	signature := base64URLEncode(sigBytes)
	token := header + "." + payloadB64 + "." + signature

	req := httptest.NewRequest("GET", "/registry/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 (both JWKS and HMAC fail), got %d", w.Code)
	}
}

// ─── base64URLDecode padding edge cases ──────────────────────────────────────

func TestBase64URLDecode_Padding2(t *testing.T) {
	// Input with 2 bytes remainder needs 2 padding chars
	input := "YWJj" // "abc" in base64url without padding
	result, err := base64URLDecode(input)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if string(result) != "abc" {
		t.Errorf("expected 'abc', got %q", string(result))
	}
}

func TestBase64URLDecode_Padding3(t *testing.T) {
	// Input with 3 bytes remainder needs 1 padding char
	input := "YWJjZA" // "abcd" in base64url without padding
	result, err := base64URLDecode(input)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if string(result) != "abcd" {
		t.Errorf("expected 'abcd', got %q", string(result))
	}
}

// ─── validateHMACJWT non-3-part error ────────────────────────────────────────

func TestValidateHMACJWT_NonThreeParts(t *testing.T) {
	_, err := validateHMACJWT("secret", "onlytwo.parts")
	if err == nil {
		t.Error("expected error for non-three-part token")
	}
}

func TestValidateHMACJWT_TooManyParts(t *testing.T) {
	_, err := validateHMACJWT("secret", "a.b.c.d")
	if err == nil {
		t.Error("expected error for too-many-parts token")
	}
}

