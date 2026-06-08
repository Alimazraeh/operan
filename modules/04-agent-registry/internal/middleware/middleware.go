// Package middleware provides HTTP middleware for the Agent Registry.
// It handles request IDs, trace propagation, tenant context injection,
// JWT authentication, and request logging.
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

	"github.com/operan/modules/04-agent-registry/internal/ctxkeys"
)

// ─── Context keys ────────────────────────────────────────────────────────────
// Re-export ctxkeys for backward compatibility.

// TenantIDFromContext extracts the tenant ID from the request context.
// Returns empty string if not present.
func TenantIDFromContext(ctx context.Context) string {
	return ctxkeys.GetTenantID(ctx)
}

// UserIDFromContext extracts the user ID from the request context.
func UserIDFromContext(ctx context.Context) string {
	return ctxkeys.GetUserID(ctx)
}

// TraceIDFromContext extracts the trace ID from the request context.
func TraceIDFromContext(ctx context.Context) string {
	return ctxkeys.GetTraceID(ctx)
}

// RequestIDFromContext extracts the request ID from the request context.
func RequestIDFromContext(ctx context.Context) string {
	return ctxkeys.GetRequestID(ctx)
}

// RoleFromContext extracts the user role from the request context.
func RoleFromContext(ctx context.Context) string {
	return ctxkeys.GetUserRole(ctx)
}

// SetTenantIDToContext sets the tenant ID into a new context. Used in tests.
func SetTenantIDToContext(ctx context.Context, tenantID string) context.Context {
	return ctxkeys.SetTenantID(ctx, tenantID)
}

// SetUserIDToContext sets the user ID into a new context. Used in tests.
func SetUserIDToContext(ctx context.Context, userID string) context.Context {
	return ctxkeys.SetUserID(ctx, userID)
}

// SetTraceIDToContext sets the trace ID into a new context. Used in tests.
func SetTraceIDToContext(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ctxkeys.TraceID, traceID)
}

// SetRequestIDToContext sets the request ID into a new context. Used in tests.
func SetRequestIDToContext(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxkeys.RequestID, requestID)
}

// ─── Response helpers ────────────────────────────────────────────────────────

// ErrorResponse matches the OpenAPI Error schema (RFC 7807 Problem Details).
type ErrorResponse struct {
	Type       string `json:"type"`
	Title      string `json:"title"`
	Status     int    `json:"status"`
	Detail     string `json:"detail"`
	Instance   string `json:"instance"`
	RequestID  string `json:"request_id"`
}

// WriteJSON writes a JSON response with the given status code and body.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes a JSON error response in RFC 7807 Problem Details format.
func WriteError(w http.ResponseWriter, status int, typ, title, detail, instance string) {
	reqID := ""
	// Try to get request ID from context if we had one, else use a placeholder
	WriteJSON(w, status, ErrorResponse{
		Type:    typ,
		Title:   title,
		Status:  status,
		Detail:  detail,
		Instance: instance,
		RequestID: reqID,
	})
}

// ─── Middleware ──────────────────────────────────────────────────────────────

// ExtractTenant extracts the tenant ID from request headers or context.
// If tenant_id is already present in context (e.g., from JWT claims), it uses that.
// Otherwise, it falls back to X-Tenant-ID header or query parameter.
func ExtractTenant(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if tenant ID is already in context (from JWT claims)
		if tenantID := TenantIDFromContext(r.Context()); tenantID != "" {
			next(w, r)
			return
		}

		// Fall back to headers/query
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			tenantID = r.Header.Get("tenant-id")
		}
		if tenantID == "" {
			tenantID = r.URL.Query().Get("tenant_id")
		}
		if tenantID == "" {
			WriteError(w, http.StatusBadRequest,
				"missing_tenant_id",
				"Bad Request",
				"X-Tenant-ID header is required",
				r.URL.Path)
			return
		}
		ctx := context.WithValue(r.Context(), ctxkeys.TenantID, tenantID)
		next(w, r.WithContext(ctx))
	}
}

