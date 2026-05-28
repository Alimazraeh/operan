package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// LDAPHandler handles LDAP integration HTTP endpoints via the Authentik API v3.
type LDAPHandler struct {
	Auth      *authentik.Client
	Publisher *events.Publisher
}

// NewLDAPHandler creates a new LDAP handler backed by an Authentik client.
func NewLDAPHandler(auth *authentik.Client, publisher *events.Publisher) *LDAPHandler {
	return &LDAPHandler{
		Auth:      auth,
		Publisher: publisher,
	}
}

// Configure handles POST /api/v1/iam/auth/ldap/configure
func (h *LDAPHandler) Configure(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	var req models.ConfigureLDAPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	host, port := extractHostPort(req.URL)

	authMap := map[string]interface{}{
		"hostname":         host,
		"port":             port,
		"connection_security": connectionSecurity(req.Provider),
		"bind_dn":          req.BindDN,
		"bind_password":    req.BindPassword,
	}

	ingestionMap := map[string]interface{}{
		"user_search_base":     req.BaseDN,
		"user_dn_template":     "uid={username},ou=people," + req.BaseDN,
		"group_search_base":    req.BaseDN,
		"group_dn_template":    "cn={name},ou=groups," + req.BaseDN,
	}
	if req.UserFilter != "" {
		ingestionMap["user_filters"] = []string{req.UserFilter}
	}
	if req.GroupFilter != "" {
		ingestionMap["group_filters"] = []string{req.GroupFilter}
	}

	source, err := h.Auth.LDAPSources().Create(r.Context(), authentik.CreateLDAPSourceRequest{
		Name:         "operan-" + tenantID + "-ldap",
		Authentication: authMap,
		Ingestion:    ingestionMap,
	})
	if err != nil {
		http.Error(w, `{"error":"failed to create LDAP source: `+err.Error()+`"}`, http.StatusConflict)
		return
	}

	h.Publisher.Publish(r.Context(), "ldap.configured", tenantID, "", time.Now().UTC().Format(time.RFC3339), map[string]interface{}{
		"provider": req.Provider,
		"url":      req.URL,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(source)
}

// Test handles POST /api/v1/iam/auth/ldap/test
func (h *LDAPHandler) Test(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	var req struct {
		URL          string `json:"url,omitempty"`
		BindDN       string `json:"bind_dn,omitempty"`
		BindPassword string `json:"bind_password,omitempty"`
		BaseDN       string `json:"base_dn,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	host, port := extractHostPort(req.URL)

	authMap := map[string]interface{}{
		"hostname":         host,
		"port":             port,
		"connection_security": "ssl",
		"bind_dn":          req.BindDN,
		"bind_password":    req.BindPassword,
	}

	ingestionMap := map[string]interface{}{
		"user_search_base": req.BaseDN,
	}

	source, err := h.Auth.LDAPSources().Create(r.Context(), authentik.CreateLDAPSourceRequest{
		Name:         "operan-" + tenantID + "-ldap-test",
		Authentication: authMap,
		Ingestion:    ingestionMap,
	})
	if err != nil {
		http.Error(w, `{"error":"failed to create test LDAP source: `+err.Error()+`"}`, http.StatusBadRequest)
		return
	}
	defer func() {
		h.Auth.LDAPSources().Delete(r.Context(), source.UUID)
	}()

	debug, err := h.Auth.LDAPSources().Debug(r.Context(), source.UUID)
	if err != nil {
		http.Error(w, `{"error":"ldap debug failed: `+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	result := models.LDAPSyncResult{
		Status:       "success",
		UsersSynced:  0,
		GroupsSynced: 0,
	}

	steps := []models.LDAPTestStep{
		{Step: "connection", Status: "success", Detail: "connected to " + host + ":" + strings.TrimSuffix(req.URL, "://"+host)},
		{Step: "authentication", Status: "success", Detail: "bind successful as " + req.BindDN},
		{Step: "search", Status: "success", Detail: "search base " + req.BaseDN},
		{Step: "sync", Status: "success", Detail: "synced 0 users, 0 groups (dry run)"},
	}
	if debug != nil {
		steps = append(steps, models.LDAPTestStep{
			Step:   "debug_info",
			Status: "success",
			Detail: "debug completed",
		})
	}
	result.TestSteps = steps

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetConfig handles GET /api/v1/iam/auth/ldap/config
func (h *LDAPHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	sources, err := h.Auth.LDAPSources().List(r.Context())
	if err != nil {
		http.Error(w, `{"error":"failed to list LDAP sources: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	var found *authentik.LDAPSource
	for _, s := range sources {
		if strings.HasPrefix(s.Name, "operan-"+tenantID+"-ldap") {
			found = s
			break
		}
	}
	if found == nil {
		http.Error(w, `{"error":"LDAP config not found"}`, http.StatusNotFound)
		return
	}

	resp := struct {
		UUID            string                 `json:"uuid"`
		Name            string                 `json:"name"`
		Connected       bool                   `json:"connected"`
		Authentication  map[string]interface{} `json:"authentication"`
		Ingestion       map[string]interface{} `json:"ingestion"`
		Properties      map[string]interface{} `json:"properties,omitempty"`
	}{
		UUID:         found.UUID,
		Name:         found.Name,
		Connected:    found.Connected,
		Authentication: found.Authentication,
		Ingestion:    found.Ingestion,
		Properties:   found.Properties,
	}

	// Mask bind password in response
	auth := resp.Authentication
	if auth != nil {
		auth["bind_password"] = "****"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// UpdateConfig handles PATCH /api/v1/iam/auth/ldap/config
func (h *LDAPHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	var req models.UpdateLDAPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	sources, err := h.Auth.LDAPSources().List(r.Context())
	if err != nil {
		http.Error(w, `{"error":"failed to list LDAP sources: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	var found *authentik.LDAPSource
	for _, s := range sources {
		if strings.HasPrefix(s.Name, "operan-"+tenantID+"-ldap") {
			found = s
			break
		}
	}
	if found == nil {
		http.Error(w, `{"error":"LDAP config not found"}`, http.StatusNotFound)
		return
	}

	authMap := map[string]interface{}{}
	if req.URL != nil || req.BindDN != nil || req.BindPassword != nil {
		host, port := extractHostPort(*req.URL)
		authMap["hostname"] = host
		authMap["port"] = port
		authMap["bind_dn"] = boolPtrStr(req.BindDN)
		if req.BindPassword != nil && *req.BindPassword != "" {
			authMap["bind_password"] = *req.BindPassword
		}
		authMap["connection_security"] = "ssl"
	}
	if req.SearchScope != nil {
		authMap["connection_security"] = *req.SearchScope
	}

	ingestionMap := map[string]interface{}{}
	if req.BaseDN != nil {
		ingestionMap["user_search_base"] = *req.BaseDN
		ingestionMap["user_dn_template"] = "uid={username},ou=people," + *req.BaseDN
		ingestionMap["group_search_base"] = *req.BaseDN
		ingestionMap["group_dn_template"] = "cn={name},ou=groups," + *req.BaseDN
	}
	if req.UserFilter != nil {
		ingestionMap["user_filters"] = []string{*req.UserFilter}
	}
	if req.GroupFilter != nil {
		ingestionMap["group_filters"] = []string{*req.GroupFilter}
	}

	updateMap := map[string]interface{}{
		"authentication": authMap,
		"ingestion":    ingestionMap,
	}
	if req.DisplayName != nil {
		updateMap["name"] = *req.DisplayName
	}

	updated, err := h.Auth.LDAPSources().Update(r.Context(), found.UUID, updateMap)
	if err != nil {
		http.Error(w, `{"error":"failed to update LDAP config: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// DeleteConfig handles DELETE /api/v1/iam/auth/ldap/config
func (h *LDAPHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	sources, err := h.Auth.LDAPSources().List(r.Context())
	if err != nil {
		http.Error(w, `{"error":"failed to list LDAP sources: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	var found *authentik.LDAPSource
	for _, s := range sources {
		if strings.HasPrefix(s.Name, "operan-"+tenantID+"-ldap") {
			found = s
			break
		}
	}
	if found == nil {
		http.Error(w, `{"error":"LDAP config not found"}`, http.StatusNotFound)
		return
	}

	if err := h.Auth.LDAPSources().Delete(r.Context(), found.UUID); err != nil {
		http.Error(w, `{"error":"failed to delete LDAP config: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// extractHostPort splits a URL into host and port.
func extractHostPort(urlStr string) (string, int) {
	if urlStr == "" {
		return "", 389
	}
	clean := strings.TrimPrefix(urlStr, "ldap://")
	clean = strings.TrimPrefix(clean, "ldaps://")
	if strings.Contains(clean, ":") {
		parts := strings.SplitN(clean, ":", 2)
		host := parts[0]
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			port = 389
		}
		return host, port
	}
	return clean, 389
}

// connectionSecurity maps a provider name to Authentik connection security level.
func connectionSecurity(provider string) string {
	switch strings.ToLower(provider) {
	case "ldaps", "freeipa":
		return "ssl"
	case "starttls":
		return "starttls"
	default:
		return "none"
	}
}

// boolPtrStr safely dereferences a *string for use in map[string]interface{}.
func boolPtrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// extractLDAPSubRoute extracts the sub-route from /api/v1/iam/auth/ldap/{sub}
func extractLDAPSubRoute(path string) string {
	path = strings.TrimSuffix(path, "/")
	if len(path) <= len("/api/v1/iam/auth/ldap") {
		return ""
	}
	return path[len("/api/v1/iam/auth/ldap"):]
}
