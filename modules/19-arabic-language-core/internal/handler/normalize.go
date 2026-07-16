package handler

import (
	"encoding/json"
	"net/http"

	"github.com/operan/arabic-language-core/internal/nlp"
)

// NormalizeRequest is the request body for POST /v1/normalize.
type NormalizeRequest struct {
	Text        string `json:"text"`
	RemoveTashkeel *bool  `json:"remove_tashkeel,omitempty"`
	NormalizeAlef    *bool  `json:"normalize_alef,omitempty"`
	ConvertNumerals  *bool  `json:"convert_numerals,omitempty"`
}

// NormalizeResponse is the response for POST /v1/normalize.
type NormalizeResponse struct {
	Normalized       string                      `json:"normalized"`
	Actions          []nlp.NormalizationAction   `json:"actions"`
	OriginalLength   int                         `json:"original_length"`
	NormalizedLength int                         `json:"normalized_length"`
}

// HandleNormalize processes normalization requests.
func HandleNormalize(w http.ResponseWriter, r *http.Request) {
	var req NormalizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	normalizer := nlp.DefaultNormalizer()

	if req.RemoveTashkeel != nil {
		normalizer.RemoveTashkeel = *req.RemoveTashkeel
	}
	if req.NormalizeAlef != nil {
		normalizer.NormalizeAlef = *req.NormalizeAlef
	}

	result := normalizer.Normalize(req.Text)
	writeJSON(w, http.StatusOK, NormalizeResponse{
		Normalized:       result.Normalized,
		Actions:          result.Actions,
		OriginalLength:   result.OriginalLength,
		NormalizedLength: result.NormalizedLength,
	})
}