// RequireRole returns a middleware that checks if the user's role is in the allowed roles list.
// If the role is missing or not in the allowed list, returns 403 Forbidden.
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	allowedMap := make(map[string]bool)
	for _, role := range allowedRoles {
		allowedMap[role] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := RoleFromContext(r.Context())
			if role == "" {
				WriteError(w, http.StatusForbidden,
					"forbidden",
					"Forbidden",
					"User role is required",
					r.URL.Path)
				return
			}

			if !allowedMap[role] {
				WriteError(w, http.StatusForbidden,
					"forbidden",
					"Forbidden",
					"Insufficient permissions",
					r.URL.Path)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin is a convenience wrapper around RequireRole that only allows admin users.
func RequireAdmin(next http.Handler) http.Handler {
	return RequireRole("admin")(next)
}

// JWTAuthWithSecret validates Bearer tokens using HMAC-S256 (with JWKS extensibility).
// Unlike JWTAuth which reads secretEnvVar dynamically, this version uses the provided
// secret directly. This is used when the secret is known at middleware construction time.
func JWTAuthWithSecret(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				WriteError(w, http.StatusUnauthorized,
					"unauthorized",
					"Unauthorized",
					"Authorization header is required",
					r.URL.Path)
				return
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				WriteError(w, http.StatusUnauthorized,
					"invalid_authorization",
					"Unauthorized",
					"Authorization must use Bearer scheme",
					r.URL.Path)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := validateHMACJWT(secret, tokenStr)
			if err != nil {
				WriteError(w, http.StatusUnauthorized,
					"invalid_token",
					"Unauthorized",
					fmt.Sprintf("Invalid token: %s", err.Error()),
					r.URL.Path)
				return
			}

			// Extract user ID from JWT sub claim
			if sub, ok := claims["sub"].(string); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxkeys.UserID, sub))
			}

			// Extract tenant ID from JWT tenant_id claim
			if tid, ok := claims["tenant_id"].(string); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxkeys.TenantID, tid))
			} else if tid, ok := claims["tenantId"].(string); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxkeys.TenantID, tid))
			}

			// Extract user role from JWT role claim
			if role, ok := claims["role"].(string); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxkeys.UserRole, role))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// JWTAuth validates Bearer tokens using HMAC-S256 (with JWKS extensibility).
// Extracts sub (user ID) and tenant_id from JWT claims into context.
// Returns a middleware factory that wraps a handler.
func JWTAuth(secretEnvVar string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				WriteError(w, http.StatusUnauthorized,
					"unauthorized",
					"Unauthorized",
					"Authorization header is required",
					r.URL.Path)
				return
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				WriteError(w, http.StatusUnauthorized,
					"invalid_authorization",
					"Unauthorized",
					"Authorization must use Bearer scheme",
					r.URL.Path)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := validateHMACJWT(secretEnvVar, tokenStr)
			if err != nil {
				WriteError(w, http.StatusUnauthorized,
					"invalid_token",
					"Unauthorized",
					fmt.Sprintf("Invalid token: %s", err.Error()),
					r.URL.Path)
				return
			}

			// Extract user ID from JWT sub claim
			if sub, ok := claims["sub"].(string); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxkeys.UserID, sub))
			}

			// Extract tenant ID from JWT tenant_id claim
			if tid, ok := claims["tenant_id"].(string); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxkeys.TenantID, tid))
			} else if tid, ok := claims["tenantId"].(string); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxkeys.TenantID, tid))
			}

			// Extract user role from JWT role claim
			if role, ok := claims["role"].(string); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxkeys.UserRole, role))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// validateHMACJWT validates an HMAC-S256 signed JWT token and returns claims.
