package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operan/policy-governance/internal/ctxkeys"
)

const testSecret = "test-secret-key-for-jwt-validation"
const testIssuer = "operan-tenant-control-plane"

func makeTestJWT(t *testing.T, tenantID string, opts ...func(*jwt.MapClaims)) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":   tenantID,
		"iss":   testIssuer,
		"roles": []interface{}{"admin", "editor"},
		"exp":   jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
	}
	for _, opt := range opts {
		opt(&claims)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return s
}

func TestAuthValidator_Validate_Success(t *testing.T) {
	v := NewAuthValidator(testSecret, testIssuer)
	tokenStr := makeTestJWT(t, "tenant-1")

	tenantID, userID, roles, err := v.Validate(&http.Request{
		Header: map[string][]string{"Authorization": {"Bearer " + tokenStr}},
	})
	require.NoError(t, err)
	assert.Equal(t, "tenant-1", tenantID)
	assert.Equal(t, "tenant-1", userID)
	assert.Equal(t, []string{"admin", "editor"}, roles)
}

func TestAuthValidator_Validate_MissingHeader(t *testing.T) {
	v := NewAuthValidator(testSecret, testIssuer)
	_, _, _, err := v.Validate(&http.Request{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing Authorization")
}

func TestAuthValidator_Validate_InvalidSignature(t *testing.T) {
	tokenStr := makeTestJWT(t, "tenant-1")
	v := NewAuthValidator("wrong-secret", testIssuer)
	_, _, _, err := v.Validate(&http.Request{
		Header: map[string][]string{"Authorization": {"Bearer " + tokenStr}},
	})
	require.Error(t, err)
}

func TestAuthValidator_Validate_ExpiredToken(t *testing.T) {
	tokenStr := makeTestJWT(t, "tenant-1", func(claims *jwt.MapClaims) {
		(*claims)["exp"] = jwt.NewNumericDate(time.Now().Add(-1 * time.Hour))
	})
	v := NewAuthValidator(testSecret, testIssuer)
	_, _, _, err := v.Validate(&http.Request{
		Header: map[string][]string{"Authorization": {"Bearer " + tokenStr}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestAuthValidator_Validate_WrongIssuer(t *testing.T) {
	tokenStr := makeTestJWT(t, "tenant-1")
	v := NewAuthValidator(testSecret, "wrong-issuer")
	_, _, _, err := v.Validate(&http.Request{
		Header: map[string][]string{"Authorization": {"Bearer " + tokenStr}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issuer")
}

func TestAuthValidator_Validate_MissingAuthHeader(t *testing.T) {
	v := NewAuthValidator(testSecret, testIssuer)
	_, _, _, err := v.Validate(&http.Request{
		Header: map[string][]string{"Authorization": {}},
	})
	require.Error(t, err)
}

func TestAuthValidator_Validate_BadScheme(t *testing.T) {
	v := NewAuthValidator(testSecret, testIssuer)
	_, _, _, err := v.Validate(&http.Request{
		Header: map[string][]string{"Authorization": {"Basic dXNlcjpwYXNz"}},
	})
	require.Error(t, err)
}

func TestAuthValidator_Validate_InvalidTokenFormat(t *testing.T) {
	v := NewAuthValidator(testSecret, testIssuer)
	_, _, _, err := v.Validate(&http.Request{
		Header: map[string][]string{"Authorization": {"Bearer invalid.token.here"}},
	})
	require.Error(t, err)
}

func TestAuthValidator_Validate_MissingTenant(t *testing.T) {
	tokenStr := makeTestJWT(t, "tenant-1", func(claims *jwt.MapClaims) {
		delete(*claims, "sub")
	})
	v := NewAuthValidator(testSecret, testIssuer)
	_, _, _, err := v.Validate(&http.Request{
		Header: map[string][]string{"Authorization": {"Bearer " + tokenStr}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing tenant")
}

func TestAuthValidator_Validate_EmptyRoles(t *testing.T) {
	tokenStr := makeTestJWT(t, "tenant-1", func(claims *jwt.MapClaims) {
		(*claims)["roles"] = []interface{}{}
	})
	v := NewAuthValidator(testSecret, testIssuer)
	tenantID, _, roles, err := v.Validate(&http.Request{
		Header: map[string][]string{"Authorization": {"Bearer " + tokenStr}},
	})
	require.NoError(t, err)
	assert.Equal(t, "tenant-1", tenantID)
	assert.Equal(t, []string{}, roles)
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	v := NewAuthValidator(testSecret, testIssuer)
	handler := JWTMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := ctxkeys.GetTenantID(r.Context())
		assert.Equal(t, "tenant-1", tenantID)
		w.WriteHeader(http.StatusOK)
	}))

	tokenStr := makeTestJWT(t, "tenant-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	v := NewAuthValidator(testSecret, testIssuer)
	handler := JWTMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTMiddleware_ContextPropagation(t *testing.T) {
	v := NewAuthValidator(testSecret, testIssuer)
	handler := JWTMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := ctxkeys.GetTenantID(r.Context())
		userID := ctxkeys.GetUserID(r.Context())
		roles := ctxkeys.GetRoles(r.Context())

		assert.Equal(t, "tenant-1", tenantID)
		assert.Equal(t, "tenant-1", userID)
		assert.Equal(t, []string{"admin", "editor"}, roles)
		w.WriteHeader(http.StatusOK)
	}))

	tokenStr := makeTestJWT(t, "tenant-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTenantMiddleware_Match(t *testing.T) {
	handler := TenantMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tokenStr := makeTestJWT(t, "tenant-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	w := httptest.NewRecorder()

	// Need auth middleware first
	auth := NewAuthValidator(testSecret, testIssuer)
	authHandler := JWTMiddleware(auth)(handler)
	authHandler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTenantMiddleware_Mismatch(t *testing.T) {
	handler := TenantMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tokenStr := makeTestJWT(t, "tenant-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req.Header.Set("X-Tenant-ID", "tenant-2")
	w := httptest.NewRecorder()

	auth := NewAuthValidator(testSecret, testIssuer)
	authHandler := JWTMiddleware(auth)(handler)
	authHandler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTenantMiddleware_MissingHeader(t *testing.T) {
	handler := TenantMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tokenStr := makeTestJWT(t, "tenant-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()

	auth := NewAuthValidator(testSecret, testIssuer)
	authHandler := JWTMiddleware(auth)(handler)
	authHandler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}