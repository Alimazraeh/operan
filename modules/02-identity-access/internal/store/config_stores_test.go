package store

import (
	"errors"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/models"
)

// ─── SSOConfigStore ──────────────────────────────────────────────────────────

func TestSSOConfigStore_CreateAndGet(t *testing.T) {
	s := NewSSOConfigStore()
	cfg := &models.SSOConfig{
		TenantID:      "tenant-1",
		Provider:      "authentik",
		Type:          "oauth2",
		Configuration: map[string]interface{}{"client_id": "abc"},
	}
	if err := s.Create(cfg); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if cfg.ID == "" {
		t.Error("Create() should auto-generate ID")
	}
	if cfg.Status != "configured" {
		t.Errorf("expected default status configured, got %s", cfg.Status)
	}
	if cfg.ConfigJSON == "" {
		t.Error("Create() should marshal Configuration to ConfigJSON")
	}

	got, err := s.GetByTenant("tenant-1")
	if err != nil {
		t.Fatalf("GetByTenant() error = %v", err)
	}
	if got.Configuration["client_id"] != "abc" {
		t.Errorf("expected client_id roundtrip, got %v", got.Configuration["client_id"])
	}

	byID, err := s.GetByID(cfg.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if byID.TenantID != "tenant-1" {
		t.Errorf("GetByID() wrong tenant: %s", byID.TenantID)
	}
}

func TestSSOConfigStore_CreateValidation(t *testing.T) {
	s := NewSSOConfigStore()
	cases := map[string]*models.SSOConfig{
		"missing tenant":   {Provider: "p", Type: "oauth2"},
		"missing provider": {TenantID: "t", Type: "oauth2"},
		"missing type":     {TenantID: "t", Provider: "p"},
	}
	for name, cfg := range cases {
		if err := s.Create(cfg); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestSSOConfigStore_NotFound(t *testing.T) {
	s := NewSSOConfigStore()
	if _, err := s.GetByTenant("nope"); !errors.Is(err, ErrSSOConfigNotFound) {
		t.Errorf("GetByTenant: expected ErrSSOConfigNotFound, got %v", err)
	}
	if _, err := s.GetByID("nope"); !errors.Is(err, ErrSSOConfigNotFound) {
		t.Errorf("GetByID: expected ErrSSOConfigNotFound, got %v", err)
	}
	if _, err := s.Update("nope", "p", "oauth2", nil, ""); !errors.Is(err, ErrSSOConfigNotFound) {
		t.Errorf("Update: expected ErrSSOConfigNotFound, got %v", err)
	}
	if err := s.Delete("nope"); !errors.Is(err, ErrSSOConfigNotFound) {
		t.Errorf("Delete: expected ErrSSOConfigNotFound, got %v", err)
	}
}

func TestSSOConfigStore_UpdateAndDelete(t *testing.T) {
	s := NewSSOConfigStore()
	cfg := &models.SSOConfig{TenantID: "t", Provider: "authentik", Type: "oauth2"}
	if err := s.Create(cfg); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := s.Update("t", "okta", "saml", map[string]interface{}{"k": "v"}, "active")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Provider != "okta" || updated.Type != "saml" || updated.Status != "active" {
		t.Errorf("Update() did not apply fields: %+v", updated)
	}
	if updated.Configuration["k"] != "v" {
		t.Errorf("Update() configuration not applied: %v", updated.Configuration)
	}

	if err := s.Delete("t"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.GetByTenant("t"); !errors.Is(err, ErrSSOConfigNotFound) {
		t.Error("Delete() should remove config")
	}
}

// ─── LDAPConfigStore ─────────────────────────────────────────────────────────

func TestLDAPConfigStore_CRUD(t *testing.T) {
	s := NewLDAPConfigStore()
	cfg := &models.LDAPConfig{TenantID: "t", Provider: "openldap", URL: "ldap://h:389"}
	if err := s.Create(cfg); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if cfg.ID == "" || cfg.Status != "configured" {
		t.Errorf("Create() defaults not set: %+v", cfg)
	}

	if _, err := s.GetByTenant("t"); err != nil {
		t.Errorf("GetByTenant() error = %v", err)
	}
	if _, err := s.GetByID(cfg.ID); err != nil {
		t.Errorf("GetByID() error = %v", err)
	}

	upd, err := s.Update("t", "Corp LDAP", "ldap://new:636", "dc=x", "cn=admin", "secret", "sub", "(uid=%s)", "(member=%s)", `{"tls":true}`, true, "active")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if upd.DisplayName != "Corp LDAP" || upd.URL != "ldap://new:636" || !upd.Enabled || upd.Status != "active" {
		t.Errorf("Update() did not apply fields: %+v", upd)
	}

	if got := s.List(); len(got) != 1 {
		t.Errorf("List() expected 1, got %d", len(got))
	}

	if err := s.Delete("t"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.GetByID(cfg.ID); !errors.Is(err, ErrLDAPConfigNotFound) {
		t.Error("Delete() should remove from byID index too")
	}
}

func TestLDAPConfigStore_Validation(t *testing.T) {
	s := NewLDAPConfigStore()
	for name, cfg := range map[string]*models.LDAPConfig{
		"missing tenant":   {Provider: "openldap", URL: "u"},
		"missing provider": {TenantID: "t", URL: "u"},
		"missing url":      {TenantID: "t", Provider: "openldap"},
	} {
		if err := s.Create(cfg); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := s.Update("nope", "", "", "", "", "", "", "", "", "", false, ""); !errors.Is(err, ErrLDAPConfigNotFound) {
		t.Errorf("Update missing: expected ErrLDAPConfigNotFound, got %v", err)
	}
	if err := s.Delete("nope"); !errors.Is(err, ErrLDAPConfigNotFound) {
		t.Errorf("Delete missing: expected ErrLDAPConfigNotFound, got %v", err)
	}
}

// ─── ADConfigStore ───────────────────────────────────────────────────────────

func TestADConfigStore_CRUD(t *testing.T) {
	s := NewADConfigStore()
	cfg := &models.ADConfig{TenantID: "t", DomainName: "corp.local", DomainController: "dc1.corp.local"}
	if err := s.Create(cfg); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if cfg.ID == "" || cfg.Status != "configured" {
		t.Errorf("Create() defaults not set: %+v", cfg)
	}

	if _, err := s.GetByTenant("t"); err != nil {
		t.Errorf("GetByTenant() error = %v", err)
	}
	if _, err := s.GetByID(cfg.ID); err != nil {
		t.Errorf("GetByID() error = %v", err)
	}

	upd, err := s.Update("t", "Corp AD", "corp2.local", "dc2.corp2.local", "cn=svc", "pw", "OU=Users", `{"ssl":true}`, true, "active")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if upd.DisplayName != "Corp AD" || upd.DomainName != "corp2.local" || upd.OrganizationUnit != "OU=Users" || !upd.Enabled {
		t.Errorf("Update() did not apply fields: %+v", upd)
	}

	if err := s.Delete("t"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.GetByID(cfg.ID); !errors.Is(err, ErrADConfigNotFound) {
		t.Error("Delete() should remove from byID index too")
	}
}

func TestADConfigStore_Validation(t *testing.T) {
	s := NewADConfigStore()
	for name, cfg := range map[string]*models.ADConfig{
		"missing tenant":     {DomainName: "d", DomainController: "dc"},
		"missing domain":     {TenantID: "t", DomainController: "dc"},
		"missing controller": {TenantID: "t", DomainName: "d"},
	} {
		if err := s.Create(cfg); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := s.Update("nope", "", "", "", "", "", "", "", false, ""); !errors.Is(err, ErrADConfigNotFound) {
		t.Errorf("Update missing: expected ErrADConfigNotFound, got %v", err)
	}
	if err := s.Delete("nope"); !errors.Is(err, ErrADConfigNotFound) {
		t.Errorf("Delete missing: expected ErrADConfigNotFound, got %v", err)
	}
}

// ─── DelegationRoleStore ─────────────────────────────────────────────────────

func TestDelegationRoleStore_CRUD(t *testing.T) {
	s := NewDelegationRoleStore()
	role := &models.DelegationRole{
		TenantID:    "t",
		Name:        "dept-admin",
		Scope:       "department",
		Permissions: []string{"users:read", "users:write"},
	}
	if err := s.Create(role); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if role.ID == "" || role.PermissionsJSON == "" {
		t.Errorf("Create() should set ID and PermissionsJSON: %+v", role)
	}

	got, err := s.GetByID(role.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if len(got.Permissions) != 2 {
		t.Errorf("GetByID() permissions roundtrip failed: %v", got.Permissions)
	}

	byName, err := s.GetByName("t", "dept-admin")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	if byName.ID != role.ID {
		t.Errorf("GetByName() returned wrong role")
	}

	list, total, err := s.List("t", 1, 10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Errorf("List() expected 1/1, got %d/%d", len(list), total)
	}

	upd, err := s.Update(role.ID, "dept-admin-v2", "desc", "team", []string{"users:read"}, 3)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if upd.Name != "dept-admin-v2" || upd.Scope != "team" || upd.MaxDelegationDepth != 3 || len(upd.Permissions) != 1 {
		t.Errorf("Update() did not apply fields: %+v", upd)
	}
	// old name should no longer resolve
	if _, err := s.GetByName("t", "dept-admin"); !errors.Is(err, ErrDelegationRoleNotFound) {
		t.Error("Update() should reindex by new name")
	}

	if err := s.Delete(role.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.GetByID(role.ID); !errors.Is(err, ErrDelegationRoleNotFound) {
		t.Error("Delete() should remove role")
	}
}

func TestDelegationRoleStore_Validation(t *testing.T) {
	s := NewDelegationRoleStore()
	for name, r := range map[string]*models.DelegationRole{
		"missing tenant": {Name: "n", Scope: "tenant"},
		"missing name":   {TenantID: "t", Scope: "tenant"},
		"missing scope":  {TenantID: "t", Name: "n"},
	} {
		if err := s.Create(r); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}

	// duplicate name within tenant
	r1 := &models.DelegationRole{TenantID: "t", Name: "dup", Scope: "tenant"}
	if err := s.Create(r1); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	r2 := &models.DelegationRole{TenantID: "t", Name: "dup", Scope: "tenant"}
	if err := s.Create(r2); err == nil {
		t.Error("Create() should reject duplicate name in same tenant")
	}

	if _, err := s.GetByID("nope"); !errors.Is(err, ErrDelegationRoleNotFound) {
		t.Errorf("GetByID missing: got %v", err)
	}
	if _, err := s.Update("nope", "", "", "", nil, 0); !errors.Is(err, ErrDelegationRoleNotFound) {
		t.Errorf("Update missing: got %v", err)
	}
	if err := s.Delete("nope"); !errors.Is(err, ErrDelegationRoleNotFound) {
		t.Errorf("Delete missing: got %v", err)
	}
}

func TestDelegationRoleStore_GrantRevoke(t *testing.T) {
	s := NewDelegationRoleStore()
	role := &models.DelegationRole{TenantID: "t", Name: "r", Scope: "tenant"}
	if err := s.Create(role); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// grant validation
	if err := s.GrantDelegation(&models.DelegationGrant{DelegationRoleID: role.ID, UserID: "u1"}); err == nil {
		t.Error("GrantDelegation should require tenant_id")
	}
	if err := s.GrantDelegation(&models.DelegationGrant{TenantID: "t", UserID: "u1"}); err == nil {
		t.Error("GrantDelegation should require delegation_role_id")
	}
	if err := s.GrantDelegation(&models.DelegationGrant{TenantID: "t", DelegationRoleID: role.ID}); err == nil {
		t.Error("GrantDelegation should require user_id")
	}
	if err := s.GrantDelegation(&models.DelegationGrant{TenantID: "t", DelegationRoleID: "missing", UserID: "u1"}); !errors.Is(err, ErrDelegationRoleNotFound) {
		t.Errorf("GrantDelegation to missing role: got %v", err)
	}

	// successful grant
	if err := s.GrantDelegation(&models.DelegationGrant{TenantID: "t", DelegationRoleID: role.ID, UserID: "u1"}); err != nil {
		t.Fatalf("GrantDelegation() error = %v", err)
	}
	got, _ := s.GetByID(role.ID)
	if len(got.DelegatedToIDs) != 1 || got.DelegatedToIDs[0] != "u1" {
		t.Errorf("grant not recorded: %v", got.DelegatedToIDs)
	}

	// revoke
	if err := s.RevokeDelegation(role.ID, "u1"); err != nil {
		t.Fatalf("RevokeDelegation() error = %v", err)
	}
	got, _ = s.GetByID(role.ID)
	if len(got.DelegatedToIDs) != 0 {
		t.Errorf("revoke did not remove user: %v", got.DelegatedToIDs)
	}
	if err := s.RevokeDelegation("missing", "u1"); !errors.Is(err, ErrDelegationRoleNotFound) {
		t.Errorf("RevokeDelegation missing role: got %v", err)
	}

	// ListDelegations is a simplified stub that returns an empty slice
	grants, err := s.ListDelegations("t", role.ID, "u1")
	if err != nil {
		t.Fatalf("ListDelegations() error = %v", err)
	}
	if len(grants) != 0 {
		t.Errorf("ListDelegations() expected empty, got %d", len(grants))
	}
}
