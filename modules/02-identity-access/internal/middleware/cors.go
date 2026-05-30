package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig holds the configuration for the CORS middleware.
type CORSConfig struct {
	AllowedOrigins   []string // Allowed request origins (default: ["*"])
	AllowedMethods   []string // Allowed HTTP methods (default: GET, POST, PUT, PATCH, DELETE, OPTIONS)
	AllowedHeaders   []string // Allowed request headers (default: ["*"])
	AllowCredentials bool     // Whether to allow credentials (default: false)
	MaxAge           int      // Max age of preflight cache in seconds (default: 86400 — 24h)
}

// DefaultCORSConfig returns a CORSConfig with sensible defaults.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		AllowCredentials: false,
		MaxAge:         86400,
	}
}

// originsContains checks if an origin is in the allowed origins list.
// It handles wildcard "*" matching: if "*" is in the list, any origin is allowed.
// Returns true and whether it was a wildcard match.
func originsContains(allowed []string, origin string) (bool, bool) {
	for _, a := range allowed {
		if a == "*" {
			return true, true
		}
		if strings.EqualFold(a, origin) {
			return true, false
		}
	}
	return false, false
}

// methodsContains checks if a method is in the allowed methods list.
func methodsContains(allowed []string, method string) bool {
	for _, a := range allowed {
		if strings.EqualFold(a, method) {
			return true
		}
	}
	return false
}

// headersContains checks if a header name is in the allowed headers list.
func headersContains(allowed []string, header string) bool {
	for _, a := range allowed {
		if a == "*" {
			return true
		}
		if strings.EqualFold(a, header) {
			return true
		}
	}
	return false
}

// CORS returns middleware that adds CORS headers to responses.
//
// For OPTIONS preflight requests:
//   - Validates the Origin, requested method, and requested headers against config.
//   - Returns 204 No Content with appropriate CORS headers.
//   - Returns 403 Forbidden if validation fails.
//
// For regular requests:
//   - Adds Access-Control-Allow-Origin, Allow-Credentials, and Max-Age headers
//     only when the origin is allowed.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	// Normalize defaults if config is partially zero-valued.
	if len(cfg.AllowedOrigins) == 0 {
		cfg.AllowedOrigins = []string{"*"}
	}
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}
	if len(cfg.AllowedHeaders) == 0 {
		cfg.AllowedHeaders = []string{"*"}
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 86400
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Skip CORS processing if no Origin header (simple browser request or non-browser client).
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Handle OPTIONS preflight requests.
			if r.Method == http.MethodOptions {
				allowed, isWildcard := originsContains(cfg.AllowedOrigins, origin)
				if !allowed {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"error":"origin not allowed"}`))
					return
				}

				// Validate requested method (Access-Control-Request-Method).
				requestMethod := r.Header.Get("Access-Control-Request-Method")
				if requestMethod != "" && !methodsContains(cfg.AllowedMethods, requestMethod) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"error":"method not allowed"}`))
					return
				}

				// Validate requested headers (Access-Control-Request-Headers).
				requestHeaders := r.Header.Get("Access-Control-Request-Headers")
				if requestHeaders != "" {
					for _, h := range strings.Split(requestHeaders, ",") {
						h = strings.TrimSpace(h)
						if !headersContains(cfg.AllowedHeaders, h) {
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(http.StatusForbidden)
							_, _ = w.Write([]byte(`{"error":"header not allowed"}`))
							return
						}
					}
				}

				// Per CORS spec, when AllowCredentials is true, the origin cannot be "*".
				// Echo back the actual requesting origin instead.
				preflightOrigin := origin
				if isWildcard && cfg.AllowCredentials {
					preflightOrigin = origin
				}

				// Set CORS headers for preflight response.
				w.Header().Set("Access-Control-Allow-Origin", preflightOrigin)
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				w.WriteHeader(http.StatusNoContent)
				return
			}

			// Regular request — add CORS headers if origin is allowed.
			allowed, isWildcard := originsContains(cfg.AllowedOrigins, origin)
			if allowed {
				// Per CORS spec, when AllowCredentials is true, the origin cannot be "*".
				// Echo back the actual requesting origin instead.
				responseOrigin := origin
				if isWildcard && !cfg.AllowCredentials {
					// Wildcard with no credentials: echo "*" as per CORS spec.
					responseOrigin = "*"
				}
				w.Header().Set("Access-Control-Allow-Origin", responseOrigin)
				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
			}

			next.ServeHTTP(w, r)
		})
	}
}
