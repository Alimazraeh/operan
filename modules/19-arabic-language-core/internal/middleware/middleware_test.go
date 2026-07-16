package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/operan/arabic-language-core/internal/config"
	"github.com/operan/arabic-language-core/internal/ctxkeys"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewAuthValidator(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)
	if v == nil {
		t.Fatal("expected non-nil auth validator")
	}
	if v.secret != "test-secret" {
		t.Errorf("expected secret='test-secret', got '%s'", v.secret)
	}
}

func TestAuthValidator_MissingAuthHeader(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	req := httptest.NewRequest("GET", "/health", nil)
	_, _, _, err := v.Validate(req)
	if err == nil {
		t.Error("expected error for missing Authorization header")
	}
}

func TestAuthValidator_InvalidAuthScheme(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Authorization", "Basic dGVzdDp0ZXN0")
	_, _, _, err := v.Validate(req)
	if err == nil {
		t.Error("expected error for non-Bearer auth scheme")
	}
}

func TestAuthValidator_InvalidTokenFormat(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-token")
	_, _, _, err := v.Validate(req)
	if err == nil {
		t.Error("expected error for invalid JWT token")
	}
}

func TestAuthValidator_ValidToken(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	token, err := makeTestJWT("test-secret", "operan-test", "tenant-1", "user-1", "admin")
	if err != nil {
		t.Fatalf("failed to create test JWT: %v", err)
	}

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	tenantID, userID, roles, err := v.Validate(req)
	if err != nil {
		t.Fatalf("expected no error for valid JWT: %v", err)
	}
	if tenantID != "tenant-1" {
		t.Errorf("expected tenant_id='tenant-1', got '%s'", tenantID)
	}
	if userID != "user-1" {
		t.Errorf("expected user_id='user-1', got '%s'", userID)
	}
	if len(roles) != 1 || roles[0] != "admin" {
		t.Errorf("expected roles=[admin], got %v", roles)
	}
}

func TestAuthValidator_WrongIssuer(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	// Create token with non-operan issuer
	token, err := makeTestJWTWithIssuer("test-secret", "wrong-issuer", "tenant-1", "user-1", []string{"admin"})
	if err != nil {
		t.Fatalf("failed to create test JWT: %v", err)
	}

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	_, _, _, err = v.Validate(req)
	if err == nil {
		t.Error("expected error for wrong JWT issuer")
	}
}

func TestJWTMiddleware(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	token, err := makeTestJWT("test-secret", "operan-test", "tenant-1", "user-1")
	if err != nil {
		t.Fatalf("failed to create test JWT: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		tid := ctx.Value(ctxkeys.TenantIDKey)
		if tid == nil || tid.(string) != "tenant-1" {
			t.Errorf("expected tenant_id=tenant-1 in context, got %v", tid)
		}
		uid := ctx.Value(ctxkeys.UserIDKey)
		if uid == nil || uid.(string) != "user-1" {
			t.Errorf("expected user_id=user-1 in context, got %v", uid)
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := JWTMiddleware(v)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	middleware(handler).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTenantMiddleware_MissingHeader(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)
	jwtMW := JWTMiddleware(v)

	token, err := makeTestJWT("test-secret", "operan-test", "tenant-1", "user-1")
	if err != nil {
		t.Fatalf("failed to create test JWT: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := TenantMiddleware()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	// JWT middleware runs FIRST (outer), then tenant middleware (inner)
	combined := jwtMW(middleware(handler))

	w := httptest.NewRecorder()
	combined.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing X-Tenant-ID header, got %d", w.Code)
	}
}

func TestTenantMiddleware_TenantMismatch(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)
	jwtMW := JWTMiddleware(v)

	token, err := makeTestJWT("test-secret", "operan-test", "tenant-1", "user-1")
	if err != nil {
		t.Fatalf("failed to create test JWT: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := TenantMiddleware()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "tenant-2")
	combined := jwtMW(middleware(handler))

	w := httptest.NewRecorder()
	combined.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for tenant mismatch, got %d", w.Code)
	}
}

func TestTenantMiddleware_MatchingTenant(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)
	jwtMW := JWTMiddleware(v)

	token, err := makeTestJWT("test-secret", "operan-test", "tenant-1", "user-1")
	if err != nil {
		t.Fatalf("failed to create test JWT: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := TenantMiddleware()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	combined := jwtMW(middleware(handler))

	w := httptest.NewRecorder()
	combined.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for matching tenant, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRBACMiddleware_WithRole(t *testing.T) {
	token, err := makeTestJWT("test-secret", "operan-test", "tenant-1", "user-1", "admin")
	if err != nil {
		t.Fatalf("failed to create test JWT: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)
	jwtMW := JWTMiddleware(v)
	chain := jwtMW(RBACMiddleware("admin")(handler))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRBACMiddleware_MissingRole(t *testing.T) {
	token, err := makeTestJWT("test-secret", "operan-test", "tenant-1", "user-1", "viewer")
	if err != nil {
		t.Fatalf("failed to create test JWT: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)
	jwtMW := JWTMiddleware(v)
	chain := jwtMW(RBACMiddleware("admin")(handler))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for missing role, got %d", w.Code)
	}
}

func TestValidate_MissingTenantClaim(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	v := NewAuthValidator(cfg)

	// Create token without tenant_id
	token, err := makeTestJWTNoTenant("test-secret", "operan-test")
	if err != nil {
		t.Fatalf("failed to create test JWT: %v", err)
	}

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	_, _, _, err = v.Validate(req)
	if err == nil {
		t.Error("expected error for JWT missing tenant_id")
	}
}

// makeTestJWT creates a valid JWT for testing.
func makeTestJWT(secret, issuer, tenantID, userID string, roles ...string) (string, error) {
	claims := jwt.MapClaims{
		"sub":      userID,
		"iss":      issuer,
		"tenant_id": tenantID,
		"exp":      time.Now().Add(time.Hour).Unix(),
	}
	if len(roles) > 0 {
		// Convert []string to []any for proper JWT claim serialization
		roleAny := make([]any, len(roles))
		for i, r := range roles {
			roleAny[i] = r
		}
		claims["roles"] = roleAny
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// makeTestJWTWithIssuer creates a JWT with a custom issuer.
func makeTestJWTWithIssuer(secret, issuer, tenantID, userID string, roles []string) (string, error) {
	return makeTestJWT(secret, issuer, tenantID, userID, roles...)
}

// makeTestJWTNoTenant creates a JWT without tenant_id.
func makeTestJWTNoTenant(secret, issuer string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   "user-1",
		"iss":   issuer,
		"exp":   time.Now().Add(time.Hour).Unix(),
		"roles": []any{"admin"},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}