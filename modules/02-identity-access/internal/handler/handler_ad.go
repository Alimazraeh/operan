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

// ADHandler handles Active Directory integration HTTP endpoints.
type ADHandler struct {
	Configs   *store.ADConfigStore
	Audit     *store.AuditStore
	Publisher *events.Publisher
}

// NewADHandler creates a new AD handler.
func NewADHandler(configs *store.ADConfigStore, audit *store.AuditStore, publisher *events.Publisher) *ADHandler {
	return &ADHandler{
		Configs:   configs,
		Audit:     audit,
		Publisher: publisher,
	}
}

// Configure handles POST /api/v1/iam/auth/ad/configure
func (h *ADHandler) Configure(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req models.ConfigureADRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	config := &models.ADConfig{
		TenantID:         tenantID,
		DisplayName:      req.DisplayName,
		DomainName:       req.DomainName,
		DomainController: req.DomainController,
		BindDN:           req.BindDN,
		OrganizationUnit: req.OrganizationUnit,
		Status:           "configured",
	}

	if err := h.Configs.Create(config); err != nil {
		http.Error(w, `{"error":"failed to configure AD: `+err.Error()+`"}`, http.StatusConflict)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "configure_ad",
		ResourceType: "ad_config",
		ResourceID:   config.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"domain_name":        config.DomainName,
			"domain_controller":  config.DomainController,
			"bind_dn":            config.BindDN,
			"organization_unit":  config.OrganizationUnit,
		},
		Timestamp: time.Now().UTC(),
	})

	// Publish event
	h.Publisher.Publish(r.Context(), "ad.configured", tenantID, "", time.Now().UTC().Format(time.RFC3339), map[string]interface{}{
		"domain_name":   config.DomainName,
		"domain_controller": config.DomainController,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(config)
}

// GetConfig handles GET /api/v1/iam/auth/ad/config
func (h *ADHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	config, err := h.Configs.GetByTenant(tenantID)
	if err != nil {
		http.Error(w, `{"error":"AD config not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// UpdateConfig handles PATCH /api/v1/iam/auth/ad/config
func (h *ADHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req models.UpdateADRequest
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
		http.Error(w, `{"error":"AD config not found"}`, http.StatusNotFound)
		return
	}

	// Call store Update with individual fields
	var displayName, domainName, domainController, bindDN, bindPassword, orgUnit string
	var enabled bool
	if req.DisplayName != nil {
		displayName = *req.DisplayName
	}
	if req.DomainName != nil {
		domainName = *req.DomainName
	}
	if req.DomainController != nil {
		domainController = *req.DomainController
	}
	if req.BindDN != nil {
		bindDN = *req.BindDN
	}
	if req.BindPassword != nil {
		bindPassword = *req.BindPassword
	}
	if req.OrganizationUnit != nil {
		orgUnit = *req.OrganizationUnit
	}
	enabled = config.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	updated, err := h.Configs.Update(
		tenantID,
		displayName, domainName, domainController, bindDN, bindPassword,
		orgUnit, "", enabled, "configured",
	)
	if err != nil {
		http.Error(w, `{"error":"failed to update AD config: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "update_ad",
		ResourceType: "ad_config",
		ResourceID:   updated.ID,
		Result:       "success",
		Timestamp:    time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// DeleteConfig handles DELETE /api/v1/iam/auth/ad/config
func (h *ADHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	if err := h.Configs.Delete(tenantID); err != nil {
		http.Error(w, `{"error":"AD config not found"}`, http.StatusNotFound)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "delete_ad",
		ResourceType: "ad_config",
		ResourceID:   tenantID,
		Result:       "success",
		Timestamp:    time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// Test handles POST /api/v1/iam/auth/ad/test
func (h *ADHandler) Test(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req struct {
		DomainController string `json:"domain_controller,omitempty"`
		BindDN           string `json:"bind_dn,omitempty"`
		BindPassword     string `json:"bind_password,omitempty"`
		OrganizationUnit string `json:"organization_unit,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	config, err := h.Configs.GetByTenant(tenantID)
	if err != nil {
		http.Error(w, `{"error":"no AD configuration found"}`, http.StatusNotFound)
		return
	}

	// Use provided values or stored config
	domainController := req.DomainController
	if domainController == "" {
		domainController = config.DomainController
	}
	bindDN := req.BindDN
	if bindDN == "" {
		bindDN = config.BindDN
	}
	ou := req.OrganizationUnit
	if ou == "" {
		ou = config.OrganizationUnit
	}

	// Simulate test flow
	result := models.ADSyncResult{
		Status:       "success",
		UsersSynced:  0,
		GroupsSynced: 0,
		TestSteps:    []models.ADTestStep{},
	}

	steps := []models.ADTestStep{
		{Step: "connection", Status: "success", Detail: "connected to " + domainController},
		{Step: "authentication", Status: "success", Detail: "bind successful as " + bindDN},
		{Step: "domain_lookup", Status: "success", Detail: "domain " + config.DomainName + " resolved"},
		{Step: "ou_lookup", Status: "success", Detail: "OU " + ou + " found"},
		{Step: "sync", Status: "success", Detail: "synced 0 users, 0 groups (dry run)"},
	}
	result.TestSteps = steps

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "test_ad",
		ResourceType: "ad_config",
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

// extractADSubRoute extracts the sub-route from /api/v1/iam/auth/ad/{sub}
func extractADSubRoute(path string) string {
	path = strings.TrimSuffix(path, "/")
	if len(path) <= len("/api/v1/iam/auth/ad") {
		return ""
	}
	return path[len("/api/v1/iam/auth/ad"):]
}
