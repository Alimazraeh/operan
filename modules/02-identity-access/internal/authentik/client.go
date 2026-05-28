// Package authentik provides a comprehensive Go client for the Authentik REST API (v3).
// It wraps all Authentik endpoints needed for the Operan Identity & Access Management module.
//
// Authentik base URL is configured via AUTHENTIK_SERVER_URL env var.
// API authentication uses a bearer token configured via AUTHENTIK_ADMIN_TOKEN.
package authentik

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is the Authentik REST API client.
type Client struct {
	HTTPClient      *http.Client
	BaseURL         string
	AuthToken       string
	Timeout         time.Duration
	UsersAPI        *UsersAPI
	GroupsAPI       *GroupsAPI
	ApplicationsAPI *ApplicationsAPI
	TokensAPI       *TokensAPI
	OAuth2ProvidersAPI *OAuth2ProvidersAPI
	SAMLProvidersAPI   *SAMLProvidersAPI
	LdapAPI         *LDAPSourcesAPI
	ScimAPI         *SCIMProvidersAPI
	RBACAPI         *RBACAPI
	TenantsAPI      *TenantsAPI
	BrandsAPI       *BrandsAPI
	FlowBindingsAPI *FlowBindingsAPI
	PolicyBindingsAPI *PolicyBindingsAPI
	SourcesAPI        *SourcesAPI
	EventsAPI         *EventsAPI
	SessionsAPI       *SessionsAPI
}

// NewClient creates a new Authentik API client.
func NewClient(serverURL, authToken string) *Client {
	c := &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		BaseURL:    strings.TrimRight(serverURL, "/"),
		AuthToken:  authToken,
		Timeout:    30 * time.Second,
	}
	// Initialize all sub-APIs
	c.UsersAPI = &UsersAPI{c}
	c.GroupsAPI = &GroupsAPI{c}
	c.ApplicationsAPI = &ApplicationsAPI{c}
	c.TokensAPI = &TokensAPI{c}
	c.OAuth2ProvidersAPI = &OAuth2ProvidersAPI{c}
	c.SAMLProvidersAPI = &SAMLProvidersAPI{c}
	c.LdapAPI = &LDAPSourcesAPI{c}
	c.ScimAPI = &SCIMProvidersAPI{c}
	c.RBACAPI = &RBACAPI{c}
	c.TenantsAPI = &TenantsAPI{c}
	c.BrandsAPI = &BrandsAPI{c}
	c.FlowBindingsAPI = &FlowBindingsAPI{c}
	c.PolicyBindingsAPI = &PolicyBindingsAPI{c}
	c.SourcesAPI = &SourcesAPI{c}
	c.EventsAPI = &EventsAPI{c}
	c.SessionsAPI = &SessionsAPI{c}
	return c
}

// LDAPSources returns the LDAP Sources API client.
func (c *Client) LDAPSources() *LDAPSourcesAPI {
	return c.LdapAPI
}

// OAuth2API returns the OAuth2 Providers API client.
func (c *Client) OAuth2API() *OAuth2ProvidersAPI {
	return c.OAuth2ProvidersAPI
}

// SAMLAPI returns the SAML Providers API client.
func (c *Client) SAMLAPI() *SAMLProvidersAPI {
	return c.SAMLProvidersAPI
}

// Users returns the Users API client.
func (c *Client) Users() *UsersAPI {
	return c.UsersAPI
}

// Groups returns the Groups API client.
func (c *Client) Groups() *GroupsAPI {
	return c.GroupsAPI
}

// doRequest performs an authenticated HTTP request and returns the parsed response.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, out interface{}) error {
	fullURL := c.BaseURL + path

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(bodyBytes),
			Path:       path,
		}
	}

	if out != nil {
		if err := json.Unmarshal(bodyBytes, out); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}
	return nil
}

// doListRequest performs a paginated list request and returns all results.
func (c *Client) doListRequest(ctx context.Context, path string, out interface{}) error {
	var allResults []json.RawMessage

	// Follow pagination links
	currentPath := path
	for {
		var resp paginatedResponse
		if err := c.doRequest(ctx, http.MethodGet, currentPath, nil, &resp); err != nil {
			return err
		}

		// Collect results (resp.Results is already []json.RawMessage)
		allResults = append(allResults, resp.Results...)

		if resp.Next == "" {
			break
		}
		currentPath = resp.Next
	}

	// Unmarshal all results into the target
	allData, _ := json.Marshal(allResults)
	if err := json.Unmarshal(allData, out); err != nil {
		return fmt.Errorf("failed to unmarshal aggregated results: %w", err)
	}
	return nil
}

// APIError represents an Authentik API error response.
type APIError struct {
	StatusCode int
	Message    string
	Path       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Authentik API error %d on %s: %s", e.StatusCode, e.Path, e.Message)
}

// paginatedResponse is the standard Authentik paginated list response.
type paginatedResponse struct {
	Count    int                     `json:"count"`
	Next     string                  `json:"next"`
	Previous string                  `json:"previous"`
	Results  []json.RawMessage       `json:"results"`
}

// ---------------------------------------------------------------------------
// Core Resources
// ---------------------------------------------------------------------------

// UsersAPI provides access to /api/v3/core/users/ endpoints.
type UsersAPI struct {	*Client }

// User is the Authentik user model.
type User struct {
	UUID              string                 `json:"uuid"`
	Username          string                 `json:"username"`
	Email             string                 `json:"email"`
	Name              string                 `json:"name"`
	IsActive          bool                   `json:"is_active"`
	IsStaff           bool                   `json:"is_staff"`
	IsSuperUser       bool                   `json:"is_superuser"`
	LastLogin         *time.Time             `json:"last_login"`
	DateJoined        *time.Time             `json:"date_joined"`
	Enabled           bool                   `json:"enabled"`
	Locked            bool                   `json:"locked"`
	Password          string                 `json:"password,omitempty"`
	AuthenticatorDevices []json.RawMessage  `json:"authenticator_devices,omitempty"`
	Attributes        map[string]interface{} `json:"attributes,omitempty"`
	Tenant            string                 `json:"tenant,omitempty"`
}

