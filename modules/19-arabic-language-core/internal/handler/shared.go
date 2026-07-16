package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/operan/arabic-language-core/internal/ctxkeys"
)

// WriteJSON is a helper that writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeJSON is an alias for WriteJSON (kept for internal use).
var writeJSON = WriteJSON

// userID safely extracts user ID from context.
func userID(ctx context.Context) string {
	uid, ok := ctx.Value(ctxkeys.UserIDKey).(string)
	if !ok || uid == "" {
		return "unknown"
	}
	return uid
}

// Logger is a simple logging interface.
type Logger interface {
	Printf(format string, v ...any)
}

// contains checks if needle is a substring of haystack.
func contains(haystack, needle string) bool {
	if needle == "" || len(needle) > len(haystack) {
		return false
	}
	return strings.Contains(haystack, needle)
}