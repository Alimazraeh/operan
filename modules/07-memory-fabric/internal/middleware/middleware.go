// Package middleware provides HTTP middleware for Module 07 (Tool Execution):
// request IDs, trace propagation, JWT auth, tenant context, rate limiting,
// and request logging.
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

	"github.com/operan/modules/07-memory-fabric/internal/ctxkeys"
)

// ─── JWT Auth ────────────────────────────────────────────────────────────────

// JWTAuth validates Bearer tokens using HMAC-SHA256 with a shared secret.
func JWTAuth(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			problem(w, r, http.StatusUnauthorized, "Unauthorized", "Authorization header required")
			return
		}
		if !strings.HasPrefix(authHeader, "Bearer ") {
			problem(w, r, http.StatusUnauthorized, "Unauthorized", "Invalid authorization scheme; must be Bearer <token>")
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := validateJWT(secret, token)
		if err != nil {
			problem(w, r, http.StatusUnauthorized, "Unauthorized", "Invalid or expired token")
			return
		}

		ctx := r.Context()
		if sub, ok := claims["sub"].(string); ok {
			ctx = context.WithValue(ctx, ctxkeys.UserID, sub)
		}
		if roles, ok := claims["roles"].([]interface{}); ok {
			roleStrings := make([]string, 0, len(roles))
			for _, rv := range roles {
				if rs, ok := rv.(string); ok {
					roleStrings = append(roleStrings, rs)
				}
			}
			ctx = context.WithValue(ctx, ctxkeys.UserRoles, roleStrings)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
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

	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
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
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ─── Rate limiting (per-tenant sliding window) ───────────────────────────────

type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{requests: make(map[string][]time.Time), limit: limit, window: window}
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
			filtered := times[:0]
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

// RateLimit returns middleware enforcing per-tenant request rate limiting.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	rl := newRateLimiter(limit, window)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.Allow(ctxkeys.TenantIDFrom(r.Context())) {
				problem(w, r, http.StatusTooManyRequests, "Too Many Requests", "Rate limit exceeded; retry after 1 minute")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ─── Request ID / Trace ID / Tenant context / Logger ─────────────────────────

// RequestID generates a unique request ID and adds it to context/response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := generateID()
		w.Header().Set("X-Request-ID", reqID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxkeys.RequestID, reqID)))
	})
}

// TraceID propagates or generates a trace ID for telemetry correlation.
func TraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-Id")
		if traceID == "" {
			traceID = generateID()
		}
		w.Header().Set("X-Trace-Id", traceID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxkeys.TraceID, traceID)))
	})
}

// TenantContext injects the X-Tenant-ID header into context, rejecting requests
// without one (400).
func TenantContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			problem(w, r, http.StatusBadRequest, "Bad Request", "X-Tenant-ID header required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxkeys.TenantID, tenantID)))
	})
}

// Logger logs request method, path, status, and timing.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lw, r)
		log.Printf("[%s] %s %d tenant=%s trace=%s req=%s duration=%s",
			r.Method, r.URL.Path, lw.statusCode,
			ctxkeys.TenantIDFrom(r.Context()), ctxkeys.TraceIDFrom(r.Context()),
			ctxkeys.RequestIDFrom(r.Context()), time.Since(start))
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

// ─── Context accessors ───────────────────────────────────────────────────────

func TenantIDFromContext(ctx context.Context) string  { return ctxkeys.TenantIDFrom(ctx) }
func UserIDFromContext(ctx context.Context) string     { return ctxkeys.UserIDFrom(ctx) }
func RequestIDFromContext(ctx context.Context) string  { return ctxkeys.RequestIDFrom(ctx) }
func TraceIDFromContext(ctx context.Context) string    { return ctxkeys.TraceIDFrom(ctx) }
func UserRolesFromContext(ctx context.Context) []string { return ctxkeys.UserRolesFrom(ctx) }

// ─── Helpers ─────────────────────────────────────────────────────────────────

func problem(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type":       "about:blank",
		"title":      title,
		"status":     status,
		"detail":     detail,
		"instance":   r.URL.Path,
		"request_id": ctxkeys.RequestIDFrom(r.Context()),
	})
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
