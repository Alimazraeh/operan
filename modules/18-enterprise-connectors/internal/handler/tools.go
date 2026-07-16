package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/operan/enterprise-connectors/internal/connectors"
)

// ToolsHandler handles HTTP requests for tool listing.
type ToolsHandler struct {
	registry *connectors.Registry
}

// NewToolsHandler creates a new ToolsHandler.
func NewToolsHandler(r *connectors.Registry) *ToolsHandler {
	return &ToolsHandler{registry: r}
}

// Routes registers tool routes on the given router.
func (h *ToolsHandler) Routes(r chi.Router) {
	r.Get("/v1/tools", h.ListAllTools)
	r.Get("/v1/connectors/{id}/tools", h.ListConnectorTools)
}

// ListAllTools handles GET /v1/tools.
func (h *ToolsHandler) ListAllTools(w http.ResponseWriter, r *http.Request) {
	tools := h.registry.ListTools()

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"tools": tools,
		"total": len(tools),
	})
}

// ListConnectorTools handles GET /v1/connectors/{id}/tools.
func (h *ToolsHandler) ListConnectorTools(w http.ResponseWriter, r *http.Request) {
	connectorID := chi.URLParam(r, "id")

	tools := h.registry.ListTools()
	if conn, err := h.registry.Get(connectorID); err == nil && conn != nil {
		tools = conn.GetTools()
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"connector_type": connectorID,
		"tools":          tools,
		"total":          len(tools),
	})
}