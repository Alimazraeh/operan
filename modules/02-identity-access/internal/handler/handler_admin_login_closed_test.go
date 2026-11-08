package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/config"
)

// adminLoginUnconfigured points the handler at a deployment with no admin
// password set — the state the live cluster was in.
func adminLoginUnconfigured(t *testing.T) *AdminLoginHandler {
	t.Helper()
	t.Setenv("ADMIN_PASSWORD", "")
	t.Setenv("IAM_ADMIN_PASSWORD_FILE", "")
	return NewAdminLoginHandler(&config.Config{TokenSecret: "test-secret-not-a-real-key"})
}

func postAdminLogin(t *testing.T, h *AdminLoginHandler, password string) *httptest.ResponseRecorder {
	t.Helper()
	body := []byte(`{"password":"` + password + `","tenant":"acme-3"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	return rec
}

// The handler used to fall back to the hardcoded string "operan-admin-2026"
// when neither ADMIN_PASSWORD nor IAM_ADMIN_PASSWORD_FILE was set. The live
// identity-access deployment set neither, so that string was the front door of
// a deployed platform, reachable by anyone who had read the source. Break-glass
// access that works out of the box is a backdoor.
func TestAdminLogin_RejectsTheOldHardcodedDefault(t *testing.T) {
	h := adminLoginUnconfigured(t)

	rec := postAdminLogin(t, h, "operan-admin-2026")
	if rec.Code == http.StatusOK {
		t.Fatal("the hardcoded default still authenticates — this is the backdoor")
	}
}

// With nothing configured the endpoint must say so rather than quietly
// accepting something.
func TestAdminLogin_FailsClosedWhenUnconfigured(t *testing.T) {
	h := adminLoginUnconfigured(t)

	rec := postAdminLogin(t, h, "anything-at-all")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("code = %d, want %d (unconfigured must fail closed); body: %s",
			rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}

// A configured password still works — fail-closed must not mean fail-always.
func TestAdminLogin_ConfiguredPasswordStillWorks(t *testing.T) {
	t.Setenv("IAM_ADMIN_PASSWORD_FILE", "")
	t.Setenv("ADMIN_PASSWORD", "a-real-break-glass-password")
	h := NewAdminLoginHandler(&config.Config{TokenSecret: "test-secret-not-a-real-key"})

	if rec := postAdminLogin(t, h, "a-real-break-glass-password"); rec.Code != http.StatusOK {
		t.Errorf("configured password rejected: code = %d, body: %s", rec.Code, rec.Body.String())
	}
	if rec := postAdminLogin(t, h, "wrong"); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// Guard against the string coming back in any form.
func TestAdminLogin_SourceCarriesNoDefaultPassword(t *testing.T) {
	src, err := os.ReadFile("handler_admin_login.go")
	if err != nil {
		t.Fatalf("read handler source: %v", err)
	}
	// The comment explaining the removal names it; a string literal assigned as
	// the stored password is what must not come back.
	if bytes.Contains(src, []byte(`storedPassword = "operan-admin-2026"`)) {
		t.Error("the hardcoded default password has been reintroduced")
	}
}
