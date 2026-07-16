package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/06-knowledge-ingestion/internal/config"
	"github.com/operan/modules/06-knowledge-ingestion/internal/ctxkeys"

	"github.com/golang-jwt/jwt/v5"
)

// WriteJSON is a local helper for writing JSON responses in tests.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// alias for compatibility with test expectations
var WriteJSON = writeJSON

func newTestConfig() *config.Config {
	return &config.Config{
		JWTSecret: "test-secret-key-for-jwt-signing",
	}
}

func issueJWT(t *testing.T, secret, tenantID string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"tenant_id": tenantID,
		"sub":         "user-123",
		"roles":      []any{"admin", "reader"},
		"iss":        "operan-auth",
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}
	return signed
}

// === JWT Middleware Tests ===

func TestJWTMiddleware_MissingAuthHeader(t *testing.T) {
	validator := NewAuthValidator(newTestConfig())
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := JWTMiddleware(validator)(next)

	req := httptest.NewRequest(http.MethodGet, "/v1/sources", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	validator := NewAuthValidator(newTestConfig())
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := JWTMiddleware(validator)(next)

	req := httptest.NewRequest(http.MethodGet, "/v1/sources", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	validator := NewAuthValidator(newTestConfig())
	var receivedTenant string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant := r.Context().Value(ctxkeys.TenantIDKey)
		if tid, ok := tenant.(string); ok {
			receivedTenant = tid
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := JWTMiddleware(validator)(next)

	token := issueJWT(t, "test-secret-key-for-jwt-signing", "tenant-abc")
	req := httptest.NewRequest(http.MethodGet, "/v1/sources", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if receivedTenant != "tenant-abc" {
		t.Errorf("expected tenant tenant-abc, got %q", receivedTenant)
	}
}

func TestJWTMiddleware_MissingTenantID(t *testing.T) {
	validator := NewAuthValidator(newTestConfig())
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := JWTMiddleware(validator)(next)

	// Token without tenant_id claim.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-123",
		"roles": []any{"admin"},
		"iss":   "operan-auth",
	})
	signed, _ := token.SignedString([]byte("test-secret-key-for-jwt-signing"))

	req := httptest.NewRequest(http.MethodGet, "/v1/sources", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing tenant_id, got %d", w.Code)
	}
}

func TestJWTMiddleware_WrongIssuerPrefix(t *testing.T) {
	validator := NewAuthValidator(newTestConfig())
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := JWTMiddleware(validator)(next)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"tenant_id": "tenant-abc",
		"sub":       "user-123",
		"roles":     []any{"admin"},
		"iss":       "wrong-issuer-",
	})
	signed, _ := token.SignedString([]byte("test-secret-key-for-jwt-signing"))

	req := httptest.NewRequest(http.MethodGet, "/v1/sources", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong issuer, got %d", w.Code)
	}
}

// === Tenant Middleware Tests ===

func TestTenantMiddleware_MissingHeader(t *testing.T) {
	validator := NewAuthValidator(newTestConfig())
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := TenantMiddleware()(JWTMiddleware(validator)(next))

	token := issueJWT(t, "test-secret-key-for-jwt-signing", "tenant-abc")
	req := httptest.NewRequest(http.MethodGet, "/v1/sources", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	// Missing X-Tenant-ID header.
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing X-Tenant-ID, got %d", w.Code)
	}
}

func TestTenantMiddleware_TenantMismatch(t *testing.T) {
	validator := NewAuthValidator(newTestConfig())
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := JWTMiddleware(validator)(TenantMiddleware()(next))

	token := issueJWT(t, "test-secret-key-for-jwt-signing", "tenant-abc")
	req := httptest.NewRequest(http.MethodGet, "/v1/sources", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "tenant-xyz") // Mismatch.
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for tenant mismatch, got %d", w.Code)
	}
}

func TestTenantMiddleware_MatchingTenant(t *testing.T) {
	validator := NewAuthValidator(newTestConfig())
	var receivedTenant string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTenant = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusOK)
	})
	handler := JWTMiddleware(validator)(TenantMiddleware()(next))

	token := issueJWT(t, "test-secret-key-for-jwt-signing", "tenant-abc")
	req := httptest.NewRequest(http.MethodGet, "/v1/sources", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if receivedTenant != "tenant-abc" {
		t.Errorf("expected tenant tenant-abc, got %q", receivedTenant)
	}
}

// === RBAC Middleware Tests ===

func TestRBACMiddleware_NoRoles(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RBACMiddleware("admin")(next)

	req := httptest.NewRequest(http.MethodGet, "/v1/sources", nil)
	// No roles in context.
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for no roles, got %d", w.Code)
	}
}

func TestRBACMiddleware_InsufficientRoles(t *testing.T) {
	validator := NewAuthValidator(newTestConfig())
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RBACMiddleware("admin")(TenantMiddleware()(JWTMiddleware(validator)(next)))

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"tenant_id": "tenant-abc",
		"sub":       "user-123",
		"roles":     []any{"reader"},
		"iss":       "operan-auth",
	})
	signed, _ := token.SignedString([]byte("test-secret-key-for-jwt-signing"))

	req := httptest.NewRequest(http.MethodGet, "/v1/sources", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for insufficient roles, got %d", w.Code)
	}
}

func TestRBACMiddleware_ValidRole(t *testing.T) {
	validator := NewAuthValidator(newTestConfig())
	var receivedOK bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedOK = true
		w.WriteHeader(http.StatusOK)
	})
	handler := JWTMiddleware(validator)(RBACMiddleware("admin")(TenantMiddleware()(next)))

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"tenant_id": "tenant-abc",
		"sub":       "user-123",
		"roles":     []any{"admin", "reader"},
		"iss":       "operan-auth",
	})
	signed, _ := token.SignedString([]byte("test-secret-key-for-jwt-signing"))

	req := httptest.NewRequest(http.MethodGet, "/v1/sources", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !receivedOK {
		t.Error("expected handler to be called with valid role")
	}
}

// === WriteJSON Tests ===

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestWriteJSON_Error(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid request") {
		t.Errorf("expected error message in body, got %q", w.Body.String())
	}
}

// === Validate Tests ===

func TestAuthValidator_Validate_MissingAuth(t *testing.T) {
	v := NewAuthValidator(newTestConfig())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, _, _, err := v.Validate(req)
	if err == nil {
		t.Fatal("expected error for missing auth header")
	}
}

func TestAuthValidator_Validate_InvalidFormat(t *testing.T) {
	v := NewAuthValidator(newTestConfig())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "InvalidFormat token")
	_, _, _, err := v.Validate(req)
	if err == nil {
		t.Fatal("expected error for invalid auth format")
	}
}

func TestAuthValidator_Validate_InvalidHMAC(t *testing.T) {
	v := NewAuthValidator(newTestConfig())
	// Create a token with RS256 (not HMAC) to test signing method check.
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"tenant_id": "test",
	})
	signed, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	_, _, _, err := v.Validate(req)
	if err == nil {
		t.Fatal("expected error for non-HMAC signing method")
	}
}