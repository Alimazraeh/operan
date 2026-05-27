package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/operan/modules/01-tenant-control-plane/internal/middleware"
	"github.com/operan/modules/01-tenant-control-plane/internal/store"
)

// ─── Tenant Status handlers ──────────────────────────────────────────────────

// GetTenantStatus handles GET /tenants/{id}/status.
func GetTenantStatus(h *middleware.Handler) http.HandlerFunc {
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

		resp := TenantStatusResponse{
			Status:             string(tenant.Status),
			AllowedTransitions: make([]string, 0),
			Transitions:        make([]TenantTransition, 0),
		}

		for _, allowed := range store.AllowedTransitions(tenant.Status) {
			resp.AllowedTransitions = append(resp.AllowedTransitions, string(allowed))
			resp.Transitions = append(resp.Transitions, TenantTransition{
				To:   string(allowed),
				From: string(tenant.Status),
			})
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// TransitionTenantStatus handles POST /tenants/{id}/status/transition.
func TransitionTenantStatus(h *middleware.Handler) http.HandlerFunc {
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

		var req struct {
			NewStatus string `json:"new_status" validate:"required"`
		}

		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		if req.NewStatus == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "new_status is required")
			return
		}

		tenant, err := h.TenantStore.Patch(id, store.TenantPatchRequest{
			Status: store.TenantStatus(req.NewStatus),
		})
		if err != nil {
			h.WriteError(w, http.StatusConflict, 409, "status transition failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, TenantStatusResponse{
			Status:     string(tenant.Status),
			UpdatedAt:  tenant.UpdatedAt,
		})
	}
}

// ─── Agent handlers ──────────────────────────────────────────────────────────

// ListAgents handles GET /tenants/{id}/agents.
func ListAgents(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		// Verify tenant exists
		_, err := h.TenantStore.GetByID(tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		page := 1
		pageSize := 20
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

		items, total, hasMore := h.AgentStore.ListByTenant(tenantID, page, pageSize)

		resp := AgentListResponse{
			Items:   make([]*AgentResponse, len(items)),
			Total:   total,
			HasMore: hasMore,
		}
		for i, a := range items {
			resp.Items[i] = &AgentResponse{
				ID:              a.ID,
				TenantID:        a.TenantID,
				Name:            a.Name,
				Model:           a.Model,
				Role:            a.Role,
				SystemPrompt:    a.SystemPrompt,
				Status:          string(a.Status),
				LastRunAt:       a.LastRunAt,
				SuccessCount:    a.SuccessCount,
				FailureCount:    a.FailureCount,
				CreatedAt:       a.CreatedAt,
				UpdatedAt:       a.UpdatedAt,
			}
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// CreateAgent handles POST /tenants/{id}/agents.
func CreateAgent(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		// Verify tenant exists
		_, err := h.TenantStore.GetByID(tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req struct {
			Name         string          `json:"name"`
			Model        string          `json:"model"`
			Role         string          `json:"role"`
			SystemPrompt string          `json:"system_prompt"`
			ToolAccess   json.RawMessage `json:"tool_access,omitempty"`
		}

		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		agent := &store.Agent{
			TenantID:       tenantID,
			Name:           req.Name,
			Model:          req.Model,
			Role:           req.Role,
			SystemPrompt:   req.SystemPrompt,
			ToolAccessJSON: req.ToolAccess,
		}

		created, err := h.AgentStore.Create(agent)
		if err != nil {
			h.WriteError(w, http.StatusConflict, 409, "agent creation failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusCreated, AgentResponse{
			ID:              created.ID,
			TenantID:        created.TenantID,
			Name:            created.Name,
			Model:           created.Model,
			Role:            created.Role,
			SystemPrompt:    created.SystemPrompt,
			Status:          string(created.Status),
			LastRunAt:       created.LastRunAt,
			SuccessCount:    created.SuccessCount,
			FailureCount:    created.FailureCount,
			CreatedAt:       created.CreatedAt,
			UpdatedAt:       created.UpdatedAt,
		})
	}
}

// GetAgent handles GET /tenants/{id}/agents/{agent_id}.
func GetAgent(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID, ok := extractPathParam(r, "agent_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "agent id is required")
			return
		}

		agent, err := h.AgentStore.GetByID(agentID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "agent not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, AgentResponse{
			ID:              agent.ID,
			TenantID:        agent.TenantID,
			Name:            agent.Name,
			Model:           agent.Model,
			Role:            agent.Role,
			SystemPrompt:    agent.SystemPrompt,
			Status:          string(agent.Status),
			CurrentWorkflow: agent.CurrentWorkflow,
			CurrentTask:     agent.CurrentTask,
			LastRunAt:       agent.LastRunAt,
			SuccessCount:    agent.SuccessCount,
			FailureCount:    agent.FailureCount,
			CreatedAt:       agent.CreatedAt,
			UpdatedAt:       agent.UpdatedAt,
		})
	}
}

// PatchAgent handles PATCH /tenants/{id}/agents/{agent_id}.
func PatchAgent(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID, ok := extractPathParam(r, "agent_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "agent id is required")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req middleware.AgentPatchRequest
		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		agent, err := h.AgentStore.Patch(agentID, store.AgentPatchRequest{
			Model:          req.Model,
			Role:           req.Role,
			SystemPrompt:   req.SystemPrompt,
			Status:         store.AgentStatus(req.Status),
			ToolAccessJSON: req.ToolAccessJSON,
		})
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "agent not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, AgentResponse{
			ID:         agent.ID,
			TenantID:   agent.TenantID,
			Name:       agent.Name,
			Model:      agent.Model,
			Role:       agent.Role,
			Status:     string(agent.Status),
			CreatedAt:  agent.CreatedAt,
			UpdatedAt:  agent.UpdatedAt,
		})
	}
}

// DeleteAgent handles DELETE /tenants/{id}/agents/{agent_id}.
func DeleteAgent(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID, ok := extractPathParam(r, "agent_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "agent id is required")
			return
		}

		err := h.AgentStore.Delete(agentID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "agent not found", err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
