// Package handlers implements the HTTP handlers for Module 07 (Memory Fabric).
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/operan/modules/07-memory-fabric/internal/events"
	"github.com/operan/modules/07-memory-fabric/internal/middleware"
	"github.com/operan/modules/07-memory-fabric/internal/store"
)

// MemoryHandlers bundles the stores and publisher used by all endpoints.
type MemoryHandlers struct {
	Vectors     *store.VectorStore
	Policies    *store.PolicyStore
	Operations  *store.OperationStore
	Publisher   *events.Publisher
	MaxPageSize int
	GCBatchSize int
}

// NewMemoryHandlers constructs a MemoryHandlers.
func NewMemoryHandlers(v *store.VectorStore, p *store.PolicyStore, o *store.OperationStore, pub *events.Publisher, maxPageSize, gcBatchSize int) *MemoryHandlers {
	if maxPageSize <= 0 {
		maxPageSize = 100
	}
	if gcBatchSize <= 0 {
		gcBatchSize = 1000
	}
	return &MemoryHandlers{Vectors: v, Policies: p, Operations: o, Publisher: pub, MaxPageSize: maxPageSize, GCBatchSize: gcBatchSize}
}

// ─── Shared helpers ──────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError emits the contract Error schema:
// { code: int, message: string, details: string, request_id: uuid }.
func writeError(w http.ResponseWriter, r *http.Request, status int, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":       status,
		"message":    http.StatusText(status),
		"details":    details,
		"request_id": middleware.RequestIDFromContext(r.Context()),
	})
}

// pagination parses page/page_size query params, clamped to [1, max].
func (h *MemoryHandlers) pagination(r *http.Request) (int, int) {
	page := 1
	pageSize := 20
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pageSize = n
		}
	}
	if pageSize > h.MaxPageSize {
		pageSize = h.MaxPageSize
	}
	return page, pageSize
}

func queryPtr(r *http.Request, key string) *string {
	if v := r.URL.Query().Get(key); v != "" {
		return &v
	}
	return nil
}
