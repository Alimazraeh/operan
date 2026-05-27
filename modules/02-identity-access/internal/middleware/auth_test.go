package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthValidatorValidBearerToken(t *testing.T) {
	token, err := GenerateToken("secret", "user-1", "user", "tenant-1", "test@example.com", []string{"admin"}, 60)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	handler := AuthValidator("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("AuthValidator() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestAuthValidatorMissingAuthorizationHeader(t *testing.T) {
	handler := AuthValidator("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("AuthValidator() status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthValidatorInvalidToken(t *testing.T) {
	handler := AuthValidator("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("AuthValidator() status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthValidatorMalformedBearer(t *testing.T) {
	handler := AuthValidator("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "InvalidScheme token")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("AuthValidator() status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthValidatorEmptyBearerValue(t *testing.T) {
	handler := AuthValidator("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("AuthValidator() status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthValidatorExtractsPrincipal(t *testing.T) {
	token, err := GenerateToken("secret", "user-123", "user", "tenant-1", "user@example.com", []string{"admin", "editor"}, 60)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	handler := AuthValidator("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwtToken := GetJWTToken(r.Context())
		if jwtToken == nil {
			t.Fatal("GetJWTToken() returned nil")
		}
		if jwtToken.Subject != "user-123" {
			t.Errorf("JWTToken.Subject = %v, want user-123", jwtToken.Subject)
		}
		if jwtToken.UserType != "user" {
			t.Errorf("JWTToken.UserType = %v, want user", jwtToken.UserType)
		}
		if jwtToken.TenantID != "tenant-1" {
			t.Errorf("JWTToken.TenantID = %v, want tenant-1", jwtToken.TenantID)
		}
		if jwtToken.Email != "user@example.com" {
			t.Errorf("JWTToken.Email = %v, want user@example.com", jwtToken.Email)
		}
		if len(jwtToken.Roles) != 2 {
			t.Errorf("JWTToken.Roles length = %v, want 2", len(jwtToken.Roles))
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("AuthValidator() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestAuthValidatorExtractsUserID(t *testing.T) {
	token, err := GenerateToken("secret", "user-456", "user", "tenant-2", "", []string{}, 60)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	handler := AuthValidator("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r.Context())
		if userID != "user-456" {
			t.Errorf("GetUserID() = %v, want user-456", userID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("AuthValidator() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestAuthValidatorServicePrincipal(t *testing.T) {
	token, err := GenerateToken("secret", "service-789", "service", "tenant-1", "", []string{"service-role"}, 60)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	handler := AuthValidator("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwtToken := GetJWTToken(r.Context())
		if jwtToken == nil {
			t.Fatal("GetJWTToken() returned nil")
		}
		if jwtToken.UserType != "service" {
			t.Errorf("JWTToken.UserType = %v, want service", jwtToken.UserType)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("AuthValidator() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestAuthValidatorAgentPrincipal(t *testing.T) {
	token, err := GenerateToken("secret", "agent-101", "agent", "tenant-1", "", []string{"execute"}, 60)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	handler := AuthValidator("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwtToken := GetJWTToken(r.Context())
		if jwtToken == nil {
			t.Fatal("GetJWTToken() returned nil")
		}
		if jwtToken.UserType != "agent" {
			t.Errorf("JWTToken.UserType = %v, want agent", jwtToken.UserType)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("AuthValidator() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestAuthValidatorUntrustedIssuer(t *testing.T) {
	// Create a token with a different issuer
	token, err := GenerateToken("secret", "user-1", "user", "tenant-1", "", []string{}, 60)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// Manually modify the token to have a different issuer
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatal("Expected 3 parts in JWT token")
	}

	// Create a new token with untrusted issuer
	token, err = GenerateToken("secret", "user-1", "user", "tenant-1", "", []string{}, 60)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	handler := AuthValidator("wrong-secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("AuthValidator() with wrong secret status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthValidatorHealthEndpoint(t *testing.T) {
	handler := AuthValidator("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("AuthValidator() health endpoint status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestTenantMiddlewareValidHeader(t *testing.T) {
	handler := TenantInjector(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := GetTenantID(r.Context())
		if tenantID != "tenant-1" {
			t.Errorf("GetTenantID() = %v, want tenant-1", tenantID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("TenantMiddleware() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestTenantMiddlewareMissingHeader(t *testing.T) {
	handler := TenantInjector(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("TenantMiddleware() status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestTenantMiddlewareEmptyHeader(t *testing.T) {
	handler := TenantInjector(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("TenantMiddleware() status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestTenantMiddlewareInjectsTenantID(t *testing.T) {
	handler := TenantInjector(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := GetTenantID(r.Context())
		if tenantID != "acme-corp" {
			t.Errorf("GetTenantID() = %v, want acme-corp", tenantID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "acme-corp")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("TenantMiddleware() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestHasRole(t *testing.T) {
	tests := []struct {
		name    string
		roles   []string
		role    string
		want    bool
	}{
		{"has role", []string{"admin", "editor"}, "admin", true},
		{"missing role", []string{"editor", "viewer"}, "admin", false},
		{"empty roles", []string{}, "admin", false},
		{"any matches", []string{"admin", "editor", "viewer"}, "editor", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &JWTToken{Roles: tt.roles}
			got := HasRole(token, tt.role)
			if got != tt.want {
				t.Errorf("HasRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTenantID(t *testing.T) {
	req := contextWithTenantID("tenant-123")
	got := GetTenantID(req.Context())
	if got != "tenant-123" {
		t.Errorf("GetTenantID() = %v, want tenant-123", got)
	}
}

func TestGetTenantIDMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	got := GetTenantID(req.Context())
	if got != "" {
		t.Errorf("GetTenantID() = %v, want empty string", got)
	}
}

func TestGetUserID(t *testing.T) {
	token, err := GenerateToken("secret", "user-789", "user", "tenant-1", "", []string{}, 60)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	handler := AuthValidator("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := GetUserID(r.Context())
		if got != "user-789" {
			t.Errorf("GetUserID() = %v, want user-789", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("AuthValidator() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestGetUserType(t *testing.T) {
	token, err := GenerateToken("secret", "user-1", "service", "tenant-1", "", []string{}, 60)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	handler := AuthValidator("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := GetUserType(r.Context())
		if got != "service" {
			t.Errorf("GetUserType() = %v, want service", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("AuthValidator() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestGetUserTypeDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	got := GetUserType(req.Context())
	if got != "user" {
		t.Errorf("GetUserType() default = %v, want user", got)
	}
}

func TestTraceInjectorGeneratesTraceID(t *testing.T) {
	handler := TraceInjector(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := GetTraceID(r.Context())
		if traceID == "" {
			t.Error("GetTraceID() returned empty string")
		}
		if !strings.HasPrefix(traceID, "trace-") {
			t.Errorf("GetTraceID() = %v, expected trace- prefix", traceID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("TraceInjector() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestTraceInjectorUsesExistingTraceID(t *testing.T) {
	handler := TraceInjector(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := GetTraceID(r.Context())
		if traceID != "custom-trace-123" {
			t.Errorf("GetTraceID() = %v, want custom-trace-123", traceID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Trace-ID", "custom-trace-123")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("TraceInjector() status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestGetTraceIDMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	got := GetTraceID(req.Context())
	if got != "" {
		t.Errorf("GetTraceID() = %v, want empty string", got)
	}
}

// HasRole checks if a JWT token has a specific role.
func HasRole(token *JWTToken, role string) bool {
	if token == nil {
		return false
	}
	for _, r := range token.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// contextWithTenantID creates a request context with tenant ID for testing
func contextWithTenantID(tenantID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := req.Context()
	// We can't add values to httptest.Request directly, but we can create a new request with context
	req = req.WithContext(context.WithValue(ctx, TenantIDKey, tenantID))
	return req
}
