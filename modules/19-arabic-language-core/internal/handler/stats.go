package handler

import (
	"net/http"
)

// HandleStats returns module statistics.
func HandleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"module":      "arabic-language-core",
		"version":     "1.0.0",
		"endpoints":   8,
		"description": "Arabic Language Core — normalization, dialect detection, terminology governance, embeddings",
	})
}