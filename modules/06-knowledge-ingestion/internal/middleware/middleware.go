package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/operan/modules/06-knowledge-ingestion/internal/config"
	"github.com/operan/modules/06-knowledge-ingestion/internal/ctxkeys"

	"github.com/golang-jwt/jwt/v5"
)

// AuthValidator validates JWT tokens from Bearer headers.
type AuthValidator struct {
	secret string
	// Issuer is the expected JWT issuer prefix (e.g., "operan-").
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

	// Extract tenant_id.
	tenantID, ok := claims["tenant_id"].(string)
	if !ok || tenantID == "" {
		tenantID, _ = claims["x_tenant_id"].(string)
	}
	if tenantID == "" {
		return "", "", nil, fmt.Errorf("JWT missing tenant_id claim")
	}

	// Extract user_id.
	userID, _ := claims["sub"].(string)
	if userID == "" {
		userID, _ = claims["user_id"].(string)
	}

	// Extract roles.
	var roles []string
	if rolesAny, ok := claims["roles"].([]any); ok {
		for _, r := range rolesAny {
			if rs, ok := r.(string); ok {
				roles = append(roles, rs)
			}
		}
	}

	// Validate issuer.
	issuer, _ := claims["iss"].(string)
	if issuer != "" && !strings.HasPrefix(issuer, v.Issuer) {
		return "", "", nil, fmt.Errorf("JWT issuer %q does not match expected prefix %q", issuer, v.Issuer)
	}

	return tenantID, userID, roles, nil
}

// JWTMiddleware creates an HTTP middleware that validates JWT and injects tenant/user context.
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

// RBACMiddleware checks that the user has at least one of the required roles.
func RBACMiddleware(requiredRoles ...string) func(http.Handler) http.Handler {
	requiredSet := make(map[string]struct{}, len(requiredRoles))
	for _, r := range requiredRoles {
		requiredSet[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rolesAny := r.Context().Value(ctxkeys.RolesKey)
			if rolesAny == nil {
				http.Error(w, `{"error":"forbidden: no roles"}`, http.StatusForbidden)
				return
			}

			userRoles, ok := rolesAny.([]string)
			if !ok {
				http.Error(w, `{"error":"forbidden: invalid roles"}`, http.StatusForbidden)
				return
			}

			for _, role := range userRoles {
				if _, required := requiredSet[role]; required {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, `{"error":"forbidden: insufficient permissions"}`, http.StatusForbidden)
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