// Package handlers provides HTTP request handlers for Module 05.
package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/events"
	"github.com/operan/modules/05-department-template-engine/internal/middleware"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

// TemplateHandlers holds all stores and the event publisher for template operations.
type TemplateHandlers struct {
	TemplateStore       *store.TemplateStore
	CustomTemplateStore *store.CustomTemplateStore
	DeploymentStore     *store.DeploymentStore
	VersionStore        *store.VersionStore
	EventPublisher      *events.Publisher
	MaxPageSize         int
}

// allowedChangedFields is the set of fields that can appear in changed_fields events.
// Matches the AsyncAPI enum: [agents, workflows, memory_topology, governance_rules, kpis, integrations, operational_policies, metadata, tags]
var allowedChangedFields = map[string]struct{}{
	"agents":               {},
	"workflows":            {},
	"memory_topology":      {},
	"governance_rules":     {},
	"kpis":                 {},
	"integrations":         {},
	"operational_policies": {},
	"metadata":             {},
	"tags":                 {},
}

// filterChangedFields filters patch keys against allowed enum values.
func filterChangedFields(patch map[string]interface{}) []string {
	changedFields := []string{}
	for field := range patch {
		if _, allowed := allowedChangedFields[field]; allowed {
			changedFields = append(changedFields, field)
		}
	}
	return changedFields
}

// NewTemplateHandlers creates a new handlers instance.
func NewTemplateHandlers(ts *store.TemplateStore, cts *store.CustomTemplateStore,
	ds *store.DeploymentStore, vs *store.VersionStore, ep *events.Publisher, maxPageSize int) *TemplateHandlers {
	return &TemplateHandlers{
		TemplateStore:       ts,
		CustomTemplateStore: cts,
		DeploymentStore:     ds,
		VersionStore:        vs,
		EventPublisher:      ep,
		MaxPageSize:         maxPageSize,
	}
}

// ─── Standard Template CRUD ──────────────────────────────────────────────────

// CreateTemplate handles POST /templates
func (h *TemplateHandlers) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Failed to read request body", r.URL.Path, reqID)
		return
	}
	defer r.Body.Close()

	var req store.TemplateCreate
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid JSON body", r.URL.Path, reqID)
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"name is required", r.URL.Path, reqID)
		return
	}

	tmpl := &store.Template{
		TenantID:            middleware.TenantIDFromContext(r.Context()),
		Name:                req.Name,
		Description:         req.Description,
		Category:            req.Category,
		Version:             "1.0.0",
		Agents:              req.Agents,
		Workflows:           req.Workflows,
		MemoryTopology:      req.MemoryTopology,
		GovernanceRules:     req.GovernanceRules,
		KPIS:                req.KPIS,
		Integrations:        req.Integrations,
		OperationalPolicies: req.OperationalPolicies,
		Metadata:            req.Metadata,
		Tags:                req.Tags,
		CreatedBy:           middleware.UserIDFromContext(r.Context()),
		Status:              "draft",
	}

	created, err := h.TemplateStore.Create(tmpl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "about:blank", "Internal Server Error",
			"Failed to create template", r.URL.Path, reqID)
		return
	}

	h.EventPublisher.PublishTemplateCreated(events.TemplateCreatedPayload{
		Event:             "template.created",
		TemplateID:        created.ID,
		Name:              created.Name,
		Category:          created.Category,
		Version:           created.Version,
		Description:       created.Description,
		AgentsCount:       len(created.Agents),
		WorkflowsCount:    len(created.Workflows),
		IntegrationsCount: len(created.Integrations),
		CreatedAt:         created.CreatedAt,
		CreatedBy:         created.CreatedBy,
		TenantID:          middleware.TenantIDFromContext(r.Context()),
	})

	writeJSON(w, http.StatusCreated, toTemplateResponse(created))
}

// ListTemplates handles GET /templates
func (h *TemplateHandlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	page := 1
	pageSize := 20
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := parsePositiveInt(p); err == nil {
			page = n
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if n, err := parsePositiveInt(ps); err == nil {
			pageSize = n
		}
	}
	if pageSize > h.MaxPageSize {
		pageSize = h.MaxPageSize
	}

	var categoryFilter *string
	if cat := r.URL.Query().Get("category"); cat != "" {
		categoryFilter = &cat
	}

	templates, total, hasMore := h.TemplateStore.List(middleware.TenantIDFromContext(r.Context()), page, pageSize, categoryFilter)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": toTemplateListResponse(templates),
		"meta": map[string]interface{}{
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"has_more":  hasMore,
		},
	})
}

// GetTemplate handles GET /templates/{id}
func (h *TemplateHandlers) GetTemplate(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())
	id := extractIDFromPath(r.URL.Path, "/templates/")

	if id == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid template ID", r.URL.Path, reqID)
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	tmpl, err := h.TemplateStore.GetByIDAndTenant(id, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Template not found", r.URL.Path, reqID)
		return
	}

	writeJSON(w, http.StatusOK, toTemplateResponse(tmpl))
}

// UpdateTemplate handles PATCH /templates/{id}
func (h *TemplateHandlers) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())
	id := extractIDFromPath(r.URL.Path, "/templates/")

	if id == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid template ID", r.URL.Path, reqID)
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	tmpl, err := h.TemplateStore.GetByIDAndTenant(id, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Template not found", r.URL.Path, reqID)
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

	updated, err := h.TemplateStore.UpdateByTenant(id, tenantID, patch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "about:blank", "Internal Server Error",
			"Failed to update template", r.URL.Path, reqID)
		return
	}

	changedFields := filterChangedFields(patch)

	h.EventPublisher.PublishTemplateUpdated(events.TemplateUpdatedPayload{
		Event:           "template.updated",
		TemplateID:      updated.ID,
		Name:            updated.Name,
		Category:        updated.Category,
		Version:         updated.Version,
		PreviousVersion: tmpl.Version,
		ChangedFields:   changedFields,
		UpdatedAt:       updated.UpdatedAt,
		UpdatedBy:       middleware.UserIDFromContext(r.Context()),
		TenantID:        middleware.TenantIDFromContext(r.Context()),
	})

	writeJSON(w, http.StatusOK, toTemplateResponse(updated))
}

// DeleteTemplate handles DELETE /templates/{id}
func (h *TemplateHandlers) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())
	id := extractIDFromPath(r.URL.Path, "/templates/")

	if id == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid template ID", r.URL.Path, reqID)
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	tmpl, err := h.TemplateStore.GetByIDAndTenant(id, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Template not found", r.URL.Path, reqID)
		return
	}

	if err := h.TemplateStore.Delete(id, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "about:blank", "Internal Server Error",
			"Failed to delete template", r.URL.Path, reqID)
		return
	}

	h.EventPublisher.PublishTemplateDeleted(events.TemplateDeletedPayload{
		Event:      "template.deleted",
		TemplateID: id,
		Name:       tmpl.Name,
		Category:   tmpl.Category,
		DeletedAt:  time.Now(),
		DeletedBy:  middleware.UserIDFromContext(r.Context()),
		TenantID:   tenantID,
	})

	w.WriteHeader(http.StatusNoContent)
}
