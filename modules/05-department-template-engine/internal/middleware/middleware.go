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
	"sync"
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

// ─── Rate Limiting ───────────────────────────────────────────────────────────

// rateLimiter implements a simple per-tenant sliding window rate limiter.
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// newRateLimiter creates a rate limiter allowing `limit` requests per `window` duration per tenant.
func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	// Start cleanup goroutine
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, times := range rl.requests {
			// Filter out old entries
			filtered := make([]time.Time, 0, len(times))
			for _, t := range times {
				if now.Sub(t) <= rl.window {
					filtered = append(filtered, t)
				}
			}
			if len(filtered) == 0 {
				delete(rl.requests, key)
			} else {
				rl.requests[key] = filtered
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	// Filter to only requests within the window
	filtered := make([]time.Time, 0, len(rl.requests[key]))
	for _, t := range rl.requests[key] {
		if t.After(windowStart) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) >= rl.limit {
		rl.requests[key] = filtered
		return false
	}

	rl.requests[key] = append(filtered, now)
	return true
}

// RateLimit returns middleware that enforces per-tenant request rate limiting.
// Default: 100 requests per minute per tenant.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	rl := newRateLimiter(limit, window)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := TenantIDFromContext(r.Context())
			if !rl.Allow(tenantID) {
				writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
					"type":       "about:blank",
					"title":      "Too Many Requests",
					"status":     http.StatusTooManyRequests,
					"detail":     "Rate limit exceeded; retry after 1 minute",
					"instance":   r.URL.Path,
					"request_id": getReqID(r.Context()),
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
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
