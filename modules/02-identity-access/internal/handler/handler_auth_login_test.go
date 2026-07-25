package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/operan/modules/02-identity-access/internal/auth"
	"github.com/operan/modules/02-identity-access/internal/models"
	"github.com/operan/modules/02-identity-access/internal/store"
)

const testSecret = "test-signing-secret"

func authFixture(t *testing.T) (*AuthHandler, *models.User) {
	t.Helper()
	users := store.NewUserStore()
	u := &models.User{
		TenantID: "smoke-tenant", Email: "dana@example.com",
		DisplayName: "Dana Q", Status: "active", RoleIDs: []string{"department_head"},
	}
	if err := users.Create(u); err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("correct-horse-9-battery")
	if err != nil {
		t.Fatal(err)
	}
	if err := users.SetPasswordHash(u.ID, hash); err != nil {
		t.Fatal(err)
	}
	return &AuthHandler{Users: users, TokenSecret: testSecret, ExpiryMins: 60}, u
}

func postLogin(h *AuthHandler, body map[string]string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/login", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.Login(w, req)
	return w
}

// The token must identify the actual person — this is the whole point of the
// endpoint. Every approval and request downstream reads `sub`.
func TestLoginMintsTokenForTheRealUser(t *testing.T) {
	h, u := authFixture(t)
	w := postLogin(h, map[string]string{
		"tenant": "smoke-tenant", "email": "dana@example.com", "password": "correct-horse-9-battery",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Token  string   `json:"token"`
		UserID string   `json:"user_id"`
		Roles  []string `json:"roles"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.UserID != u.ID {
		t.Errorf("user_id = %q, want %q", resp.UserID, u.ID)
	}

	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(resp.Token, claims, func(*jwt.Token) (interface{}, error) {
		return []byte(testSecret), nil
	}); err != nil {
		t.Fatalf("token does not verify: %v", err)
	}
	if claims["sub"] != u.ID {
		t.Errorf("sub = %v, want the real user id %q — not a synthetic admin", claims["sub"], u.ID)
	}
	if claims["sub"] == "admin-001" {
		t.Error("token still carries the shared synthetic identity")
	}
	if claims["tenant_id"] != "smoke-tenant" || claims["email"] != "dana@example.com" {
		t.Errorf("claims lost tenant/email: %v", claims)
	}
	// Every module pins this issuer; changing it breaks the platform.
	if claims["iss"] != "operan-tenant-control-plane" {
		t.Errorf("iss = %v, want operan-tenant-control-plane", claims["iss"])
	}
	if claims["role"] != "department_head" {
		t.Errorf("role = %v, want the user's own role", claims["role"])
	}
}

// A login endpoint must not reveal which accounts exist.
func TestLoginDoesNotAllowAccountEnumeration(t *testing.T) {
	h, _ := authFixture(t)
	// No such user, real user with the wrong password, and a user with no
	// credential set must be indistinguishable to the client.
	noUser := postLogin(h, map[string]string{"tenant": "smoke-tenant", "email": "ghost@example.com", "password": "correct-horse-9-battery"})
	wrongPw := postLogin(h, map[string]string{"tenant": "smoke-tenant", "email": "dana@example.com", "password": "wrong-horse-9-battery"})

	users := h.Users.(*store.UserStore)
	noCred := &models.User{TenantID: "smoke-tenant", Email: "new@example.com", DisplayName: "New", Status: "active"}
	users.Create(noCred)
	noCredResp := postLogin(h, map[string]string{"tenant": "smoke-tenant", "email": "new@example.com", "password": "correct-horse-9-battery"})

	for name, w := range map[string]*httptest.ResponseRecorder{
		"unknown user": noUser, "wrong password": wrongPw, "no credential": noCredResp,
	} {
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, w.Code)
		}
	}
	if noUser.Body.String() != wrongPw.Body.String() || wrongPw.Body.String() != noCredResp.Body.String() {
		t.Errorf("responses differ and leak which accounts exist:\n unknown=%s wrong=%s nocred=%s",
			noUser.Body.String(), wrongPw.Body.String(), noCredResp.Body.String())
	}
}

func TestLoginRejectsDeactivatedAccount(t *testing.T) {
	h, u := authFixture(t)
	users := h.Users.(*store.UserStore)
	users.Deactivate(u.ID)
	w := postLogin(h, map[string]string{
		"tenant": "smoke-tenant", "email": "dana@example.com", "password": "correct-horse-9-battery",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("deactivated account logged in: status %d", w.Code)
	}
}

// The response must never carry credential material.
func TestLoginResponseCarriesNoHash(t *testing.T) {
	h, _ := authFixture(t)
	w := postLogin(h, map[string]string{
		"tenant": "smoke-tenant", "email": "dana@example.com", "password": "correct-horse-9-battery",
	})
	for _, forbidden := range []string{"password_hash", "$2a$", "correct-horse"} {
		if bytes.Contains(w.Body.Bytes(), []byte(forbidden)) {
			t.Errorf("login response leaks %q: %s", forbidden, w.Body.String())
		}
	}
}

// The deactivate path (DELETE /users/{id}) writes status "inactive". An earlier
// deny-list here named only "deactivated" and "suspended", so deactivating a
// user did not stop them signing in. Every non-signin status is checked, and
// the check is an allow-list so a status invented later fails closed.
func TestLoginRefusesAccountsThatMayNotSignIn(t *testing.T) {
	for _, status := range []string{"inactive", "deactivated", "suspended", "locked", "disabled", "archived"} {
		t.Run(status, func(t *testing.T) {
			h, u := authFixture(t)
			if _, err := h.Users.Update(u.ID, &models.UpdateUserRequest{Status: &status}); err != nil {
				t.Fatal(err)
			}
			w := postLogin(h, map[string]string{
				"tenant": "smoke-tenant", "email": "dana@example.com", "password": "correct-horse-9-battery",
			})
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status %q signed in: got %d, want 401 (%s)", status, w.Code, w.Body.String())
			}
		})
	}
}

// A user created through the API starts "pending" and is only reachable once an
// admin sets a password. Refusing pending would mean no API-created user could
// ever sign in — and setting the credential is what promotes them to active, so
// the status column keeps describing something real.
func TestSettingACredentialActivatesAPendingAccount(t *testing.T) {
	users := store.NewUserStore()
	u := &models.User{TenantID: "smoke-tenant", Email: "new@example.com", DisplayName: "New", Status: "pending"}
	if err := users.Create(u); err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("correct-horse-9-battery")
	if err != nil {
		t.Fatal(err)
	}
	if err := users.SetPasswordHash(u.ID, hash); err != nil {
		t.Fatal(err)
	}
	after, err := users.GetByID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != "active" {
		t.Fatalf("status after setting a credential = %q, want active", after.Status)
	}

	h := &AuthHandler{Users: users, TokenSecret: testSecret, ExpiryMins: 60}
	w := postLogin(h, map[string]string{
		"tenant": "smoke-tenant", "email": "new@example.com", "password": "correct-horse-9-battery",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("activated user could not sign in: %d %s", w.Code, w.Body.String())
	}
}
