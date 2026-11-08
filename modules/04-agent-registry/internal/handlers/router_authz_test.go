package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// postAgentAs issues POST /registry/agents carrying a token whose "role" claim
// is exactly role, and reports the status the router produced.
//
// The assertions below deliberately test for "not 403" rather than for 201.
// What is under test is the authorization layer, not CreateAgent's own
// validation: a body the handler later rejects still proves the request got
// past RequireRole, and tying these cases to a valid payload would make them
// fail for reasons that have nothing to do with roles.
func postAgentAs(t *testing.T, role string) int {
	t.Helper()

	h := newTestHandlers()
	body, _ := json.Marshal(map[string]interface{}{
		"name":      "it-manager-01",
		"role":      "manager",
		"tenant_id": "tenant-1",
	})

	req := httptest.NewRequest("POST", "/registry/agents", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Authorization", "Bearer "+makeValidJWT(t, "test-secret",
		map[string]interface{}{"role": role}))
	req = req.WithContext(withTenant(context.Background(), "tenant-1"))

	w := httptest.NewRecorder()
	RegisterRoutes(h).ServeHTTP(w, req)
	return w.Code
}

// The regression this file exists for.
//
// Module 02 used to seed its administrator with the role "admin", which is
// what this module's allow-list was written against. Correcting that seed to
// the platform's real role — platform_admin, the only role the portal grants
// platform.admin for — silently turned every department deployment into a 403
// at deploy_swarm, because Module 05 forwards the signed-in user's token
// verbatim when it registers the department's agents.
//
// Both names have to work: one is what the platform issues to real users, the
// other is what break-glass admin login still mints.
func TestCreateAgent_AcceptsEveryAdministratorRoleThePlatformIssues(t *testing.T) {
	for _, role := range []string{
		"platform_admin", // Module 02 per-user login, and the portal's vocabulary
		"admin",          // break-glass admin login (handler_admin_login.go)
		"registry_admin", // service tokens
	} {
		t.Run(role, func(t *testing.T) {
			if got := postAgentAs(t, role); got == http.StatusForbidden {
				t.Errorf("role %q was refused with 403 — a department deployment "+
					"carrying this token would fail at deploy_swarm", role)
			}
		})
	}
}

// Widening the allow-list must not have opened writes to everyone. These are
// the remaining roles in the portal's vocabulary; none of them should be able
// to register an agent.
func TestCreateAgent_StillRefusesNonAdministrators(t *testing.T) {
	for _, role := range []string{"executive", "department_head", "supervisor", "employee", "user"} {
		t.Run(role, func(t *testing.T) {
			if got := postAgentAs(t, role); got != http.StatusForbidden {
				t.Errorf("role %q got %d, want 403 — registry writes must stay "+
					"restricted to administrators", role, got)
			}
		})
	}
}

// adminRoles and readerRoles drifting apart is how this bug would come back:
// readerRoles is not wired to a route today, so nothing else would catch it.
func TestReaderRolesCoverEveryAdminRole(t *testing.T) {
	readers := make(map[string]bool, len(readerRoles))
	for _, r := range readerRoles {
		readers[r] = true
	}
	for _, r := range adminRoles {
		if !readers[r] {
			t.Errorf("%q may write but not read — readerRoles has fallen behind adminRoles", r)
		}
	}
}
