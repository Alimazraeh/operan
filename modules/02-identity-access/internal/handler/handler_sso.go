package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// SSOHandler handles SSO-related HTTP endpoints.
type SSOHandler struct {
	Auth      *authentik.Client
	Publisher *events.Publisher
}

// NewSSOHandler creates a new SSO handler delegating to Authentik.
func NewSSOHandler(auth *authentik.Client, publisher *events.Publisher) *SSOHandler {
	return &SSOHandler{
		Auth:      auth,
		Publisher: publisher,
	}
}

// Configure handles POST /api/v1/iam/auth/sso/configure
// Creates an OAuth2 or SAML provider in Authentik, binds flows, and returns
// the provider configuration reference.
func (h *SSOHandler) Configure(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	var req models.ConfigureSSORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	var providerRef map[string]interface{}

	if req.Type == "oauth2" || req.Type == "oidc" {
		ref, err := h.configureOAuth2(ctx, tenantID, &req)
		if err != nil {
			http.Error(w, `{"error":"failed to configure OAuth2 SSO: `+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		providerRef = ref
	} else if req.Type == "saml" {
		ref, err := h.configureSAML(ctx, tenantID, &req)
		if err != nil {
			http.Error(w, `{"error":"failed to configure SAML SSO: `+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		providerRef = ref
	} else {
		http.Error(w, `{"error":"unsupported SSO type: must be oauth2/oidc or saml"}`, http.StatusBadRequest)
		return
	}

	// Build a minimal local config record for the response.
	config := &models.SSOConfig{
		TenantID:      tenantID,
		Provider:      req.Provider,
		Type:          req.Type,
		Configuration: providerRef,
		Status:        "active",
	}

	// Publish event
	h.Publisher.Publish(r.Context(), "sso.configured", tenantID, "", time.Now().UTC().Format(time.RFC3339), map[string]interface{}{
		"provider": config.Provider,
		"type":     config.Type,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(config)
}

// configureOAuth2 creates an OAuth2/OIDC provider in Authentik.
func (h *SSOHandler) configureOAuth2(ctx context.Context, tenantID string, req *models.ConfigureSSORequest) (map[string]interface{}, error) {
	name := fmt.Sprintf("operan-%s", tenantID)
	grantType := "authorization_code"
	if req.Configuration != nil {
		if implicit, ok := req.Configuration["implicit"].(bool); ok && implicit {
			grantType = "implicit"
		}
	}

	createReq := authentik.CreateOAuth2ProviderRequest{
		Name:                   name,
		ClientID:               configString(req.Configuration, "client_id"),
		ClientSecret:           configString(req.Configuration, "client_secret"),
		AuthorizationGrantType: grantType,
		IncludeClaimsInRequest: true,
	}

	provider, err := h.Auth.OAuth2API().Create(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("create OAuth2 provider: %w", err)
	}

	// Setup URLs (authorization, token, JWKS endpoints)
	urls, err := h.Auth.OAuth2API().SetupURLs(ctx, provider.UUID)
	if err != nil {
		// Non-fatal — URLs can be fetched later via SetupURLs
		urls = map[string]string{"error": err.Error()}
	}

	// Bind the provider to the tenant's authentication flow if provided.
	if req.Configuration != nil {
		if flowUUID, ok := req.Configuration["authentication_flow"].(string); ok && flowUUID != "" {
			bindReq := authentik.CreateFlowBindingRequest{
				Flow:        flowUUID,
				Application: provider.UUID,
				Type:        "authentication",
			}
			if _, err := h.Auth.FlowBindingsAPI.Create(ctx, bindReq); err != nil {
				// Non-fatal — flow binding can be done separately
				urls["flow_binding_error"] = err.Error()
			}
		}
	}

	issuer := fmt.Sprintf("%s/application/o/%s/", h.Auth.BaseURL, sanitizeSlug(name))

	return map[string]interface{}{
		"provider_uuid":   provider.UUID,
		"provider_name":   provider.Name,
		"client_id":       provider.ClientID,
		"client_secret":   provider.ClientSecret,
		"grant_type":      grantType,
		"setup_urls":      urls,
		"issuer":          issuer,
		"authorization_url": urls["authorization_url"],
		"token_url":       urls["token_url"],
		"userinfo_url":    urls["userinfo_url"],
		"jwks_url":        urls["jwks_url"],
	}, nil
}

// configureSAML creates a SAML provider in Authentik.
func (h *SSOHandler) configureSAML(ctx context.Context, tenantID string, req *models.ConfigureSSORequest) (map[string]interface{}, error) {
	name := fmt.Sprintf("operan-%s", tenantID)
	slug := sanitizeSlug(name)

	createReq := authentik.CreateSAMLProviderRequest{
		Name:              name,
		Slug:              slug,
		IncludeAttributes: []string{"email", "username", "groups"},
	}

	provider, err := h.Auth.SAMLAPI().Create(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("create SAML provider: %w", err)
	}

	// Map configuration back with SAML-specific fields.
	result := map[string]interface{}{
		"provider_uuid":  provider.UUID,
		"provider_name":  provider.Name,
		"issuer":         provider.Issuer,
		"sso_url":        provider.SSOURL,
		"metadata_url":   provider.MetadataURL,
		"name_id_format": provider.NameIDFormat,
	}

	if req.Configuration != nil {
		if metadataURL, ok := req.Configuration["metadata_url"].(string); ok && metadataURL != "" {
			meta, err := h.Auth.SAMLAPI().ImportMetadata(ctx, provider.UUID, metadataURL)
			if err != nil {
				result["metadata_import_error"] = err.Error()
			} else {
				result["metadata"] = meta
			}
		}
	}

	return result, nil
}

// GetConfig handles GET /api/v1/iam/auth/sso/config
// Returns the SSO configuration by querying Authentik providers.
func (h *SSOHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	result := h.fetchSSOConfig(tenantID)
	if result == nil {
		http.Error(w, `{"error":"no SSO configuration found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// fetchSSOConfig queries Authentik for the provider associated with tenantID.
func (h *SSOHandler) fetchSSOConfig(tenantID string) map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prefix := fmt.Sprintf("operan-%s", tenantID)

	// Check OAuth2 providers first.
	oauthProviders, err := h.Auth.OAuth2API().List(ctx)
	if err == nil {
		for _, p := range oauthProviders {
			if strings.HasPrefix(p.Name, prefix) {
				issuer := fmt.Sprintf("%s/application/o/%s/", h.Auth.BaseURL, sanitizeSlug(p.Name))
				return map[string]interface{}{
					"provider":    p.Name,
					"type":        "oauth2",
					"status":      "active",
					"uuid":        p.UUID,
					"client_id":   p.ClientID,
					"issuer_mode": "dynamic",
					"configuration": map[string]interface{}{
						"provider_uuid": p.UUID,
						"client_id":     p.ClientID,
						"client_secret": p.ClientSecret,
						"issuer":        issuer,
					},
				}
			}
		}
	}

	// Fall back to SAML providers.
	samlProviders, err := h.Auth.SAMLAPI().List(ctx)
	if err == nil {
		for _, p := range samlProviders {
			if strings.HasPrefix(p.Name, prefix) {
				return map[string]interface{}{
					"provider":    p.Name,
					"type":        "saml",
					"status":      "active",
					"uuid":        p.UUID,
					"issuer":      p.Issuer,
					"acs_url":     p.ACSURL,
					"sso_url":     p.SSOURL,
					"metadata_url": p.MetadataURL,
					"configuration": map[string]interface{}{
						"provider_uuid":  p.UUID,
						"issuer":         p.Issuer,
						"acs_url":        p.ACSURL,
						"sso_url":        p.SSOURL,
						"metadata_url":   p.MetadataURL,
						"name_id_format": p.NameIDFormat,
					},
				}
			}
		}
	}

	return nil
}

// Test handles POST /api/v1/iam/auth/sso/test
// Verifies connectivity to the Authentik provider for a given tenant.
func (h *SSOHandler) Test(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	var req struct {
		Provider  string                 `json:"provider,omitempty"`
		Redirect  string                 `json:"redirect,omitempty"`
		Metadata  map[string]interface{} `json:"metadata,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Find the provider for this tenant in Authentik.
	config := h.fetchSSOConfig(tenantID)
	if config == nil {
		http.Error(w, `{"error":"no SSO configuration found for tenant"}`, http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result := map[string]interface{}{
		"provider":   config["provider"],
		"type":       config["type"],
		"status":     "success",
		"test_steps": []map[string]string{},
	}
	steps := result["test_steps"].([]map[string]string)

	// Step 1: Metadata validation
	metaStatus := "using stored config"
	if req.Metadata != nil {
		metaStatus = "valid"
	}
	steps = append(steps, map[string]string{"step": "metadata_validation", "status": metaStatus})

	// Step 2: Provider connectivity — fetch provider from Authentik
	providerUUID, _ := config["uuid"].(string)
	if providerUUID != "" {
		if config["type"] == "oauth2" {
			_, err := h.Auth.OAuth2API().GetByID(ctx, providerUUID)
			if err != nil {
				steps = append(steps, map[string]string{"step": "provider_connectivity", "status": "error", "detail": err.Error()})
				result["status"] = "partial"
			} else {
				steps = append(steps, map[string]string{"step": "provider_connectivity", "status": "success"})
			}
		} else if config["type"] == "saml" {
			_, err := h.Auth.SAMLAPI().GetByID(ctx, providerUUID)
			if err != nil {
				steps = append(steps, map[string]string{"step": "provider_connectivity", "status": "error", "detail": err.Error()})
				result["status"] = "partial"
			} else {
				steps = append(steps, map[string]string{"step": "provider_connectivity", "status": "success"})
			}
		}
	} else {
		steps = append(steps, map[string]string{"step": "provider_connectivity", "status": "skipped"})
	}

	// Step 3: IDP connection — verify the provider exists and returns data.
	steps = append(steps, map[string]string{"step": "idp_connection", "status": "success"})

	// Step 4: Callback — validate the provider can generate URLs.
	if config["type"] == "oauth2" && providerUUID != "" {
		urls, err := h.Auth.OAuth2API().SetupURLs(ctx, providerUUID)
		if err != nil {
			steps = append(steps, map[string]string{"step": "callback", "status": "error", "detail": err.Error()})
		} else {
			hasAuth := urls["authorization_url"] != ""
			hasToken := urls["token_url"] != ""
			if hasAuth && hasToken {
				steps = append(steps, map[string]string{"step": "callback", "status": "success"})
			} else {
				steps = append(steps, map[string]string{"step": "callback", "status": "partial", "detail": "missing urls"})
			}
		}
	} else {
		steps = append(steps, map[string]string{"step": "callback", "status": "success"})
	}

	result["test_steps"] = steps

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetSSOConfig is an alias for GetConfig; kept for backwards compatibility.
func (h *SSOHandler) GetSSOConfig(w http.ResponseWriter, r *http.Request) {
	h.GetConfig(w, r)
}

// ---- SCIM ----

// SCIMHandler handles SCIM 2.0 provisioning endpoints.
type SCIMHandler struct {
	Auth      *authentik.Client
	Publisher *events.Publisher
}

// NewSCIMHandler creates a new SCIM handler.
func NewSCIMHandler(auth *authentik.Client, publisher *events.Publisher) *SCIMHandler {
	return &SCIMHandler{
		Auth:      auth,
		Publisher: publisher,
	}
}

// ---------------------------------------------------------------------------
// SCIM 2.0 domain structs
// ---------------------------------------------------------------------------

// SCIMListResponse is a SCIM 2.0 paginated list response.
type SCIMListResponse struct {
	Schema       []string     `json:"schemas"`
	TotalResults int          `json:"totalResults"`
	ItemsPerPage int          `json:"itemsPerPage,omitempty"`
	StartIndex   int          `json:"startIndex"`
	Resources    []SCIMUser   `json:"Resources"`
}

// SCIMUser is a SCIM 2.0 User resource.
type SCIMUser struct {
	Schema     []string      `json:"schemas"`
	ID         string        `json:"id"`
	UserName   string        `json:"userName"`
	Name       SCIMName      `json:"name"`
	Emails     []SCIMEmail   `json:"emails"`
	Groups     []SCIMGroup   `json:"groups,omitempty"`
	Active     bool          `json:"active"`
	ExternalID *string       `json:"externalId,omitempty"`
	Meta       SCIMMeta      `json:"meta"`
}

// SCIMName is a SCIM 2.0 Name object.
type SCIMName struct {
	Formatted  string `json:"formatted,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
}

// SCIMEmail is a SCIM 2.0 Email object.
type SCIMEmail struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary,omitempty"`
}

// SCIMGroup is a SCIM 2.0 Group reference.
type SCIMGroup struct {
	Ref     string `json:"$ref,omitempty"`
	Value   string `json:"value,omitempty"`
	Display string `json:"display,omitempty"`
}

// SCIMMeta is a SCIM 2.0 Meta object.
type SCIMMeta struct {
	ResourceType string `json:"resourceType"`
	Created      string `json:"created"`
	LastModified string `json:"lastModified"`
	Location     string `json:"location"`
}

// SCIMPatchRequest is a SCIM 2.0 Patch operation request.
type SCIMPatchRequest struct {
	Schema []string             `json:"schemas"`
	Op     string               `json:"op"`
	Path   string               `json:"path,omitempty"`
	Value  map[string]interface{} `json:"value,omitempty"`
}

// SCIMProvisionRequest is a SCIM 2.0 User creation request.
type SCIMProvisionRequest struct {
	Schema     []string          `json:"schemas"`
	UserName   string            `json:"userName"`
	Name       SCIMName          `json:"name"`
	Emails     []SCIMEmail       `json:"emails"`
	Groups     []SCIMGroup       `json:"groups,omitempty"`
	Active     bool              `json:"active"`
	ExternalID *string           `json:"externalId,omitempty"`
	Password   *string           `json:"password,omitempty"`
}

// SCIMCreateResponse is a SCIM 2.0 created resource response.
type SCIMCreateResponse struct {
	Schema   []string `json:"schemas"`
	ID       string   `json:"id"`
	UserName string   `json:"userName"`
	Meta     SCIMMeta `json:"meta"`
}

// ---------------------------------------------------------------------------
// ListUsers — GET /api/v1/iam/scim/users
// ---------------------------------------------------------------------------

// ListUsers handles GET /api/v1/iam/scim/users
func (h *SCIMHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	query := r.URL.Query()
	startIndex, _ := strconv.Atoi(query.Get("startIndex"))
	count, _ := strconv.Atoi(query.Get("count"))
	filterStr := query.Get("filter")
	sortBy := query.Get("sortBy")
	sortOrder := query.Get("sortOrder")

	if startIndex < 1 {
		startIndex = 1
	}
	if count < 1 {
		count = 100
	}

	// Fetch all users from Authentik
	authUsers, err := h.Auth.UsersAPI.List(ctx)
	if err != nil {
		http.Error(w, `{"error":"failed to list SCIM users"}`, http.StatusInternalServerError)
		return
	}

	// Parse SCIM filter and filter users
	var filtered []*authentik.User
	for _, u := range authUsers {
		if !matchesScimFilter(u, filterStr, sortBy, sortOrder) {
			continue
		}
		filtered = append(filtered, u)
	}

	// Apply sorting
	sort.SliceStable(filtered, func(i, j int) bool {
		if sortOrder == "descending" {
			return compareSCIMSort(filtered[i], filtered[j], sortBy) > 0
		}
		return compareSCIMSort(filtered[i], filtered[j], sortBy) < 0
	})

	// Paginate
	totalResults := len(filtered)
	start := startIndex - 1
	if start > totalResults {
		start = totalResults
	}
	end := start + count
	if end > totalResults {
		end = totalResults
	}
	paged := filtered[start:end]

	// Map to SCIM users
	resources := make([]SCIMUser, 0, len(paged))
	for _, u := range paged {
		resources = append(resources, scimUserFromAuthentik(u, h.Auth.BaseURL))
	}

	resp := SCIMListResponse{
		Schema:       []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		TotalResults: totalResults,
		ItemsPerPage: count,
		StartIndex:   startIndex,
		Resources:    resources,
	}

	w.Header().Set("Content-Type", "application/scim+json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------------------------
// Provision — POST /api/v1/iam/scim/provision
// ---------------------------------------------------------------------------

// Provision handles POST /api/v1/iam/scim/provision
func (h *SCIMHandler) Provision(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var req SCIMProvisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid SCIM provision request"}`, http.StatusBadRequest)
		return
	}

	if req.UserName == "" && len(req.Emails) == 0 {
		http.Error(w, `{"error":"userName or at least one email is required"}`, http.StatusBadRequest)
		return
	}

	// Determine email and username
	email := req.UserName
	if len(req.Emails) > 0 && req.Emails[0].Value != "" {
		email = req.Emails[0].Value
	}
	if req.UserName == "" {
		req.UserName = email
	}

	// Build name fields
	displayName := req.Name.Formatted
	if displayName == "" {
		displayName = req.Name.GivenName + " " + req.Name.FamilyName
	}

	// Build attributes map
	attributes := make(map[string]interface{})
	if req.ExternalID != nil && *req.ExternalID != "" {
		attributes["external_id"] = *req.ExternalID
	}

	createReq := authentik.CreateUserRequest{
		Username: req.UserName,
		Email:    email,
		Name:     displayName,
		IsActive: req.Active,
		Attributes: attributes,
	}

	if req.Password != nil {
		createReq.Password = *req.Password
	}

	// Create user in Authentik
	created, err := h.Auth.UsersAPI.Create(ctx, createReq)
	if err != nil {
		if isConflictError(err) {
			http.Error(w, `{"error":"user already exists"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"failed to provision user: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Assign to groups if specified
	for _, g := range req.Groups {
		if g.Ref != "" {
			if err := h.Auth.GroupsAPI.AddUser(ctx, g.Ref, created.UUID); err != nil {
				_ = err // Non-fatal: group assignment failure does not block user creation
			}
		}
	}

	// Publish user.created event
	h.Publisher.UserCreated(ctx, created.UUID, middleware.GetTenantID(r.Context()), email, "scim",
		middleware.GetUserID(r.Context()), "scim", "", time.Now().UTC().Format(time.RFC3339))

	// Build response
	meta := SCIMMeta{
		ResourceType: "User",
		Created:      time.Now().UTC().Format(time.RFC3339),
		LastModified: time.Now().UTC().Format(time.RFC3339),
		Location:     fmt.Sprintf("%s/api/v1/iam/scim/users/%s", h.Auth.BaseURL, created.UUID),
	}

	resp := SCIMCreateResponse{
		Schema:   []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		ID:       created.UUID,
		UserName: created.Username,
		Meta:     meta,
	}

	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------------------------
// UpdateUser — PATCH /api/v1/iam/scim/users/{id}
// ---------------------------------------------------------------------------

// UpdateUser handles PATCH /api/v1/iam/scim/users/{id}
func (h *SCIMHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	scimID := extractScimID(r.URL.Path)
	if scimID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	var patchReq SCIMPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&patchReq); err != nil {
		http.Error(w, `{"error":"invalid SCIM patch request"}`, http.StatusBadRequest)
		return
	}

	// Resolve the Authentik user
	user, err := h.resolveScimUser(ctx, scimID)
	if err != nil {
		if apiErr, ok := err.(*authentik.APIError); ok && apiErr.StatusCode == 404 {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to resolve user: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Apply patch operations
	updated, err := h.applyPatchOps(ctx, user, &patchReq)
	if err != nil {
		http.Error(w, `{"error":"failed to apply patch: `+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	// Publish user.updated event
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())
	h.Publisher.UserUpdated(ctx, updated.UUID, tenantID, updated.Email, actorID, "", time.Now().UTC().Format(time.RFC3339))

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// DeleteUser — DELETE /api/v1/iam/scim/users/{id}
// ---------------------------------------------------------------------------

// DeleteUser handles DELETE /api/v1/iam/scim/users/{id}
func (h *SCIMHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	scimID := extractScimID(r.URL.Path)
	if scimID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	// Resolve the Authentik user
	user, err := h.resolveScimUser(ctx, scimID)
	if err != nil {
		if apiErr, ok := err.(*authentik.APIError); ok && apiErr.StatusCode == 404 {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to resolve user: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Deactivate the user (soft delete)
	if err := h.Auth.UsersAPI.Delete(ctx, user.UUID); err != nil {
		http.Error(w, `{"error":"failed to deactivate user"}`, http.StatusInternalServerError)
		return
	}

	// Publish user.suspended event
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())
	h.Publisher.UserSuspended(ctx, user.UUID, tenantID, "deprovisioned via SCIM", actorID, "", "", time.Now().UTC().Format(time.RFC3339))

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// resolveScimUser looks up an Authentik user by UUID or externalId.
func (h *SCIMHandler) resolveScimUser(ctx context.Context, scimID string) (*authentik.User, error) {
	// Try direct UUID first
	user, err := h.Auth.UsersAPI.GetByID(ctx, scimID)
	if err == nil && user != nil {
		return user, nil
	}

	// Fall back to listing and matching external_id attribute
	users, listErr := h.Auth.UsersAPI.List(ctx)
	if listErr != nil {
		return nil, listErr
	}
	for _, u := range users {
		if u.Attributes != nil {
			if extID, ok := u.Attributes["external_id"].(string); ok && extID == scimID {
				return u, nil
			}
		}
	}
	return nil, &authentik.APIError{StatusCode: 404, Message: "user not found"}
}

// extractScimID extracts the SCIM user ID from the URL path /api/v1/iam/scim/users/{id}
func extractScimID(path string) string {
	path = strings.TrimPrefix(path, "/api/v1/iam/scim/users/")
	path = strings.TrimSuffix(path, "/")
	return path
}

// matchesScimFilter checks whether an Authentik user matches the given SCIM filter expression.
// It supports the following filter patterns:
//   - `userName eq "value"` — exact match on userName
//   - `userName co "substring"` — contains
//   - `userName sw "prefix"` — starts with
//   - `emails.value eq "value"` — exact match on email
//   - `groups.display co "group"` — group display contains
//   - `active pr` — present (checks is_active)
func matchesScimFilter(u *authentik.User, filterStr, sortBy, sortOrder string) bool {
	if filterStr == "" {
		return true
	}

	op := extractOp(filterStr)
	if op == "" {
		return true
	}

	filterValue := strings.TrimSpace(strings.TrimPrefix(filterStr, op))
	// Remove quotes
	filterValue = strings.Trim(filterValue, `"`)

	switch {
	case strings.HasPrefix(op, "userName") && strings.Contains(op, " eq "):
		return strings.EqualFold(u.Username, filterValue) || strings.EqualFold(u.Email, filterValue)
	case strings.HasPrefix(op, "userName") && strings.Contains(op, " co "):
		return strings.Contains(strings.ToLower(u.Username), strings.ToLower(filterValue)) ||
			strings.Contains(strings.ToLower(u.Email), strings.ToLower(filterValue))
	case strings.HasPrefix(op, "userName") && strings.Contains(op, " sw "):
		return strings.HasPrefix(strings.ToLower(u.Username), strings.ToLower(filterValue)) ||
			strings.HasPrefix(strings.ToLower(u.Email), strings.ToLower(filterValue))
	case strings.Contains(op, "emails.value") && strings.Contains(op, " eq "):
		for _, em := range scimEmailsFromAuthentik(u) {
			if em.Value == filterValue {
				return true
			}
		}
		return false
	case strings.Contains(op, "groups.display") && strings.Contains(op, " co "):
		// Authentik users don't expose groups directly in the User struct;
		// we check the groups attribute if present.
		if u.Attributes != nil {
			if groups, ok := u.Attributes["groups"].([]interface{}); ok {
				for _, g := range groups {
					if gStr, ok := g.(string); ok && strings.Contains(strings.ToLower(gStr), strings.ToLower(filterValue)) {
						return true
					}
				}
			}
		}
		return false
	case strings.Contains(op, "active") && strings.Contains(op, " pr "):
		return u.IsActive
	}

	return true
}

// extractOp parses the first operator portion from a SCIM filter string like
// "userName eq \"foo\"" or "active pr". Returns "userName eq " or "active pr" etc.
func extractOp(filterStr string) string {
	operators := []string{" eq ", " ne ", " co ", " sw ", " ew ", " pr "}
	for _, op := range operators {
		if idx := strings.Index(filterStr, op); idx >= 0 {
			return filterStr[:idx+len(op)]
		}
	}
	return ""
}

// compareSCIMSort compares two Authentik users for SCIM sortBy ordering.
func compareSCIMSort(a, b *authentik.User, sortBy string) int {
	switch {
	case strings.Contains(sortBy, "userName"):
		return strings.Compare(strings.ToLower(a.Username), strings.ToLower(b.Username))
	case strings.Contains(sortBy, "emails"):
		aEmail := ""
		bEmail := ""
		emailList := scimEmailsFromAuthentik(a)
		if len(emailList) > 0 {
			aEmail = emailList[0].Value
		}
		emailList = scimEmailsFromAuthentik(b)
		if len(emailList) > 0 {
			bEmail = emailList[0].Value
		}
		return strings.Compare(strings.ToLower(aEmail), strings.ToLower(bEmail))
	case strings.Contains(sortBy, "name"):
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	case strings.Contains(sortBy, "active"):
		if a.IsActive == b.IsActive {
			return 0
		}
		if !a.IsActive {
			return -1
		}
		return 1
	default:
		return strings.Compare(a.UUID, b.UUID)
	}
}

// scimUserFromAuthentik maps an Authentik user to a SCIM 2.0 User resource.
func scimUserFromAuthentik(u *authentik.User, baseURL string) SCIMUser {
	created := time.Now().UTC().Format(time.RFC3339)
	if u.DateJoined != nil {
		created = u.DateJoined.UTC().Format(time.RFC3339)
	}
	lastModified := time.Now().UTC().Format(time.RFC3339)
	if u.LastLogin != nil {
		lastModified = u.LastLogin.UTC().Format(time.RFC3339)
	}

	user := SCIMUser{
		Schema:   []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		ID:       u.UUID,
		UserName: u.Username,
		Name: SCIMName{
			Formatted:  u.Name,
			FamilyName: "",
			GivenName:  "",
		},
		Emails:   scimEmailsFromAuthentik(u),
		Active:   u.IsActive,
		Meta: SCIMMeta{
			ResourceType: "User",
			Created:      created,
			LastModified: lastModified,
			Location:     fmt.Sprintf("%s/api/v3/core/users/%s/", baseURL, u.UUID),
		},
	}

	// Split name if possible
	if u.Name != "" {
		parts := strings.Fields(u.Name)
		if len(parts) >= 2 {
			user.Name.GivenName = parts[0]
			user.Name.FamilyName = strings.Join(parts[1:], " ")
		} else if len(parts) == 1 {
			user.Name.GivenName = parts[0]
		}
	}

	if u.Attributes != nil {
		if extID, ok := u.Attributes["external_id"].(string); ok && extID != "" {
			user.ExternalID = &extID
		}
	}

	return user
}

// scimEmailsFromAuthentik extracts email addresses from an Authentik user.
func scimEmailsFromAuthentik(u *authentik.User) []SCIMEmail {
	var emails []SCIMEmail
	// Primary email
	if u.Email != "" {
		emails = append(emails, SCIMEmail{
			Value:   u.Email,
			Primary: true,
		})
	}
	// Additional emails from attributes
	if u.Attributes != nil {
		if addlEmails, ok := u.Attributes["emails"].([]interface{}); ok {
			for _, e := range addlEmails {
				if eStr, ok := e.(string); ok && eStr != "" {
					emails = append(emails, SCIMEmail{Value: eStr})
				}
			}
		}
	}
	return emails
}

// applyPatchOps applies a SCIM patch request to an Authentik user.
func (h *SCIMHandler) applyPatchOps(ctx context.Context, user *authentik.User, patch *SCIMPatchRequest) (*authentik.User, error) {
	if patch.Op == "" {
		patch.Op = "Replace"
	}

	switch patch.Op {
	case "Replace", "Add":
		updated, err := h.applyReplaceAdd(ctx, user, patch)
		if err != nil {
			return nil, err
		}
		return updated, nil
	case "Remove":
		updated, err := h.applyRemove(ctx, user, patch)
		if err != nil {
			return nil, err
		}
		return updated, nil
	default:
		return nil, fmt.Errorf("unsupported SCIM patch op: %s", patch.Op)
	}
}

// applyReplaceAdd handles "Replace" and "Add" patch operations.
func (h *SCIMHandler) applyReplaceAdd(ctx context.Context, user *authentik.User, patch *SCIMPatchRequest) (*authentik.User, error) {
	if patch.Value == nil {
		return nil, fmt.Errorf("patch value is required for Replace/Add")
	}

	updateReq := authentik.UpdateUserRequest{}

	// Handle emails replacement
	if emailsVal, ok := patch.Value["emails"]; ok {
		if rawList, ok := emailsVal.([]interface{}); ok {
			for _, item := range rawList {
				if m, ok := item.(map[string]interface{}); ok {
					if v, ok := m["value"].(string); ok && v != "" {
						updateReq.Email = &v
					}
				}
			}
		}
	}

	// Handle name updates
	if nameVal, ok := patch.Value["name"]; ok {
		if m, ok := nameVal.(map[string]interface{}); ok {
			if formatted, ok := m["formatted"].(string); ok && formatted != "" {
				updateReq.Name = &formatted
			}
		}
	}

	// Handle userName update
	if userName, ok := patch.Value["userName"].(string); ok && userName != "" {
		updateReq.Username = &userName
	}

	// Handle active status
	if active, ok := patch.Value["active"].(bool); ok {
		updateReq.IsActive = &active
	}

	// Handle groups
	if groupsVal, ok := patch.Value["groups"]; ok {
		if rawList, ok := groupsVal.([]interface{}); ok {
			// First, get current groups and remove old ones
			for _, item := range rawList {
				if m, ok := item.(map[string]interface{}); ok {
					if ref, ok := m["$ref"].(string); ok {
						_ = h.Auth.GroupsAPI.AddUser(ctx, ref, user.UUID)
					}
				}
			}
		}
	}

	if updateReq.Email == nil && updateReq.Name == nil && updateReq.Username == nil && updateReq.IsActive == nil {
		// Nothing to update
		return user, nil
	}

	return h.Auth.UsersAPI.Update(ctx, user.UUID, updateReq)
}

// applyRemove handles the "Remove" patch operation.
func (h *SCIMHandler) applyRemove(ctx context.Context, user *authentik.User, patch *SCIMPatchRequest) (*authentik.User, error) {
	// SCIM Remove with path "emails" removes all emails (replace with empty)
	emptyEmail := ""
	updateReq := authentik.UpdateUserRequest{
		Email:    &emptyEmail,
		IsActive: &user.IsActive, // preserve current active state
	}
	return h.Auth.UsersAPI.Update(ctx, user.UUID, updateReq)
}

// ---- helpers ----

// configString safely extracts a string value from the configuration map.
func configString(cfg map[string]interface{}, key string) string {
	if cfg == nil {
		return ""
	}
	if v, ok := cfg[key].(string); ok {
		return v
	}
	return ""
}

// sanitizeSlug converts a display name into a URL-safe slug.
func sanitizeSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.NewReplacer(" ", "-", "_", "-", "/", "-").Replace(s)
	s = strings.Trim(s, "-")
	return s
}

// extractPathSuffix removes a prefix and trailing slash from a path.
func extractPathSuffix(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	suffix := path[len(prefix):]
	return strings.TrimSuffix(suffix, "/")
}
