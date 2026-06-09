// Package middleware provides HTTP middleware for the orchestration engine.
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

	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── Context keys ────────────────────────────────────────────────────────────

type contextKey string

const (
	ctxKeyTenantID  contextKey = "tenant_id"
	ctxKeyTraceID   contextKey = "trace_id"
	ctxKeyRequestID contextKey = "request_id"
	ctxKeyUserID    contextKey = "user_id"
	ctxKeyUserRole  contextKey = "user_role"
	ctxKeyUserRoles contextKey = "user_roles"
)

// JWTAuth validates Bearer tokens using HMAC-S256 with a shared secret.
// For MVP, uses a configurable JWT_SECRET env var. In production, this
// should be refactored to delegate to Module 02 (IAM) for token validation.
func JWTAuth(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeUnauthorized(w, "Authorization header required")
			return
		}
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeUnauthorized(w, "Invalid authorization scheme; must be Bearer <token>")
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := validateJWT(secret, token)
		if err != nil {
			writeUnauthorized(w, "Invalid or expired token")
			return
		}

		ctx := r.Context()
		if sub, ok := claims["sub"].(string); ok {
			ctx = context.WithValue(ctx, ctxKeyUserID, sub)
		}
		if roles, ok := claims["roles"].([]interface{}); ok {
			roleStrings := make([]string, len(roles))
			for i, r := range roles {
				if rs, ok := r.(string); ok {
					roleStrings[i] = rs
				}
			}
			ctx = context.WithValue(ctx, ctxKeyUserRoles, roleStrings)
		}
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// validateJWT validates an HMAC-S256 signed JWT and returns the claims.
func validateJWT(secret, tokenStr string) (map[string]interface{}, error) {
	// Split the token into header, payload, signature
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Verify signature
	signingInput := parts[0] + "." + parts[1]
	expectedSig := computeHMAC(secret, signingInput)
	if !secureCompare(parts[2], expectedSig) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Decode payload
	decoded, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid JWT payload encoding")
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, fmt.Errorf("invalid JWT claims")
	}

	// Check expiration
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().UTC().Unix() > int64(exp) {
			return nil, fmt.Errorf("token expired")
		}
	}

	return claims, nil
}

// computeHMAC computes HMAC-S256 hash and returns base64url-encoded signature.
func computeHMAC(secret, data string) string {
	hash := hmacSHA256([]byte(secret), []byte(data))
	return base64URLEncode(hash)
}

// base64URLDecode decodes base64url-encoded string (no padding).
func base64URLDecode(s string) ([]byte, error) {
	dec, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return dec, nil
}

// base64URLEncode encodes bytes to base64url without padding.
func base64URLEncode(data []byte) string {
	enc := encodeBase64(data)
	enc = strings.ReplaceAll(enc, "+", "-")
	enc = strings.ReplaceAll(enc, "/", "_")
	enc = strings.TrimRight(enc, "=")
	return enc
}

// secureCompare performs constant-time string comparison.
func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, `{"error":{"code":401,"message":"%s"}}`, message)
}

// ─── Middleware chain ────────────────────────────────────────────────────────

// RequestID generates a unique request ID and adds it to the context/response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := generateID()
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, reqID)
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
		ctx := context.WithValue(r.Context(), ctxKeyTraceID, traceID)
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
			h := &Handler{}
			h.WriteError(w, http.StatusBadRequest, 400, "X-Tenant-ID header required", "")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyTenantID, tenantID)
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
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, lw.statusCode, duration)
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

// ─── Handler struct ──────────────────────────────────────────────────────────

// Handler holds all stores and provides HTTP route registration.
type Handler struct {
	WorkflowStore    *store.WorkflowStore
	ScheduleStore    *store.ScheduleStore
	AgentStore       *store.AgentStore
	EventPublisher   *events.Publisher
}

// NewHandler creates a new Handler with all stores.
func NewHandler() *Handler {
	return &Handler{
		WorkflowStore:  store.NewWorkflowStore(),
		ScheduleStore:  store.NewScheduleStore(),
		AgentStore:     store.NewAgentStore(),
		EventPublisher: events.NewPublisher(),
	}
}

// ─── Helper methods ──────────────────────────────────────────────────────────

// WriteJSON writes a JSON response with the given status code.
func (h *Handler) WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes an error response matching the OpenAPI Error schema.
func (h *Handler) WriteError(w http.ResponseWriter, status int, code int, message string, details string) {
	resp := ErrorResponse{
		Code:      code,
		Message:   message,
		Details:   details,
		RequestID: generateID(),
	}
	h.WriteJSON(w, status, resp)
}

// TenantIDFromContext extracts the tenant ID from the request context.
func TenantIDFromContext(ctx context.Context) string {
	v := ctx.Value(ctxKeyTenantID)
	if v == nil {
		return ""
	}
	return v.(string)
}

// UserIDFromContext extracts the user ID from the request context.
func UserIDFromContext(ctx context.Context) string {
	v := ctx.Value(ctxKeyUserID)
	if v == nil {
		return ""
	}
	return v.(string)
}

// UserRolesFromContext extracts the user roles from the request context.
func UserRolesFromContext(ctx context.Context) []string {
	v := ctx.Value(ctxKeyUserRoles)
	if v == nil {
		return nil
	}
	roles, ok := v.([]string)
	if !ok {
		return nil
	}
	return roles
}

// SetTenantIDToContext returns a new context with the given tenant ID.
// This is primarily useful for tests.
func SetTenantIDToContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ctxKeyTenantID, tenantID)
}

// RequestIDFromContext extracts the request ID from the request context.
func RequestIDFromContext(ctx context.Context) string {
	v := ctx.Value(ctxKeyRequestID)
	if v == nil {
		return ""
	}
	return v.(string)
}

// TraceIDFromContext extracts the trace ID from the request context.
func TraceIDFromContext(ctx context.Context) string {
	v := ctx.Value(ctxKeyTraceID)
	if v == nil {
		return ""
	}
	return v.(string)
}

// ─── Response types ──────────────────────────────────────────────────────────

// ErrorResponse matches the OpenAPI Error schema.
type ErrorResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Details   string `json:"details"`
	RequestID string `json:"request_id"`
}

// PaginatedResponse is the base wrapper for paginated lists.
type PaginatedResponse[T any] struct {
	Data    []*T `json:"data"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

// ─── ID generation ───────────────────────────────────────────────────────────

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ─── HMAC and Base64 helpers ─────────────────────────────────────────────────

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func encodeBase64(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeBase64(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
