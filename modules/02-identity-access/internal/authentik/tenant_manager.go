package authentik

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TenantManagerConfig holds configuration for tenant manager behavior.
type TenantManagerConfig struct {
	CacheTTL         time.Duration // TTL for cached tenant state (0 = no expiry, default: 1h)
	CleanupInterval  time.Duration // How often background cleanup runs (0 = disabled, default: 10m)
	MaxCachedTenants int           // Max tenants to cache (0 = unlimited, default: 1000)
}

// TenantManagerConfigDefaults returns sensible defaults for tenant manager config.
func TenantManagerConfigDefaults() TenantManagerConfig {
	return TenantManagerConfig{
		CacheTTL:         1 * time.Hour,
		CleanupInterval:  10 * time.Minute,
		MaxCachedTenants: 1000,
	}
}

// TenantStateWithExpiry wraps TenantState with cache expiry metadata.
type TenantStateWithExpiry struct {
	State    *TenantState
	ExpiryAt time.Time
}

// IsExpired checks if the cached state has passed its TTL.
func (e *TenantStateWithExpiry) IsExpired() bool {
	return time.Now().After(e.ExpiryAt)
}

// TenantManager handles per-tenant Authentik tenant creation, application setup,
// SSO configuration, and provider management.
type TenantManager struct {
	client       *Client
	mu           sync.RWMutex
	cache        map[string]*TenantStateWithExpiry
	cacheOrder   []string // Tracks insertion order for LRU eviction
	config       TenantManagerConfig
	done         chan struct{} // Signals cleanup goroutine to stop
}

// NewTenantManager creates a new tenant manager with default config.
func NewTenantManager(client *Client) *TenantManager {
	return NewTenantManagerWithConfig(client, TenantManagerConfigDefaults())
}

// NewTenantManagerWithConfig creates a new tenant manager with custom config.
func NewTenantManagerWithConfig(client *Client, cfg TenantManagerConfig) *TenantManager {
	tm := &TenantManager{
		client:     client,
		cache:      make(map[string]*TenantStateWithExpiry),
		cacheOrder: make([]string, 0),
		config:     cfg,
		done:       make(chan struct{}),
	}

	// Start background cleanup if interval is positive
	if cfg.CleanupInterval > 0 {
		go tm.cleanupLoop(cfg.CleanupInterval)
	}

	return tm
}

// cleanupLoop runs periodic cleanup of expired cache entries.
func (tm *TenantManager) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.CleanupExpiredCache()
		case <-tm.done:
			return
		}
	}
}

// Stop shuts down the background cleanup goroutine.
func (tm *TenantManager) Stop() {
	select {
	case <-tm.done:
		// Already closed
	default:
		close(tm.done)
	}
}

// evictOldestEvicted removes the oldest expired or stale entry from cache.
func (tm *TenantManager) evictOldestEvicted() {
	for _, id := range tm.cacheOrder {
		entry, exists := tm.cache[id]
		if exists && entry.IsExpired() {
			delete(tm.cache, id)
			tm.cacheOrder = append(tm.cacheOrder[:0], tm.cacheOrder[1:]...)
			return
		}
	}
	// No expired entries — evict oldest overall
	if len(tm.cacheOrder) > 0 {
		oldestID := tm.cacheOrder[0]
		delete(tm.cache, oldestID)
		tm.cacheOrder = tm.cacheOrder[1:]
	}
}

// evictToMakeRoom ensures cache size is within MaxCachedTenants.
func (tm *TenantManager) evictToMakeRoom() {
	for len(tm.cache) > tm.config.MaxCachedTenants && tm.config.MaxCachedTenants > 0 {
		tm.evictOldestEvicted()
	}
}

// InvalidateCache removes a tenant's cached state.
func (tm *TenantManager) InvalidateCache(tenantID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.cache, tenantID)
	for i, id := range tm.cacheOrder {
		if id == tenantID {
			tm.cacheOrder = append(tm.cacheOrder[:i], tm.cacheOrder[i+1:]...)
			break
		}
	}
}

// CleanupExpiredCache removes all expired entries from the cache.
func (tm *TenantManager) CleanupExpiredCache() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	var removed int
	for id, entry := range tm.cache {
		if entry.IsExpired() {
			delete(tm.cache, id)
			removed++
		}
	}

	// Rebuild order slice
	if removed > 0 {
		newOrder := make([]string, 0, len(tm.cacheOrder)-removed)
		for _, id := range tm.cacheOrder {
			if _, exists := tm.cache[id]; exists {
				newOrder = append(newOrder, id)
			}
		}
		tm.cacheOrder = newOrder
	}

	return removed
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

// SetupTenant provisions all Authentik resources for a new Operan tenant.
// It creates: tenant, application, OIDC provider, SAML provider, flow bindings.
func (tm *TenantManager) SetupTenant(ctx context.Context, tenantID, tenantSlug, tenantName string) (*TenantState, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check cache (including TTL)
	if entry, ok := tm.cache[tenantID]; ok && !entry.IsExpired() {
		return entry.State, nil
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

	// Cache and return with TTL
	state := &TenantState{
		TenantUUID: tenant.UUID,
		AppUUID:    app.UUID,
		OAuth2UUID: oidc.UUID,
		SAMLUUID:   saml.UUID,
		OIDCConfig: oidcConfig,
		SAMLConfig: samlConfig,
	}
	expiryAt := time.Now().UTC().Add(tm.config.CacheTTL)
	tm.cache[tenantID] = &TenantStateWithExpiry{
		State:    state,
		ExpiryAt: expiryAt,
	}
	tm.cacheOrder = append(tm.cacheOrder, tenantID)

	return state, nil
}

// GetTenantState retrieves the state for a tenant (from cache or API).
// Returns an error if the tenant is not cached or if the cached entry has expired.
func (tm *TenantManager) GetTenantState(ctx context.Context, tenantID string) (*TenantState, error) {
	tm.mu.RLock()
	entry, ok := tm.cache[tenantID]
	tm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tenant %s not found in cache — call SetupTenant first", tenantID)
	}

	if entry.IsExpired() {
		// Remove expired entry
		tm.InvalidateCache(tenantID)
		return nil, fmt.Errorf("tenant %s cache expired — call SetupTenant again", tenantID)
	}

	return entry.State, nil
}

// RemoveTenant tears down all Authentik resources for a tenant.
func (tm *TenantManager) RemoveTenant(ctx context.Context, tenantID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry, ok := tm.cache[tenantID]
	if !ok {
		return fmt.Errorf("tenant %s not found", tenantID)
	}

	// Delete SAML provider
	_ = tm.client.SAMLProvidersAPI.Delete(ctx, entry.State.SAMLUUID)

	// Delete OAuth2 provider
	_ = tm.client.OAuth2ProvidersAPI.Delete(ctx, entry.State.OAuth2UUID)

	// Delete application
	_ = tm.client.ApplicationsAPI.Delete(ctx, entry.State.AppUUID)

	// Delete tenant
	_ = tm.client.TenantsAPI.Delete(ctx, entry.State.TenantUUID)

	// Remove from cache
	delete(tm.cache, tenantID)
	for i, id := range tm.cacheOrder {
		if id == tenantID {
			tm.cacheOrder = append(tm.cacheOrder[:i], tm.cacheOrder[i+1:]...)
			break
		}
	}
	return nil
}