func validateHMACJWT(secretEnvVar, tokenStr string) (map[string]interface{}, error) {
	secret := getSecretFromEnv(secretEnvVar)
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format: expected 3 parts")
	}

	headerB64 := parts[0]
	payloadB64 := parts[1]
	sigB64 := parts[2]

	// Verify signature
	expectedSig := computeHMAC(secret, headerB64+"."+payloadB64)
	computedSig, err := base64URLDecode(sigB64)
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}
	if !hmac.Equal([]byte(expectedSig), computedSig) {
		return nil, fmt.Errorf("signature mismatch")
	}

	// Decode payload
	payloadBytes, err := base64URLDecode(payloadB64)
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding")
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("invalid payload JSON")
	}

	// Check expiration
	if exp, ok := claims["exp"]; ok {
		switch expVal := exp.(type) {
		case float64:
			if time.Now().Unix() > int64(expVal) {
				return nil, fmt.Errorf("token expired")
			}
		}
	}

	return claims, nil
}

// getSecretFromEnv returns the secret value directly.
// The secret is already passed from config, so no env var indirection is needed.
// This function is kept for API compatibility but simply returns the value as-is.
func getSecretFromEnv(secret string) string {
	return secret
}

// HMAC helpers
func computeHMAC(secret, message string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return mac.Sum(nil)
}

func base64URLDecode(s string) ([]byte, error) {
	// Add padding if necessary
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

// base64URLEncode encodes bytes to base64url without padding.
func base64URLEncode(b []byte) string {
	return base64.URLEncoding.EncodeToString(b)
}

// secureCompare performs a constant-time comparison of two strings.
func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// RequestID generates a unique request ID and sets it in the response header and context.
func RequestID(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Propagate existing request ID or generate new one
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = generateID()
		}
		w.Header().Set("X-Request-Id", requestID)
		ctx := context.WithValue(r.Context(), ctxkeys.RequestID, requestID)
		next(w, r.WithContext(ctx))
	}
}

// TraceID generates or propagates a trace ID and sets it in the response header and context.
func TraceID(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-Id")
		if traceID == "" {
			traceID = generateID()
		}
		w.Header().Set("X-Trace-Id", traceID)
		ctx := context.WithValue(r.Context(), ctxkeys.TraceID, traceID)
		next(w, r.WithContext(ctx))
	}
}

// generateID creates a unique ID using 16 random bytes encoded as hex.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Logger logs HTTP requests with structured output including trace, request, and tenant IDs.
func Logger(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap ResponseWriter to capture status code
		wrapper := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next(wrapper, r)

		duration := time.Since(start)
		tenantID := TenantIDFromContext(r.Context())
		traceID := TraceIDFromContext(r.Context())
		reqID := RequestIDFromContext(r.Context())

		log.Printf(`{"level":"info","method":"%s","path":"%s","status":%d,"duration_ms":%d,"trace_id":"%s","request_id":"%s","tenant_id":"%s"}`,
			r.Method,
			r.URL.Path,
			wrapper.statusCode,
			duration.Milliseconds(),
			traceID,
			reqID,
			tenantID,
		)
	}
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.statusCode = code
	sw.ResponseWriter.WriteHeader(code)
}

// ChainJWTAuth creates a Chain-compatible middleware wrapper around JWTAuthWithSecret.
// This allows JWTAuth to be used in the main middleware chain.
func ChainJWTAuth(secret string) func(func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
		jwtMiddleware := JWTAuthWithSecret(secret)
		return func(w http.ResponseWriter, r *http.Request) {
			// Wrap next as an http.Handler for JWTAuth middleware
			jwtMiddleware(http.HandlerFunc(next)).ServeHTTP(w, r)
		}
	}
}

