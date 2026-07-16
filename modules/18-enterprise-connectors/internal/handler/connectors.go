package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/operan/enterprise-connectors/internal/ctxkeys"
	"github.com/operan/enterprise-connectors/internal/store"
)

// ConnectorHandler handles HTTP requests for connector CRUD operations.
type ConnectorHandler struct {
	store *store.ConnectorStore
}

// NewConnectorHandler creates a new ConnectorHandler.
func NewConnectorHandler(s *store.ConnectorStore) *ConnectorHandler {
	return &ConnectorHandler{store: s}
}

// Routes registers connector routes on the given router.
func (h *ConnectorHandler) Routes(r chi.Router) {
	r.Get("/v1/connectors", h.ListConnectors)
	r.Post("/v1/connectors", h.CreateConnector)
	r.Get("/v1/connectors/{id}", h.GetConnector)
	r.Delete("/v1/connectors/{id}", h.DeleteConnector)
}

// ListConnectors handles GET /v1/connectors.
func (h *ConnectorHandler) ListConnectors(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	connectorType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	connectors, total, err := h.store.List(r.Context(), tenantID, connectorType, status, page, pageSize)
	if err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"connectors": connectors,
		"page":       page,
		"page_size":  pageSize,
		"total":      total,
		"has_more":   (page * pageSize) < total,
	})
}

// CreateConnector handles POST /v1/connectors.
func (h *ConnectorHandler) CreateConnector(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())

	var req struct {
		Name          string                 `json:"name"`
		ConnectorType string                 `json:"connector_type"`
		Description   *string                `json:"description,omitempty"`
		AuthMethod    string                 `json:"auth_method"`
		Config        map[string]interface{} `json:"config"`
		Credentials   map[string]interface{} `json:"credentials"`
		SyncFrequency string                 `json:"sync_frequency"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, `{"error":"bad-request","message":"name is required"}`, http.StatusBadRequest)
		return
	}
	if req.ConnectorType == "" {
		http.Error(w, `{"error":"bad-request","message":"connector_type is required"}`, http.StatusBadRequest)
		return
	}

	conn := &store.Connector{
		TenantID:        tenantID,
		Name:            req.Name,
		Description:     req.Description,
		ConnectorType:   req.ConnectorType,
		AuthMethod:      req.AuthMethod,
		Config:          req.Config,
		Credentials:     req.Credentials,
		SyncFrequency:   req.SyncFrequency,
		ToolsRegistered: false,
	}
	if conn.SyncFrequency == "" {
		conn.SyncFrequency = "manual"
	}

	if err := h.store.Create(r.Context(), conn); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"connector": conn,
	})
}

// GetConnector handles GET /v1/connectors/{id}.
func (h *ConnectorHandler) GetConnector(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid connector ID"}`, http.StatusBadRequest)
		return
	}

	conn, err := h.store.GetByID(r.Context(), id, tenantID)
	if err != nil {
		if err == store.ErrNotFound {
			http.Error(w, `{"error":"not-found","message":"connector not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"connector": conn,
	})
}

// DeleteConnector handles DELETE /v1/connectors/{id}.
func (h *ConnectorHandler) DeleteConnector(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid connector ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.store.Delete(r.Context(), id, tenantID); err != nil {
		if err == store.ErrNotFound {
			http.Error(w, `{"error":"not-found","message":"connector not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "connector deleted",
		"id":      id,
	})
}