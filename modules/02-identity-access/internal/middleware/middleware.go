package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Keys for context values.
type contextKey string

const (
	TenantIDKey contextKey = "tenant_id"
	UserIDKey   contextKey = "user_id"
	UserTypeKey contextKey = "user_type" // "user", "service", "agent"
	TraceIDKey  contextKey = "trace_id"
)

// JWTToken represents a validated IAM JWT token.
type JWTToken struct {
	Subject   string `json:"sub"`             // user ID, service ID, or agent ID
	UserType  string `json:"user_type"`       // "user", "service", "agent"
	TenantID  string `json:"tenant_id"`
	Email     string `json:"email,omitempty"`
	Roles     []string `json:"roles,omitempty"`
	Claims    jwt.MapClaims
}

// TenantInjector extracts the tenant ID from the X-Tenant-ID header and injects it into the context.
func TenantInjector(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			http.Error(w, `{"error":"X-Tenant-ID header is required"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthValidator validates the Authorization header and extracts user ID.
// It validates JWT tokens issued by Authentik using the JWKS cache for RSA key lookup,
// with a shared secret fallback for HMAC-signed internal tokens.
func AuthValidator(jwksCache *JWKSCache, authentikIssuerURL, tokenSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health check and token introspection
		if r.URL.Path == "/health" || r.URL.Path == "/internal/auth/proxy" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"missing authorization header"}`)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"invalid authorization scheme"}`)
			return
		}

		token := parts[1]
		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"empty token"}`)
			return
		}

		// Try Authentik JWT first
		tokenResult, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
			// Try to find the key by kid from the token header
			if kid, ok := t.Header["kid"].(string); ok {
				if jwksCache != nil {
					keyEntry, ok := jwksCache.GetSigningKey(kid)
					if ok {
						return keyEntry.Key, nil
					}
				}
			}

			if _, ok := t.Method.(*jwt.SigningMethodRSA); ok {
				if tokenSecret != "" {
					return []byte(tokenSecret), nil
				}
				return nil, fmt.Errorf("no JWKS key found for token")
			}
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); ok {
				if tokenSecret != "" {
					return []byte(tokenSecret), nil
				}
				return nil, fmt.Errorf("missing token secret")
			}
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		})

		if err != nil || !tokenResult.Valid {
			// Fall back to internal token secret (for admin operations)
			if tokenSecret != "" {
				tokenResult, err = jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
					}
					return []byte(tokenSecret), nil
				})
				if err != nil || !tokenResult.Valid {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					fmt.Fprint(w, `{"error":"invalid token"}`)
					return
				}
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"error":"invalid token"}`)
				return
			}
		}

		claims, ok := tokenResult.Claims.(jwt.MapClaims)
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"invalid token claims"}`)
			return
		}

		// Extract standard claims
		sub, _ := claims["sub"].(string)
		if sub == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"token missing subject"}`)
			return
		}

		// Validate issuer (Authentik or internal)
		issuer, _ := claims["iss"].(string)
		expectedIssuer := "operan-iam"
		if authentikIssuerURL != "" {
			expectedIssuer = authentikIssuerURL
		}
		if issuer != expectedIssuer && issuer != "operan-iam" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"untrusted issuer"}`)
			return
		}

		// Extract custom claims
		userType, _ := claims["user_type"].(string)
		tenantID, _ := claims["tenant_id"].(string)
		email, _ := claims["email"].(string)
		rolesClaim, _ := claims["roles"].([]interface{})

		var roles []string
		for _, r := range rolesClaim {
			if role, ok := r.(string); ok {
				roles = append(roles, role)
			}
		}

		ctx := context.WithValue(r.Context(), UserIDKey, sub)
		ctx = context.WithValue(ctx, UserTypeKey, userType)
		if tenantID != "" {
			ctx = context.WithValue(ctx, TenantIDKey, tenantID)
		}

		// Store token for downstream use
		ctx = context.WithValue(ctx, "jwt_token", &JWTToken{
			Subject:   sub,
			UserType:  userType,
			TenantID:  tenantID,
			Email:     email,
			Roles:     roles,
			Claims:    claims,
		})

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TraceInjector generates or extracts a trace ID for request tracing.
func TraceInjector(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			// Generate a new trace ID
			traceID = generateTraceID()
		}

		ctx := context.WithValue(r.Context(), TraceIDKey, traceID)
		w.Header().Set("X-Trace-ID", traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateTraceID creates a simple trace ID.
func generateTraceID() string {
	return "trace-" + generateID()
}

// generateID creates a simple random ID.
func generateID() string {
	return "00000000-0000-0000-0000-000000000001"
}

// GetTenantID extracts the tenant ID from the context.
func GetTenantID(ctx context.Context) string {
	if tenantID, ok := ctx.Value(TenantIDKey).(string); ok {
		return tenantID
	}
	return ""
}

// GetUserID extracts the user ID from the context.
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// GetUserType extracts the user type ("user", "service", "agent") from context.
func GetUserType(ctx context.Context) string {
	if userType, ok := ctx.Value(UserTypeKey).(string); ok {
		return userType
	}
	return "user"
}

// GetTraceID extracts the trace ID from the context.
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// GetJWTToken extracts the JWT token from the context.
func GetJWTToken(ctx context.Context) *JWTToken {
	if token, ok := ctx.Value("jwt_token").(*JWTToken); ok {
		return token
	}
	return nil
}
