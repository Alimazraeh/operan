package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func createTestToken(secret, tenantID, userID string, exp time.Time) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  tenantID,
		"user": userID,
		"roles": []interface{}{"admin", "user"},
		"iss":   "operan-auth",
		"exp":   exp.Unix(),
		"iat":   time.Now().Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(secret))
	return tokenStr
}

func TestAuthValidator_Validate_ValidToken(t *testing.T) {
	validator := NewAuthValidator("secret", "operan-auth")

	req, _ := http.NewRequest("GET", "/channels", nil)
	req.Header.Set("Authorization", "Bearer "+createTestToken("secret", "tenant-1", "agent-1", time.Now().Add(24*time.Hour)))

	tenantID, userID, roles, err := validator.Validate(req)
	assert.NoError(t, err)
	assert.Equal(t, "tenant-1", tenantID)
	assert.Equal(t, "tenant-1", userID) // middleware uses sub for both
	assert.Len(t, roles, 2)
}

func TestAuthValidator_Validate_InvalidSignature(t *testing.T) {
	validator := NewAuthValidator("secret", "operan-auth")

	req, _ := http.NewRequest("GET", "/channels", nil)
	req.Header.Set("Authorization", "Bearer "+createTestToken("wrong-secret", "tenant-1", "agent-1", time.Now().Add(24*time.Hour)))

	_, _, _, err := validator.Validate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired JWT token")
}

func TestAuthValidator_Validate_ExpiredToken(t *testing.T) {
	validator := NewAuthValidator("secret", "operan-auth")

	req, _ := http.NewRequest("GET", "/channels", nil)
	req.Header.Set("Authorization", "Bearer "+createTestToken("secret", "tenant-1", "agent-1", time.Now().Add(-24*time.Hour)))

	_, _, _, err := validator.Validate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired JWT token")
}

func TestAuthValidator_Validate_WrongIssuer(t *testing.T) {
	validator := NewAuthValidator("secret", "operan-auth")

	req, _ := http.NewRequest("GET", "/channels", nil)
	req.Header.Set("Authorization", "Bearer "+createTestToken("secret", "tenant-1", "agent-1", time.Now().Add(24*time.Hour)))
	// Override issuer in token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "tenant-1",
		"iss": "evil-issuer",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte("secret"))
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	_, _, _, err := validator.Validate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "issuer mismatch")
}

func TestAuthValidator_Validate_MissingAuthHeader(t *testing.T) {
	validator := NewAuthValidator("secret", "operan-auth")

	req, _ := http.NewRequest("GET", "/channels", nil)

	_, _, _, err := validator.Validate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing Authorization header")
}

func TestAuthValidator_Validate_InvalidAuthScheme(t *testing.T) {
	validator := NewAuthValidator("secret", "operan-auth")

	req, _ := http.NewRequest("GET", "/channels", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	_, _, _, err := validator.Validate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Authorization format")
}

func TestAuthValidator_Validate_InvalidTokenFormat(t *testing.T) {
	validator := NewAuthValidator("secret", "operan-auth")

	req, _ := http.NewRequest("GET", "/channels", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")

	_, _, _, err := validator.Validate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired JWT token")
}

func TestAuthValidator_Validate_MissingTenant(t *testing.T) {
	validator := NewAuthValidator("secret", "operan-auth")

	req, _ := http.NewRequest("GET", "/channels", nil)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user": "agent-1",
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte("secret"))
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	_, _, _, err := validator.Validate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing tenant in JWT")
}

func TestAuthValidator_Validate_MissingRoles(t *testing.T) {
	validator := NewAuthValidator("secret", "operan-auth")

	req, _ := http.NewRequest("GET", "/channels", nil)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "tenant-1",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iss": "operan-auth",
	})
	tokenStr, _ := token.SignedString([]byte("secret"))
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	tenantID, _, roles, err := validator.Validate(req)
	assert.NoError(t, err)
	assert.Equal(t, "tenant-1", tenantID)
	assert.Empty(t, roles)
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	validator := NewAuthValidator("secret", "operan-auth")
	jwtMiddleware := JWTMiddleware(validator)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/channels", nil)
	req.Header.Set("Authorization", "Bearer "+createTestToken("secret", "tenant-1", "agent-1", time.Now().Add(24*time.Hour)))

	rec := httptest.NewRecorder()
	jwtMiddleware(nextHandler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	validator := NewAuthValidator("secret", "operan-auth")
	jwtMiddleware := JWTMiddleware(validator)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/channels", nil)

	rec := httptest.NewRecorder()
	jwtMiddleware(nextHandler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "unauthorized")
}

func TestJWTMiddleware_TenantContext(t *testing.T) {
	validator := NewAuthValidator("secret", "operan-auth")
	jwtMiddleware := JWTMiddleware(validator)

	var ctx context.Context
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/channels", nil)
	req.Header.Set("Authorization", "Bearer "+createTestToken("secret", "tenant-1", "agent-1", time.Now().Add(24*time.Hour)))

	rec := httptest.NewRecorder()
	jwtMiddleware(nextHandler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotNil(t, ctx)
}