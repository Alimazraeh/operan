package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/operan/modules/08-tool-execution/internal/events"
	"github.com/operan/modules/08-tool-execution/internal/middleware"
	"github.com/operan/modules/08-tool-execution/internal/store"
)

// toolRegisterRequest is the body for POST /tools/register.
type toolRegisterRequest struct {
	TenantID             string                 `json:"tenant_id"`
	Name                 string                 `json:"name"`
	Description          string                 `json:"description"`
	Category             string                 `json:"category"`
	InputSchema          map[string]interface{} `json:"input_schema"`
	OutputSchema         map[string]interface{} `json:"output_schema"`
	AuthRequirements     []string               `json:"auth_requirements"`
	RateLimit            *store.RateLimit       `json:"rate_limit"`
	CostPerCall          *store.CostPerCall     `json:"cost_per_call"`
	SecurityRequirements []string               `json:"security_requirements"`
}

// RegisterTool handles POST /tools/register.
func (h *ToolHandlers) RegisterTool(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req toolRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, r, http.StatusBadRequest, "name is required")
		return
	}
	// Body tenant_id, when present, must match the authenticated tenant.
	if req.TenantID != "" && req.TenantID != tenantID {
		writeError(w, r, http.StatusConflict, "tenant_id does not match authenticated tenant")
		return
	}

	tool, err := h.Tools.Create(&store.Tool{
		TenantID:             tenantID,
		Name:                 req.Name,
		Description:          req.Description,
		Category:             req.Category,
		InputSchema:          req.InputSchema,
		OutputSchema:         req.OutputSchema,
		AuthRequirements:     req.AuthRequirements,
		RateLimit:            req.RateLimit,
		CostPerCall:          req.CostPerCall,
		SecurityRequirements: req.SecurityRequirements,
		CreatedBy:            middleware.UserIDFromContext(r.Context()),
	})
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// Record the initial version.
	_, _ = h.Versions.Create(&store.ToolVersion{
		ToolID:        tool.ID,
		TenantID:      tenantID,
		Version:       tool.Version,
		ChangeSummary: "major",
		InputSchema:   tool.InputSchema,
		OutputSchema:  tool.OutputSchema,
		CreatedBy:     tool.CreatedBy,
	})

	if h.Publisher != nil {
		_ = h.Publisher.PublishToolRegistered(events.ToolRegisteredPayload{
			ToolID: tool.ID, Name: tool.Name, Category: tool.Category,
			Version: tool.Version, TenantID: tenantID, CreatedBy: tool.CreatedBy,
			CreatedAt: tool.CreatedAt,
		})
	}

	writeJSON(w, http.StatusCreated, tool)
}

// ListTools handles GET /tools.
func (h *ToolHandlers) ListTools(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	page, pageSize := h.pagination(r)
	items, total, hasMore := h.Tools.List(tenantID, page, pageSize, queryPtr(r, "category"), queryPtr(r, "status"))
	if items == nil {
		items = []store.Tool{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items, "total": total, "page": page, "page_size": pageSize, "has_more": hasMore,
	})
}

// GetTool handles GET /tools/{id}.
func (h *ToolHandlers) GetTool(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	tool, err := h.Tools.GetByIDAndTenant(r.PathValue("id"), tenantID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "tool not found")
		return
	}
	writeJSON(w, http.StatusOK, tool)
}

// toolUpdateRequest is the body for PATCH /tools/{id}.
type toolUpdateRequest struct {
	Description  *string                `json:"description"`
	Category     *string                `json:"category"`
	InputSchema  map[string]interface{} `json:"input_schema"`
	OutputSchema map[string]interface{} `json:"output_schema"`
	RateLimit    *store.RateLimit       `json:"rate_limit"`
	Status       *string                `json:"status"`
	CostPerCall  *store.CostPerCall     `json:"cost_per_call"`
}

// UpdateTool handles PATCH /tools/{id}.
func (h *ToolHandlers) UpdateTool(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req toolUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	tool, err := h.Tools.Update(r.PathValue("id"), tenantID, func(t *store.Tool) {
		if req.Description != nil {
			t.Description = *req.Description
		}
		if req.Category != nil {
			t.Category = *req.Category
		}
		if req.InputSchema != nil {
			t.InputSchema = req.InputSchema
		}
		if req.OutputSchema != nil {
			t.OutputSchema = req.OutputSchema
		}
		if req.RateLimit != nil {
			t.RateLimit = req.RateLimit
		}
		if req.Status != nil {
			t.Status = *req.Status
		}
		if req.CostPerCall != nil {
			t.CostPerCall = req.CostPerCall
		}
	})
	if err != nil {
		writeError(w, r, http.StatusNotFound, "tool not found")
		return
	}
	writeJSON(w, http.StatusOK, tool)
}

// ListToolVersions handles GET /tools/{id}/versions.
func (h *ToolHandlers) ListToolVersions(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	id := r.PathValue("id")
	if _, err := h.Tools.GetByIDAndTenant(id, tenantID); err != nil {
		writeError(w, r, http.StatusNotFound, "tool not found")
		return
	}
	page, pageSize := h.pagination(r)
	items, total, hasMore := h.Versions.ListByTool(id, page, pageSize)
	if items == nil {
		items = []store.ToolVersion{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items, "total": total, "page": page, "page_size": pageSize, "has_more": hasMore,
	})
}
