package handler

import (
	"net/http"

	"github.com/operan/model-routing/internal/ctxkeys"
)

// ModelsHandler lists available models for a tenant.
type ModelsHandler struct{}

// NewModelsHandler creates a new models handler.
func NewModelsHandler(_ interface{}) *ModelsHandler {
	return &ModelsHandler{}
}

// HandleGet handles GET /v1/models
func (h *ModelsHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	_ = r.Context().Value(ctxkeys.TenantIDKey).(string)

	// Models are managed by M12; M13 lists aliases available to the tenant.
	models := []string{"qwen-turbo", "qwen-plus", "qwen-max", "text-embedding-ada-002"}

	WriteJSON(w, http.StatusOK, models)
}