// Package middleware provides HTTP middleware for Module 05.
// It handles request IDs, trace propagation, tenant context injection,
// authentication, and request logging.
package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/ctxkeys"
)

// ─── JWT Auth ────────────────────────────────────────────────────────────────

// JWTAuth validates Bearer tokens using HMAC-S256 with a shared secret.
func JWTAuth(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"type":      "about:blank",
				"title":     "Unauthorized",
				"status":    http.StatusUnauthorized,
				"detail":    "Authorization header required",
				"instance":  r.URL.Path,
				"request_id": getReqID(r.Context()),
			})
			return
		}
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"type":      "about:blank",
				"title":     "Unauthorized",
				"status":    http.StatusUnauthorized,
				"detail":    "Invalid authorization scheme; must be Bearer <token>",
				"instance":  r.URL.Path,
				"request_id": getReqID(r.Context()),
			})
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := validateJWT(secret, token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"type":      "about:blank",
				"title":     "Unauthorized",
				"status":    http.StatusUnauthorized,
				"detail":    "Invalid or expired token",
				"instance":  r.URL.Path,
				"request_id": getReqID(r.Context()),
			})
			return
		}

		ctx := r.Context()
		if sub, ok := claims["sub"].(string); ok {
			ctx = context.WithValue(ctx, ctxkeys.UserID, sub)
		}
		if roles, ok := claims["roles"].([]interface{}); ok {
			roleStrings := make([]string, len(roles))
			for i, r := range roles {
				if rs, ok := r.(string); ok {
					roleStrings[i] = rs
				}
			}
			ctx = context.WithValue(ctx, ctxkeys.UserRoles, roleStrings)
		}
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func validateJWT(secret, tokenStr string) (map[string]interface{}, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSig := computeHMAC(secret, signingInput)
	if !secureCompare(parts[2], expectedSig) {
		return nil, fmt.Errorf("invalid signature")
	}

	decoded, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid JWT payload encoding")
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, fmt.Errorf("invalid JWT claims")
	}

	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().UTC().Unix() > int64(exp) {
			return nil, fmt.Errorf("token expired")
		}
	}

	return claims, nil
}

func computeHMAC(secret, data string) string {
	hash := hmacSHA256([]byte(secret), []byte(data))
	return base64URLEncode(hash)
}

func base64URLDecode(s string) ([]byte, error) {
	dec, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return dec, nil
}

func base64URLEncode(data []byte) string {
	enc := encodeBase64(data)
	enc = strings.ReplaceAll(enc, "+", "-")
	enc = strings.ReplaceAll(enc, "/", "_")
	enc = strings.TrimRight(enc, "=")
	return enc
}

func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ─── Middleware chain ────────────────────────────────────────────────────────

// RequestID generates a unique request ID and adds it to context/response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := generateID()
		ctx := context.WithValue(r.Context(), ctxkeys.RequestID, reqID)
		w.Header().Set("X-Request-ID", reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TraceID generates a trace ID for OpenTelemetry correlation.
func TraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-Id")
		if traceID == "" {
			traceID = generateID()
		}
		ctx := context.WithValue(r.Context(), ctxkeys.TraceID, traceID)
		w.Header().Set("X-Trace-Id", traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TenantContext injects tenant ID from X-Tenant-ID header into context.
// Rejects requests without a tenant ID with 400 Bad Request.
func TenantContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"type":      "about:blank",
				"title":     "Bad Request",
				"status":    http.StatusBadRequest,
				"detail":    "X-Tenant-ID header required",
				"instance":  r.URL.Path,
				"request_id": getReqID(r.Context()),
			})
			return
		}
		ctx := context.WithValue(r.Context(), ctxkeys.TenantID, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logger wraps each request with timing and status logging.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lw, r)
		duration := time.Since(start)
		tenantID := ctxkeys.TenantIDFrom(r.Context())
		traceID := ctxkeys.TraceIDFrom(r.Context())
		requestID := ctxkeys.RequestIDFrom(r.Context())
		log.Printf("[%s] %s %d %s tenant=%s trace=%s req=%s duration=%s",
			r.Method, r.URL.Path, lw.statusCode, duration, tenantID, traceID, requestID, duration)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.statusCode = code
	lw.ResponseWriter.WriteHeader(code)
}

// ─── Helper functions ────────────────────────────────────────────────────────

// RequestIDFromContext extracts the request ID from the request's context.
func RequestIDFromContext(ctx context.Context) string {
	return ctxkeys.RequestIDFrom(ctx)
}

// TenantIDFromContext extracts the tenant ID from the request's context.
func TenantIDFromContext(ctx context.Context) string {
	return ctxkeys.TenantIDFrom(ctx)
}

// UserIDFromContext extracts the user ID from the request's context.
func UserIDFromContext(ctx context.Context) string {
	return ctxkeys.UserIDFrom(ctx)
}

// UserRolesFromContext extracts the user roles from the request's context.
func UserRolesFromContext(ctx context.Context) []string {
	return ctxkeys.UserRolesFrom(ctx)
}

func getReqID(ctx context.Context) string {
	v := ctx.Value(ctxkeys.RequestID)
	if v == nil {
		return ""
	}
	return v.(string)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func encodeBase64(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
