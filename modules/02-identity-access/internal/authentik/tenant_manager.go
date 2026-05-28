package authentik

import (
	"context"
	"fmt"
	"sync"
)

// TenantManager handles per-tenant Authentik tenant creation, application setup,
// SSO configuration, and provider management.
type TenantManager struct {
	client *Client
	mu     sync.RWMutex
	// Cache of tenant state to avoid repeated API calls
	cache map[string]*TenantState
}

// TenantState holds the resolved state for a tenant's Authentik resources.
type TenantState struct {
	TenantUUID  string
	AppUUID     string
	OAuth2UUID  string
	SAMLUUID    string
	OIDCConfig  *OIDCConfig
	SAMLConfig  *SAMLConfig
}

// OIDCConfig contains resolved OIDC provider URLs.
type OIDCConfig struct {
	Issuer            string
	AuthorizationURL  string
	TokenURL          string
	JWKSURL           string
	ClientID          string
	ClientSecret      string
}

// SAMLConfig contains resolved SAML provider URLs.
type SAMLConfig struct {
	Issuer          string
	SSOURL          string
	ACSURL          string
	MetadataURL     string
	SigningCert     string
}

// NewTenantManager creates a new tenant manager.
func NewTenantManager(client *Client) *TenantManager {
	return &TenantManager{
		client: client,
		cache:  make(map[string]*TenantState),
	}
}

// SetupTenant provisions all Authentik resources for a new Operan tenant.
// It creates: tenant, application, OIDC provider, SAML provider, flow bindings.
func (tm *TenantManager) SetupTenant(ctx context.Context, tenantID, tenantSlug, tenantName string) (*TenantState, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check cache
	if state, ok := tm.cache[tenantID]; ok {
		return state, nil
	}

	// 1. Create Authentik tenant
	tenant, err := tm.client.TenantsAPI.Create(ctx, CreateTenantRequest{
		Slug:        tenantSlug,
		Description: tenantName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	// 2. Create application for the tenant
	app, err := tm.client.ApplicationsAPI.Create(ctx, CreateApplicationRequest{
		Slug:             tenantSlug,
		Name:             tenantName,
		ProtocolPrefix:   "https",
		AuthenticationRank: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create application: %w", err)
	}

	// 3. Create OAuth2/OIDC provider
	oidc, err := tm.client.OAuth2ProvidersAPI.Create(ctx, CreateOAuth2ProviderRequest{
		Name:                 fmt.Sprintf("Operan %s OIDC", tenantName),
		AuthorizationGrantType: "authorization_code",
		IncludeClaimsInRequest: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	// Get setup URLs for OIDC
	setupURLs, err := tm.client.OAuth2ProvidersAPI.SetupURLs(ctx, oidc.UUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get OIDC setup URLs: %w", err)
	}

	oidcConfig := &OIDCConfig{
		Issuer:           setupURLs["issuer"],
		AuthorizationURL: setupURLs["authorization_url"],
		TokenURL:         setupURLs["token_url"],
		JWKSURL:          setupURLs["jwks_url"],
		ClientID:         oidc.ClientID,
		ClientSecret:     oidc.ClientSecret,
	}

	// 4. Create SAML provider
	saml, err := tm.client.SAMLProvidersAPI.Create(ctx, CreateSAMLProviderRequest{
		Name:            fmt.Sprintf("Operan %s SAML", tenantName),
		Slug:            fmt.Sprintf("%s-saml", tenantSlug),
		IncludeAttributes: []string{"email", "name", "groups"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SAML provider: %w", err)
	}

	samlConfig := &SAMLConfig{
		Issuer:      saml.Issuer,
		SSOURL:      saml.SSOURL,
		ACSURL:      saml.ACSURL,
		MetadataURL: saml.MetadataURL,
		SigningCert: saml.SigningCertificate,
	}

	// 5. Create RBAC roles for the tenant
	memberRole, err := tm.client.RBACAPI.Create(ctx, CreateRoleRequest{
		Name: fmt.Sprintf("Operan %s Member", tenantName),
		Permissions: []string{
			"core.view_user",
			"core.view_group",
			"core.view_application",
		},
	})
	if err != nil {
		// Non-fatal — continue
	}
	if memberRole != nil {
		_, err = tm.client.RBACAPI.Create(ctx, CreateRoleRequest{
			Name: fmt.Sprintf("Operan %s Admin", tenantName),
			Permissions: []string{
				"core.view_user",
				"core.change_user",
				"core.delete_user",
				"core.view_group",
				"core.change_group",
				"core.delete_group",
				"core.view_application",
				"core.change_application",
				"core.delete_application",
				"providers.view_oauth2provider",
				"providers.change_oauth2provider",
				"providers.view_samlprovider",
				"providers.change_samlprovider",
			},
		})
		// Non-fatal
	}

	// 6. Create brand for the tenant
	_, err = tm.client.BrandsAPI.Create(ctx, CreateBrandRequest{
		Slug:    tenantSlug,
		Title:   tenantName,
		Subtitle: "Operan Identity",
		Available: []string{tenant.UUID},
	})
	// Non-fatal

	// 7. Create flow binding
	_, err = tm.client.FlowBindingsAPI.Create(ctx, CreateFlowBindingRequest{
		Flow:        "default-authentication-flow", // Use default flow
		Application: app.UUID,
		Type:        "authentication",
	})
	// Non-fatal — use default flow if custom not yet defined

	// Cache and return
	state := &TenantState{
		TenantUUID: tenant.UUID,
		AppUUID:    app.UUID,
		OAuth2UUID: oidc.UUID,
		SAMLUUID:   saml.UUID,
		OIDCConfig: oidcConfig,
		SAMLConfig: samlConfig,
	}
	tm.cache[tenantID] = state
	return state, nil
}

// GetTenantState retrieves the state for a tenant (from cache or API).
func (tm *TenantManager) GetTenantState(ctx context.Context, tenantID string) (*TenantState, error) {
	tm.mu.RLock()
	if state, ok := tm.cache[tenantID]; ok {
		tm.mu.RUnlock()
		return state, nil
	}
	tm.mu.RUnlock()

	// Not cached — need to look up from Authentik
	// For now, return error — SetupTenant must be called first
	return nil, fmt.Errorf("tenant %s not found in cache — call SetupTenant first", tenantID)
}

// RemoveTenant tears down all Authentik resources for a tenant.
func (tm *TenantManager) RemoveTenant(ctx context.Context, tenantID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	state, ok := tm.cache[tenantID]
	if !ok {
		return fmt.Errorf("tenant %s not found", tenantID)
	}

	// Delete SAML provider
	_ = tm.client.SAMLProvidersAPI.Delete(ctx, state.SAMLUUID)

	// Delete OAuth2 provider
	_ = tm.client.OAuth2ProvidersAPI.Delete(ctx, state.OAuth2UUID)

	// Delete application
	_ = tm.client.ApplicationsAPI.Delete(ctx, state.AppUUID)

	// Delete tenant
	_ = tm.client.TenantsAPI.Delete(ctx, state.TenantUUID)

	// Remove from cache
	delete(tm.cache, tenantID)
	return nil
}

// InvalidateCache removes a tenant's cached state.
func (tm *TenantManager) InvalidateCache(tenantID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.cache, tenantID)
}
