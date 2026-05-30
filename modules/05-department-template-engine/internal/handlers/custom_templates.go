package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/operan/modules/05-department-template-engine/internal/middleware"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

// CreateCustomTemplate handles POST /templates/custom
func (h *TemplateHandlers) CreateCustomTemplate(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Failed to read request body", r.URL.Path, reqID)
		return
	}
	defer r.Body.Close()

	var req store.CustomTemplateCreate
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

	ct := &store.CustomTemplate{
		TenantID:    middleware.TenantIDFromContext(r.Context()),
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		Content:     req.Content,
		OwnerID:     middleware.UserIDFromContext(r.Context()),
		SharedWith:  req.SharedWith,
		Version:     "1.0.0",
		Status:      "draft",
		CreatedBy:   middleware.UserIDFromContext(r.Context()),
	}

	created, err := h.CustomTemplateStore.Create(ct)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "about:blank", "Internal Server Error",
			"Failed to create custom template", r.URL.Path, reqID)
		return
	}

	writeJSON(w, http.StatusCreated, toCustomTemplateResponse(created))
}

// ListCustomTemplates handles GET /templates/custom
func (h *TemplateHandlers) ListCustomTemplates(w http.ResponseWriter, r *http.Request) {
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

	templates, total, hasMore := h.CustomTemplateStore.List(middleware.TenantIDFromContext(r.Context()), page, pageSize, categoryFilter)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": toCustomTemplateListResponse(templates),
		"meta": map[string]interface{}{
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"has_more":  hasMore,
		},
	})
}

// GetCustomTemplate handles GET /templates/custom/{id}
func (h *TemplateHandlers) GetCustomTemplate(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())
	id := extractIDFromPath(r.URL.Path, "/templates/custom/")

	if id == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid custom template ID", r.URL.Path, reqID)
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	ct, err := h.CustomTemplateStore.GetByIDAndTenant(id, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Custom template not found", r.URL.Path, reqID)
		return
	}

	writeJSON(w, http.StatusOK, toCustomTemplateResponse(ct))
}

// UpdateCustomTemplate handles PATCH /templates/custom/{id}
func (h *TemplateHandlers) UpdateCustomTemplate(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())
	id := extractIDFromPath(r.URL.Path, "/templates/custom/")

	if id == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid custom template ID", r.URL.Path, reqID)
		return
	}

	_, err := h.CustomTemplateStore.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Custom template not found", r.URL.Path, reqID)
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

	updated, err := h.CustomTemplateStore.Update(id, patch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "about:blank", "Internal Server Error",
			"Failed to update custom template", r.URL.Path, reqID)
		return
	}

	writeJSON(w, http.StatusOK, toCustomTemplateResponse(updated))
}

// DeleteCustomTemplate handles DELETE /templates/custom/{id}
func (h *TemplateHandlers) DeleteCustomTemplate(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())
	id := extractIDFromPath(r.URL.Path, "/templates/custom/")

	if id == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid custom template ID", r.URL.Path, reqID)
		return
	}

	if err := h.CustomTemplateStore.Delete(id, middleware.TenantIDFromContext(r.Context())); err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Custom template not found", r.URL.Path, reqID)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
