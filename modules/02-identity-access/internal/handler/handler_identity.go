package handler

import (
	"encoding/json"
	"net/http"
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

// Create handles POST /tenants/{id}/iam/service-identities
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
		TenantID: req.TenantID,
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

// GetByID handles GET /tenants/{id}/iam/service-identities/{identity_id}
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

// extractIdentityID extracts the identity_id from the URL path.
func extractIdentityID(path string) string {
	parts := splitPath(path)
	if len(parts) >= 6 && parts[4] == "service-identities" {
		return parts[5]
	}
	return ""
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

// Register handles POST /tenants/{id}/iam/agent-identities
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
		TenantID:          req.TenantID,
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
			"agent_id":            identity.AgentID,
			"capabilities":        identity.Capabilities,
			"allowed_tools":       identity.AllowedTools,
			"escalation_targets":  identity.EscalationTargets,
		},
		Timestamp: time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(identity)
}

// GetByAgent handles GET /tenants/{id}/iam/agent-identities/agent/{agent_id}
func (h *AgentIdentityHandler) GetByAgent(w http.ResponseWriter, r *http.Request) {
	// Extract agent_id from URL path: /tenants/{id}/iam/agent-identities/agent/{agent_id}
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
func extractAgentID(path string) string {
	parts := splitPath(path)
	// Expected: /tenants/{id}/iam/agent-identities/agent/{agent_id}
	if len(parts) >= 8 && parts[4] == "agent-identities" && parts[6] == "agent" {
		return parts[7]
	}
	return ""
}
