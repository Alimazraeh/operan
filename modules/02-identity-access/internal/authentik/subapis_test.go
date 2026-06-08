package authentik

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// universalHandler returns a JSON body that satisfies every unmarshal target
// used by the sub-APIs: single-object reads (uuid/name/pk/key), doListRequest
// pagination (count/next/results), RBAC check (check), and the string-map
// SetupURLs endpoint (handled separately).
func universalHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if strings.Contains(r.URL.Path, "setup_urls") {
		// map[string]string target — all values must be strings.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"login":"http://x/login","logout":"http://x/logout"}`))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"uuid":"id1","name":"n","pk":"pk1","key":"k1","check":true,"count":0,"next":"","results":[]}`))
}

// TestSubAPIs_HappyPath drives every sub-API method through the shared
// doRequest/doListRequest plumbing against a permissive server, asserting the
// wrappers marshal requests and parse responses without error.
func TestSubAPIs_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, universalHandler)
	ctx := context.Background()

	check := func(name string, err error) {
		t.Helper()
		if err != nil {
			t.Errorf("%s: unexpected error: %v", name, err)
		}
	}

	// Applications
	_, err := c.ApplicationsAPI.Create(ctx, CreateApplicationRequest{})
	check("Applications.Create", err)
	_, err = c.ApplicationsAPI.GetByID(ctx, "a1")
	check("Applications.GetByID", err)
	_, err = c.ApplicationsAPI.List(ctx)
	check("Applications.List", err)
	check("Applications.Delete", c.ApplicationsAPI.Delete(ctx, "a1"))

	// Tokens
	_, err = c.TokensAPI.Create(ctx, CreateTokenRequest{})
	check("Tokens.Create", err)
	_, err = c.TokensAPI.ViewKey(ctx, "t1")
	check("Tokens.ViewKey", err)
	_, err = c.TokensAPI.SetKey(ctx, "t1")
	check("Tokens.SetKey", err)

	// OAuth2 Providers
	_, err = c.OAuth2API().Create(ctx, CreateOAuth2ProviderRequest{})
	check("OAuth2.Create", err)
	_, err = c.OAuth2API().GetByID(ctx, "o1")
	check("OAuth2.GetByID", err)
	_, err = c.OAuth2API().List(ctx)
	check("OAuth2.List", err)
	check("OAuth2.Delete", c.OAuth2API().Delete(ctx, "o1"))
	urls, err := c.OAuth2API().SetupURLs(ctx, "o1")
	check("OAuth2.SetupURLs", err)
	if urls["login"] == "" {
		t.Error("SetupURLs should return string map")
	}

	// SAML Providers
	_, err = c.SAMLAPI().Create(ctx, CreateSAMLProviderRequest{})
	check("SAML.Create", err)
	_, err = c.SAMLAPI().GetByID(ctx, "s1")
	check("SAML.GetByID", err)
	_, err = c.SAMLAPI().List(ctx)
	check("SAML.List", err)
	check("SAML.Delete", c.SAMLAPI().Delete(ctx, "s1"))
	_, err = c.SAMLAPI().ImportMetadata(ctx, "s1", "http://meta")
	check("SAML.ImportMetadata", err)

	// LDAP Sources
	_, err = c.LDAPSources().Create(ctx, CreateLDAPSourceRequest{})
	check("LDAP.Create", err)
	_, err = c.LDAPSources().GetByID(ctx, "l1")
	check("LDAP.GetByID", err)
	_, err = c.LDAPSources().Update(ctx, "l1", map[string]interface{}{"name": "x"})
	check("LDAP.Update", err)
	_, err = c.LDAPSources().List(ctx)
	check("LDAP.List", err)
	check("LDAP.Delete", c.LDAPSources().Delete(ctx, "l1"))
	_, err = c.LDAPSources().SyncStatus(ctx, "l1")
	check("LDAP.SyncStatus", err)
	_, err = c.LDAPSources().Debug(ctx, "l1")
	check("LDAP.Debug", err)

	// SCIM Providers
	_, err = c.ScimAPI.Create(ctx, CreateSCIMProviderRequest{})
	check("SCIM.Create", err)
	_, err = c.ScimAPI.GetByID(ctx, "sc1")
	check("SCIM.GetByID", err)
	_, err = c.ScimAPI.List(ctx)
	check("SCIM.List", err)
	check("SCIM.Delete", c.ScimAPI.Delete(ctx, "sc1"))

	// RBAC
	_, err = c.RBACAPI.Create(ctx, CreateRoleRequest{})
	check("RBAC.Create", err)
	_, err = c.RBACAPI.GetByID(ctx, "r1")
	check("RBAC.GetByID", err)
	_, err = c.RBACAPI.List(ctx)
	check("RBAC.List", err)
	check("RBAC.Delete", c.RBACAPI.Delete(ctx, "r1"))
	check("RBAC.AssignUser", c.RBACAPI.AssignUser(ctx, "r1", "u1"))
	check("RBAC.RemoveUser", c.RBACAPI.RemoveUser(ctx, "r1", "u1"))
	_, err = c.RBACAPI.ListPermissions(ctx)
	check("RBAC.ListPermissions", err)
	check("RBAC.AssignPermission", c.RBACAPI.AssignPermission(ctx, "r1", "p1"))
	check("RBAC.UnassignPermission", c.RBACAPI.UnassignPermission(ctx, "r1", "p1"))
	allowed, err := c.RBACAPI.CheckPermission(ctx, CheckPermissionRequest{})
	check("RBAC.CheckPermission", err)
	if !allowed {
		t.Error("CheckPermission should parse check=true")
	}

	// Tenants
	_, err = c.TenantsAPI.Create(ctx, CreateTenantRequest{})
	check("Tenants.Create", err)
	_, err = c.TenantsAPI.GetByID(ctx, "tn1")
	check("Tenants.GetByID", err)
	_, err = c.TenantsAPI.List(ctx)
	check("Tenants.List", err)
	check("Tenants.Delete", c.TenantsAPI.Delete(ctx, "tn1"))

	// Brands
	_, err = c.BrandsAPI.Create(ctx, CreateBrandRequest{})
	check("Brands.Create", err)
	_, err = c.BrandsAPI.GetByID(ctx, "b1")
	check("Brands.GetByID", err)
	_, err = c.BrandsAPI.List(ctx)
	check("Brands.List", err)
	_, err = c.BrandsAPI.Update(ctx, "b1", CreateBrandRequest{})
	check("Brands.Update", err)
	check("Brands.Delete", c.BrandsAPI.Delete(ctx, "b1"))

	// Flow Bindings
	_, err = c.FlowBindingsAPI.Create(ctx, CreateFlowBindingRequest{})
	check("FlowBindings.Create", err)
	_, err = c.FlowBindingsAPI.GetByID(ctx, "fb1")
	check("FlowBindings.GetByID", err)
	_, err = c.FlowBindingsAPI.List(ctx)
	check("FlowBindings.List", err)
	check("FlowBindings.Delete", c.FlowBindingsAPI.Delete(ctx, "fb1"))

	// Policy Bindings
	_, err = c.PolicyBindingsAPI.CreateBinding(ctx, "p1", "f1")
	check("PolicyBindings.CreateBinding", err)
	check("PolicyBindings.DeleteBinding", c.PolicyBindingsAPI.DeleteBinding(ctx, "pb1"))

	// Sources
	_, err = c.SourcesAPI.Create(ctx, CreateSourceRequest{})
	check("Sources.Create", err)
	_, err = c.SourcesAPI.GetByID(ctx, "src1")
	check("Sources.GetByID", err)
	_, err = c.SourcesAPI.List(ctx)
	check("Sources.List", err)
	check("Sources.Delete", c.SourcesAPI.Delete(ctx, "src1"))

	// Events
	_, err = c.EventsAPI.List(ctx, "actor", "user")
	check("Events.List", err)
	_, err = c.EventsAPI.List(ctx, "", "")
	check("Events.List(no filter)", err)

	// Sessions
	_, err = c.SessionsAPI.List(ctx, "u1")
	check("Sessions.List", err)
	check("Sessions.Delete", c.SessionsAPI.Delete(ctx, "sess1"))

	// Flows
	fapi := &FlowsAPI{c}
	_, err = fapi.Create(ctx, CreateFlowRequest{})
	check("Flows.Create", err)
	_, err = fapi.GetByID(ctx, "f1")
	check("Flows.GetByID", err)
	_, err = fapi.List(ctx)
	check("Flows.List", err)
	check("Flows.Delete", fapi.Delete(ctx, "f1"))

	// Groups.List (other Group methods covered in client_test.go)
	_, err = c.Groups().List(ctx)
	check("Groups.List", err)

	// Generic Call passthrough
	check("Call", c.Call(ctx, http.MethodGet, "/api/v3/anything/", nil, nil))
}

