package handler

import (
	"encoding/json"
	"net/http"

	"github.com/operan/arabic-language-core/internal/nlp"
)

// DetectDialectRequest is the request body for POST /v1/detect-dialect.
type DetectDialectRequest struct {
	Text string `json:"text"`
}

// DetectDialectResponse is the response for POST /v1/detect-dialect.
type DetectDialectResponse struct {
	Dialect    string                `json:"dialect"`
	Confidence float64               `json:"confidence"`
	AllScores  []nlp.KeywordResult   `json:"all_scores"`
	IsMSA      bool                  `json:"is_msa"`
}

// HandleDetectDialect processes dialect detection requests.
func HandleDetectDialect(w http.ResponseWriter, r *http.Request) {
	var req DetectDialectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	result := nlp.DetectDialect(req.Text)
	writeJSON(w, http.StatusOK, DetectDialectResponse{
		Dialect:    result.Dialect,
		Confidence: result.Confidence,
		AllScores:  result.AllScores,
		IsMSA:      result.IsMSA,
	})
}