package bootstrap

import (
	"testing"

	"github.com/operan/modules/02-identity-access/internal/store"
)

const (
	tenant = "acme-3"
	email  = "ali@adri.nz"
	// Long enough for the credential policy (12 characters minimum) and never
	// used anywhere real.
	password = "test-only-bootstrap-pw"
)

// A fresh tenant has no users at all, which is exactly why signing in fell back
// to the shared admin password. Provisioning the first administrator is what
// makes real per-user login usable on a new deployment.
func TestEnsureAdminUser_CreatesTheFirstAdministrator(t *testing.T) {
	users := store.NewUserStore()

	outcome, err := EnsureAdminUser(users, tenant, email, password)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != Created {
		t.Fatalf("outcome = %v, want Created", outcome)
	}

	got, err := users.GetByTenantAndEmail(tenant, email)
	if err != nil {
		t.Fatalf("the account was not stored: %v", err)
	}
	if got.PasswordHash == "" {
		t.Error("no password hash stored — the account exists but cannot sign in")
	}
	if got.PasswordHash == password {
		t.Error("the password was stored in the clear")
	}
	if got.Status != "active" {
		t.Errorf("status = %q, want active — setting a credential must promote a pending account", got.Status)
	}
}

// The role has to be one the platform's permission vocabulary knows. The portal
// maps role -> permissions from a fixed set (perm.js ROLE_PERMISSIONS:
// platform_admin, executive, department_head, supervisor, employee) and grants
// nothing at all for anything else — so seeding "admin" produced an
// administrator who signed in successfully and then found every navigation
// entry that requires a permission missing, with no error to explain it.
func TestEnsureAdminUser_GrantsARoleThePlatformKnows(t *testing.T) {
	known := map[string]bool{
		"platform_admin": true, "executive": true,
		"department_head": true, "supervisor": true, "employee": true,
	}
	if !known[AdminRole] {
		t.Fatalf("AdminRole = %q, which perm.js grants no permissions for", AdminRole)
	}
	if AdminRole != "platform_admin" {
		t.Errorf("AdminRole = %q — only platform_admin carries platform.admin, "+
			"which the Tenants, IAM, Connectors and Observability views require", AdminRole)
	}

	users := store.NewUserStore()
	if _, err := EnsureAdminUser(users, tenant, email, password); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := users.GetByTenantAndEmail(tenant, email)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if len(got.RoleIDs) != 1 || got.RoleIDs[0] != "platform_admin" {
		t.Errorf("roles = %v, want [platform_admin]", got.RoleIDs)
	}
}

// Running on every boot must not touch an account that already exists:
// re-asserting the configured password would silently undo a rotation, and
// re-activating would revive an account someone had deliberately disabled.
func TestEnsureAdminUser_LeavesAnExistingAccountAlone(t *testing.T) {
	users := store.NewUserStore()
	if _, err := EnsureAdminUser(users, tenant, email, password); err != nil {
		t.Fatalf("first call: %v", err)
	}
	before, err := users.GetByTenantAndEmail(tenant, email)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	outcome, err := EnsureAdminUser(users, tenant, email, "a-different-password")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if outcome != AlreadyPresent {
		t.Errorf("outcome = %v, want AlreadyPresent", outcome)
	}

	after, err := users.GetByTenantAndEmail(tenant, email)
	if err != nil {
		t.Fatalf("after: %v", err)
	}
	if after.PasswordHash != before.PasswordHash {
		t.Error("the existing credential was overwritten — a rotation would be silently undone")
	}
}

// Each tenant needs its own administrator; one tenant's account must not count
// as another's.
func TestEnsureAdminUser_IsPerTenant(t *testing.T) {
	users := store.NewUserStore()
	if _, err := EnsureAdminUser(users, "acme-2", email, password); err != nil {
		t.Fatalf("first tenant: %v", err)
	}

	outcome, err := EnsureAdminUser(users, "acme-3", email, password)
	if err != nil {
		t.Fatalf("second tenant: %v", err)
	}
	if outcome != Created {
		t.Errorf("outcome = %v, want Created — the same email in another tenant is a different account", outcome)
	}
}

func TestEnsureAdminUser_RequiresAllThreeInputs(t *testing.T) {
	for name, tc := range map[string]struct{ tenant, email, password string }{
		"no tenant":   {"", email, password},
		"no email":    {tenant, "", password},
		"no password": {tenant, email, ""},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := EnsureAdminUser(store.NewUserStore(), tc.tenant, tc.email, tc.password); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

// A password the policy rejects must leave nothing behind, or the next boot
// finds a half-provisioned account that can never log in.
func TestEnsureAdminUser_WeakPasswordProvisionsNothing(t *testing.T) {
	users := store.NewUserStore()

	if _, err := EnsureAdminUser(users, tenant, email, "short"); err == nil {
		t.Fatal("expected the credential policy to reject a short password")
	}
	if u, err := users.GetByTenantAndEmail(tenant, email); err == nil && u != nil {
		t.Error("an account was created despite the password being rejected")
	}
}

// Guards the assumption the production path depends on: the store it is handed
// must be able to hold a credential.
func TestSetCredential_RefusesAStoreThatCannotHoldOne(t *testing.T) {
	if err := setCredential(storeWithoutSetter{}, "some-id", "some-hash"); err != ErrNoPasswordSetter {
		t.Errorf("err = %v, want ErrNoPasswordSetter", err)
	}
}

type storeWithoutSetter struct{ store.UserStoreAPI }

var _ store.UserStoreAPI = storeWithoutSetter{}
