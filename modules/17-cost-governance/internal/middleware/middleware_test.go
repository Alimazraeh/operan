package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/operan/cost-governance/internal/config"
	"github.com/operan/cost-governance/internal/ctxkeys"

	"github.com/golang-jwt/jwt/v5"
)

func createTestToken(t *testing.T, secret string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"tenant_id": "test-tenant",
		"sub":        "user-123",
		"roles":      []any{"admin"},
		"iss":        "operan-auth",
		"exp":        time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenStr
}

func TestAuthValidator_Validate(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	tokenStr := createTestToken(t, "test-secret")

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	tenantID, userID, roles, err := v.Validate(req)
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}
	if tenantID != "test-tenant" {
		t.Errorf("expected tenant_id test-tenant, got %s", tenantID)
	}
	if userID != "user-123" {
		t.Errorf("expected user_id user-123, got %s", userID)
	}
	if len(roles) != 1 || roles[0] != "admin" {
		t.Errorf("expected [admin], got %v", roles)
	}
}

func TestAuthValidator_MissingAuthorization(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	req := httptest.NewRequest("GET", "/", nil)
	_, _, _, err := v.Validate(req)
	if err == nil {
		t.Fatal("expected error for missing Authorization header")
	}
}

func TestAuthValidator_InvalidToken(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	_, _, _, err := v.Validate(req)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestAuthValidator_WrongSecret(t *testing.T) {
	cfg := &config.Config{JWTSecret: "correct-secret"}
	v := NewAuthValidator(cfg)

	// Token signed with different secret
	tokenStr := createTestToken(t, "wrong-secret")

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	_, _, _, err := v.Validate(req)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestAuthValidator_MissingTenantID(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte("test-secret"))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	_, _, _, err := v.Validate(req)
	if err == nil {
		t.Fatal("expected error for missing tenant_id")
	}
}

func TestAuthValidator_BearerFormat(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	_, _, _, err := v.Validate(req)
	if err == nil {
		t.Fatal("expected error for non-Bearer format")
	}
}

func TestJWTMiddleware(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)
		if tenantID != "test-tenant" {
			t.Errorf("expected tenant_id test-tenant, got %s", tenantID)
		}
	})

	handler := JWTMiddleware(v)(nextHandler)

	tokenStr := createTestToken(t, "test-secret")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("expected next handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestJWTMiddleware_Unauthorized(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := JWTMiddleware(v)(nextHandler)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if nextCalled {
		t.Error("next handler should not be called on unauthorized")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestTenantMiddleware_Match(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := JWTMiddleware(v)(TenantMiddleware()(nextHandler))

	tokenStr := createTestToken(t, "test-secret")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("expected next handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestTenantMiddleware_Mismatch(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := JWTMiddleware(v)(TenantMiddleware()(nextHandler))

	tokenStr := createTestToken(t, "test-secret")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req.Header.Set("X-Tenant-ID", "other-tenant")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if nextCalled {
		t.Error("next handler should not be called on tenant mismatch")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestTenantMiddleware_MissingHeader(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := JWTMiddleware(v)(TenantMiddleware()(nextHandler))

	tokenStr := createTestToken(t, "test-secret")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if nextCalled {
		t.Error("next handler should not be called on missing X-Tenant-ID")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestQuote(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"hello", `"hello"`},
		{`say "hi"`, `"say \"hi\""`},
		{"line1\nline2", `"line1\nline2"`},
	}
	for _, tt := range tests {
		result := quote(tt.input)
		if result != tt.expect {
			t.Errorf("quote(%q) = %q, want %q", tt.input, result, tt.expect)
		}
	}
}