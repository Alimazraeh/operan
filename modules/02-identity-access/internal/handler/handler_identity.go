package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
	"github.com/operan/modules/02-identity-access/internal/store"
)

// ServiceIdentityHandler handles service identity-related HTTP endpoints.
type ServiceIdentityHandler struct {
	IDs       *store.ServiceIdentityStore
	Audit     *store.AuditStore
	Publisher *events.Publisher
}

// NewServiceIdentityHandler creates a new service identity handler.
func NewServiceIdentityHandler(ids *store.ServiceIdentityStore, audit *store.AuditStore, publisher *events.Publisher) *ServiceIdentityHandler {
	return &ServiceIdentityHandler{
		IDs:       ids,
		Audit:     audit,
		Publisher: publisher,
	}
}

// Create handles POST /api/v1/iam/service-identities
func (h *ServiceIdentityHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req models.CreateServiceIdentityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	identity := &models.ServiceIdentity{
		TenantID: tenantID,
		Name:     req.Name,
		Roles:    req.Roles,
	}
	if req.Metadata != nil {
		identity.Metadata = *req.Metadata
	}

	if err := h.IDs.Create(identity); err != nil {
		http.Error(w, `{"error":"failed to create service identity: `+err.Error()+`"}`, http.StatusConflict)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "create_service_identity",
		ResourceType: "service_identity",
		ResourceID:   identity.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"name":    identity.Name,
			"roles":   identity.Roles,
			"api_key": identity.APIKeyID[:10] + "...", // Don't log full key
		},
		Timestamp: time.Now().UTC(),
	})

	// Publish event
	h.Publisher.IdentityRotated(r.Context(), identity.ID, tenantID, "service", identity.APIKeyID, actorID, "", time.Now().UTC().Format(time.RFC3339))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(identity)
}

// List handles GET /api/v1/iam/service-identities
func (h *ServiceIdentityHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")

	page := 1
	pageSize := 50
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	identities, err := h.IDs.List(tenantID)
	if err != nil {
		http.Error(w, `{"error":"failed to list service identities"}`, http.StatusInternalServerError)
		return
	}
	total := len(identities)

	// Redact API keys in response
	for _, id := range identities {
		id.APIKeyID = "[REDACTED]"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service_identities": identities,
		"total":              total,
		"page":               page,
		"page_size":          pageSize,
		"total_pages":        (total + pageSize - 1) / pageSize,
	})
}

// GetByID handles GET /api/v1/iam/service-identities/{id}
func (h *ServiceIdentityHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	identityID := extractIdentityID(r.URL.Path)
	if identityID == "" {
		http.Error(w, `{"error":"identity_id is required"}`, http.StatusBadRequest)
		return
	}

	identity, err := h.IDs.GetByID(identityID)
	if err != nil {
		http.Error(w, `{"error":"service identity not found"}`, http.StatusNotFound)
		return
	}

	// Redact API key in response
	identity.APIKeyID = "[REDACTED]"

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(identity)
}

// extractIdentityID extracts the service identity ID from the URL path.
// Handles: /api/v1/iam/service-identities/{id}
func extractIdentityID(path string) string {
	path = strings.TrimPrefix(path, "/api/v1/iam/service-identities/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return ""
	}
	return path
}

// AgentIdentityHandler handles agent identity-related HTTP endpoints.
type AgentIdentityHandler struct {
	IDs       *store.AgentIdentityStore
	Audit     *store.AuditStore
	Publisher *events.Publisher
}

// NewAgentIdentityHandler creates a new agent identity handler.
func NewAgentIdentityHandler(ids *store.AgentIdentityStore, audit *store.AuditStore, publisher *events.Publisher) *AgentIdentityHandler {
	return &AgentIdentityHandler{
		IDs:       ids,
		Audit:     audit,
		Publisher: publisher,
	}
}

// Register handles POST /api/v1/iam/agent-identities
func (h *AgentIdentityHandler) Register(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req models.RegisterAgentIdentityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	identity := &models.AgentIdentity{
		TenantID:          tenantID,
		AgentID:           req.AgentID,
		Capabilities:      req.Capabilities,
		MemoryScope:       req.MemoryScope,
		AllowedTools:      req.AllowedTools,
		EscalationTargets: req.EscalationTargets,
	}

	if err := h.IDs.Create(identity); err != nil {
		http.Error(w, `{"error":"failed to register agent identity: `+err.Error()+`"}`, http.StatusConflict)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "register_agent_identity",
		ResourceType: "agent_identity",
		ResourceID:   identity.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"agent_id":             identity.AgentID,
			"capabilities":         identity.Capabilities,
			"allowed_tools":        identity.AllowedTools,
			"escalation_targets":   identity.EscalationTargets,
		},
		Timestamp: time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(identity)
}

// List handles GET /api/v1/iam/agent-identities
func (h *AgentIdentityHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")

	page := 1
	pageSize := 50
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	identities, err := h.IDs.ListByTenant(tenantID)
	if err != nil {
		http.Error(w, `{"error":"failed to list agent identities"}`, http.StatusInternalServerError)
		return
	}
	total := len(identities)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_identities": identities,
		"total":            total,
		"page":             page,
		"page_size":        pageSize,
		"total_pages":      (total + pageSize - 1) / pageSize,
	})
}

// GetByAgent handles GET /api/v1/iam/agent-identities/agent/{agent_id}
func (h *AgentIdentityHandler) GetByAgent(w http.ResponseWriter, r *http.Request) {
	// Extract agent_id from URL path: /api/v1/iam/agent-identities/agent/{agent_id}
	agentID := extractAgentID(r.URL.Path)
	if agentID == "" {
		http.Error(w, `{"error":"agent_id is required"}`, http.StatusBadRequest)
		return
	}

	identity, err := h.IDs.GetByAgent(agentID)
	if err != nil {
		http.Error(w, `{"error":"agent identity not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(identity)
}

// extractAgentID extracts the agent_id from the URL path.
// Handles: /api/v1/iam/agent-identities/agent/{agent_id}
func extractAgentID(path string) string {
	const prefix = "/api/v1/iam/agent-identities/agent/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	agentID := path[len(prefix):]
	agentID = strings.TrimSuffix(agentID, "/")
	if agentID == "" {
		return ""
	}
	return agentID
}
