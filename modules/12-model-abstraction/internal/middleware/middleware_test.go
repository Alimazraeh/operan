package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/operan/model-abstraction/internal/config"
	"github.com/operan/model-abstraction/internal/ctxkeys"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-jwt-secret-key-for-testing-1234567890"

func newTestValidator(t *testing.T) *AuthValidator {
	cfg := &config.Config{JWTSecret: testSecret}
	return NewAuthValidator(cfg)
}

func makeTestJWT(t *testing.T, claims jwt.MapClaims) string {
	claims["iss"] = "operan-platform"
	claims["tenant_id"] = "tenant-001"
	claims["sub"] = "user-123"
	claims["roles"] = []any{"model_admin", "user"}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}
	return signed
}

func TestAuthValidator_MissingHeader(t *testing.T) {
	v := newTestValidator(t)
	_, _, _, err := v.Validate(&http.Request{})
	if err == nil {
		t.Fatal("expected error for missing Authorization header")
	}
}

func TestAuthValidator_InvalidFormat(t *testing.T) {
	v := newTestValidator(t)
	req := &http.Request{Header: http.Header{}}
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	_, _, _, err := v.Validate(req)
	if err == nil {
		t.Fatal("expected error for non-Bearer auth format")
	}
}

func TestAuthValidator_ValidJWT(t *testing.T) {
	v := newTestValidator(t)
	tokenStr := makeTestJWT(t, jwt.MapClaims{})
	req := &http.Request{Header: http.Header{}}
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	tenantID, userID, roles, err := v.Validate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tenantID != "tenant-001" {
		t.Errorf("expected tenant_id 'tenant-001', got %q", tenantID)
	}
	if userID != "user-123" {
		t.Errorf("expected user_id 'user-123', got %q", userID)
	}
	if len(roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(roles))
	}
}

func TestAuthValidator_ExpiredJWT(t *testing.T) {
	v := newTestValidator(t)

	// Manually create a token that's definitely expired.
	claims := jwt.MapClaims{
		"exp":       time.Now().Add(-time.Hour).Unix(),
		"tenant_id": "tenant-001",
		"sub":       "user-123",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}

	req := &http.Request{Header: http.Header{}}
	req.Header.Set("Authorization", "Bearer "+signed)

	_, _, _, err = v.Validate(req)
	if err == nil {
		t.Fatal("expected error for expired JWT")
	}
}

func TestAuthValidator_WrongIssuer(t *testing.T) {
	v := newTestValidator(t)
	claims := jwt.MapClaims{
		"iss":       "wrong-issuer",
		"tenant_id": "tenant-001",
		"sub":       "user-123",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}

	req := &http.Request{Header: http.Header{}}
	req.Header.Set("Authorization", "Bearer "+signed)

	_, _, _, err = v.Validate(req)
	if err == nil {
		t.Fatal("expected error for wrong issuer")
	}
}

func TestAuthValidator_InvalidSignature(t *testing.T) {
	v := newTestValidator(t)

	// Sign with a different secret.
	claims := jwt.MapClaims{
		"tenant_id": "tenant-001",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("wrong-secret"))
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}

	req := &http.Request{Header: http.Header{}}
	req.Header.Set("Authorization", "Bearer "+signed)

	_, _, _, err = v.Validate(req)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestAuthValidator_MissingTenantID(t *testing.T) {
	v := newTestValidator(t)
	claims := jwt.MapClaims{
		"sub": "user-123",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}

	req := &http.Request{Header: http.Header{}}
	req.Header.Set("Authorization", "Bearer "+signed)

	_, _, _, err = v.Validate(req)
	if err == nil {
		t.Fatal("expected error for missing tenant_id")
	}
}

func TestJWTMiddleware_HandlerCalled(t *testing.T) {
	v := newTestValidator(t)
	mw := JWTMiddleware(v)

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	tokenStr := makeTestJWT(t, jwt.MapClaims{})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()

	mw(handler).ServeHTTP(w, req)
	if !called {
		t.Fatal("handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestJWTMiddleware_MissingToken(t *testing.T) {
	v := newTestValidator(t)
	mw := JWTMiddleware(v)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without valid token")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	mw(handler).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestTenantMiddleware_Mismatch(t *testing.T) {
	v := newTestValidator(t)
	mw := JWTMiddleware(v)
	tenantMW := TenantMiddleware()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with tenant mismatch")
	})

	tokenStr := makeTestJWT(t, jwt.MapClaims{})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req.Header.Set("X-Tenant-ID", "wrong-tenant")
	w := httptest.NewRecorder()

	// JWT middleware runs first, then tenant middleware.
	mw(tenantMW(handler)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestTenantMiddleware_MissingHeader(t *testing.T) {
	v := newTestValidator(t)
	mw := JWTMiddleware(v)
	tenantMW := TenantMiddleware()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without X-Tenant-ID")
	})

	tokenStr := makeTestJWT(t, jwt.MapClaims{})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()

	// JWT middleware runs first, then tenant middleware.
	mw(tenantMW(handler)).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRBACMiddleware_HasPermission(t *testing.T) {
	mw := RBACMiddleware("model_admin")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx := context.WithValue(context.Background(), ctxkeys.RolesKey, []string{"model_admin", "user"})
	req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	mw(handler).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRBACMiddleware_NoPermission(t *testing.T) {
	mw := RBACMiddleware("model_admin")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without permission")
	})

	ctx := context.WithValue(context.Background(), ctxkeys.RolesKey, []string{"user", "viewer"})
	req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	mw(handler).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRBACMiddleware_NoRoles(t *testing.T) {
	mw := RBACMiddleware("model_admin")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without roles")
	})

	ctx := context.WithValue(context.Background(), ctxkeys.RolesKey, []string{})
	req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	mw(handler).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}