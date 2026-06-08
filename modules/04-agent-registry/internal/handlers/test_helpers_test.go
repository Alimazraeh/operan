package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// makeValidJWT creates a valid HMAC-SHA256 JWT token for tests.
// It uses the same algorithm as the middleware package's JWTAuth.
func makeValidJWT(t *testing.T, secret string, claims map[string]interface{}) string {
	t.Helper()

	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))

	payloadMap := map[string]interface{}{
		"sub":         "user-1",
		"tenant_id":   "tenant-1",
		"tenantId":    "tenant-1",
		"role":        "admin",
		"iat":         time.Now().Unix(),
		"exp":         time.Now().Add(24 * time.Hour).Unix(),
	}
	for k, v := range claims {
		payloadMap[k] = v
	}

	payloadBytes, _ := json.Marshal(payloadMap)
	payloadB64 := base64URLEncode(payloadBytes)

	sig := computeHMAC(secret, header+"."+payloadB64)
	signature := base64URLEncode(sig)

	return header + "." + payloadB64 + "." + signature
}

func computeHMAC(secret, message string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return mac.Sum(nil)
}

func base64URLEncode(b []byte) string {
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)
}

// testJWT creates a valid JWT with default admin claims for router tests.
func testJWT(t *testing.T) string {
	return makeValidJWT(t, "test-secret", nil)
}

// testJWTUser creates a valid JWT with a non-admin user role.
func testJWTUser(t *testing.T) string {
	return makeValidJWT(t, "test-secret", map[string]interface{}{
		"role": "user",
	})
}
