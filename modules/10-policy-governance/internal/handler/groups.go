package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/operan/policy-governance/internal/ctxkeys"
	"github.com/operan/policy-governance/internal/store"
)

// GroupHandler handles policy group CRUD endpoints.
type GroupHandler struct {
	store *store.GroupStore
}

// NewGroupHandler creates a new GroupHandler.
func NewGroupHandler(store *store.GroupStore) *GroupHandler {
	return &GroupHandler{store: store}
}

// CreateGroup handles POST /v1/policy-groups
func (h *GroupHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())

	var req struct {
		Name        string                 `json:"name"`
		Description *string                `json:"description"`
		Priority    *int                   `json:"priority"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "invalid JSON body",
		})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "name is required",
		})
		return
	}

	priority := 50
	if req.Priority != nil {
		priority = *req.Priority
	}

	group := &store.PolicyGroup{
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Priority:    priority,
		IsActive:    true,
		Metadata:    req.Metadata,
	}

	if err := h.store.Create(r.Context(), group); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":          group.ID,
		"name":        group.Name,
		"priority":    group.Priority,
		"is_active":   group.IsActive,
		"created_at":  group.CreatedAt,
		"updated_at":  group.UpdatedAt,
	})
}

// ListGroups handles GET /v1/policy-groups
func (h *GroupHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	groups, total, err := h.store.List(r.Context(), tenantID, page, pageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}

	result := make([]map[string]interface{}, len(groups))
	for i, g := range groups {
		result[i] = map[string]interface{}{
			"id":        g.ID,
			"name":      g.Name,
			"priority":  g.Priority,
			"is_active": g.IsActive,
			"created_at": g.CreatedAt,
			"updated_at": g.UpdatedAt,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"groups":  result,
		"page":    page,
		"page_size": pageSize,
		"total":   total,
		"has_more": (page * pageSize) < total,
	})
}

// GetGroup handles GET /v1/policy-groups/{id}
func (h *GroupHandler) GetGroup(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "invalid group ID",
		})
		return
	}

	group, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error":   "not-found",
				"message": "policy group not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}
	if group.TenantID != tenantID {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error":   "forbidden",
			"message": "tenant mismatch",
		})
		return
	}

	writeJSON(w, http.StatusOK, group)
}

// UpdateGroup handles PATCH /v1/policy-groups/{id}
func (h *GroupHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "invalid group ID",
		})
		return
	}

	group, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error":   "not-found",
				"message": "policy group not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}
	if group.TenantID != tenantID {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error":   "forbidden",
			"message": "tenant mismatch",
		})
		return
	}

	var req struct {
		Name        *string                `json:"name"`
		Description *string                `json:"description"`
		Priority    *int                   `json:"priority"`
		IsActive    *bool                  `json:"is_active"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "invalid JSON body",
		})
		return
	}

	if req.Name != nil {
		group.Name = *req.Name
	}
	if req.Description != nil {
		group.Description = req.Description
	}
	if req.Priority != nil {
		group.Priority = *req.Priority
	}
	if req.IsActive != nil {
		group.IsActive = *req.IsActive
	}
	if req.Metadata != nil {
		group.Metadata = req.Metadata
	}

	if err := h.store.Update(r.Context(), group); err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error":   "not-found",
				"message": "policy group not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, group)
}

// DeleteGroup handles DELETE /v1/policy-groups/{id}
func (h *GroupHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "invalid group ID",
		})
		return
	}

	if err := h.store.Delete(r.Context(), id, tenantID); err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error":   "not-found",
				"message": "policy group not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}