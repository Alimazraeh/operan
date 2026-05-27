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

// SSOHandler handles SSO-related HTTP endpoints.
type SSOHandler struct {
	Configs   *store.SSOConfigStore
	Audit     *store.AuditStore
	Publisher *events.Publisher
}

// NewSSOHandler creates a new SSO handler.
func NewSSOHandler(configs *store.SSOConfigStore, audit *store.AuditStore, publisher *events.Publisher) *SSOHandler {
	return &SSOHandler{
		Configs:   configs,
		Audit:     audit,
		Publisher: publisher,
	}
}

// Configure handles POST /tenants/{id}/iam/sso/configure
func (h *SSOHandler) Configure(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req models.ConfigureSSORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	config := &models.SSOConfig{
		TenantID:      tenantID,
		Provider:      req.Provider,
		Type:          req.Type,
		Configuration: req.Configuration,
	}

	if err := h.Configs.Create(config); err != nil {
		http.Error(w, `{"error":"failed to configure SSO: `+err.Error()+`"}`, http.StatusConflict)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "configure_sso",
		ResourceType: "sso_config",
		ResourceID:   config.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"provider": config.Provider,
			"type":     config.Type,
		},
		Timestamp: time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(config)
}

// GetConfig handles GET /tenants/{id}/iam/sso/config
func (h *SSOHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	config, err := h.Configs.GetByTenant(tenantID)
	if err != nil {
		http.Error(w, `{"error":"SSO config not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// SCIMHandler handles SCIM provisioning endpoints.
type SCIMHandler struct {
	Users     *store.UserStore
	Audit     *store.AuditStore
	Publisher *events.Publisher
}

// NewSCIMHandler creates a new SCIM handler.
func NewSCIMHandler(users *store.UserStore, audit *store.AuditStore, publisher *events.Publisher) *SCIMHandler {
	return &SCIMHandler{
		Users:     users,
		Audit:     audit,
		Publisher: publisher,
	}
}

// Provision handles POST /tenants/{id}/iam/scim/provision
func (h *SCIMHandler) Provision(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := "scim"

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Extract fields
	email, _ := req["email"].(string)
	displayName, _ := req["displayName"].(string)
	roles, _ := req["roles"].([]interface{})
	status, _ := req["status"].(string)

	if email == "" || displayName == "" {
		http.Error(w, `{"error":"email and display_name are required"}`, http.StatusBadRequest)
		return
	}

	// Convert roles to string slice
	roleList := make([]string, 0, len(roles))
	for _, r := range roles {
		if role, ok := r.(string); ok {
			roleList = append(roleList, role)
		}
	}

	user := &models.User{
		TenantID:           tenantID,
		Email:              email,
		DisplayName:        displayName,
		Roles:              roleList,
		AuthenticationMethod: "scim",
	}

	if status == "active" {
		user.Status = "active"
	} else {
		user.Status = "pending"
	}

	if err := h.Users.Create(user); err != nil {
		http.Error(w, `{"error":"failed to provision user: `+err.Error()+`"}`, http.StatusConflict)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "scim",
		Action:       "provision_user",
		ResourceType: "user",
		ResourceID:   user.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"email": user.Email,
			"roles": roleList,
		},
		Timestamp: time.Now().UTC(),
	})

	// Publish event
	h.Publisher.UserCreated(r.Context(), user.ID, tenantID, user.Email, "default", actorID, "scim", "", time.Now().UTC().Format(time.RFC3339))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}
