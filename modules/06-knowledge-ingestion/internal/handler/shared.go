package handler

import (
	"encoding/json"
	"net/http"
)

// WriteJSON serializes data as JSON and writes it with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{
		"error": message,
	})
}