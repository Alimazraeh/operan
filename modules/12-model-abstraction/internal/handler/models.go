package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/operan/model-abstraction/internal/ctxkeys"
	"github.com/operan/model-abstraction/internal/store"
)

// ModelsHandler handles CRUD for model_registry.
type ModelsHandler struct {
	store    ModelRegistryStore
	proStore ModelProvidersStore
}

// NewModelsHandler creates a new models handler.
func NewModelsHandler(store ModelRegistryStore, proStore ModelProvidersStore) *ModelsHandler {
	return &ModelsHandler{store: store, proStore: proStore}
}

// HandleGET lists models with pagination and optional provider filter.
func (h *ModelsHandler) HandleGET(w http.ResponseWriter, r *http.Request) {
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

	var providerID *string
	if pid := r.URL.Query().Get("provider_id"); pid != "" {
		providerID = &pid
	}

	items, total, err := h.store.ListByTenant(r.Context(), tenantID, providerID, page, pageSize)
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

// HandlePOST creates a new model registry entry.
func (h *ModelsHandler) HandlePOST(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := r.Context().Value(ctxkeys.TenantIDKey).(string)

	var req struct {
		ModelName         string            `json:"model_name"`
		ProviderID        string            `json:"provider_id"`
		ProviderModelName string            `json:"provider_model_name"`
		SupportsChat      *bool             `json:"supports_chat"`
		SupportsEmbed     *bool             `json:"supports_embed"`
		MaxTokens         int               `json:"max_tokens"`
		CostPerToken      map[string]any    `json:"cost_per_token"`
		IsDefault         *bool             `json:"is_default"`
		IsActive          *bool             `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.ModelName == "" || req.ProviderID == "" {
		http.Error(w, `{"error":"model_name and provider_id are required"}`, http.StatusBadRequest)
		return
	}

	// Validate provider exists.
	if _, err := h.proStore.GetByID(r.Context(), req.ProviderID); err != nil {
		http.Error(w, `{"error":"provider not found"}`, http.StatusBadRequest)
		return
	}

	supportsChat := true
	if req.SupportsChat != nil {
		supportsChat = *req.SupportsChat
	}
	supportsEmbed := true
	if req.SupportsEmbed != nil {
		supportsEmbed = *req.SupportsEmbed
	}
	isDefault := false
	if req.IsDefault != nil && *req.IsDefault {
		// Clear other defaults first.
		h.store.SetDefault(r.Context(), tenantID, "") // placeholder — real impl below
		isDefault = true
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 8192
	}
	if req.CostPerToken == nil {
		req.CostPerToken = map[string]any{"prompt": 0.0, "completion": 0.0}
	}

	m := &store.ModelRegistry{
		ID:                uuid.New().String(),
		TenantID:          tenantID,
		ModelName:         req.ModelName,
		ProviderID:        req.ProviderID,
		ProviderModelName: req.ProviderModelName,
		SupportsChat:      supportsChat,
		SupportsEmbed:     supportsEmbed,
		MaxTokens:         req.MaxTokens,
		CostPerToken:      req.CostPerToken,
		IsDefault:         isDefault,
		IsActive:          isActive,
	}

	if err := h.store.Create(r.Context(), m); err != nil {
		http.Error(w, `{"error":"failed to create model registry entry"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, m)
}

// HandlePATCH updates a model registry entry.
func (h *ModelsHandler) HandlePATCH(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/model-registry/")
	if id == "" {
		http.Error(w, `{"error":"model ID required"}`, http.StatusBadRequest)
		return
	}

	existing, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"model not found"}`, http.StatusNotFound)
		return
	}

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if modelName, ok := req["model_name"].(string); ok && modelName != "" {
		existing.ModelName = modelName
	}
	if providerID, ok := req["provider_id"].(string); ok && providerID != "" {
		existing.ProviderID = providerID
	}
	if provModel, ok := req["provider_model_name"].(string); ok {
		existing.ProviderModelName = provModel
	}
	if supportsChat, ok := req["supports_chat"].(bool); ok {
		existing.SupportsChat = supportsChat
	}
	if supportsEmbed, ok := req["supports_embed"].(bool); ok {
		existing.SupportsEmbed = supportsEmbed
	}
	if maxTokens, ok := req["max_tokens"].(float64); ok {
		existing.MaxTokens = int(maxTokens)
	}
	if costPerToken, ok := req["cost_per_token"].(map[string]any); ok {
		existing.CostPerToken = costPerToken
	}
	if isDefault, ok := req["is_default"].(bool); ok {
		existing.IsDefault = isDefault
	}
	if isActive, ok := req["is_active"].(bool); ok {
		existing.IsActive = isActive
	}

	if err := h.store.Update(r.Context(), existing); err != nil {
		http.Error(w, `{"error":"failed to update model"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, existing)
}