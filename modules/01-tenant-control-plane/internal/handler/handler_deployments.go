package handler

import (
	"encoding/json"
	"net/http"

	"github.com/operan/modules/01-tenant-control-plane/internal/middleware"
	"github.com/operan/modules/01-tenant-control-plane/internal/store"
)

// ─── Deployment request/response types ───────────────────────────────────────

// DeploymentCreateRequest for creating a new deployment.
type DeploymentCreateRequest struct {
	Name           string                 `json:"name"`
	Version        string                 `json:"version"`
	Strategy       string                 `json:"strategy,omitempty"`
	Manifest       json.RawMessage        `json:"manifest,omitempty"`
	DesiredState   map[string]interface{} `json:"desired_state,omitempty"`
	ResourceRefs   []string               `json:"resource_refs,omitempty"`
	NamespaceID    string                 `json:"namespace_id,omitempty"`
	PreviousID     *string                `json:"previous_id,omitempty"`
	CreatedBy      string                 `json:"created_by,omitempty"`
	Notes          string                 `json:"notes,omitempty"`
}

// DeploymentResponse represents a deployment in API responses.
type DeploymentResponse struct {
	ID             string                 `json:"id"`
	TenantID       string                 `json:"tenant_id"`
	Name           string                 `json:"name"`
	Version        string                 `json:"version"`
	Status         string                 `json:"status"`
	Strategy       string                 `json:"strategy"`
	Manifest       json.RawMessage        `json:"manifest,omitempty"`
	DesiredState   map[string]interface{} `json:"desired_state,omitempty"`
	CurrentState   map[string]interface{} `json:"current_state,omitempty"`
	Error          string                 `json:"error,omitempty"`
	ResourceRefs   []string               `json:"resource_refs,omitempty"`
	NamespaceID    string                 `json:"namespace_id,omitempty"`
	PreviousID     *string                `json:"previous_id,omitempty"`
	CreatedBy      string                 `json:"created_by,omitempty"`
	Notes          string                 `json:"notes,omitempty"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
	DeployedAt     *string                `json:"deployed_at,omitempty"`
	DeprecatedAt   *string                `json:"deprecated_at,omitempty"`
}

// ─── Deployment handlers ─────────────────────────────────────────────────────

// ListDeployments handles GET /v1/tenants/{id}/deployments.
func ListDeployments(h *middleware.Handler) http.HandlerFunc {
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
		items, total, hasMore := h.DeploymentStore.ListByTenant(tenantID, page, pageSize)

		respItems := make([]*DeploymentResponse, 0, len(items))
		for _, d := range items {
			respItems = append(respItems, deploymentToResponse(d))
		}

		h.WriteJSON(w, http.StatusOK, &DeploymentListResponse{
			Items:    respItems,
			Page:     page,
			PageSize: pageSize,
			Total:    total,
			HasMore:  hasMore,
		})
	}
}

// CreateDeployment handles POST /v1/tenants/{id}/deployments.
func CreateDeployment(h *middleware.Handler) http.HandlerFunc {
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

		var req DeploymentCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", err.Error())
			return
		}

		if req.Name == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid field", "Name is required")
			return
		}

		var manifestBytes []byte
		if req.Manifest != nil {
			manifestBytes = []byte(req.Manifest)
		}

		d := &store.Deployment{
			TenantID:        tenantID,
			Name:            req.Name,
			Version:         req.Version,
			Strategy:        store.DeploymentStrategy(req.Strategy),
			Manifest:        manifestBytes,
			DesiredState:    req.DesiredState,
			ResourceRefs:    req.ResourceRefs,
			NamespaceID:     req.NamespaceID,
			PreviousID:      req.PreviousID,
			CreatedBy:       req.CreatedBy,
			Notes:           req.Notes,
		}

		d, err = h.DeploymentStore.Create(d)
		if err != nil {
			h.WriteError(w, http.StatusConflict, 409, "deployment creation failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusCreated, deploymentToResponse(d))
	}
}

// GetDeployment handles GET /v1/tenants/{id}/deployments/{deploy_id}.
func GetDeployment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		deployID, ok := extractPathParam(r, "deploy_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "deployment id is required")
			return
		}

		d, err := h.DeploymentStore.GetByIDAndTenant(deployID, tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "deployment not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, deploymentToResponse(d))
	}
}

// PatchDeployment handles PATCH /v1/tenants/{id}/deployments/{deploy_id}.
func PatchDeployment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deployID, ok := extractPathParam(r, "deploy_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "deployment id is required")
			return
		}

		var req store.DeploymentPatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", err.Error())
			return
		}

		d, err := h.DeploymentStore.Patch(deployID, req)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "deployment not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, deploymentToResponse(d))
	}
}

// DeleteDeployment handles DELETE /v1/tenants/{id}/deployments/{deploy_id}.
func DeleteDeployment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deployID, ok := extractPathParam(r, "deploy_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "deployment id is required")
			return
		}

		if err := h.DeploymentStore.Delete(deployID); err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "deployment not found", err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// DeployDeployment handles POST /v1/tenants/{id}/deployments/{deploy_id}/deploy.
func DeployDeployment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deployID, ok := extractPathParam(r, "deploy_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "deployment id is required")
			return
		}

		d, err := h.DeploymentStore.Deploy(deployID)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "deployment deploy failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, deploymentToResponse(d))
	}
}

// StopDeployment handles POST /v1/tenants/{id}/deployments/{deploy_id}/stop.
func StopDeployment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deployID, ok := extractPathParam(r, "deploy_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "deployment id is required")
			return
		}

		d, err := h.DeploymentStore.Stop(deployID)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "deployment stop failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, deploymentToResponse(d))
	}
}

// RollbackDeployment handles POST /v1/tenants/{id}/deployments/{deploy_id}/rollback.
func RollbackDeployment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deployID, ok := extractPathParam(r, "deploy_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "deployment id is required")
			return
		}

		d, err := h.DeploymentStore.Rollback(deployID, "")
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "deployment rollback failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, deploymentToResponse(d))
	}
}

// DeprecateDeployment handles POST /v1/tenants/{id}/deployments/{deploy_id}/deprecate.
func DeprecateDeployment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deployID, ok := extractPathParam(r, "deploy_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "deployment id is required")
			return
		}

		d, err := h.DeploymentStore.Deprecate(deployID)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "deployment deprecate failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, deploymentToResponse(d))
	}
}

// deploymentToResponse converts a store.Deployment to API response.
func deploymentToResponse(d *store.Deployment) *DeploymentResponse {
	var deployedAt *string
	if d.DeployedAt != nil {
		t := d.DeployedAt.Format("2006-01-02T15:04:05Z")
		deployedAt = &t
	}
	var deprecatedAt *string
	if d.DeprecatedAt != nil {
		t := d.DeprecatedAt.Format("2006-01-02T15:04:05Z")
		deprecatedAt = &t
	}

	var manifest json.RawMessage
	if len(d.Manifest) > 0 {
		manifest = json.RawMessage(d.Manifest)
	}

	return &DeploymentResponse{
		ID:             d.ID,
		TenantID:       d.TenantID,
		Name:           d.Name,
		Version:        d.Version,
		Status:         string(d.Status),
		Strategy:       string(d.Strategy),
		Manifest:       manifest,
		DesiredState:   d.DesiredState,
		CurrentState:   d.CurrentState,
		Error:          d.Error,
		ResourceRefs:   d.ResourceRefs,
		NamespaceID:    d.NamespaceID,
		PreviousID:     d.PreviousID,
		CreatedBy:      d.CreatedBy,
		Notes:          d.Notes,
		CreatedAt:      d.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      d.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		DeployedAt:     deployedAt,
		DeprecatedAt:   deprecatedAt,
	}
}

// RolloutDeployment handles POST /v1/tenants/{id}/deployments/{deployment_id}/rollout.
func RolloutDeployment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		deploymentID, ok := extractPathParam(r, "deployment_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "deployment_id is required")
			return
		}

		deployment, err := h.DeploymentStore.GetByIDAndTenant(deploymentID, tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "deployment not found", err.Error())
			return
		}

		_, err = h.DeploymentStore.Deploy(deploymentID)
		if err != nil {
			h.WriteError(w, http.StatusInternalServerError, 500, "failed to rollout deployment", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, deploymentToResponse(deployment))
	}
}

// ScaleDeployment handles POST /v1/tenants/{id}/deployments/{deployment_id}/scale.
func ScaleDeployment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		deploymentID, ok := extractPathParam(r, "deployment_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "deployment_id is required")
			return
		}

		deployment, err := h.DeploymentStore.GetByIDAndTenant(deploymentID, tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "deployment not found", err.Error())
			return
		}

		// Scale by updating desired state
		_, err = h.DeploymentStore.Patch(deploymentID, store.DeploymentPatchRequest{
			DesiredState: map[string]interface{}{},
		})
		if err != nil {
			h.WriteError(w, http.StatusInternalServerError, 500, "failed to scale deployment", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, deploymentToResponse(deployment))
	}
}

// GetDeploymentStatus handles GET /v1/tenants/{id}/deployments/{deployment_id}/status.
func GetDeploymentStatus(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		deploymentID, ok := extractPathParam(r, "deployment_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "deployment_id is required")
			return
		}

		deployment, err := h.DeploymentStore.GetByIDAndTenant(deploymentID, tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "deployment not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, deploymentToResponse(deployment))
	}
}

// PauseDeployment handles POST /v1/tenants/{id}/deployments/{deployment_id}/pause.
func PauseDeployment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		deploymentID, ok := extractPathParam(r, "deployment_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "deployment_id is required")
			return
		}

		deployment, err := h.DeploymentStore.GetByIDAndTenant(deploymentID, tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "deployment not found", err.Error())
			return
		}

		_, err = h.DeploymentStore.Stop(deploymentID)
		if err != nil {
			h.WriteError(w, http.StatusInternalServerError, 500, "failed to pause deployment", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, deploymentToResponse(deployment))
	}
}

// ResumeDeployment handles POST /v1/tenants/{id}/deployments/{deployment_id}/resume.
func ResumeDeployment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		deploymentID, ok := extractPathParam(r, "deployment_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "deployment_id is required")
			return
		}

		deployment, err := h.DeploymentStore.GetByIDAndTenant(deploymentID, tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "deployment not found", err.Error())
			return
		}

		_, err = h.DeploymentStore.Deploy(deploymentID)
		if err != nil {
			h.WriteError(w, http.StatusInternalServerError, 500, "failed to resume deployment", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, deploymentToResponse(deployment))
	}
}