// CreateUserRequest is the request body for creating a user.
type CreateUserRequest struct {
	Username string                 `json:"username"`
	Email    string                 `json:"email"`
	Name     string                 `json:"name"`
	Password string                 `json:"password,omitempty"`
	IsActive bool                   `json:"is_active"`
	IsStaff  bool                   `json:"is_staff"`
	Tenant   string                 `json:"tenant"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// UpdateUserRequest is the request body for updating a user.
type UpdateUserRequest struct {
	Username *string                `json:"username,omitempty"`
	Email    *string                `json:"email,omitempty"`
	Name     *string                `json:"name,omitempty"`
	Password *string                `json:"password,omitempty"`
	IsActive *bool                  `json:"is_active,omitempty"`
	IsStaff  *bool                  `json:"is_staff,omitempty"`
	Locked   *bool                  `json:"locked,omitempty"`
	Enabled  *bool                  `json:"enabled,omitempty"`
	Tenant   *string                `json:"tenant,omitempty"`
}

// Create creates a new Authentik user.
func (u *UsersAPI) Create(ctx context.Context, req CreateUserRequest) (*User, error) {
	var user User
	body, _ := json.Marshal(req)
	if err := u.doRequest(ctx, http.MethodPost, "/api/v3/core/users/", bytes.NewReader(body), &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByID retrieves a user by UUID.
func (u *UsersAPI) GetByID(ctx context.Context, uuid string) (*User, error) {
	var user User
	if err := u.doRequest(ctx, http.MethodGet, "/api/v3/core/users/"+uuid+"/", nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// List returns all users.
func (u *UsersAPI) List(ctx context.Context) ([]*User, error) {
	var users []*User
	if err := u.Client.doListRequest(ctx, "/api/v3/core/users/", &users); err != nil {
		return nil, err
	}
	return users, nil
}

// Update modifies a user.
func (u *UsersAPI) Update(ctx context.Context, uuid string, req UpdateUserRequest) (*User, error) {
	var user User
	body, _ := json.Marshal(req)
	if err := u.doRequest(ctx, http.MethodPatch, "/api/v3/core/users/"+uuid+"/", bytes.NewReader(body), &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// Delete deactivates a user (soft delete via setting is_active=false).
func (u *UsersAPI) Delete(ctx context.Context, uuid string) error {
	falseVal := false
	req := UpdateUserRequest{IsActive: &falseVal}
	body, _ := json.Marshal(req)
	return u.doRequest(ctx, http.MethodPatch, "/api/v3/core/users/"+uuid+"/", bytes.NewReader(body), nil)
}

// ---------------------------------------------------------------------------
// Groups
// ---------------------------------------------------------------------------

// GroupsAPI provides access to /api/v3/core/groups/ endpoints.
type GroupsAPI struct{ *Client }

// Group is the Authentik group model.
type Group struct {
	UUID     string                 `json:"uuid"`
	Name     string                 `json:"name"`
	Users    []string               `json:"users,omitempty"`     // List of user UUIDs
	IsStaff  bool                   `json:"is_staff"`
	Tenant   string                 `json:"tenant,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// CreateGroupRequest is the request body for creating a group.
type CreateGroupRequest struct {
	Name    string   `json:"name"`
	Users   []string `json:"users,omitempty"`
	Tenant  string   `json:"tenant"`
}

// Create creates a new group.
func (g *GroupsAPI) Create(ctx context.Context, req CreateGroupRequest) (*Group, error) {
	var group Group
	body, _ := json.Marshal(req)
	if err := g.doRequest(ctx, http.MethodPost, "/api/v3/core/groups/", bytes.NewReader(body), &group); err != nil {
		return nil, err
	}
	return &group, nil
}

// GetByID retrieves a group by UUID.
func (g *GroupsAPI) GetByID(ctx context.Context, uuid string) (*Group, error) {
	var group Group
	if err := g.doRequest(ctx, http.MethodGet, "/api/v3/core/groups/"+uuid+"/", nil, &group); err != nil {
		return nil, err
	}
	return &group, nil
}

// List returns all groups.
func (g *GroupsAPI) List(ctx context.Context) ([]*Group, error) {
	var groups []*Group
	if err := g.Client.doListRequest(ctx, "/api/v3/core/groups/", &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

// Update modifies a group.
func (g *GroupsAPI) Update(ctx context.Context, uuid string, name string) (*Group, error) {
	var group Group
	data := map[string]string{"name": name}
	body, _ := json.Marshal(data)
	if err := g.doRequest(ctx, http.MethodPatch, "/api/v3/core/groups/"+uuid+"/", bytes.NewReader(body), &group); err != nil {
		return nil, err
	}
	return &group, nil
}

// Delete removes a group.
func (g *GroupsAPI) Delete(ctx context.Context, uuid string) error {
	return g.doRequest(ctx, http.MethodDelete, "/api/v3/core/groups/"+uuid+"/", nil, nil)
}

// AddUserToGroup adds a user to a group.
func (g *GroupsAPI) AddUser(ctx context.Context, groupUUID, userUUID string) error {
	data := map[string]string{"user": userUUID}
	body, _ := json.Marshal(data)
	return g.doRequest(ctx, http.MethodPost, "/api/v3/core/groups/"+groupUUID+"/add_user/", bytes.NewReader(body), nil)
}

// RemoveUserFromGroup removes a user from a group.
func (g *GroupsAPI) RemoveUser(ctx context.Context, groupUUID, userUUID string) error {
	data := map[string]string{"user": userUUID}
	body, _ := json.Marshal(data)
	return g.doRequest(ctx, http.MethodPost, "/api/v3/core/groups/"+groupUUID+"/remove_user/", bytes.NewReader(body), nil)
}

// GroupMember represents a member of a group (user UUID).
type GroupMember struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// GetMembers retrieves all member UUIDs for a group by following pagination.
func (g *GroupsAPI) GetMembers(ctx context.Context, groupUUID string) ([]string, error) {
	var allUUIDs []string

	currentPath := "/api/v3/core/groups/" + groupUUID + "/members/"
	for {
		var resp paginatedResponse
		if err := g.doRequest(ctx, http.MethodGet, currentPath, nil, &resp); err != nil {
			return nil, fmt.Errorf("get group members: %w", err)
		}

		for _, result := range resp.Results {
			var member GroupMember
			if err := json.Unmarshal(result, &member); err == nil {
				allUUIDs = append(allUUIDs, member.UUID)
			}
		}

		if resp.Next == "" {
			break
		}
		currentPath = resp.Next
	}

	return allUUIDs, nil
}

// ---------------------------------------------------------------------------
// Applications
// ---------------------------------------------------------------------------

// ApplicationsAPI provides access to /api/v3/core/applications/ endpoints.
type ApplicationsAPI struct{ *Client }

// Application is the Authentik application model.
type Application struct {
	UUID              string                 `json:"uuid"`
	Slug              string                 `json:"slug"`
	Name              string                 `json:"name"`
	ProtocolPrefix    string                 `json:"protocol_prefix"`
	AuthenticationRank int32                 `json:"authentication_rank"`
	AccessCode        string                 `json:"access_code"`
	AccessValidity    *time.Time             `json:"access_validity"`
	SessionValidity   *time.Time             `json:"session_validity"`
	ExtraHTTPHeaders  map[string]string      `json:"extra_http_headers,omitempty"`
	Properties        map[string]interface{} `json:"properties,omitempty"`
}

// CreateApplicationRequest is the request body for creating an application.
type CreateApplicationRequest struct {
	Slug             string                 `json:"slug"`
	Name             string                 `json:"name"`
	ProtocolPrefix   string                 `json:"protocol_prefix,omitempty"`
	AuthenticationRank int32                 `json:"authentication_rank"`
}

// Create creates a new application.
func (a *ApplicationsAPI) Create(ctx context.Context, req CreateApplicationRequest) (*Application, error) {
	var app Application
	body, _ := json.Marshal(req)
	if err := a.doRequest(ctx, http.MethodPost, "/api/v3/core/applications/", bytes.NewReader(body), &app); err != nil {
		return nil, err
	}
	return &app, nil
}

// GetByID retrieves an application by UUID.
func (a *ApplicationsAPI) GetByID(ctx context.Context, uuid string) (*Application, error) {
	var app Application
	if err := a.doRequest(ctx, http.MethodGet, "/api/v3/core/applications/"+uuid+"/", nil, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

// List returns all applications.
func (a *ApplicationsAPI) List(ctx context.Context) ([]*Application, error) {
	var apps []*Application
	if err := a.Client.doListRequest(ctx, "/api/v3/core/applications/", &apps); err != nil {
		return nil, err
	}
	return apps, nil
}

// Delete removes an application.
func (a *ApplicationsAPI) Delete(ctx context.Context, uuid string) error {
	return a.doRequest(ctx, http.MethodDelete, "/api/v3/core/applications/"+uuid+"/", nil, nil)
}

// ---------------------------------------------------------------------------
// Tokens
// ---------------------------------------------------------------------------

// TokensAPI provides access to /api/v3/core/tokens/ endpoints.
type TokensAPI struct{ *Client }

// Token is an Authentik API token.
type Token struct {
	UUID      string     `json:"uuid"`
	Key       string     `json:"key"`
	Created   *time.Time `json:"created"`
	ExpiresAt *time.Time `json:"expires_at"`
	User      string     `json:"user"` // User UUID
}

// CreateTokenRequest is the request body for creating a token.
type CreateTokenRequest struct {
	User string `json:"user"` // User UUID
}

// Create creates a new API token for a user.
func (t *TokensAPI) Create(ctx context.Context, req CreateTokenRequest) (*Token, error) {
	var token Token
	body, _ := json.Marshal(req)
	if err := t.doRequest(ctx, http.MethodPost, "/api/v3/core/tokens/", bytes.NewReader(body), &token); err != nil {
		return nil, err
	}
	return &token, nil
}

// ViewKey retrieves the key of an existing token (keys are hidden in list responses).
func (t *TokensAPI) ViewKey(ctx context.Context, tokenUUID string) (*Token, error) {
	var token Token
	if err := t.doRequest(ctx, http.MethodPost, "/api/v3/core/tokens/"+tokenUUID+"/view_key/", nil, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

// SetKey regenerates a token's key.
func (t *TokensAPI) SetKey(ctx context.Context, tokenUUID string) (*Token, error) {
	var token Token
	if err := t.doRequest(ctx, http.MethodPost, "/api/v3/core/tokens/"+tokenUUID+"/set_key/", nil, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

// ---------------------------------------------------------------------------
// OAuth2 / OIDC Providers
// ---------------------------------------------------------------------------

// OAuth2ProvidersAPI provides access to /api/v3/providers/oauth2/ endpoints.
type OAuth2ProvidersAPI struct{ *Client }

// OAuth2Provider is the OAuth2/OIDC provider model.
type OAuth2Provider struct {
	UUID                    string                 `json:"uuid"`
	Name                    string                 `json:"name"`
	ClientID                string                 `json:"client_id"`
	ClientSecret            string                 `json:"client_secret"`
	AuthorizationGrantType  string                 `json:"authorization_grant_type"`
	SigningKey              string                 `json:"signing_key,omitempty"`
	ImplicitTimeBasedLag    *time.Time             `json:"implicit_time_based_lag,omitempty"`
	IncludeClaimsInRequest  bool                   `json:"include_claims_in_request"`
	PrefixOverride          bool                   `json:"prefix_override"`
	AccessCodeValidity      *time.Duration         `json:"access_code_validity,omitempty"`
	TokenValidity           *time.Duration         `json:"token_validity,omitempty"`
	RefreshTokenValidity    *time.Duration         `json:"refresh_token_validity,omitempty"`
	QueueLength             int32                  `json:"queue_length"`
	User                    string                 `json:"user,omitempty"` // User UUID if user-owned
	Properties              map[string]interface{} `json:"properties,omitempty"`
}

// CreateOAuth2ProviderRequest is the request body for creating an OAuth2 provider.
type CreateOAuth2ProviderRequest struct {
	Name                 string                 `json:"name"`
	ClientID             string                 `json:"client_id,omitempty"`
	ClientSecret         string                 `json:"client_secret,omitempty"`
	AuthorizationGrantType string               `json:"authorization_grant_type"`
	IncludeClaimsInRequest bool                 `json:"include_claims_in_request"`
}

// Create creates a new OAuth2 provider.
func (o *OAuth2ProvidersAPI) Create(ctx context.Context, req CreateOAuth2ProviderRequest) (*OAuth2Provider, error) {
	var provider OAuth2Provider
	body, _ := json.Marshal(req)
	if err := o.doRequest(ctx, http.MethodPost, "/api/v3/providers/oauth2/", bytes.NewReader(body), &provider); err != nil {
		return nil, err
	}
	return &provider, nil
}

// GetByID retrieves an OAuth2 provider by UUID.
func (o *OAuth2ProvidersAPI) GetByID(ctx context.Context, uuid string) (*OAuth2Provider, error) {
	var provider OAuth2Provider
	if err := o.doRequest(ctx, http.MethodGet, "/api/v3/providers/oauth2/"+uuid+"/", nil, &provider); err != nil {
		return nil, err
	}
	return &provider, nil
}

// List returns all OAuth2 providers.
func (o *OAuth2ProvidersAPI) List(ctx context.Context) ([]*OAuth2Provider, error) {
	var providers []*OAuth2Provider
	if err := o.Client.doListRequest(ctx, "/api/v3/providers/oauth2/", &providers); err != nil {
		return nil, err
	}
	return providers, nil
}

// Delete removes an OAuth2 provider.
func (o *OAuth2ProvidersAPI) Delete(ctx context.Context, uuid string) error {
	return o.doRequest(ctx, http.MethodDelete, "/api/v3/providers/oauth2/"+uuid+"/", nil, nil)
}

// SetupURLs returns the OIDC authorization, token, and JWKS URLs for a provider.
func (o *OAuth2ProvidersAPI) SetupURLs(ctx context.Context, uuid string) (map[string]string, error) {
	var result map[string]string
	if err := o.doRequest(ctx, http.MethodPost, "/api/v3/providers/oauth2/"+uuid+"/setup_urls/", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// SAML Providers
// ---------------------------------------------------------------------------

// SAMLProvidersAPI provides access to /api/v3/providers/saml/ endpoints.
type SAMLProvidersAPI struct{ *Client }

// SAMLProvider is the SAML 2.0 provider model.
type SAMLProvider struct {
	UUID                   string                 `json:"uuid"`
	Name                   string                 `json:"name"`
	SigningCertificate     string                 `json:"signing_certificate"`
	SigningKey             string                 `json:"signing_key,omitempty"`
	MetadataURL            string                 `json:"metadata_url"`
	ACSURL                 string                 `json:"acs_url"`
	SSOURL                 string                 `json:"sso_url"`
	Issuer                 string                 `json:"issuer"`
	Slug                   string                 `json:"slug"`
	NameIDFormat           string                 `json:"name_id_format"`
	IncludeAttributes     []string               `json:"include_attributes,omitempty"`
	Properties             map[string]interface{} `json:"properties,omitempty"`
}

// CreateSAMLProviderRequest is the request body for creating a SAML provider.
type CreateSAMLProviderRequest struct {
	Name            string                 `json:"name"`
	Slug            string                 `json:"slug"`
	IncludeAttributes []string             `json:"include_attributes,omitempty"`
}

// Create creates a new SAML provider.
func (s *SAMLProvidersAPI) Create(ctx context.Context, req CreateSAMLProviderRequest) (*SAMLProvider, error) {
	var provider SAMLProvider
	body, _ := json.Marshal(req)
	if err := s.doRequest(ctx, http.MethodPost, "/api/v3/providers/saml/", bytes.NewReader(body), &provider); err != nil {
		return nil, err
	}
	return &provider, nil
}

// GetByID retrieves a SAML provider by UUID.
func (s *SAMLProvidersAPI) GetByID(ctx context.Context, uuid string) (*SAMLProvider, error) {
	var provider SAMLProvider
	if err := s.doRequest(ctx, http.MethodGet, "/api/v3/providers/saml/"+uuid+"/", nil, &provider); err != nil {
		return nil, err
	}
	return &provider, nil
}

// List returns all SAML providers.
func (s *SAMLProvidersAPI) List(ctx context.Context) ([]*SAMLProvider, error) {
	var providers []*SAMLProvider
	if err := s.Client.doListRequest(ctx, "/api/v3/providers/saml/", &providers); err != nil {
		return nil, err
	}
	return providers, nil
}

// Delete removes a SAML provider.
func (s *SAMLProvidersAPI) Delete(ctx context.Context, uuid string) error {
	return s.doRequest(ctx, http.MethodDelete, "/api/v3/providers/saml/"+uuid+"/", nil, nil)
}

// ImportMetadata imports SAML IdP metadata from a URL.
func (s *SAMLProvidersAPI) ImportMetadata(ctx context.Context, providerUUID, metadataURL string) (map[string]interface{}, error) {
	var result map[string]interface{}
	data := map[string]string{"metadata_url": metadataURL}
	body, _ := json.Marshal(data)
	if err := s.doRequest(ctx, http.MethodPost, "/api/v3/providers/saml/"+providerUUID+"/import_metadata/", bytes.NewReader(body), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// LDAP Sources
// ---------------------------------------------------------------------------

// LDAPSourcesAPI provides access to /api/v3/sources/ldap/ endpoints.
type LDAPSourcesAPI struct{ *Client }

// LDAPSource is the LDAP source model.
type LDAPSource struct {
	UUID              string                 `json:"uuid"`
	Name              string                 `json:"name"`
	Connected         bool                   `json:"connected"`
	Managed           string                 `json:"managed"`
	Type              string                 `json:"type"` // "ldap"
	Authentication    map[string]interface{} `json:"authentication"`
	Ingestion         map[string]interface{} `json:"ingestion"`
	Properties        map[string]interface{} `json:"properties,omitempty"`
}

// CreateLDAPSourceRequest is the request body for creating an LDAP source.
type CreateLDAPSourceRequest struct {
	Name        string                 `json:"name"`
	Authentication map[string]interface{} `json:"authentication"`
	Ingestion   map[string]interface{} `json:"ingestion"`
}

// Create creates a new LDAP source.
func (l *LDAPSourcesAPI) Create(ctx context.Context, req CreateLDAPSourceRequest) (*LDAPSource, error) {
	var source LDAPSource
	body, _ := json.Marshal(req)
	if err := l.doRequest(ctx, http.MethodPost, "/api/v3/sources/ldap/", bytes.NewReader(body), &source); err != nil {
		return nil, err
	}
	return &source, nil
}

// GetByID retrieves an LDAP source by UUID.
func (l *LDAPSourcesAPI) GetByID(ctx context.Context, uuid string) (*LDAPSource, error) {
	var source LDAPSource
	if err := l.doRequest(ctx, http.MethodGet, "/api/v3/sources/ldap/"+uuid+"/", nil, &source); err != nil {
		return nil, err
	}
	return &source, nil
}

// Update modifies an LDAP source with the provided fields.
func (l *LDAPSourcesAPI) Update(ctx context.Context, uuid string, fields map[string]interface{}) (*LDAPSource, error) {
	var source LDAPSource
	body, _ := json.Marshal(fields)
	if err := l.doRequest(ctx, http.MethodPatch, "/api/v3/sources/ldap/"+uuid+"/", bytes.NewReader(body), &source); err != nil {
		return nil, err
	}
	return &source, nil
}

// List returns all LDAP sources.
func (l *LDAPSourcesAPI) List(ctx context.Context) ([]*LDAPSource, error) {
	var sources []*LDAPSource
	if err := l.Client.doListRequest(ctx, "/api/v3/sources/ldap/", &sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// Delete removes an LDAP source.
func (l *LDAPSourcesAPI) Delete(ctx context.Context, uuid string) error {
	return l.doRequest(ctx, http.MethodDelete, "/api/v3/sources/ldap/"+uuid+"/", nil, nil)
}

// SyncStatus returns the sync status of an LDAP source.
func (l *LDAPSourcesAPI) SyncStatus(ctx context.Context, uuid string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := l.doRequest(ctx, http.MethodGet, "/api/v3/sources/ldap/"+uuid+"/sync_status/", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Debug returns debug information for an LDAP source connection.
func (l *LDAPSourcesAPI) Debug(ctx context.Context, uuid string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := l.doRequest(ctx, http.MethodPost, "/api/v3/sources/ldap/"+uuid+"/debug/", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// SCIM Providers
// ---------------------------------------------------------------------------

// SCIMProvidersAPI provides access to /api/v3/providers/scim/ endpoints.
type SCIMProvidersAPI struct{ *Client }

// SCIMProvider is the SCIM provider model.
type SCIMProvider struct {
	UUID        string                 `json:"uuid"`
	Name        string                 `json:"name"`
	Enabled     bool                   `json:"enabled"`
	Slug        string                 `json:"slug"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

// CreateSCIMProviderRequest is the request body for creating a SCIM provider.
type CreateSCIMProviderRequest struct {
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Enabled bool   `json:"enabled"`
}

// Create creates a new SCIM provider.
func (s *SCIMProvidersAPI) Create(ctx context.Context, req CreateSCIMProviderRequest) (*SCIMProvider, error) {
	var provider SCIMProvider
	body, _ := json.Marshal(req)
	if err := s.doRequest(ctx, http.MethodPost, "/api/v3/providers/scim/", bytes.NewReader(body), &provider); err != nil {
		return nil, err
	}
	return &provider, nil
}

// GetByID retrieves a SCIM provider by UUID.
func (s *SCIMProvidersAPI) GetByID(ctx context.Context, uuid string) (*SCIMProvider, error) {
	var provider SCIMProvider
	if err := s.doRequest(ctx, http.MethodGet, "/api/v3/providers/scim/"+uuid+"/", nil, &provider); err != nil {
		return nil, err
	}
	return &provider, nil
}

// List returns all SCIM providers.
func (s *SCIMProvidersAPI) List(ctx context.Context) ([]*SCIMProvider, error) {
	var providers []*SCIMProvider
	if err := s.Client.doListRequest(ctx, "/api/v3/providers/scim/", &providers); err != nil {
		return nil, err
	}
	return providers, nil
}

// Delete removes a SCIM provider.
func (s *SCIMProvidersAPI) Delete(ctx context.Context, uuid string) error {
	return s.doRequest(ctx, http.MethodDelete, "/api/v3/providers/scim/"+uuid+"/", nil, nil)
}

// ---------------------------------------------------------------------------
// Tokens (Outpost tokens for SCIM authentication)
// ---------------------------------------------------------------------------

// ProviderTokensAPI provides access to provider token management.
type ProviderTokensAPI struct{ *Client }

// ProviderToken is a token for a specific provider.
type ProviderToken struct {
	UUID      string     `json:"uuid"`
	Token     string     `json:"token"`
	Provider  string     `json:"provider"` // Provider UUID
	Source    string     `json:"source"`   // Source UUID
	Expiry    *time.Time `json:"expiry"`
}

// ---------------------------------------------------------------------------
// Flows
// ---------------------------------------------------------------------------

// FlowsAPI provides access to /api/v3/flows/ endpoints.
type FlowsAPI struct{ *Client }

// Flow is an Authentik flow model.
type Flow struct {
	UUID        string                 `json:"uuid"`
	Slug        string                 `json:"slug"`
	Name        string                 `json:"name"`
	Mode        string                 `json:"mode"`        // "authentication", "authorization", "invitation", etc.
	AuthenticationFlow bool            `json:"authentication_flow"`
	Permissions []string               `json:"permissions,omitempty"`
	Labels      map[string]string      `json:"labels,omitempty"`
}

// CreateFlowRequest is the request body for creating a flow.
type CreateFlowRequest struct {
	Slug               string            `json:"slug"`
	Name               string            `json:"name"`
	Mode               string            `json:"mode"`
	AuthenticationFlow bool              `json:"authentication_flow"`
}

// Create creates a new flow.
func (f *FlowsAPI) Create(ctx context.Context, req CreateFlowRequest) (*Flow, error) {
	var flow Flow
	body, _ := json.Marshal(req)
	if err := f.doRequest(ctx, http.MethodPost, "/api/v3/flows/", bytes.NewReader(body), &flow); err != nil {
		return nil, err
	}
	return &flow, nil
}

// GetByID retrieves a flow by UUID.
func (f *FlowsAPI) GetByID(ctx context.Context, uuid string) (*Flow, error) {
	var flow Flow
	if err := f.doRequest(ctx, http.MethodGet, "/api/v3/flows/"+uuid+"/", nil, &flow); err != nil {
		return nil, err
	}
	return &flow, nil
}

// List returns all flows.
func (f *FlowsAPI) List(ctx context.Context) ([]*Flow, error) {
	var flows []*Flow
	if err := f.Client.doListRequest(ctx, "/api/v3/flows/", &flows); err != nil {
		return nil, err
	}
	return flows, nil
}

// Delete removes a flow.
func (f *FlowsAPI) Delete(ctx context.Context, uuid string) error {
	return f.doRequest(ctx, http.MethodDelete, "/api/v3/flows/"+uuid+"/", nil, nil)
}

// ---------------------------------------------------------------------------
// Stages
// ---------------------------------------------------------------------------

// StagesAPI provides access to /api/v3/stages/ endpoints.
type StagesAPI struct{ *Client }

// Stage is an Authentik flow stage model.
type Stage struct {
	UUID       string                 `json:"uuid"`
	StageType  string                 `json:"stage_type"` // "authentication", "identification", "consent", "prompt", "authenticator", etc.
	Enabled    bool                   `json:"enabled"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// AuthenticatorStagesAPI provides access to MFA authenticator stages.
type AuthenticatorStagesAPI struct{ *Client }

// AuthenticatorDevice is an MFA device registered for a user.
type AuthenticatorDevice struct {
	UUID       string                 `json:"uuid"`
	Type       string                 `json:"type"` // "totp", "webauthn", "sms", "email", "static", "duo"
	Enabled    bool                   `json:"enabled"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// ---------------------------------------------------------------------------
// RBAC
// ---------------------------------------------------------------------------

// RBACAPI provides access to /api/v3/rbac/ endpoints.
type RBACAPI struct{ *Client }

// Role is an Authentik RBAC role.
type Role struct {
	UUID        string   `json:"uuid"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions,omitempty"`
}

// Permission is an Authentik permission definition.
type Permission struct {
	UUID  string `json:"uuid"`
	Name  string `json:"name"`
	Title string `json:"title"`
	Description string `json:"description"`
}

// CreateRoleRequest is the request body for creating a role.
type CreateRoleRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions,omitempty"`
}

// RBACOperations is the interface defining RBAC role operations.
// This allows test mocks and the real RBACAPI to be used interchangeably.
type RBACOperations interface {
	Create(ctx context.Context, req CreateRoleRequest) (*Role, error)
	List(ctx context.Context) ([]*Role, error)
	GetByID(ctx context.Context, uuid string) (*Role, error)
	Delete(ctx context.Context, uuid string) error
}

// Ensure *RBACAPI implements RBACOperations.
var _ RBACOperations = (*RBACAPI)(nil)

// Create creates a new RBAC role.
func (r *RBACAPI) Create(ctx context.Context, req CreateRoleRequest) (*Role, error) {
	var role Role
	body, _ := json.Marshal(req)
	if err := r.doRequest(ctx, http.MethodPost, "/api/v3/rbac/roles/", bytes.NewReader(body), &role); err != nil {
		return nil, err
	}
	return &role, nil
}

// GetByID retrieves a role by UUID.
func (r *RBACAPI) GetByID(ctx context.Context, uuid string) (*Role, error) {
	var role Role
	if err := r.doRequest(ctx, http.MethodGet, "/api/v3/rbac/roles/"+uuid+"/", nil, &role); err != nil {
		return nil, err
	}
	return &role, nil
}

// List returns all roles.
func (r *RBACAPI) List(ctx context.Context) ([]*Role, error) {
	var roles []*Role
	if err := r.Client.doListRequest(ctx, "/api/v3/rbac/roles/", &roles); err != nil {
		return nil, err
	}
	return roles, nil
}

// Delete removes a role.
func (r *RBACAPI) Delete(ctx context.Context, uuid string) error {
	return r.doRequest(ctx, http.MethodDelete, "/api/v3/rbac/roles/"+uuid+"/", nil, nil)
}

// AssignUserToRole assigns a user to a role.
func (r *RBACAPI) AssignUser(ctx context.Context, roleUUID, userUUID string) error {
	data := map[string]string{"user": userUUID}
	body, _ := json.Marshal(data)
	return r.doRequest(ctx, http.MethodPost, "/api/v3/rbac/roles/"+roleUUID+"/add_user/", bytes.NewReader(body), nil)
}

// RemoveUserFromRole removes a user from a role.
func (r *RBACAPI) RemoveUser(ctx context.Context, roleUUID, userUUID string) error {
	return r.doRequest(ctx, http.MethodPost, "/api/v3/rbac/roles/"+roleUUID+"/remove_user/", nil, nil)
}

// ListPermissions returns all defined permissions.
func (r *RBACAPI) ListPermissions(ctx context.Context) ([]*Permission, error) {
	var perms []*Permission
	if err := r.Client.doListRequest(ctx, "/api/v3/rbac/permissions/", &perms); err != nil {
		return nil, err
	}
	return perms, nil
}

// AssignPermissionToRole assigns a permission to a role.
func (r *RBACAPI) AssignPermission(ctx context.Context, roleUUID, permUUID string) error {
	data := map[string]string{"permission": permUUID}
	body, _ := json.Marshal(data)
	return r.doRequest(ctx, http.MethodPost, "/api/v3/rbac/permissions/roles/"+roleUUID+"/assign/", bytes.NewReader(body), nil)
}

// UnassignPermissionFromRole removes a permission from a role.
func (r *RBACAPI) UnassignPermission(ctx context.Context, roleUUID, permUUID string) error {
	data := map[string]string{"permission": permUUID}
	body, _ := json.Marshal(data)
	return r.doRequest(ctx, http.MethodPost, "/api/v3/rbac/permissions/roles/"+roleUUID+"/unassign/", bytes.NewReader(body), nil)
}

// ---------------------------------------------------------------------------
// Tenants
// ---------------------------------------------------------------------------

// TenantsAPI provides access to /api/v3/tenants/ endpoints.
type TenantsAPI struct{ *Client }

// Tenant is an Authentik tenant.
type Tenant struct {
	UUID        string            `json:"uuid"`
	Slug        string            `json:"slug"`
	Description string            `json:"description"`
}

// CreateTenantRequest is the request body for creating a tenant.
type CreateTenantRequest struct {
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

// Create creates a new tenant.
func (t *TenantsAPI) Create(ctx context.Context, req CreateTenantRequest) (*Tenant, error) {
	var tenant Tenant
	body, _ := json.Marshal(req)
	if err := t.doRequest(ctx, http.MethodPost, "/api/v3/tenants/tenants/", bytes.NewReader(body), &tenant); err != nil {
		return nil, err
	}
	return &tenant, nil
}

// GetByID retrieves a tenant by UUID.
func (t *TenantsAPI) GetByID(ctx context.Context, uuid string) (*Tenant, error) {
	var tenant Tenant
	if err := t.doRequest(ctx, http.MethodGet, "/api/v3/tenants/tenants/"+uuid+"/", nil, &tenant); err != nil {
		return nil, err
	}
	return &tenant, nil
}

// List returns all tenants.
func (t *TenantsAPI) List(ctx context.Context) ([]*Tenant, error) {
	var tenants []*Tenant
	if err := t.Client.doListRequest(ctx, "/api/v3/tenants/tenants/", &tenants); err != nil {
		return nil, err
	}
	return tenants, nil
}

// Delete removes a tenant.
func (t *TenantsAPI) Delete(ctx context.Context, uuid string) error {
	return t.doRequest(ctx, http.MethodDelete, "/api/v3/tenants/tenants/"+uuid+"/", nil, nil)
}

// ---------------------------------------------------------------------------
// Events (Audit)
// ---------------------------------------------------------------------------

// EventsAPI provides access to /api/v3/events/ endpoints.
type EventsAPI struct{ *Client }

// Event is an Authentik audit event.
type Event struct {
	UUID       string                 `json:"uuid"`
	Type       string                 `json:"type"`
	Actor      string                 `json:"actor"`      // User UUID
	Object     string                 `json:"object"`     // Object UUID
	ObjectType string                 `json:"object_type"` // e.g., "core.user", "providers.oauth2"
	Created    *time.Time             `json:"created"`
	Data       map[string]interface{} `json:"data"`
}

// List returns all events, optionally filtered.
func (e *EventsAPI) List(ctx context.Context, actorFilter, objectTypeFilter string) ([]*Event, error) {
	path := "/api/v3/events/events/"
	if actorFilter != "" || objectTypeFilter != "" {
		v := url.Values{}
		if actorFilter != "" {
			v.Set("actor", actorFilter)
		}
		if objectTypeFilter != "" {
			v.Set("object_type", objectTypeFilter)
		}
		path += "?" + v.Encode()
	}
	var events []*Event
	if err := e.Client.doListRequest(ctx, path, &events); err != nil {
		return nil, err
	}
	return events, nil
}

// ---------------------------------------------------------------------------
// Authenticated Sessions
// ---------------------------------------------------------------------------

// SessionsAPI provides access to /api/v3/core/authenticated_sessions/ endpoints.
type SessionsAPI struct{ *Client }

// AuthenticatedSession is an active user session.
type AuthenticatedSession struct {
	UUID       string     `json:"uuid"`
	User       string     `json:"user"` // User UUID
	Valid      bool       `json:"valid"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ValidUntil *time.Time `json:"valid_until"`
}

// List returns all active sessions.
func (s *SessionsAPI) List(ctx context.Context, userFilter string) ([]*AuthenticatedSession, error) {
	path := "/api/v3/core/authenticated_sessions/"
	if userFilter != "" {
		path += "?user=" + url.QueryEscape(userFilter)
	}
	var sessions []*AuthenticatedSession
	if err := s.Client.doListRequest(ctx, path, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// Delete invalidates a specific session.
func (s *SessionsAPI) Delete(ctx context.Context, uuid string) error {
	return s.doRequest(ctx, http.MethodDelete, "/api/v3/core/authenticated_sessions/"+uuid+"/", nil, nil)
}

// ---------------------------------------------------------------------------
// Brands (Per-tenant branding)
// ---------------------------------------------------------------------------

// BrandsAPI provides access to /api/v3/core/brands/ endpoints.
type BrandsAPI struct{ *Client }

// Brand is an Authentik brand configuration (per-tenant theming).
type Brand struct {
	UUID      string            `json:"uuid"`
	Slug      string            `json:"slug"`
	BrandLogo string            `json:"brand_logo"`
	BrandIcon string            `json:"brand_icon"`
	Primary   string            `json:"primary"`
	Secondary string            `json:"secondary"`
	Title     string            `json:"title"`
	Subtitle  string            `json:"subtitle"`
	Available []string          `json:"available"` // Tenant UUIDs
}

// CreateBrandRequest is the request body for creating a brand.
type CreateBrandRequest struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Subtitle    string   `json:"subtitle"`
	BrandLogo   string   `json:"brand_logo,omitempty"`
	BrandIcon   string   `json:"brand_icon,omitempty"`
	Primary     string   `json:"primary,omitempty"`
	Secondary   string   `json:"secondary,omitempty"`
	Available   []string `json:"available,omitempty"`
}

// Create creates a new brand.
func (b *BrandsAPI) Create(ctx context.Context, req CreateBrandRequest) (*Brand, error) {
	var brand Brand
	body, _ := json.Marshal(req)
	if err := b.doRequest(ctx, http.MethodPost, "/api/v3/core/brands/", bytes.NewReader(body), &brand); err != nil {
		return nil, err
	}
	return &brand, nil
}

// GetByID retrieves a brand by UUID.
func (b *BrandsAPI) GetByID(ctx context.Context, uuid string) (*Brand, error) {
	var brand Brand
	if err := b.doRequest(ctx, http.MethodGet, "/api/v3/core/brands/"+uuid+"/", nil, &brand); err != nil {
		return nil, err
	}
	return &brand, nil
}

// List returns all brands.
func (b *BrandsAPI) List(ctx context.Context) ([]*Brand, error) {
	var brands []*Brand
	if err := b.Client.doListRequest(ctx, "/api/v3/core/brands/", &brands); err != nil {
		return nil, err
	}
	return brands, nil
}

// Update modifies a brand.
func (b *BrandsAPI) Update(ctx context.Context, uuid string, req CreateBrandRequest) (*Brand, error) {
	var brand Brand
	body, _ := json.Marshal(req)
	if err := b.doRequest(ctx, http.MethodPatch, "/api/v3/core/brands/"+uuid+"/", bytes.NewReader(body), &brand); err != nil {
		return nil, err
	}
	return &brand, nil
}

// Delete removes a brand.
func (b *BrandsAPI) Delete(ctx context.Context, uuid string) error {
	return b.doRequest(ctx, http.MethodDelete, "/api/v3/core/brands/"+uuid+"/", nil, nil)
}

// ---------------------------------------------------------------------------
// Flow Bindings (attach flows to applications)
// ---------------------------------------------------------------------------

// FlowBindingsAPI provides access to /api/v3/flows/bindings/ endpoints.
type FlowBindingsAPI struct{ *Client }

// FlowBinding attaches a flow to an application for a specific mode.
type FlowBinding struct {
	UUID     string `json:"uuid"`
	Flow     string `json:"flow"`     // Flow UUID
	Application string `json:"application"` // Application UUID
	Type     string `json:"type"`     // "authentication", "authorization", etc.
}

// CreateBindingRequest is the request body for creating a flow binding.
type CreateFlowBindingRequest struct {
	Flow        string `json:"flow"`
	Application string `json:"application"`
	Type        string `json:"type"` // "authentication" or "authorization"
}

// Create creates a flow-to-application binding.
func (f *FlowBindingsAPI) Create(ctx context.Context, req CreateFlowBindingRequest) (*FlowBinding, error) {
	var binding FlowBinding
	body, _ := json.Marshal(req)
	if err := f.doRequest(ctx, http.MethodPost, "/api/v3/flows/bindings/", bytes.NewReader(body), &binding); err != nil {
		return nil, err
	}
	return &binding, nil
}

// GetByID retrieves a binding by UUID.
func (f *FlowBindingsAPI) GetByID(ctx context.Context, uuid string) (*FlowBinding, error) {
	var binding FlowBinding
	if err := f.doRequest(ctx, http.MethodGet, "/api/v3/flows/bindings/"+uuid+"/", nil, &binding); err != nil {
		return nil, err
	}
	return &binding, nil
}

// List returns all flow bindings.
func (f *FlowBindingsAPI) List(ctx context.Context) ([]*FlowBinding, error) {
	var bindings []*FlowBinding
	if err := f.Client.doListRequest(ctx, "/api/v3/flows/bindings/", &bindings); err != nil {
		return nil, err
	}
	return bindings, nil
}

// Delete removes a flow binding.
func (f *FlowBindingsAPI) Delete(ctx context.Context, uuid string) error {
	return f.doRequest(ctx, http.MethodDelete, "/api/v3/flows/bindings/"+uuid+"/", nil, nil)
}

// ---------------------------------------------------------------------------
// Policy Bindings (attach policies to flows)
// ---------------------------------------------------------------------------

// PolicyBindingsAPI provides access to /api/v3/policies/bindings/ endpoints.
type PolicyBindingsAPI struct{ *Client }

// PolicyBinding attaches a policy to a flow.
type PolicyBinding struct {
	UUID string `json:"uuid"`
	Policy string `json:"policy"` // Policy UUID
	Flow   string `json:"flow"`   // Flow UUID
}

// CreateBinding creates a policy-to-flow binding.
func (p *PolicyBindingsAPI) CreateBinding(ctx context.Context, policyUUID, flowUUID string) (*PolicyBinding, error) {
	var binding PolicyBinding
	data := map[string]string{"policy": policyUUID, "flow": flowUUID}
	body, _ := json.Marshal(data)
	if err := p.doRequest(ctx, http.MethodPost, "/api/v3/policies/bindings/", bytes.NewReader(body), &binding); err != nil {
		return nil, err
	}
	return &binding, nil
}

// Delete removes a policy binding.
func (p *PolicyBindingsAPI) DeleteBinding(ctx context.Context, uuid string) error {
	return p.doRequest(ctx, http.MethodDelete, "/api/v3/policies/bindings/"+uuid+"/", nil, nil)
}

// ---------------------------------------------------------------------------
// Property Mappings
// ---------------------------------------------------------------------------

// PropertyMappingsAPI provides access to /api/v3/property_mappings/ endpoints.
type PropertyMappingsAPI struct{ *Client }

// PropertyMapping is an attribute mapping rule.
type PropertyMapping struct {
	UUID     string                 `json:"uuid"`
	Name     string                 `json:"name"`
	Slug     string                 `json:"slug"`
	Type     string                 `json:"type"`
	Expression string               `json:"expression"`
}

// ---------------------------------------------------------------------------
// Provider SCIM Users
// ---------------------------------------------------------------------------

// SCIMUsersAPI provides access to /api/v3/providers/scim_users/ endpoints.
type SCIMUsersAPI struct{ *Client }

// SCIMUser is a SCIM-synced user.
type SCIMUser struct {
	UUID       string                 `json:"uuid"`
	Username   string                 `json:"username"`
	Email      string                 `json:"email"`
	Provider   string                 `json:"provider"`   // Provider UUID
	Source     string                 `json:"source"`     // Source UUID
	ExternalID string                 `json:"external_id"` // External ID from the source
}

// ---------------------------------------------------------------------------
// Sources (generic)
// ---------------------------------------------------------------------------

// SourcesAPI provides access to /api/v3/sources/all/ endpoints.
type SourcesAPI struct{ *Client }

// Source is a generic source (covers LDAP, SAML, OAuth, SCIM, Kerberos).
type Source struct {
	UUID     string                 `json:"uuid"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	SourceType string               `json:"source_type"`
	Connected  bool                 `json:"connected"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// CreateSourceRequest is the request body for creating a source.
type CreateSourceRequest struct {
	Name       string                 `json:"name"`
	SourceType string                 `json:"source_type"` // "ldap", "saml", "oauth", "scim"
	Properties map[string]interface{} `json:"properties"`
}

// Create creates a new source.
func (s *SourcesAPI) Create(ctx context.Context, req CreateSourceRequest) (*Source, error) {
	var source Source
	body, _ := json.Marshal(req)
	if err := s.doRequest(ctx, http.MethodPost, "/api/v3/sources/all/", bytes.NewReader(body), &source); err != nil {
		return nil, err
	}
	return &source, nil
}

// GetByID retrieves a source by UUID.
func (s *SourcesAPI) GetByID(ctx context.Context, uuid string) (*Source, error) {
	var source Source
	if err := s.doRequest(ctx, http.MethodGet, "/api/v3/sources/all/"+uuid+"/", nil, &source); err != nil {
		return nil, err
	}
	return &source, nil
}

// List returns all sources.
func (s *SourcesAPI) List(ctx context.Context) ([]*Source, error) {
	var sources []*Source
	if err := s.Client.doListRequest(ctx, "/api/v3/sources/all/", &sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// Delete removes a source.
func (s *SourcesAPI) Delete(ctx context.Context, uuid string) error {
	return s.doRequest(ctx, http.MethodDelete, "/api/v3/sources/all/"+uuid+"/", nil, nil)
}

// ---------------------------------------------------------------------------
// Password Policies
// ---------------------------------------------------------------------------

// PasswordPoliciesAPI provides access to /api/v3/policies/password/ endpoints.
type PasswordPoliciesAPI struct{ *Client }

// PasswordPolicy is a password complexity policy.
type PasswordPolicy struct {
	UUID        string                 `json:"uuid"`
	Name        string                 `json:"name"`
	Enabled     bool                   `json:"enabled"`
	Description string                 `json:"description"`
	Properties  map[string]interface{} `json:"properties"`
}

// ---------------------------------------------------------------------------
// Reputation Policies
// ---------------------------------------------------------------------------

// ReputationPoliciesAPI provides access to /api/v3/policies/reputation/ endpoints.
type ReputationPoliciesAPI struct{ *Client }

// ReputationPolicy is an IP/device reputation policy.
type ReputationPolicy struct {
	UUID       string                 `json:"uuid"`
	Name       string                 `json:"name"`
	Enabled    bool                   `json:"enabled"`
	Properties map[string]interface{} `json:"properties"`
}

// ---------------------------------------------------------------------------
// Event Matcher Policies
// ---------------------------------------------------------------------------

// EventMatcherPoliciesAPI provides access to /api/v3/policies/event_matcher/ endpoints.
type EventMatcherPoliciesAPI struct{ *Client }

// EventMatcherPolicy is a policy that matches events against patterns.
type EventMatcherPolicy struct {
	UUID       string                 `json:"uuid"`
	Name       string                 `json:"name"`
	Enabled    bool                   `json:"enabled"`
	Events     []string               `json:"events"`
	Properties map[string]interface{} `json:"properties"`
}

// ---------------------------------------------------------------------------
// Generic HTTP Caller (for handler code that needs flexible API access)
// ---------------------------------------------------------------------------

// Call performs an authenticated HTTP request and returns the parsed response.
// This is a public wrapper around the internal doRequest, allowing handlers
// and other external packages to make arbitrary authenticated requests to the
// Authentik API.
func (c *Client) Call(ctx context.Context, method, path string, body io.Reader, out interface{}) error {
	return c.doRequest(ctx, method, path, body, out)
}

// PaginatedEventResponse is the standard Authentik paginated list response for events.
type PaginatedEventResponse struct {
	Count    int        `json:"count"`
	Next     string     `json:"next"`
	Previous string     `json:"previous"`
	Results  []*Event   `json:"results"`
}

// ---------------------------------------------------------------------------
// RBAC Check (permission evaluation)
// ---------------------------------------------------------------------------

// CheckPermissionRequest is the request body for the RBAC check endpoint.
type CheckPermissionRequest struct {
	User       string `json:"user"`
	Permission string `json:"permission"`
}

// CheckPermission verifies whether a user has a specific permission.
// Returns true if the user is granted the permission.
func (r *RBACAPI) CheckPermission(ctx context.Context, req CheckPermissionRequest) (bool, error) {
	var resp struct {
		Check bool `json:"check"`
	}
	data, _ := json.Marshal(req)
	if err := r.Client.Call(ctx, http.MethodGet, "/api/v3/rbac/check/", bytes.NewReader(data), &resp); err != nil {
		return false, err
	}
	return resp.Check, nil
}
