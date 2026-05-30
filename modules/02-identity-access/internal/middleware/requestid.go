package middleware

import (
	"net/http"

	"github.com/google/uuid"
)

// RequestIDMiddleware intercepts requests and ensures every request has a unique
// request ID. It preserves an existing X-Request-ID header if present, otherwise
// generates a new UUID v4. The ID is stored in context and propagated to the
// response headers so downstream services and log consumers can correlate traces.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx := SetRequestID(r.Context(), requestID)
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
