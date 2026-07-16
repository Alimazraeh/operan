package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/operan/arabic-language-core/internal/clients"
	"github.com/operan/arabic-language-core/internal/events"
	"github.com/operan/arabic-language-core/internal/store"
	"github.com/operan/arabic-language-core/internal/ctxkeys"
)

// EmbedArabicRequest is the request body for POST /v1/embeddings.
type EmbedArabicRequest struct {
	Text  string `json:"text"`
	Model string `json:"model"`
}

// EmbedArabicResponse is the response for POST /v1/embeddings.
type EmbedArabicResponse struct {
	Embedding  []float64 `json:"embedding"`
	Model      string    `json:"model"`
	Tokens     int       `json:"tokens"`
	DurationMs int       `json:"duration_ms"`
}

// HandleEmbedArabic delegates embedding to M12.
func HandleEmbedArabic(w http.ResponseWriter, r *http.Request, m12 *clients.M12Client, broker *events.Broker, dbStore *store.TerminologyStore, jwtSecret string) {
	var req EmbedArabicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text is required"})
		return
	}

	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	model := req.Model
	if model == "" {
		model = "arabic-embedding-v1"
	}

	start := time.Now()

	if m12 == nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "M12 client not configured"})
		return
	}

	jwtToken := r.Header.Get("Authorization")
	if jwtToken == "" {
		jwtToken = "service-19to12"
	}

	embedResp, err := m12.EmbedArabic(r.Context(), tenantID, model, req.Text, jwtToken)
	durationMs := int(time.Since(start).Milliseconds())

	logReq := &store.ArabicEmbeddingRequest{
		TenantID:       tenantID,
		TextLength:     len(req.Text),
		EmbeddingModel: model,
		Status:         "success",
		DurationMs:     durationMs,
	}

	if err != nil {
		logReq.Status = "failed"
		logReq.ErrorMessage = err.Error()
		if dbStore != nil {
			_ = dbStore.LogEmbeddingRequest(r.Context(), logReq)
		}
		if broker != nil {
			_ = broker.EmbeddingRequested(r.Context(), tenantID, len(req.Text), model, "failed")
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "M12 call failed: " + err.Error()})
		return
	}

	logReq.Status = "success"
	if len(embedResp.Data) > 0 {
		logReq.VectorDim = len(embedResp.Data[0].Embedding)
	}

	if dbStore != nil {
		_ = dbStore.LogEmbeddingRequest(r.Context(), logReq)
	}
	if broker != nil {
		_ = broker.EmbeddingRequested(r.Context(), tenantID, len(req.Text), model, "success")
	}

	embedding := []float64{}
	if len(embedResp.Data) > 0 {
		embedding = embedResp.Data[0].Embedding
	}

	writeJSON(w, http.StatusOK, EmbedArabicResponse{
		Embedding:  embedding,
		Model:      embedResp.Model,
		Tokens:     len(embedding),
		DurationMs: durationMs,
	})

	// Unused params
	_ = jwtSecret
}