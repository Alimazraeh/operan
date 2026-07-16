package middleware

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/operan/policy-governance/internal/ctxkeys"
)

// AuthValidator handles JWT token validation.
type AuthValidator struct {
	secret string
	issuer string
}

// NewAuthValidator creates a new AuthValidator with the given JWT secret and expected issuer.
func NewAuthValidator(secret, issuer string) *AuthValidator {
	return &AuthValidator{
		secret: secret,
		issuer: issuer,
	}
}

// Validate extracts and validates a JWT from the Authorization header.
// Returns (tenantID, userID, roles, error).
func (a *AuthValidator) Validate(r *http.Request) (tenantID, userID string, roles []string, err error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", "", nil, fmt.Errorf("missing Authorization header")
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenStr == authHeader {
		return "", "", nil, fmt.Errorf("invalid Authorization format")
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.secret), nil
	})
	if err != nil || !token.Valid {
		return "", "", nil, fmt.Errorf("invalid or expired JWT token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", nil, fmt.Errorf("invalid token claims")
	}

	// Validate issuer
	if a.issuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != a.issuer {
			return "", "", nil, fmt.Errorf("token issuer mismatch")
		}
	}

	// Extract tenant from JWT subject claim
	if sub, ok := claims["sub"].(string); ok {
		tenantID = sub
	}
	if tenantID == "" {
		return "", "", nil, fmt.Errorf("missing tenant in JWT")
	}

	// Extract user ID from the "jti" or "uid" claim, falling back to subject
	if uid, ok := claims["uid"].(string); ok {
		userID = uid
	} else {
		userID = tenantID
	}

	// Extract roles array
	if rolesRaw, ok := claims["roles"].([]interface{}); ok {
		roles = make([]string, len(rolesRaw))
		for i, r := range rolesRaw {
			if rs, ok := r.(string); ok {
				roles[i] = rs
			}
		}
	}

	return tenantID, userID, roles, nil
}

// JWTMiddleware returns a chi middleware that validates JWT and injects context values.
func JWTMiddleware(authValidator *AuthValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, userID, roles, err := authValidator.Validate(r)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error":   "unauthorized",
					"message": err.Error(),
				})
				return
			}
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxkeys.TenantIDKey, tenantID)
			ctx = context.WithValue(ctx, ctxkeys.UserIDKey, userID)
			ctx = context.WithValue(ctx, ctxkeys.RolesKey, roles)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TenantMiddleware validates the X-Tenant-ID header matches the JWT tenant.
func TenantMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := ctxkeys.GetTenantID(r.Context())
			headerTenant := chi.URLParam(r, "tenantID")
			if headerTenant == "" {
				headerTenant = r.Header.Get("X-Tenant-ID")
			}
			if headerTenant == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error":   "missing-tenant",
					"message": "X-Tenant-ID header required",
				})
				return
			}
			if headerTenant != tenantID {
				writeJSON(w, http.StatusForbidden, map[string]string{
					"error":   "tenant-mismatch",
					"message": "X-Tenant-ID does not match JWT tenant",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SetupCORS returns a middleware that adds CORS headers.
func SetupCORS() func(http.Handler) http.Handler {
	return corsHandler()
}

// Logger is a simple request logger middleware.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// RequestID generates a unique request ID for each request.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := generateID()
		ctx := context.WithValue(r.Context(), ctxkeys.RequestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TraceID extracts or generates a trace ID.
func TraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = generateID()
		}
		ctx := context.WithValue(r.Context(), ctxkeys.TraceIDKey, traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// best-effort: headers already written
	}
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x%x%x%x", b[:2], b[2:4], b[4:6], b[6:])
}

func corsHandler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Tenant-ID, X-Request-ID")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}