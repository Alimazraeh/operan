package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/operan/modules/07-memory-fabric/internal/events"
	"github.com/operan/modules/07-memory-fabric/internal/middleware"
	"github.com/operan/modules/07-memory-fabric/internal/store"
)

// semanticSearchRequest matches the OpenAPI SemanticSearchRequest schema.
type semanticSearchRequest struct {
	TenantID           string                 `json:"tenant_id"`
	Query              string                 `json:"query"`
	EmbeddingType      string                 `json:"embedding_type"`
	TopN               int                    `json:"top_n"`
	RelevanceThreshold float64                `json:"relevance_threshold"`
	VectorIDs          []string               `json:"vector_ids"`
	Filters            map[string]interface{} `json:"filters"`
	IncludeContext     *bool                  `json:"include_context"`
	QueryVector        []float64              `json:"query_vector"` // optional pre-computed embedding
}

// SearchMemory handles POST /search.
func (h *MemoryHandlers) SearchMemory(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	started := time.Now()

	var req semanticSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if req.TenantID != "" && req.TenantID != tenantID {
		writeError(w, r, http.StatusForbidden, "tenant_id in body does not match authenticated tenant")
		return
	}
	if req.Query == "" {
		writeError(w, r, http.StatusBadRequest, "query is required")
		return
	}
	if !store.ValidEmbeddingType(req.EmbeddingType) {
		writeError(w, r, http.StatusBadRequest, "invalid embedding_type")
		return
	}
	if req.TopN <= 0 {
		req.TopN = 10
	}
	if req.RelevanceThreshold < 0 || req.RelevanceThreshold > 1 {
		writeError(w, r, http.StatusBadRequest, "relevance_threshold must be between 0 and 1")
		return
	}

	// Vectorize the query through the embeddings gateway when the caller
	// didn't supply a vector; failures fall back to token-overlap scoring.
	queryVector := req.QueryVector
	if len(queryVector) == 0 && h.Embedder != nil {
		if vecs, err := h.Embedder.Embed(r.Context(), []string{req.Query}); err == nil && len(vecs) == 1 {
			queryVector = vecs[0]
		}
	}

	scored := h.Vectors.Search(tenantID, req.Query, queryVector, req.EmbeddingType, req.TopN, req.RelevanceThreshold, req.VectorIDs)

	items := make([]map[string]interface{}, 0, len(scored))
	for i, sv := range scored {
		items = append(items, map[string]interface{}{
			"id":              uuid.New().String(),
			"vector_id":       sv.Vector.ID,
			"score":           sv.Score,
			"content":         sv.Vector.SemanticContent,
			"metadata":        sv.Vector.Metadata,
			"source_type":     sourceTypeFor(sv.Vector.SegmentType),
			"document_id":     sv.Vector.DocumentID,
			"rank":            i + 1,
			"embedding_model": sv.Vector.EmbeddingModel,
		})
	}

	h.Publisher.PublishVectorSearched(events.MemoryVectorSearchedPayload{
		QueryID:        uuid.New().String(),
		TenantID:       tenantID,
		QueryType:      req.EmbeddingType,
		TopN:           req.TopN,
		ResultsCount:   len(items),
		ResponseTimeMs: int(time.Since(started).Milliseconds()),
	}, middleware.TraceIDFromContext(r.Context()))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":     items,
		"total":     len(items),
		"page":      1,
		"page_size": req.TopN,
		"has_more":  false,
	})
}

// sourceTypeFor maps a stored segment type to the SearchResult source_type
// enum [knowledge, memory, tool_output, policy].
func sourceTypeFor(s store.SegmentType) string {
	switch s {
	case store.SegmentToolOutput:
		return "tool_output"
	case store.SegmentPolicy:
		return "policy"
	case store.SegmentFact:
		return "knowledge"
	default: // context, memory, unset
		return "memory"
	}
}
