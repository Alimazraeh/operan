package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/operan/modules/07-memory-fabric/internal/middleware"
	"github.com/operan/modules/07-memory-fabric/internal/store"
)

// retentionPolicyRequest matches the OpenAPI RetentionPolicy schema
// (id and creation_date are server-assigned on create).
type retentionPolicyRequest struct {
	TenantID       string `json:"tenant_id"`
	MemoryType     string `json:"memory_type"`
	MaxAgeDays     int    `json:"max_age_days"`
	MaxMemoryCount int    `json:"max_memory_count"`
	TTLSeconds     int    `json:"ttl_seconds"`
	AutoGCEnabled  bool   `json:"auto_gc_enabled"`
}

// CreateRetentionPolicy handles POST /retention-policies.
func (h *MemoryHandlers) CreateRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req retentionPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if req.TenantID != "" && req.TenantID != tenantID {
		writeError(w, r, http.StatusForbidden, "tenant_id in body does not match authenticated tenant")
		return
	}
	if !store.ValidMemoryType(req.MemoryType) {
		writeError(w, r, http.StatusBadRequest, "invalid memory_type")
		return
	}
	if req.MaxAgeDays < 0 || req.MaxMemoryCount < 0 || req.TTLSeconds < 0 {
		writeError(w, r, http.StatusBadRequest, "max_age_days, max_memory_count, and ttl_seconds must not be negative")
		return
	}

	created, err := h.Policies.Create(&store.RetentionPolicy{
		TenantID:       tenantID,
		MemoryType:     store.MemoryType(req.MemoryType),
		MaxAgeDays:     req.MaxAgeDays,
		MaxMemoryCount: req.MaxMemoryCount,
		TTLSeconds:     req.TTLSeconds,
		AutoGCEnabled:  req.AutoGCEnabled,
	})
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// ListRetentionPolicies handles GET /retention-policies.
func (h *MemoryHandlers) ListRetentionPolicies(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	page, pageSize := h.pagination(r)

	items, total, hasMore := h.Policies.List(tenantID, page, pageSize)
	if items == nil {
		items = []store.RetentionPolicy{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"has_more":  hasMore,
	})
}
