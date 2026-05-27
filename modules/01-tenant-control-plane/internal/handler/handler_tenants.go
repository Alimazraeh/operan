// Package handler implements HTTP request handlers for tenant-control-plane.
// Each handler function processes a request, calls the appropriate store method,
// and returns a JSON response matching the OpenAPI spec.
package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/operan/modules/01-tenant-control-plane/internal/events"
	"github.com/operan/modules/01-tenant-control-plane/internal/middleware"
	"github.com/operan/modules/01-tenant-control-plane/internal/store"
)

// ─── Tenant handlers ─────────────────────────────────────────────────────────

// CreateTenant handles POST /tenants.
func CreateTenant(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req struct {
			Name           string                 `json:"name" validate:"required,min=1,max=128"`
			DisplayName    string                 `json:"display_name,omitempty"`
			Plan           store.Plan             `json:"plan" validate:"required"`
			Region         store.Region           `json:"region" validate:"required"`
			IsolationLevel store.IsolationLevel   `json:"isolation_level" validate:"required"`
			ContactEmail   string                 `json:"contact_email,omitempty"`
			AdminEmail     string                 `json:"admin_email,omitempty"`
			Quota          *store.QuotaConfig     `json:"quota,omitempty"`
			CustomMetadata map[string]interface{} `json:"custom_metadata,omitempty"`
		}

		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		if req.Name == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "name is required")
			return
		}
		if req.Plan == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "plan is required")
			return
		}
		if req.Region == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "region is required")
			return
		}
		if req.IsolationLevel == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "isolation_level is required")
			return
		}

		quota := store.PlanDefaults(req.Plan)
		if req.Quota != nil {
			if req.Quota.MaxAgents > 0 {
				quota.MaxAgents = req.Quota.MaxAgents
			}
			if req.Quota.MaxWorkflowsPerDay > 0 {
				quota.MaxWorkflowsPerDay = req.Quota.MaxWorkflowsPerDay
			}
			if req.Quota.MaxStorageGB > 0 {
				quota.MaxStorageGB = req.Quota.MaxStorageGB
			}
			if req.Quota.MaxMonthlyTokens > 0 {
				quota.MaxMonthlyTokens = req.Quota.MaxMonthlyTokens
			}
			if req.Quota.MaxConcurrentWorkflows > 0 {
				quota.MaxConcurrentWorkflows = req.Quota.MaxConcurrentWorkflows
			}
		}

		tenant := &store.Tenant{
			Name:           req.Name,
			DisplayName:    req.DisplayName,
			Plan:           req.Plan,
			Region:         req.Region,
			IsolationLevel: req.IsolationLevel,
			Status:         store.TenantStatusProvisioning,
			Quota:          quota,
			ContactEmail:   req.ContactEmail,
			AdminEmail:     req.AdminEmail,
			CustomMetadata: req.CustomMetadata,
		}

		created, err := h.TenantStore.Create(tenant)
		if err != nil {
			h.WriteError(w, http.StatusConflict, 409, "tenant creation failed", err.Error())
			return
		}

		_ = h.EventPublisher.PublishTenantProvisioned(events.TenantProvisionedEvent{
			TenantID:   created.ID,
			TenantName: created.Name,
			Plan:       string(created.Plan),
			Region:     string(created.Region),
		})

		h.WriteJSON(w, http.StatusCreated, tenantResponse(created))
	}
}

// ListTenants handles GET /tenants.
func ListTenants(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := 1
		pageSize := 20
		var status *string

		if p := r.URL.Query().Get("page"); p != "" {
			if n, err := strconv.Atoi(p); err == nil && n > 0 {
				page = n
			}
		}
		if ps := r.URL.Query().Get("page_size"); ps != "" {
			if n, err := strconv.Atoi(ps); err == nil && n > 0 {
				pageSize = n
			}
		}
		if s := r.URL.Query().Get("status"); s != "" {
			status = &s
		}

		items, total, hasMore := h.TenantStore.List(page, pageSize, status)

		resp := TenantListResponse{
			Items:    make([]*TenantResponse, len(items)),
			Page:     page,
			PageSize: pageSize,
			Total:    total,
			HasMore:  hasMore,
		}
		for i, t := range items {
			resp.Items[i] = tenantResponse(t)
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// GetTenant handles GET /tenants/{id}.
func GetTenant(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		tenant, err := h.TenantStore.GetByID(id)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, tenantResponse(tenant))
	}
}

// PatchTenant handles PATCH /tenants/{id}.
func PatchTenant(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req middleware.TenantPatchRequest
		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		tenant, err := h.TenantStore.Patch(id, store.TenantPatchRequest{
			Name:           req.Name,
			DisplayName:    req.DisplayName,
			Status:         req.Status,
			Plan:           req.Plan,
			Region:         req.Region,
			IsolationLevel: req.IsolationLevel,
			ContactEmail:   req.ContactEmail,
			AdminEmail:     req.AdminEmail,
			CustomMetadata: req.CustomMetadata,
			Quota:          req.Quota,
		})
		if err != nil {
			if strings.Contains(err.Error(), "invalid status transition") {
				h.WriteError(w, http.StatusConflict, 409, "invalid status transition", err.Error())
				return
			}
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		_ = tenant

		h.WriteJSON(w, http.StatusOK, tenantResponse(tenant))
	}
}

// DeleteTenant handles DELETE /tenants/{id}.
func DeleteTenant(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		err := h.TenantStore.Delete(id)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		_ = h.EventPublisher.PublishTenantDeprovisioned(events.TenantDeprovisionedEvent{
			TenantID:   id,
			Source:     "tenant-control-plane",
			Timestamp:  time.Now(),
		})

		w.WriteHeader(http.StatusNoContent)
	}
}

// ─── Helper ──────────────────────────────────────────────────────────────────

func tenantResponse(t *store.Tenant) *TenantResponse {
	return &TenantResponse{
		ID:              t.ID,
		Name:            t.Name,
		DisplayName:     t.DisplayName,
		Plan:            string(t.Plan),
		Region:          string(t.Region),
		IsolationLevel:  string(t.IsolationLevel),
		Status:          string(t.Status),
		Quota:           quotaResponse(t.Quota),
		ContactEmail:    t.ContactEmail,
		AdminEmail:      t.AdminEmail,
		CustomMetadata:  t.CustomMetadata,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

func quotaResponse(q store.QuotaConfig) QuotaResponse {
	return QuotaResponse{
		MaxAgents:              q.MaxAgents,
		MaxWorkflowsPerDay:     q.MaxWorkflowsPerDay,
		MaxStorageGB:           q.MaxStorageGB,
		MaxMonthlyTokens:       q.MaxMonthlyTokens,
		MaxConcurrentWorkflows: q.MaxConcurrentWorkflows,
	}
}