// TestSubAPIs_ErrorPath drives every sub-API method against a server that
// always returns 500, exercising the error-return branch of each wrapper.
func TestSubAPIs_ErrorPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"boom"}`, http.StatusInternalServerError)
	})
	ctx := context.Background()

	wantErr := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}

	var err error
	_, err = c.ApplicationsAPI.Create(ctx, CreateApplicationRequest{})
	wantErr("Applications.Create", err)
	_, err = c.ApplicationsAPI.GetByID(ctx, "a1")
	wantErr("Applications.GetByID", err)
	_, err = c.ApplicationsAPI.List(ctx)
	wantErr("Applications.List", err)
	wantErr("Applications.Delete", c.ApplicationsAPI.Delete(ctx, "a1"))

	_, err = c.TokensAPI.Create(ctx, CreateTokenRequest{})
	wantErr("Tokens.Create", err)
	_, err = c.TokensAPI.ViewKey(ctx, "t1")
	wantErr("Tokens.ViewKey", err)
	_, err = c.TokensAPI.SetKey(ctx, "t1")
	wantErr("Tokens.SetKey", err)

	_, err = c.OAuth2API().Create(ctx, CreateOAuth2ProviderRequest{})
	wantErr("OAuth2.Create", err)
	_, err = c.OAuth2API().GetByID(ctx, "o1")
	wantErr("OAuth2.GetByID", err)
	_, err = c.OAuth2API().List(ctx)
	wantErr("OAuth2.List", err)
	wantErr("OAuth2.Delete", c.OAuth2API().Delete(ctx, "o1"))
	_, err = c.OAuth2API().SetupURLs(ctx, "o1")
	wantErr("OAuth2.SetupURLs", err)

	_, err = c.SAMLAPI().Create(ctx, CreateSAMLProviderRequest{})
	wantErr("SAML.Create", err)
	_, err = c.SAMLAPI().GetByID(ctx, "s1")
	wantErr("SAML.GetByID", err)
	_, err = c.SAMLAPI().List(ctx)
	wantErr("SAML.List", err)
	wantErr("SAML.Delete", c.SAMLAPI().Delete(ctx, "s1"))
	_, err = c.SAMLAPI().ImportMetadata(ctx, "s1", "http://meta")
	wantErr("SAML.ImportMetadata", err)

	_, err = c.LDAPSources().Create(ctx, CreateLDAPSourceRequest{})
	wantErr("LDAP.Create", err)
	_, err = c.LDAPSources().GetByID(ctx, "l1")
	wantErr("LDAP.GetByID", err)
	_, err = c.LDAPSources().Update(ctx, "l1", map[string]interface{}{})
	wantErr("LDAP.Update", err)
	_, err = c.LDAPSources().List(ctx)
	wantErr("LDAP.List", err)
	wantErr("LDAP.Delete", c.LDAPSources().Delete(ctx, "l1"))
	_, err = c.LDAPSources().SyncStatus(ctx, "l1")
	wantErr("LDAP.SyncStatus", err)
	_, err = c.LDAPSources().Debug(ctx, "l1")
	wantErr("LDAP.Debug", err)

	_, err = c.ScimAPI.Create(ctx, CreateSCIMProviderRequest{})
	wantErr("SCIM.Create", err)
	_, err = c.ScimAPI.GetByID(ctx, "sc1")
	wantErr("SCIM.GetByID", err)
	_, err = c.ScimAPI.List(ctx)
	wantErr("SCIM.List", err)
	wantErr("SCIM.Delete", c.ScimAPI.Delete(ctx, "sc1"))

	_, err = c.RBACAPI.Create(ctx, CreateRoleRequest{})
	wantErr("RBAC.Create", err)
	_, err = c.RBACAPI.GetByID(ctx, "r1")
	wantErr("RBAC.GetByID", err)
	_, err = c.RBACAPI.List(ctx)
	wantErr("RBAC.List", err)
	wantErr("RBAC.Delete", c.RBACAPI.Delete(ctx, "r1"))
	wantErr("RBAC.AssignUser", c.RBACAPI.AssignUser(ctx, "r1", "u1"))
	wantErr("RBAC.RemoveUser", c.RBACAPI.RemoveUser(ctx, "r1", "u1"))
	_, err = c.RBACAPI.ListPermissions(ctx)
	wantErr("RBAC.ListPermissions", err)
	wantErr("RBAC.AssignPermission", c.RBACAPI.AssignPermission(ctx, "r1", "p1"))
	wantErr("RBAC.UnassignPermission", c.RBACAPI.UnassignPermission(ctx, "r1", "p1"))
	_, err = c.RBACAPI.CheckPermission(ctx, CheckPermissionRequest{})
	wantErr("RBAC.CheckPermission", err)

	_, err = c.TenantsAPI.Create(ctx, CreateTenantRequest{})
	wantErr("Tenants.Create", err)
	_, err = c.TenantsAPI.GetByID(ctx, "tn1")
	wantErr("Tenants.GetByID", err)
	_, err = c.TenantsAPI.List(ctx)
	wantErr("Tenants.List", err)
	wantErr("Tenants.Delete", c.TenantsAPI.Delete(ctx, "tn1"))

	_, err = c.BrandsAPI.Create(ctx, CreateBrandRequest{})
	wantErr("Brands.Create", err)
	_, err = c.BrandsAPI.GetByID(ctx, "b1")
	wantErr("Brands.GetByID", err)
	_, err = c.BrandsAPI.List(ctx)
	wantErr("Brands.List", err)
	_, err = c.BrandsAPI.Update(ctx, "b1", CreateBrandRequest{})
	wantErr("Brands.Update", err)
	wantErr("Brands.Delete", c.BrandsAPI.Delete(ctx, "b1"))

	_, err = c.FlowBindingsAPI.Create(ctx, CreateFlowBindingRequest{})
	wantErr("FlowBindings.Create", err)
	_, err = c.FlowBindingsAPI.GetByID(ctx, "fb1")
	wantErr("FlowBindings.GetByID", err)
	_, err = c.FlowBindingsAPI.List(ctx)
	wantErr("FlowBindings.List", err)
	wantErr("FlowBindings.Delete", c.FlowBindingsAPI.Delete(ctx, "fb1"))

	_, err = c.PolicyBindingsAPI.CreateBinding(ctx, "p1", "f1")
	wantErr("PolicyBindings.CreateBinding", err)
	wantErr("PolicyBindings.DeleteBinding", c.PolicyBindingsAPI.DeleteBinding(ctx, "pb1"))

	_, err = c.SourcesAPI.Create(ctx, CreateSourceRequest{})
	wantErr("Sources.Create", err)
	_, err = c.SourcesAPI.GetByID(ctx, "src1")
	wantErr("Sources.GetByID", err)
	_, err = c.SourcesAPI.List(ctx)
	wantErr("Sources.List", err)
	wantErr("Sources.Delete", c.SourcesAPI.Delete(ctx, "src1"))

	_, err = c.EventsAPI.List(ctx, "a", "u")
	wantErr("Events.List", err)
	_, err = c.SessionsAPI.List(ctx, "u1")
	wantErr("Sessions.List", err)
	wantErr("Sessions.Delete", c.SessionsAPI.Delete(ctx, "sess1"))

	fapi := &FlowsAPI{c}
	_, err = fapi.Create(ctx, CreateFlowRequest{})
	wantErr("Flows.Create", err)
	_, err = fapi.GetByID(ctx, "f1")
	wantErr("Flows.GetByID", err)
	_, err = fapi.List(ctx)
	wantErr("Flows.List", err)
	wantErr("Flows.Delete", fapi.Delete(ctx, "f1"))

	_, err = c.Users().Create(ctx, CreateUserRequest{})
	wantErr("Users.Create", err)
	_, err = c.Users().List(ctx)
	wantErr("Users.List", err)
	_, err = c.Users().Update(ctx, "u1", UpdateUserRequest{})
	wantErr("Users.Update", err)
	wantErr("Users.Delete", c.Users().Delete(ctx, "u1"))
	_, err = c.Groups().Create(ctx, CreateGroupRequest{})
	wantErr("Groups.Create", err)
	_, err = c.Groups().List(ctx)
	wantErr("Groups.List", err)
	_, err = c.Groups().GetMembers(ctx, "g1")
	wantErr("Groups.GetMembers", err)
}
