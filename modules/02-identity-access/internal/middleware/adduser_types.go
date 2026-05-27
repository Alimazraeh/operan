package middleware

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateToken creates a signed JWT for the given identity.
func GenerateToken(secret string, subject string, userType string, tenantID string, email string, roles []string, expiryMin int) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":       subject,
		"iss":       "operan-iam",
		"user_type": userType,
		"tenant_id": tenantID,
		"iat":       now.Unix(),
		"exp":       now.Add(time.Duration(expiryMin) * time.Minute).Unix(),
	}
	if email != "" {
		claims["email"] = email
	}
	if len(roles) > 0 {
		claims["roles"] = roles
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseAndValidateToken parses and validates a JWT, returning the claims.
func ParseAndValidateToken(secret, tokenStr string) (jwt.MapClaims, error) {
	tokenResult, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !tokenResult.Valid {
		return nil, jwt.ErrSignatureInvalid
	}
	return tokenResult.Claims.(jwt.MapClaims), nil
}
