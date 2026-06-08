package authentik

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient spins up an httptest server with the given handler and returns
// a Client pointed at it.
func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "test-token"), srv
}

func writeJSON(t *testing.T, w http.ResponseWriter, code int, v interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func TestNewClient_WiresSubAPIs(t *testing.T) {
	c := NewClient("http://example.com/", "tok")
	if c.BaseURL != "http://example.com" {
		t.Errorf("BaseURL should trim trailing slash, got %q", c.BaseURL)
	}
	if c.Users() == nil || c.Groups() == nil || c.LDAPSources() == nil ||
		c.OAuth2API() == nil || c.SAMLAPI() == nil {
		t.Error("NewClient should initialize sub-API accessors")
	}
}

func TestAPIError_Error(t *testing.T) {
	e := &APIError{StatusCode: 404, Path: "/api/v3/core/users/x/", Message: "not found"}
	got := e.Error()
	if !strings.Contains(got, "404") || !strings.Contains(got, "/api/v3/core/users/x/") || !strings.Contains(got, "not found") {
		t.Errorf("APIError.Error() = %q", got)
	}
}

func TestUsersAPI_CRUD(t *testing.T) {
	var gotAuth string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/core/users/":
			writeJSON(t, w, 201, User{UUID: "u1", Username: "alice", Email: "a@x.io"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/core/users/u1/":
			writeJSON(t, w, 200, User{UUID: "u1", Username: "alice"})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v3/core/users/u1/":
			writeJSON(t, w, 200, User{UUID: "u1", Username: "alice2"})
		default:
			writeJSON(t, w, 404, map[string]string{"detail": "nope"})
		}
	})
	ctx := context.Background()

	created, err := c.Users().Create(ctx, CreateUserRequest{Username: "alice", Email: "a@x.io"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.UUID != "u1" {
		t.Errorf("Create() uuid = %q", created.UUID)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q", gotAuth)
	}

	got, err := c.Users().GetByID(ctx, "u1")
	if err != nil || got.Username != "alice" {
		t.Fatalf("GetByID() = %+v, err = %v", got, err)
	}

	upd, err := c.Users().Update(ctx, "u1", UpdateUserRequest{})
	if err != nil || upd.Username != "alice2" {
		t.Fatalf("Update() = %+v, err = %v", upd, err)
	}

	if err := c.Users().Delete(ctx, "u1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestUsersAPI_GetByID_APIError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 403, map[string]string{"detail": "forbidden"})
	})
	_, err := c.Users().GetByID(context.Background(), "u1")
	if err == nil {
		t.Fatal("expected error on 403")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
	}
}

func TestUsersAPI_List_Pagination(t *testing.T) {
	// First page links to ?page=2; second page ends pagination. The client
	// treats the `next` field as a path appended to BaseURL, so return a path.
	c, srv := newTestClient(t, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/core/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			writeJSON(t, w, 200, paginatedResponse{
				Results: []json.RawMessage{mustJSON(t, User{UUID: "u2"})},
			})
			return
		}
		writeJSON(t, w, 200, paginatedResponse{
			Next:    "/api/v3/core/users/?page=2",
			Results: []json.RawMessage{mustJSON(t, User{UUID: "u1"})},
		})
	})
	srv.Config.Handler = mux

	users, err := c.Users().List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(users) != 2 || users[0].UUID != "u1" || users[1].UUID != "u2" {
		t.Errorf("List() aggregated = %+v", users)
	}
}

func TestGroupsAPI_CRUDAndMembership(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/core/groups/":
			writeJSON(t, w, 201, Group{UUID: "g1", Name: "admins"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/core/groups/g1/":
			writeJSON(t, w, 200, Group{UUID: "g1", Name: "admins"})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v3/core/groups/g1/":
			writeJSON(t, w, 200, Group{UUID: "g1", Name: "renamed"})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v3/core/groups/g1/":
			writeJSON(t, w, 204, nil)
		case r.URL.Path == "/api/v3/core/groups/g1/add_user/":
			writeJSON(t, w, 204, nil)
		case r.URL.Path == "/api/v3/core/groups/g1/remove_user/":
			writeJSON(t, w, 204, nil)
		case r.URL.Path == "/api/v3/core/groups/g1/members/":
			writeJSON(t, w, 200, paginatedResponse{
				Results: []json.RawMessage{mustJSON(t, GroupMember{UUID: "u1"}), mustJSON(t, GroupMember{UUID: "u2"})},
			})
		default:
			writeJSON(t, w, 404, nil)
		}
	})
	ctx := context.Background()

	g, err := c.Groups().Create(ctx, CreateGroupRequest{Name: "admins", Tenant: "t1"})
	if err != nil || g.UUID != "g1" {
		t.Fatalf("Create() = %+v, err = %v", g, err)
	}
	if got, err := c.Groups().GetByID(ctx, "g1"); err != nil || got.Name != "admins" {
		t.Fatalf("GetByID() = %+v, err = %v", got, err)
	}
	if upd, err := c.Groups().Update(ctx, "g1", "renamed"); err != nil || upd.Name != "renamed" {
		t.Fatalf("Update() = %+v, err = %v", upd, err)
	}
	if err := c.Groups().AddUser(ctx, "g1", "u1"); err != nil {
		t.Fatalf("AddUser() error = %v", err)
	}
	if err := c.Groups().RemoveUser(ctx, "g1", "u1"); err != nil {
		t.Fatalf("RemoveUser() error = %v", err)
	}
	members, err := c.Groups().GetMembers(ctx, "g1")
	if err != nil || len(members) != 2 {
		t.Fatalf("GetMembers() = %v, err = %v", members, err)
	}
	if err := c.Groups().Delete(ctx, "g1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func mustJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
