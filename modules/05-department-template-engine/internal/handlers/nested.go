package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/events"
	"github.com/operan/modules/05-department-template-engine/internal/middleware"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

// HandleTemplateNested is a single route handler for all nested operations under /templates/{id}/.
// It extracts the template ID and dispatches based on the operation:
//   - /templates/{id}/deploy → DeployTemplate
//   - /templates/{id}/deployments → ListDeployments
//   - /templates/{id}/deployments/{deploymentId} → GetDeployment, UpdateDeployment
//   - /templates/{id}/versions → ListTemplateVersions
//   - /templates/{id}/versions/{versionId} → GetTemplateVersion
//   - /templates/{id}/clone → CloneTemplate
func (h *TemplateHandlers) HandleTemplateNested(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// Extract template ID from path like /templates/abc123/deploy
	templateID := extractTemplateIDFromNestedPath(r.URL.Path)
	if templateID == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid template ID", r.URL.Path, reqID)
		return
	}

	// Verify template exists
	_, err := h.TemplateStore.GetByID(templateID)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Template not found", r.URL.Path, reqID)
		return
	}

	// Get the remaining path after /templates/{id}/
	remaining := strings.TrimPrefix(r.URL.Path, "/templates/"+templateID+"/")

	switch r.Method {
	case http.MethodPost:
		switch remaining {
		case "deploy":
			h.handleDeploy(w, r, reqID)
		case "clone":
			h.handleClone(w, r, reqID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "about:blank", "Method Not Allowed",
				"Invalid operation", r.URL.Path, reqID)
		}

	case http.MethodGet:
		switch {
		case remaining == "deployments":
			h.handleListDeployments(w, r, reqID)
		case remaining == "versions":
			h.handleListVersions(w, r, reqID)
		case strings.HasPrefix(remaining, "deployments/"):
			deploymentID := strings.TrimPrefix(remaining, "deployments/")
			h.handleGetDeployment(w, r, reqID, templateID, deploymentID)
		case strings.HasPrefix(remaining, "versions/"):
			versionID := strings.TrimPrefix(remaining, "versions/")
			h.handleGetVersion(w, r, reqID, templateID, versionID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "about:blank", "Method Not Allowed",
				"Invalid operation", r.URL.Path, reqID)
		}

	case http.MethodPatch:
		if strings.HasPrefix(remaining, "deployments/") {
			deploymentID := strings.TrimPrefix(remaining, "deployments/")
			h.handleUpdateDeployment(w, r, reqID, templateID, deploymentID)
		}

	default:
		writeError(w, http.StatusMethodNotAllowed, "about:blank", "Method Not Allowed",
			"Method not allowed", r.URL.Path, reqID)
	}
}

func (h *TemplateHandlers) handleDeploy(w http.ResponseWriter, r *http.Request, reqID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Failed to read request body", r.URL.Path, reqID)
		return
	}
	defer r.Body.Close()

	var req store.DeployRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid JSON body", r.URL.Path, reqID)
		return
	}

	if req.Environment == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"environment is required", r.URL.Path, reqID)
		return
	}

	templateID := extractTemplateIDFromNestedPath(r.URL.Path)
	tenantID := middleware.TenantIDFromContext(r.Context())

	now := time.Now()
	deployment := &store.TemplateDeployment{
		TenantID:      tenantID,
		TemplateID:    templateID,
		Version:       req.Version,
		Status:        "select",
		Environment:   req.Environment,
		Configuration: req.Configuration,
		DeployedBy:    middleware.UserIDFromContext(r.Context()),
		StartedAt:     &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	created, err := h.DeploymentStore.Create(deployment)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "about:blank", "Internal Server Error",
			"Failed to create deployment", r.URL.Path, reqID)
		return
	}

	h.EventPublisher.PublishTemplateDeployed(events.TemplateDeployedPayload{
		Event:        "template.deployed",
		DeploymentID: created.ID,
		TemplateID:   templateID,
		Version:      created.Version,
		Environment:  created.Environment,
		Status:       created.Status,
		DeployedAt:   created.CreatedAt,
		DeployedBy:   created.DeployedBy,
		TenantID:     middleware.TenantIDFromContext(r.Context()),
	})

	writeJSON(w, http.StatusCreated, toDeploymentResponse(created))
}

func (h *TemplateHandlers) handleClone(w http.ResponseWriter, r *http.Request, reqID string) {
	templateID := extractTemplateIDFromNestedPath(r.URL.Path)

	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	var req struct {
		Name        string                 `json:"name"`
		Category    string                 `json:"category"`
		Metadata    map[string]interface{} `json:"metadata"`
		Tags        []string               `json:"tags"`
	}
	json.Unmarshal(body, &req)

	tenantID := middleware.TenantIDFromContext(r.Context())
	tmpl, err := h.TemplateStore.GetByIDAndTenant(templateID, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Template not found", r.URL.Path, reqID)
		return
	}

	clone := &store.Template{
		TenantID:            tenantID,
		Name:                req.Name,
		Description:         tmpl.Description,
		Category:            req.Category,
		Version:             "1.0.0",
		Agents:              tmpl.Agents,
		Workflows:           tmpl.Workflows,
		MemoryTopology:      tmpl.MemoryTopology,
		GovernanceRules:     tmpl.GovernanceRules,
		KPIS:                tmpl.KPIS,
		Integrations:        tmpl.Integrations,
		OperationalPolicies: tmpl.OperationalPolicies,
		Metadata:            req.Metadata,
		Tags:                req.Tags,
		CreatedBy:           middleware.UserIDFromContext(r.Context()),
		Status:              "draft",
	}

	created, err := h.TemplateStore.Create(clone)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "about:blank", "Internal Server Error",
			"Failed to clone template", r.URL.Path, reqID)
		return
	}

	h.EventPublisher.PublishTemplateCloned(events.TemplateClonedPayload{
		Event:            "template.cloned",
		SourceTemplateID: templateID,
		ClonedTemplateID: created.ID,
		Name:             created.Name,
		Category:         created.Category,
		CreatedAt:        created.CreatedAt,
		CreatedBy:        created.CreatedBy,
		TenantID:         middleware.TenantIDFromContext(r.Context()),
	})

	writeJSON(w, http.StatusCreated, toTemplateResponse(created))
}

