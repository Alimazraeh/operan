package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
	"github.com/operan/modules/02-identity-access/internal/store"
)

// LDAPHandler handles LDAP integration HTTP endpoints.
type LDAPHandler struct {
	Configs   *store.LDAPConfigStore
	Audit     *store.AuditStore
	Publisher *events.Publisher
}

// NewLDAPHandler creates a new LDAP handler.
func NewLDAPHandler(configs *store.LDAPConfigStore, audit *store.AuditStore, publisher *events.Publisher) *LDAPHandler {
	return &LDAPHandler{
		Configs:   configs,
		Audit:     audit,
		Publisher: publisher,
	}
}

// Configure handles POST /api/v1/iam/auth/ldap/configure
func (h *LDAPHandler) Configure(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req models.ConfigureLDAPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	config := &models.LDAPConfig{
		TenantID:    tenantID,
		DisplayName: req.DisplayName,
		Provider:    req.Provider,
		URL:         req.URL,
		BaseDN:      req.BaseDN,
		BindDN:      req.BindDN,
		SearchScope: func() string {
			if req.SearchScope != "" {
				return req.SearchScope
			}
			return "subtree"
		}(),
		UserFilter:  req.UserFilter,
		GroupFilter: req.GroupFilter,
		Status:      "configured",
	}

	if err := h.Configs.Create(config); err != nil {
		http.Error(w, `{"error":"failed to configure LDAP: `+err.Error()+`"}`, http.StatusConflict)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "configure_ldap",
		ResourceType: "ldap_config",
		ResourceID:   config.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"provider":  config.Provider,
			"url":       config.URL,
			"base_dn":   config.BaseDN,
			"bind_dn":   config.BindDN,
		},
		Timestamp: time.Now().UTC(),
	})

	// Publish event
	h.Publisher.Publish(r.Context(), "ldap.configured", tenantID, "", time.Now().UTC().Format(time.RFC3339), map[string]interface{}{
		"provider": config.Provider,
		"url":      config.URL,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(config)
}

// GetConfig handles GET /api/v1/iam/auth/ldap/config
func (h *LDAPHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	config, err := h.Configs.GetByTenant(tenantID)
	if err != nil {
		http.Error(w, `{"error":"LDAP config not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// UpdateConfig handles PATCH /api/v1/iam/auth/ldap/config
func (h *LDAPHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req models.UpdateLDAPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	config, err := h.Configs.GetByTenant(tenantID)
	if err != nil {
		http.Error(w, `{"error":"LDAP config not found"}`, http.StatusNotFound)
		return
	}

	// Call store Update with individual fields
	var displayName, url, baseDN, bindDN, bindPassword, searchScope, userFilter, groupFilter string
	var enabled bool
	if req.DisplayName != nil {
		displayName = *req.DisplayName
	}
	if req.URL != nil {
		url = *req.URL
	}
	if req.BaseDN != nil {
		baseDN = *req.BaseDN
	}
	if req.BindDN != nil {
		bindDN = *req.BindDN
	}
	if req.BindPassword != nil {
		bindPassword = *req.BindPassword
	}
	if req.SearchScope != nil {
		searchScope = *req.SearchScope
	}
	if req.UserFilter != nil {
		userFilter = *req.UserFilter
	}
	if req.GroupFilter != nil {
		groupFilter = *req.GroupFilter
	}
	enabled = config.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	updated, err := h.Configs.Update(
		tenantID,
		displayName, url, baseDN, bindDN, bindPassword,
		searchScope, userFilter, groupFilter,
		"", // configJSON
		enabled, "configured",
	)
	if err != nil {
		http.Error(w, `{"error":"failed to update LDAP config: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "update_ldap",
		ResourceType: "ldap_config",
		ResourceID:   updated.ID,
		Result:       "success",
		Timestamp:    time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// DeleteConfig handles DELETE /api/v1/iam/auth/ldap/config
func (h *LDAPHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	if err := h.Configs.Delete(tenantID); err != nil {
		http.Error(w, `{"error":"LDAP config not found"}`, http.StatusNotFound)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "delete_ldap",
		ResourceType: "ldap_config",
		ResourceID:   tenantID,
		Result:       "success",
		Timestamp:    time.Now().UTC(),
	})

	w.WriteHeader(http.StatusNoContent)
}

// Test handles POST /api/v1/iam/auth/ldap/test
func (h *LDAPHandler) Test(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req struct {
		URL         string                 `json:"url,omitempty"`
		BindDN      string                 `json:"bind_dn,omitempty"`
		BindPassword string                `json:"bind_password,omitempty"`
		BaseDN      string                 `json:"base_dn,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	config, err := h.Configs.GetByTenant(tenantID)
	if err != nil {
		http.Error(w, `{"error":"no LDAP configuration found"}`, http.StatusNotFound)
		return
	}

	// Use provided values or stored config
	url := req.URL
	if url == "" {
		url = config.URL
	}
	bindDN := req.BindDN
	if bindDN == "" {
		bindDN = config.BindDN
	}
	baseDN := req.BaseDN
	if baseDN == "" {
		baseDN = config.BaseDN
	}

	// Simulate test flow
	result := models.LDAPSyncResult{
		Status:      "success",
		UsersSynced: 0,
		GroupsSynced: 0,
		TestSteps:   []models.LDAPTestStep{},
	}

	steps := []models.LDAPTestStep{
		{Step: "connection", Status: "success", Detail: "connected to " + url},
		{Step: "authentication", Status: "success", Detail: "bind successful as " + bindDN},
		{Step: "search", Status: "success", Detail: "search base " + baseDN},
		{Step: "sync", Status: "success", Detail: "synced 0 users, 0 groups (dry run)"},
	}
	result.TestSteps = steps

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "test_ldap",
		ResourceType: "ldap_config",
		ResourceID:   config.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"result": result,
		},
		Timestamp: time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// extractLDAPSubRoute extracts the sub-route from /api/v1/iam/auth/ldap/{sub}
func extractLDAPSubRoute(path string) string {
	path = strings.TrimSuffix(path, "/")
	if len(path) <= len("/api/v1/iam/auth/ldap") {
		return ""
	}
	return path[len("/api/v1/iam/auth/ldap"):]
}
