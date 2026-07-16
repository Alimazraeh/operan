package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/operan/cost-governance/internal/config"
	"github.com/operan/cost-governance/internal/ctxkeys"

	"github.com/golang-jwt/jwt/v5"
)

// AuthValidator validates JWT tokens from Bearer headers.
type AuthValidator struct {
	secret string
	Issuer string
}

// NewAuthValidator creates a JWT validator.
func NewAuthValidator(cfg *config.Config) *AuthValidator {
	return &AuthValidator{
		secret: cfg.JWTSecret,
		Issuer: "operan-",
	}
}

// Validate extracts and validates the JWT from the Authorization header.
func (v *AuthValidator) Validate(r *http.Request) (string, string, []string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", "", nil, fmt.Errorf("missing Authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", "", nil, fmt.Errorf("invalid Authorization format; expected Bearer <token>")
	}

	token, err := jwt.Parse(parts[1], func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(v.secret), nil
	})
	if err != nil || !token.Valid {
		return "", "", nil, fmt.Errorf("invalid JWT: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", nil, fmt.Errorf("invalid JWT claims")
	}

	tenantID, ok := claims["tenant_id"].(string)
	if !ok || tenantID == "" {
		tenantID, _ = claims["x_tenant_id"].(string)
	}
	if tenantID == "" {
		return "", "", nil, fmt.Errorf("JWT missing tenant_id claim")
	}

	userID, _ := claims["sub"].(string)
	if userID == "" {
		userID, _ = claims["user_id"].(string)
	}

	var roles []string
	if rolesAny, ok := claims["roles"].([]any); ok {
		for _, r := range rolesAny {
			if rs, ok := r.(string); ok {
				roles = append(roles, rs)
			}
		}
	}

	return tenantID, userID, roles, nil
}

// JWTMiddleware validates JWT and injects tenant/user context.
func JWTMiddleware(v *AuthValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, userID, roles, err := v.Validate(r)
			if err != nil {
				http.Error(w, `{"error":"unauthorized: "+`+quote(err.Error())+`}`, http.StatusUnauthorized)
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

// TenantMiddleware validates that the X-Tenant-ID header matches the JWT-decoded tenant.
func TenantMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headerTenant := r.Header.Get("X-Tenant-ID")
			if headerTenant == "" {
				http.Error(w, `{"error":"missing X-Tenant-ID header"}`, http.StatusUnauthorized)
				return
			}

			tenantFromJWT, ok := r.Context().Value(ctxkeys.TenantIDKey).(string)
			if !ok || tenantFromJWT == "" {
				http.Error(w, `{"error":"tenant context missing"}`, http.StatusUnauthorized)
				return
			}

			if headerTenant != tenantFromJWT {
				http.Error(w, `{"error":"tenant mismatch"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// quote wraps a string in double quotes for JSON embedding.
func quote(s string) string {
	result := "\""
	for _, c := range s {
		switch c {
		case '"':
			result += `\"`
		case '\\':
			result += `\\`
		case '\n':
			result += `\n`
		case '\r':
			result += `\r`
		case '\t':
			result += `\t`
		default:
			result += string(c)
		}
	}
	result += "\""
	return result
}