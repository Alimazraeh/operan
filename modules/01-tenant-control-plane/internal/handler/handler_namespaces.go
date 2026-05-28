package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/operan/modules/01-tenant-control-plane/internal/middleware"
	"github.com/operan/modules/01-tenant-control-plane/internal/store"
)

// ─── Namespace request/response types ────────────────────────────────────────

// NamespaceCreateRequest for creating a new namespace.
type NamespaceCreateRequest struct {
	Name           string               `json:"name"`
	Description    string               `json:"description,omitempty"`
	NetworkPolicy  string               `json:"network_policy,omitempty"`
	IsolationLevel string               `json:"isolation_level,omitempty"`
	Tags           map[string]string    `json:"tags,omitempty"`
	ExtraConfig    map[string]interface{} `json:"extra_config,omitempty"`
	MaxAgents      int                  `json:"max_agents,omitempty"`
	MaxStorageGB   int                  `json:"max_storage_gb,omitempty"`
	MaxConcurrentWorkflows int          `json:"max_concurrent_workflows,omitempty"`
}

// NamespaceResponse represents a namespace in API responses.
type NamespaceResponse struct {
	ID                  string                 `json:"id"`
	TenantID            string                 `json:"tenant_id"`
	Name                string                 `json:"name"`
	Description         string                 `json:"description,omitempty"`
	Status              string                 `json:"status"`
	NetworkPolicy       string                 `json:"network_policy"`
	IsolationLevel      string                 `json:"isolation_level"`
	Tags                map[string]string      `json:"tags,omitempty"`
	MaxAgents           int                    `json:"max_agents"`
	MaxStorageGB        int                    `json:"max_storage_gb"`
	MaxConcurrentWorkflows int                 `json:"max_concurrent_workflows"`
	CreatedAt           string                 `json:"created_at"`
	UpdatedAt           string                 `json:"updated_at"`
}

// ─── Namespace handlers ──────────────────────────────────────────────────────

// ListNamespaces handles GET /v1/tenants/{id}/namespaces.
func ListNamespaces(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		_, err := h.TenantStore.GetByID(tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		page, pageSize := paginationFrom(r)
		items, total, hasMore := h.NamespaceStore.ListByTenant(tenantID, page, pageSize)

		respItems := make([]*NamespaceResponse, 0, len(items))
		for _, ns := range items {
			respItems = append(respItems, namespaceToResponse(ns))
		}

		h.WriteJSON(w, http.StatusOK, &NamespaceListResponse{
			Items:    respItems,
			Page:     page,
			PageSize: pageSize,
			Total:    total,
			HasMore:  hasMore,
		})
	}
}

// CreateNamespace handles POST /v1/tenants/{id}/namespaces.
func CreateNamespace(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		_, err := h.TenantStore.GetByID(tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		var req NamespaceCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", err.Error())
			return
		}

		if req.Name == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid field", "Name is required")
			return
		}

		ns := &store.Namespace{
			TenantID: tenantID,
			Name:     req.Name,
			Description: req.Description,
			Config: store.NamespaceConfig{
				NetworkPolicy:  req.NetworkPolicy,
				IsolationLevel: store.IsolationLevel(req.IsolationLevel),
				Tags:           req.Tags,
				ExtraConfig:    req.ExtraConfig,
			},
			ResourceQuota: &store.NamespaceQuota{
				MaxAgents:            req.MaxAgents,
				MaxStorageGB:         req.MaxStorageGB,
				MaxConcurrentWorkflows: req.MaxConcurrentWorkflows,
			},
		}

		ns, err = h.NamespaceStore.Create(ns)
		if err != nil {
			h.WriteError(w, http.StatusConflict, 409, "namespace creation failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusCreated, namespaceToResponse(ns))
	}
}

// GetNamespace handles GET /v1/tenants/{id}/namespaces/{ns_id}.
func GetNamespace(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nsID, ok := extractPathParam(r, "ns_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "namespace id is required")
			return
		}

		ns, err := h.NamespaceStore.GetByID(nsID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "namespace not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, namespaceToResponse(ns))
	}
}

// PatchNamespace handles PATCH /v1/tenants/{id}/namespaces/{ns_id}.
func PatchNamespace(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nsID, ok := extractPathParam(r, "ns_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "namespace id is required")
			return
		}

		var req store.NamespacePatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", err.Error())
			return
		}

		ns, err := h.NamespaceStore.Patch(nsID, req)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "namespace not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, namespaceToResponse(ns))
	}
}

// DeleteNamespace handles DELETE /v1/tenants/{id}/namespaces/{ns_id}.
func DeleteNamespace(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nsID, ok := extractPathParam(r, "ns_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "namespace id is required")
			return
		}

		if err := h.NamespaceStore.Delete(nsID); err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "namespace not found", err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// GetNamespaceQuota handles GET /v1/tenants/{id}/namespaces/{ns_id}/quota.
func GetNamespaceQuota(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nsID, ok := extractPathParam(r, "ns_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "namespace id is required")
			return
		}

		ns, err := h.NamespaceStore.GetByID(nsID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "namespace not found", err.Error())
			return
		}

		quota := ns.ResourceQuota
		h.WriteJSON(w, http.StatusOK, map[string]any{
			"max_agents":             quota.MaxAgents,
			"max_storage_gb":         quota.MaxStorageGB,
			"max_concurrent_workflows": quota.MaxConcurrentWorkflows,
		})
	}
}

// CheckNamespaceQuota handles GET /v1/tenants/{id}/namespaces/{ns_id}/quota/check.
func CheckNamespaceQuota(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nsID, ok := extractPathParam(r, "ns_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "namespace id is required")
			return
		}

		ns, err := h.NamespaceStore.GetByID(nsID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "namespace not found", err.Error())
			return
		}

		quota := ns.ResourceQuota
		h.WriteJSON(w, http.StatusOK, map[string]any{
			"allowed":  quota.MaxAgents > 0,
			"used":     0,
			"limit":    quota.MaxAgents,
		})
	}
}

// namespaceToResponse converts a store.Namespace to API response.
func namespaceToResponse(ns *store.Namespace) *NamespaceResponse {
	quota := ns.ResourceQuota
	if quota == nil {
		quota = &store.NamespaceQuota{}
	}
	return &NamespaceResponse{
		ID:                   ns.ID,
		TenantID:             ns.TenantID,
		Name:                 ns.Name,
		Description:          ns.Description,
		Status:               string(ns.Status),
		NetworkPolicy:        ns.Config.NetworkPolicy,
		IsolationLevel:       string(ns.Config.IsolationLevel),
		Tags:                 ns.Config.Tags,
		MaxAgents:            quota.MaxAgents,
		MaxStorageGB:         quota.MaxStorageGB,
		MaxConcurrentWorkflows: quota.MaxConcurrentWorkflows,
		CreatedAt:            ns.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:            ns.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// paginationFrom extracts page and pageSize from request.
func paginationFrom(r *http.Request) (int, int) {
	page := 1
	pageSize := 20

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
			pageSize = v
		}
	}
	return page, pageSize
}
