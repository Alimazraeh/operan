package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/operan/agent-collaboration/internal/ctxkeys"
)

type AuthValidator struct {
	secret string
	issuer string
}

func NewAuthValidator(secret, issuer string) *AuthValidator {
	return &AuthValidator{secret: secret, issuer: issuer}
}

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

	if iss, ok := claims["iss"].(string); ok && a.issuer != "" {
		if iss != a.issuer {
			return "", "", nil, fmt.Errorf("token issuer mismatch")
		}
	}

	if sub, ok := claims["sub"].(string); ok {
		tenantID = sub
	}
	if tenantID == "" {
		return "", "", nil, fmt.Errorf("missing tenant in JWT")
	}

	if uid, ok := claims["sub"].(string); ok && uid != "" {
		userID = uid
	}

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

func TenantMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := ctxkeys.GetTenantID(r.Context())
			headerTenant := chi.URLParam(r, "tenantID")
			if headerTenant == "" {
				headerTenant = r.Header.Get("X-Tenant-ID")
			}
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

func SetupCORS() func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-Tenant-ID", "X-Request-ID", "X-Trace-ID"},
		ExposedHeaders: []string{"Link"},
		MaxAge:         300,
	})
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), ctxkeys.RequestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := uuid.New().String()
		w.Header().Set("X-Trace-ID", traceID)
		ctx := context.WithValue(r.Context(), ctxkeys.TraceIDKey, traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}