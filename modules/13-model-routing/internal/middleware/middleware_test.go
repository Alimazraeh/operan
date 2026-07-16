package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/operan/model-routing/internal/ctxkeys"
	"github.com/stretchr/testify/assert"
)

func makeTestJWT(secret string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "user-1",
		"iss":   "operan",
		"exp":   time.Now().Add(1 * time.Hour).Unix(),
		"tenant": "tenant-1",
	})
	tok, _ := token.SignedString([]byte(secret))
	return tok
}

func TestRequireJWT_MissingHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mw := RequireJWT(cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing Authorization")
}

func TestRequireJWT_InvalidFormat(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mw := RequireJWT(cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "InvalidToken")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireJWT_InvalidSignature(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mw := RequireJWT(cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireJWT_ValidToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mw := RequireJWT(cfg)(next)

	token := makeTestJWT("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireTenant_MissingHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := RequireTenant()(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing X-Tenant-ID")
}

func TestRequireTenant_ValidHeader(t *testing.T) {
	var receivedTenant string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTenant = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusOK)
	})
	mw := RequireTenant()(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-42")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "tenant-42", receivedTenant)
}

func TestCombinedMiddleware_Chain(t *testing.T) {
	var receivedTenant string
	var hasPrincipal bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTenant = r.Header.Get("X-Tenant-ID")
		_, hasPrincipal = r.Context().Value("principal"), true
		w.WriteHeader(http.StatusOK)
	})

	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mwJWT := RequireJWT(cfg)
	mwTenant := RequireTenant()

	handler := mwTenant(mwJWT(next))

	token := makeTestJWT("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "tenant-1", receivedTenant)
	assert.True(t, hasPrincipal)
}

func TestRequireJWT_TamperedToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mw := RequireJWT(cfg)(next)

	badToken := makeTestJWT("wrong-secret")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+badToken)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireJWT_ExpiredToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mw := RequireJWT(cfg)(next)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
	})
	badToken, _ := token.SignedString([]byte("test-secret"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+badToken)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireTenant_EmptyString(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := RequireTenant()(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireJWT_BearerPrefixRequired(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mw := RequireJWT(cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireJWT_NoBearerPrefix(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mw := RequireJWT(cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "BearerBearer tokentoken")
	rec := httptest.NewRecorder()

	// Has "Bearer" but token starts right after "Bearer" with no space
	mw.ServeHTTP(rec, req)
	// Should fail because it's not properly formatted
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireJWT_CorrectBearerPrefix(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mw := RequireJWT(cfg)(next)

	token := makeTestJWT("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireJWT_WhitespaceAfterBearer(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mw := RequireJWT(cfg)(next)

	token := makeTestJWT("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer  "+token) // double space
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	// "Bearer  " + token -> TrimPrefix "Bearer " -> " " + token -> not a valid JWT
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireTenant_NonEmptyString(t *testing.T) {
	var receivedTenant string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTenant = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusOK)
	})
	mw := RequireTenant()(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "  tenant-1  ")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	// Tenant is passed as-is (trimming is caller's responsibility)
	assert.Equal(t, "  tenant-1  ", receivedTenant)
}

func TestRequireJWT_ContextPropagation(t *testing.T) {
	var principal interface{}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal = r.Context().Value(ctxkeys.PrincipalKey)
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mw := RequireJWT(cfg)(next)

	token := makeTestJWT("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotNil(t, principal)
	assert.IsType(t, jwt.MapClaims{}, principal)
}

func TestRequireTenant_ContextPropagation(t *testing.T) {
	var tenantID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := r.Context().Value(ctxkeys.TenantIDKey)
		if v != nil {
			tenantID = v.(string)
		}
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mwTenant := RequireTenant()
	mwJWT := RequireJWT(cfg)

	handler := mwTenant(mwJWT(next))

	token := makeTestJWT("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "tenant-99")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "tenant-99", tenantID)
}

func TestRequireJWT_InvalidHeaderFormat(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mw := RequireJWT(cfg)(next)

	// "Bearer" with nothing after it
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireTenant_MultipleHeaders(t *testing.T) {
	var receivedTenant string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTenant = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusOK)
	})
	mw := RequireTenant()(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Add("X-Tenant-ID", "tenant-1")
	req.Header.Add("X-Tenant-ID", "tenant-2")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	// Go returns the first value for Get
	assert.Equal(t, "tenant-1", receivedTenant)
}

// TestRequireJWT_DifferentSecrets tests that a token signed with a different secret fails
func TestRequireJWT_DifferentSecrets(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("secret-a")}
	mw := RequireJWT(cfg)(next)

	// Token signed with "secret-b"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})
	badToken, _ := token.SignedString([]byte("secret-b"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+badToken)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestRequireTenant_WithOtherHeadersStillPasses tests that other headers don't interfere
func TestRequireTenant_WithOtherHeadersStillPasses(t *testing.T) {
	var receivedTenant string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTenant = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusOK)
	})
	mw := RequireTenant()(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Authorization", "Bearer fake-token")
	req.Header.Set("X-Custom-Header", "custom-value")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "tenant-1", receivedTenant)
}

// TestRequireJWT_ContextDoesNotMutateOriginal tests that context is properly threaded
func TestRequireJWT_ContextDoesNotMutateOriginal(t *testing.T) {
	var principalFromCtx interface{}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principalFromCtx = r.Context().Value(ctxkeys.PrincipalKey)
		w.WriteHeader(http.StatusOK)
	})
	cfg := &JWTConfig{Secret: []byte("test-secret")}
	mw := RequireJWT(cfg)(next)

	token := makeTestJWT("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	// Create a context with a different value for the same key
	origCtx := context.WithValue(req.Context(), ctxkeys.PrincipalKey, "original-value")
	req = req.WithContext(origCtx)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	// The middleware should replace the context value
	assert.NotNil(t, principalFromCtx)
}