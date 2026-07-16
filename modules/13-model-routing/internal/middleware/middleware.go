package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/operan/model-routing/internal/ctxkeys"
)

// JWTConfig holds the parameters needed to validate JWTs.
type JWTConfig struct {
	Secret  []byte
	Issuer  string
}

// RequireJWT returns a middleware that requires a valid Bearer JWT.
func RequireJWT(cfg *JWTConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(auth, "Bearer ")
			if tokenStr == auth {
				http.Error(w, `{"error":"invalid Authorization format, use: Bearer <token>"}`, http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, http.ErrAbortHandler
				}
				return cfg.Secret, nil
			})
			if err != nil || !token.Valid {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxkeys.PrincipalKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireTenant returns a middleware that requires the X-Tenant-ID header.
func RequireTenant() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := r.Header.Get("X-Tenant-ID")
			if tenantID == "" {
				http.Error(w, `{"error":"missing X-Tenant-ID header"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxkeys.TenantIDKey, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}