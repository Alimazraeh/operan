package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"time"
)

const testJWTSecret = "test-secret-key-for-jwt-validation"
const testIssuer = "operan-tenant-control-plane"

func makeTestJWT(tenantID string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   tenantID,
		"iss":   testIssuer,
		"roles": []interface{}{"admin"},
		"exp":   jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	s, _ := token.SignedString([]byte(testJWTSecret))
	return s
}

func TestValidate_ValidToken(t *testing.T) {
	av := NewAuthValidator(testJWTSecret, testIssuer)
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))

	tenantID, userID, roles, err := av.Validate(r)
	require.NoError(t, err)
	require.Equal(t, "tenant-1", tenantID)
	require.Equal(t, "tenant-1", userID)
	require.Len(t, roles, 1)
	require.Equal(t, "admin", roles[0])
}

func TestValidate_MissingAuthHeader(t *testing.T) {
	av := NewAuthValidator(testJWTSecret, testIssuer)
	r := httptest.NewRequest("GET", "/test", nil)

	_, _, _, err := av.Validate(r)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing Authorization")
}

func TestValidate_WrongIssuer(t *testing.T) {
	av := NewAuthValidator(testJWTSecret, "different-issuer")
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))

	_, _, _, err := av.Validate(r)
	require.Error(t, err)
	require.Contains(t, err.Error(), "issuer mismatch")
}

func TestValidate_BadFormat(t *testing.T) {
	av := NewAuthValidator(testJWTSecret, testIssuer)
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "WrongFormat token")

	_, _, _, err := av.Validate(r)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid Authorization format")
}

func TestJWTMiddleware_Success(t *testing.T) {
	av := NewAuthValidator(testJWTSecret, testIssuer)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := JWTMiddleware(av)(next)
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	av := NewAuthValidator(testJWTSecret, testIssuer)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := JWTMiddleware(av)(next)
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Bearer invalid-token-here")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestTenantMiddleware_Mismatch(t *testing.T) {
	av := NewAuthValidator(testJWTSecret, testIssuer)
	tenantMW := TenantMiddleware()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := JWTMiddleware(av)(tenantMW(next))
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	r.Header.Set("X-Tenant-ID", "tenant-2")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestTenantMiddleware_MissingHeader(t *testing.T) {
	av := NewAuthValidator(testJWTSecret, testIssuer)
	tenantMW := TenantMiddleware()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := JWTMiddleware(av)(tenantMW(next))
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	// No X-Tenant-ID header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetupCORS(t *testing.T) {
	handler := SetupCORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest("OPTIONS", "/test", nil)
	r.Header.Set("Origin", "http://example.com")
	r.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestLogger(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Logger(next)
	r := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRequestID(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RequestID(next)
	r := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestTraceID(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := TraceID(next)
	r := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotEmpty(t, w.Header().Get("X-Trace-ID"))
}