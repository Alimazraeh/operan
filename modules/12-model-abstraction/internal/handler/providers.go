package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/operan/model-abstraction/internal/config"
	"github.com/operan/model-abstraction/internal/ctxkeys"
	"github.com/operan/model-abstraction/internal/store"
)

// ProvidersHandler handles CRUD for model_providers.
type ProvidersHandler struct {
	store ModelProvidersStore
	cfg   *config.Config
}

// NewProvidersHandler creates a new providers handler.
func NewProvidersHandler(store ModelProvidersStore, cfg *config.Config) *ProvidersHandler {
	return &ProvidersHandler{store: store, cfg: cfg}
}

// HandleGET lists providers with pagination.
func (h *ProvidersHandler) HandleGET(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := r.Context().Value(ctxkeys.TenantIDKey).(string)

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	pageSize := 20
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
			pageSize = v
		}
	}

	items, total, err := h.store.ListByTenant(r.Context(), tenantID, page, pageSize)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// HandlePOST creates a new provider.
func (h *ProvidersHandler) HandlePOST(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := r.Context().Value(ctxkeys.TenantIDKey).(string)

	var req struct {
		Name             string            `json:"name"`
		Description      string            `json:"description"`
		Type             string            `json:"type"`
		BaseURL          string            `json:"base_url"`
		APIKeySecretName string            `json:"api_key_secret_name"`
		IsActive         *bool             `json:"is_active"`
		Priority         int               `json:"priority"`
		MaxRetries       int               `json:"max_retries"`
		TimeoutMs        int               `json:"timeout_ms"`
		Metadata         map[string]any    `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Type == "" || req.BaseURL == "" {
		http.Error(w, `{"error":"name, type, and base_url are required"}`, http.StatusBadRequest)
		return
	}

	validTypes := map[string]bool{
		"openai": true, "anthropic": true, "litellm": true,
		"ollama": true, "azure": true, "custom": true,
	}
	if !validTypes[req.Type] {
		http.Error(w, `{"error":"invalid type: must be openai, anthropic, litellm, ollama, azure, or custom"}`, http.StatusBadRequest)
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	if req.Priority == 0 {
		req.Priority = 50
	}
	if req.MaxRetries == 0 {
		req.MaxRetries = 2
	}
	if req.TimeoutMs == 0 {
		req.TimeoutMs = 30000
	}
	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}

	p := &store.ModelProvider{
		ID:                uuid.New().String(),
		TenantID:          tenantID,
		Name:              req.Name,
		Description:       req.Description,
		Type:              req.Type,
		BaseURL:           req.BaseURL,
		APIKeySecretName:  req.APIKeySecretName,
		IsActive:          isActive,
		Priority:          req.Priority,
		MaxRetries:        req.MaxRetries,
		TimeoutMs:         req.TimeoutMs,
		Metadata:          req.Metadata,
	}

	if err := h.store.Create(r.Context(), p); err != nil {
		http.Error(w, `{"error":"failed to create provider"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, p)
}

// HandlePATCH updates a provider.
func (h *ProvidersHandler) HandlePATCH(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/model-providers/")
	if id == "" {
		http.Error(w, `{"error":"provider ID required"}`, http.StatusBadRequest)
		return
	}

	existing, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"provider not found"}`, http.StatusNotFound)
		return
	}

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if name, ok := req["name"].(string); ok && name != "" {
		existing.Name = name
	}
	if desc, ok := req["description"].(string); ok {
		existing.Description = desc
	}
	if typ, ok := req["type"].(string); ok && typ != "" {
		existing.Type = typ
	}
	if url, ok := req["base_url"].(string); ok && url != "" {
		existing.BaseURL = url
	}
	if isActive, ok := req["is_active"].(bool); ok {
		existing.IsActive = isActive
	}
	if priority, ok := req["priority"].(float64); ok {
		existing.Priority = int(priority)
	}
	if maxRetries, ok := req["max_retries"].(float64); ok {
		existing.MaxRetries = int(maxRetries)
	}
	if timeoutMs, ok := req["timeout_ms"].(float64); ok {
		existing.TimeoutMs = int(timeoutMs)
	}
	if meta, ok := req["metadata"].(map[string]any); ok {
		existing.Metadata = meta
	}

	if err := h.store.Update(r.Context(), existing); err != nil {
		http.Error(w, `{"error":"failed to update provider"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

// HandleDELETE soft-deletes a provider.
func (h *ProvidersHandler) HandleDELETE(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/model-providers/")
	if id == "" {
		http.Error(w, `{"error":"provider ID required"}`, http.StatusBadRequest)
		return
	}

	if err := h.store.SoftDelete(r.Context(), id); err != nil {
		http.Error(w, `{"error":"failed to delete provider"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}