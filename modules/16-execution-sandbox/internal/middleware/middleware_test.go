package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuthValidator_Validate_Success(t *testing.T) {
	secret := "test-secret"
	issuer := "operan-tenant-control-plane"
	av := NewAuthValidator(secret, issuer)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  "tenant-123",
		"iss":  issuer,
		"roles": []interface{}{"admin", "viewer"},
		"exp":  jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	tokenStr, _ := token.SignedString([]byte(secret))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req.Header.Set("X-Tenant-ID", "tenant-123")

	tenantID, userID, roles, err := av.Validate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tenantID != "tenant-123" {
		t.Errorf("expected tenant 'tenant-123', got '%s'", tenantID)
	}
	if userID != "tenant-123" {
		t.Errorf("expected user 'tenant-123', got '%s'", userID)
	}
	if len(roles) != 2 || roles[0] != "admin" {
		t.Errorf("expected [admin, viewer], got %v", roles)
	}
}

func TestAuthValidator_Validate_MissingHeader(t *testing.T) {
	av := NewAuthValidator("secret", "")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	_, _, _, err := av.Validate(req)
	if err == nil {
		t.Error("expected error for missing auth header")
	}
}

func TestAuthValidator_Validate_BadFormat(t *testing.T) {
	av := NewAuthValidator("secret", "")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "InvalidFormat token")

	_, _, _, err := av.Validate(req)
	if err == nil {
		t.Error("expected error for invalid auth format")
	}
}

func TestAuthValidator_Validate_InvalidToken(t *testing.T) {
	av := NewAuthValidator("secret", "")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")

	_, _, _, err := av.Validate(req)
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestAuthValidator_Validate_WrongIssuer(t *testing.T) {
	av := NewAuthValidator("secret", "correct-issuer")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "tenant-1",
		"iss": "wrong-issuer",
		"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	tokenStr, _ := token.SignedString([]byte("secret"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	_, _, _, err := av.Validate(req)
	if err == nil {
		t.Error("expected error for issuer mismatch")
	}
}

func TestAuthValidator_Validate_MissingSub(t *testing.T) {
	av := NewAuthValidator("secret", "")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "operan-tenant-control-plane",
		"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	tokenStr, _ := token.SignedString([]byte("secret"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	_, _, _, err := av.Validate(req)
	if err == nil {
		t.Error("expected error for missing sub")
	}
}

func TestAuthValidator_Validate_Tampered(t *testing.T) {
	av := NewAuthValidator("secret", "")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "tenant-1",
		"iss": "operan-tenant-control-plane",
		"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	tokenStr, _ := token.SignedString([]byte("secret"))
	// Tamper with the token
	tokenStr = tokenStr[:len(tokenStr)-10] + "tampered!"

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	_, _, _, err := av.Validate(req)
	if err == nil {
		t.Error("expected error for tampered token")
	}
}

func futureExpTime() jwt.NumericDate {
	return jwt.NumericDate{Time: time.Now().Add(time.Hour)}
}