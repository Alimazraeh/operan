package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/operan/enterprise-connectors/internal/ctxkeys"
)

// AuthValidator handles JWT validation.
type AuthValidator struct {
	secret string
	issuer string
}

// NewAuthValidator creates a JWT auth validator.
func NewAuthValidator(secret, issuer string) *AuthValidator {
	return &AuthValidator{secret: secret, issuer: issuer}
}

// Validate extracts and validates the JWT, returning tenantID, userID, roles.
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
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); ok {
			return []byte(a.secret), nil
		}
		return nil, fmt.Errorf("unsupported signing method")
	})
	if err != nil || !token.Valid {
		return "", "", nil, fmt.Errorf("invalid or expired JWT token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", nil, fmt.Errorf("invalid token claims")
	}

	// Validate issuer
	if iss, ok := claims["iss"].(string); ok && a.issuer != "" {
		if iss != a.issuer {
			return "", "", nil, fmt.Errorf("token issuer mismatch")
		}
	}

	// Tenant comes from the tenant_id claim; sub is the user, kept only as
	// a legacy fallback for tokens minted before tenant_id existed.
	if tid, ok := claims["tenant_id"].(string); ok && tid != "" {
		tenantID = tid
	} else if sub, ok := claims["sub"].(string); ok {
		tenantID = sub
	}
	if tenantID == "" {
		return "", "", nil, fmt.Errorf("missing tenant in JWT")
	}

	// Extract user ID
	if uid, ok := claims["sub"].(string); ok && uid != "" {
		userID = uid
	}

	// Extract roles
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

// JWTMiddleware returns a chi middleware that validates JWT.
func JWTMiddleware(authValidator *AuthValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, userID, roles, err := authValidator.Validate(r)
			if err != nil {
				http.Error(w, `{"error":"unauthorized","message":"`+err.Error()+`"}`, http.StatusUnauthorized)
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

// TenantMiddleware validates X-Tenant-ID header matches the JWT tenant.
func TenantMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := ctxkeys.GetTenantID(r.Context())
			headerTenant := r.Header.Get("X-Tenant-ID")
			if headerTenant == "" {
				http.Error(w, `{"error":"missing-tenant","message":"X-Tenant-ID header required"}`, http.StatusBadRequest)
				return
			}
			if headerTenant != tenantID {
				http.Error(w, `{"error":"tenant-mismatch","message":"X-Tenant-ID does not match JWT tenant"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SetupCORS returns a chi middleware for CORS.
func SetupCORS() func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-Tenant-ID", "X-Request-ID", "X-Trace-ID"},
		ExposedHeaders: []string{"Link"},
		MaxAge:         300,
	})
}

// Logger is a simple request logger middleware.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}

// RequestID generates a unique request ID and sets it in context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), ctxkeys.RequestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TraceID generates a trace ID for distributed tracing.
func TraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := uuid.New().String()
		w.Header().Set("X-Trace-ID", traceID)
		ctx := context.WithValue(r.Context(), ctxkeys.TraceIDKey, traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}