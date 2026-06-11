package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/operan/modules/07-memory-fabric/internal/events"
	"github.com/operan/modules/07-memory-fabric/internal/middleware"
	"github.com/operan/modules/07-memory-fabric/internal/store"
)

// vectorWriteItem matches the OpenAPI VectorWriteItem schema.
type vectorWriteItem struct {
	DocumentID      string                 `json:"document_id"`
	EmbeddingType   string                 `json:"embedding_type"`
	SemanticContent string                 `json:"semantic_content"`
	Metadata        map[string]interface{} `json:"metadata"`
	ChunkID         string                 `json:"chunk_id"`
	SegmentType     string                 `json:"segment_type"`
	EmbeddingVector []float64              `json:"embedding_vector"`
	TTL             *time.Time             `json:"ttl"`
}

// vectorIngestRequest matches the OpenAPI VectorIngestRequest schema.
type vectorIngestRequest struct {
	TenantID string            `json:"tenant_id"`
	Items    []vectorWriteItem `json:"items"`
}

// IngestVectors handles POST /vectors.
func (h *MemoryHandlers) IngestVectors(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req vectorIngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if req.TenantID != "" && req.TenantID != tenantID {
		writeError(w, r, http.StatusForbidden, "tenant_id in body does not match authenticated tenant")
		return
	}
	if len(req.Items) == 0 {
		writeError(w, r, http.StatusBadRequest, "items must contain at least one vector")
		return
	}

	// Vectorize items that arrived without embeddings (best effort: an
	// embeddings outage degrades search to token overlap, never fails ingest).
	var computed [][]float64
	embeddingModel := ""
	if h.Embedder != nil {
		var texts []string
		for _, item := range req.Items {
			if len(item.EmbeddingVector) == 0 && item.SemanticContent != "" {
				texts = append(texts, item.SemanticContent)
			}
		}
		if len(texts) > 0 {
			if vecs, err := h.Embedder.Embed(r.Context(), texts); err == nil {
				computed = vecs
				embeddingModel = h.Embedder.Model()
			}
		}
	}

	ingested, failed := 0, 0
	nextComputed := 0
	var errs []string
	for i, item := range req.Items {
		vector := item.EmbeddingVector
		model := ""
		if len(vector) == 0 && item.SemanticContent != "" && nextComputed < len(computed) {
			vector = computed[nextComputed]
			model = embeddingModel
			nextComputed++
		}
		v := &store.MemoryVector{
			TenantID:        tenantID,
			DocumentID:      item.DocumentID,
			EmbeddingType:   store.EmbeddingType(item.EmbeddingType),
			SemanticContent: item.SemanticContent,
			Metadata:        item.Metadata,
			ChunkID:         item.ChunkID,
			SegmentType:     store.SegmentType(item.SegmentType),
			EmbeddingVector: vector,
			EmbeddingModel:  model,
			TTL:             item.TTL,
		}
		created, err := h.Vectors.Create(v)
		if err != nil {
			failed++
			errs = append(errs, fmt.Sprintf("item %d: %v", i, err))
			continue
		}
		ingested++
		h.Publisher.PublishVectorIngested(events.MemoryVectorIngestedPayload{
			VectorID:            created.ID,
			TenantID:            tenantID,
			DocumentID:          created.DocumentID,
			ChunkID:             created.ChunkID,
			EmbeddingModel:      created.EmbeddingModel,
			EmbeddingDimensions: len(created.EmbeddingVector),
			SegmentType:         string(created.SegmentType),
			CreatedAt:           created.CreatedAt.Format(time.RFC3339),
		}, middleware.TraceIDFromContext(r.Context()))
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"ingested": ingested,
		"failed":   failed,
		"errors":   errs,
	})
}

// ListVectors handles GET /vectors.
func (h *MemoryHandlers) ListVectors(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	page, pageSize := h.pagination(r)

	if v := queryPtr(r, "embedding_type"); v != nil && !store.ValidEmbeddingType(*v) {
		writeError(w, r, http.StatusBadRequest, "invalid embedding_type filter")
		return
	}
	if v := queryPtr(r, "segment_type"); v != nil && !store.ValidSegmentType(*v) {
		writeError(w, r, http.StatusBadRequest, "invalid segment_type filter")
		return
	}

	items, total, hasMore := h.Vectors.List(tenantID, page, pageSize,
		queryPtr(r, "embedding_type"), queryPtr(r, "segment_type"), queryPtr(r, "document_id"))

	if items == nil {
		items = []store.MemoryVector{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"has_more":  hasMore,
	})
}

// GetVector handles GET /vectors/{id}.
func (h *MemoryHandlers) GetVector(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	v, err := h.Vectors.GetByIDAndTenant(r.PathValue("id"), tenantID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "vector not found")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// vectorUpdateRequest matches the OpenAPI VectorUpdateRequest schema.
// TTL uses json.RawMessage to distinguish absent from explicit null.
type vectorUpdateRequest struct {
	SemanticContent *string                `json:"semantic_content"`
	Metadata        map[string]interface{} `json:"metadata"`
	SegmentType     *string                `json:"segment_type"`
	TTL             json.RawMessage        `json:"ttl"`
}

// UpdateVector handles PUT /vectors/{id}.
func (h *MemoryHandlers) UpdateVector(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req vectorUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if req.SegmentType != nil && !store.ValidSegmentType(*req.SegmentType) {
		writeError(w, r, http.StatusBadRequest, "invalid segment_type")
		return
	}

	var ttl *time.Time
	ttlSet := len(req.TTL) > 0
	if ttlSet && string(req.TTL) != "null" {
		var t time.Time
		if err := json.Unmarshal(req.TTL, &t); err != nil {
			writeError(w, r, http.StatusBadRequest, "ttl must be an RFC 3339 date-time or null")
			return
		}
		ttl = &t
	}

	updateType := "metadata_refresh"
	updated, err := h.Vectors.Update(r.PathValue("id"), tenantID, func(v *store.MemoryVector) error {
		if req.SemanticContent != nil {
			v.SemanticContent = *req.SemanticContent
			updateType = "content_update"
		}
		if req.Metadata != nil {
			v.Metadata = req.Metadata
		}
		if req.SegmentType != nil {
			v.SegmentType = store.SegmentType(*req.SegmentType)
		}
		if ttlSet {
			v.TTL = ttl
		}
		return nil
	})
	if err != nil {
		writeError(w, r, http.StatusNotFound, "vector not found")
		return
	}

	h.Publisher.PublishVectorUpdated(events.MemoryVectorUpdatedPayload{
		VectorID:   updated.ID,
		TenantID:   tenantID,
		DocumentID: updated.DocumentID,
		UpdateType: updateType,
		UpdatedBy:  middleware.UserIDFromContext(r.Context()),
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
	}, middleware.TraceIDFromContext(r.Context()))

	writeJSON(w, http.StatusOK, updated)
}

// DeleteVector handles DELETE /vectors/{id}.
func (h *MemoryHandlers) DeleteVector(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	id := r.PathValue("id")

	if err := h.Vectors.Delete(id, tenantID); err != nil {
		writeError(w, r, http.StatusNotFound, "vector not found")
		return
	}

	h.Publisher.PublishVectorDeleted(events.MemoryVectorDeletedPayload{
		VectorIDs: []string{id},
		TenantID:  tenantID,
		Reason:    "document_deleted",
		DeletedBy: middleware.UserIDFromContext(r.Context()),
		DeletedAt: time.Now().UTC().Format(time.RFC3339),
	}, middleware.TraceIDFromContext(r.Context()))

	w.WriteHeader(http.StatusNoContent)
}