func (h *TemplateHandlers) handleListDeployments(w http.ResponseWriter, r *http.Request, reqID string) {
	templateID := extractTemplateIDFromNestedPath(r.URL.Path)
	tenantID := middleware.TenantIDFromContext(r.Context())

	page := 1
	pageSize := 20
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := parsePositiveInt(p); err == nil {
			page = n
		}
	}

	deployments, total, hasMore := h.DeploymentStore.ListByTemplate(templateID, tenantID, page, pageSize)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": toDeploymentListResponse(deployments),
		"meta": map[string]interface{}{
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"has_more":  hasMore,
		},
	})
}

func (h *TemplateHandlers) handleGetDeployment(w http.ResponseWriter, r *http.Request, reqID, templateID, deploymentID string) {
	if deploymentID == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid deployment ID", r.URL.Path, reqID)
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	deployment, err := h.DeploymentStore.GetByIDAndTenant(deploymentID, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Deployment not found", r.URL.Path, reqID)
		return
	}

	writeJSON(w, http.StatusOK, toDeploymentResponse(deployment))
}

func (h *TemplateHandlers) handleUpdateDeployment(w http.ResponseWriter, r *http.Request, reqID, templateID, deploymentID string) {
	if deploymentID == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid deployment ID", r.URL.Path, reqID)
		return
	}

	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	var patch map[string]interface{}
	if err := json.Unmarshal(body, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid JSON body", r.URL.Path, reqID)
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	deployment, err := h.DeploymentStore.GetByIDAndTenant(deploymentID, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Deployment not found", r.URL.Path, reqID)
		return
	}

	// Verify ownership
	if deployment.TenantID != tenantID {
		writeError(w, http.StatusForbidden, "about:blank", "Forbidden",
			"Access denied", r.URL.Path, reqID)
		return
	}

	if status, ok := patch["status"]; ok {
		if statusStr, ok := status.(string); ok {
			updated, err := h.DeploymentStore.UpdateStatus(deploymentID, statusStr, middleware.UserIDFromContext(r.Context()))
			if err != nil {
				writeError(w, http.StatusNotFound, "about:blank", "Not Found",
					"Deployment not found", r.URL.Path, reqID)
				return
			}

			msg := ""
			if errMsg, exists := patch["error_message"]; exists {
				if s, ok := errMsg.(string); ok {
					msg = s
				}
			}

			if statusStr == "failed" {
				updated.ErrorMessage = msg
				h.DeploymentStore.UpdateStatus(deploymentID, statusStr, middleware.UserIDFromContext(r.Context()))
				h.EventPublisher.PublishTemplateDeploymentFailed(events.TemplateDeploymentFailedPayload{
					Event:          "template.deployment_failed",
					DeploymentID:   deploymentID,
					TemplateID:     templateID,
					Version:        updated.Version,
					Environment:    updated.Environment,
					ErrorMessage:   msg,
					FailedAt:       time.Now(),
					FailedBy:       middleware.UserIDFromContext(r.Context()),
					TenantID:       middleware.TenantIDFromContext(r.Context()),
				})
			} else if statusStr == "operational" {
				h.EventPublisher.PublishTemplateDeployed(events.TemplateDeployedPayload{
					Event:        "template.deployed",
					DeploymentID: deploymentID,
					TemplateID:   templateID,
					Version:      updated.Version,
					Environment:  updated.Environment,
					Status:       "operational",
					DeployedAt:   time.Now(),
					DeployedBy:   middleware.UserIDFromContext(r.Context()),
					TenantID:     middleware.TenantIDFromContext(r.Context()),
				})
			}

			writeJSON(w, http.StatusOK, toDeploymentResponse(updated))
			return
		}
	}

	updated, err := h.DeploymentStore.GetByIDAndTenant(deploymentID, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Deployment not found", r.URL.Path, reqID)
		return
	}

	writeJSON(w, http.StatusOK, toDeploymentResponse(updated))
}

func (h *TemplateHandlers) handleListVersions(w http.ResponseWriter, r *http.Request, reqID string) {
	templateID := extractTemplateIDFromNestedPath(r.URL.Path)
	tenantID := middleware.TenantIDFromContext(r.Context())

	versions := h.VersionStore.ListByTemplate(templateID, tenantID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"versions": versions,
		},
		"meta": map[string]interface{}{
			"total": len(versions),
		},
	})
}

func (h *TemplateHandlers) handleGetVersion(w http.ResponseWriter, r *http.Request, reqID, templateID, versionID string) {
	if versionID == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid version ID", r.URL.Path, reqID)
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	v, err := h.VersionStore.GetByIDAndTenant(versionID, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Version not found", r.URL.Path, reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":            v.ID,
		"template_id":   v.TemplateID,
		"version":       v.Version,
		"snapshot":      v.Snapshot,
		"created_at":    v.CreatedAt,
	})
}