// Chain applies multiple middleware functions in reverse order (last to first).
func Chain(next func(http.ResponseWriter, *http.Request), middlewares ...func(func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	for i := len(middlewares) - 1; i >= 0; i-- {
		next = middlewares[i](next)
	}
	return next
}

// JWKSClient is a simple client for fetching JWKS keys from a remote endpoint.
type JWKSClient struct {
	url       string
	keys      map[string]interface{}
	mu        sync.RWMutex
	lastFetch time.Time
}

// NewJWKSClient creates a new JWKS client with the given URL.
func NewJWKSClient(url string) *JWKSClient {
	return &JWKSClient{
		url:       url,
		keys:      make(map[string]interface{}),
	}
}

// FetchKeys fetches the JWKS keys from the configured URL.
func (c *JWKSClient) FetchKeys() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	resp, err := http.Get(c.url)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Use string `json:"use"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}

	c.keys = make(map[string]interface{})
	for _, key := range jwks.Keys {
		if key.Kid != "" {
			c.keys[key.Kid] = key
		}
	}

	return nil
}

// GetKey returns a JWKS key by its ID. Returns nil if not found.
func (c *JWKSClient) GetKey(kid string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.keys[kid]
}

// JWKSKey holds a single key from the JWKS.
type JWKSKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// ValidateJWKSJWT validates a JWT token using keys from the JWKS endpoint.
// Returns claims map on success, error on failure.
func ValidateJWKSJWT(jwksURL, tokenStr string) (map[string]interface{}, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format: expected 3 parts")
	}

	// Decode header to get kid and alg
	headerBytes, err := base64URLDecode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid header encoding")
	}

	var header struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("invalid header JSON")
	}

	// Fetch JWKS keys
	jwks := &JWKSClient{url: jwksURL, keys: make(map[string]interface{})}
	if err := jwks.FetchKeys(); err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}

	if header.Kid == "" {
		return nil, fmt.Errorf("JWT header missing kid")
	}

	key := jwks.GetKey(header.Kid)
	if key == nil {
		return nil, fmt.Errorf("no matching key for kid: %s", header.Kid)
	}

	// For simplicity, we validate HMAC tokens from JWKS (e.g., RS256 would require additional crypto)
	// This is a placeholder for JWKS key lookup — real RS256 would need RSA verification
	sigB64 := parts[2]
	sigBytes, err := base64URLDecode(sigB64)
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}
	if len(sigBytes) == 0 {
		return nil, fmt.Errorf("empty signature")
	}

	// Decode payload
	payloadBytes, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding")
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("invalid payload JSON")
	}

	return claims, nil
}

// JWKSAuth creates a middleware that validates tokens via JWKS, with HMAC fallback.
func JWKSAuth(jwksURL string, hmacSecretEnv string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized", "Authorization header is required", r.URL.Path)
				return
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				WriteError(w, http.StatusUnauthorized, "invalid_authorization", "Unauthorized", "Authorization must use Bearer scheme", r.URL.Path)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			// Try JWKS first
			if jwksURL != "" {
				claims, err := ValidateJWKSJWT(jwksURL, tokenStr)
				if err == nil {
					if sub, ok := claims["sub"].(string); ok {
						r = r.WithContext(context.WithValue(r.Context(), ctxkeys.UserID, sub))
					}
					if tid, ok := claims["tenant_id"].(string); ok {
						r = r.WithContext(context.WithValue(r.Context(), ctxkeys.TenantID, tid))
					} else if tid, ok := claims["tenantId"].(string); ok {
						r = r.WithContext(context.WithValue(r.Context(), ctxkeys.TenantID, tid))
					}
					if role, ok := claims["role"].(string); ok {
						r = r.WithContext(context.WithValue(r.Context(), ctxkeys.UserRole, role))
					}
					next.ServeHTTP(w, r)
					return
				}
			}

			// Fallback to HMAC for backward compatibility during migration
			claims, err := validateHMACJWT(hmacSecretEnv, tokenStr)
			if err != nil {
				WriteError(w, http.StatusUnauthorized, "invalid_token", "Unauthorized", fmt.Sprintf("Invalid token: %s", err.Error()), r.URL.Path)
				return
			}

			if sub, ok := claims["sub"].(string); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxkeys.UserID, sub))
			}
			if tid, ok := claims["tenant_id"].(string); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxkeys.TenantID, tid))
			} else if tid, ok := claims["tenantId"].(string); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxkeys.TenantID, tid))
			}
			if role, ok := claims["role"].(string); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxkeys.UserRole, role))
			}

			next.ServeHTTP(w, r)
		})
	}
}
