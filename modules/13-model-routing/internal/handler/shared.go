package handler

import (
	"encoding/json"
	"net/http"
)

// PaginatedResponse is the standard platform paginated response wrapper.
type PaginatedResponse struct {
	Data  interface{} `json:"data"`
	Page  int         `json:"page"`
	PageSize int       `json:"page_size"`
	Total  int         `json:"total"`
}

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}