package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/operan/modules/07-memory-fabric/internal/events"
	"github.com/operan/modules/07-memory-fabric/internal/middleware"
	"github.com/operan/modules/07-memory-fabric/internal/store"
)

// gcRequest matches the OpenAPI GCRequest schema.
type gcRequest struct {
	MemoryType string `json:"memory_type"`
	MaxAgeDays int    `json:"max_age_days"`
	DryRun     bool   `json:"dry_run"`
}

// TriggerGC handles POST /gc. The collection runs synchronously (the
// in-memory store makes it fast); the response is the resulting
// OperationStatus per the contract's 202 semantics.
func (h *MemoryHandlers) TriggerGC(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req gcRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			writeError(w, r, http.StatusBadRequest, "invalid JSON body: "+err.Error())
			return
		}
	}
	var embeddingType *store.EmbeddingType
	if req.MemoryType != "" {
		if !store.ValidMemoryType(req.MemoryType) {
			writeError(w, r, http.StatusBadRequest, "invalid memory_type")
			return
		}
		et := store.EmbeddingTypeForMemoryType(store.MemoryType(req.MemoryType))
		embeddingType = &et
	}
	if req.MaxAgeDays < 0 {
		writeError(w, r, http.StatusBadRequest, "max_age_days must not be negative")
		return
	}

	op := h.Operations.Start(tenantID)
	collected := h.Vectors.CollectExpired(tenantID, embeddingType, req.MaxAgeDays, h.GCBatchSize, req.DryRun)
	h.Operations.Complete(op.ID, len(collected))

	if !req.DryRun && len(collected) > 0 {
		h.Publisher.PublishVectorGarbageCollected(events.MemoryVectorGarbageCollectedPayload{
			BatchSize:   len(collected),
			DeletedAt:   time.Now().UTC().Format(time.RFC3339),
			Reason:      "cleanup_job",
			TriggeredBy: middleware.UserIDFromContext(r.Context()),
		}, tenantID, middleware.TraceIDFromContext(r.Context()))
	}

	final, _ := h.Operations.Get(op.ID)
	writeJSON(w, http.StatusAccepted, final)
}
