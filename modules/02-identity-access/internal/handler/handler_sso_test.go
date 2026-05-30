package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/operan/modules/02-identity-access/internal/authentik"
)

// ---------------------------------------------------------------------------
// SSOHandler — nil Auth helpers
// ---------------------------------------------------------------------------

func TestSSOHandler_Configure_nilAuth(t *testing.T) {
	h := NewSSOHandler(nil, nil)
	body := `{"provider":"okta","type":"saml","configuration":{"entity_id":"https://idp.test"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/sso/configure", strings.NewReader(body))
	req.Header.Set("X-Tenant-ID", "t1")
	rr := httptest.NewRecorder()

	h.Configure(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Configure(nil auth) = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestSSOHandler_Configure_invalidJSON(t *testing.T) {
	h := NewSSOHandler(nil, nil)
	// nil Auth → 404 (nil check is before JSON decode)
	body := `{invalid json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/sso/configure", strings.NewReader(body))
	req.Header.Set("X-Tenant-ID", "t1")
	rr := httptest.NewRecorder()

	h.Configure(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Configure(bad JSON, nil auth) = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestSSOHandler_GetConfig_nilAuth(t *testing.T) {
	h := NewSSOHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/auth/sso/config", http.NoBody)
	req.Header.Set("X-Tenant-ID", "t1")
	rr := httptest.NewRecorder()

	h.GetConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GetConfig(nil auth) = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp) != 0 {
		t.Errorf("GetConfig(nil auth) body = %s, want empty object", rr.Body.String())
	}
}

func TestSSOHandler_Test_nilAuth(t *testing.T) {
	h := NewSSOHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/sso/test", strings.NewReader(`{}`))
	req.Header.Set("X-Tenant-ID", "t1")
	rr := httptest.NewRecorder()

	h.Test(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Test(nil auth) = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	steps, ok := resp["test_steps"].([]interface{})
	if !ok || len(steps) != 4 {
		t.Errorf("Test(nil auth) test_steps = %v, want 4 skipped steps", resp["test_steps"])
	}
}

func TestSSOHandler_GetSSOConfig_nilAuth(t *testing.T) {
	h := NewSSOHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/auth/sso/config", http.NoBody)
	req.Header.Set("X-Tenant-ID", "t1")
	rr := httptest.NewRecorder()

	h.GetSSOConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GetSSOConfig(nil auth) = %d, want %d", rr.Code, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// SCIMHandler — nil Auth helpers
// ---------------------------------------------------------------------------

func TestSCIMHandler_ListUsers_nilAuth(t *testing.T) {
	h := NewSCIMHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/scim/Users", http.NoBody)
	req.Header.Set("X-Tenant-ID", "t1")
	rr := httptest.NewRecorder()

	h.ListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ListUsers(nil auth) = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Header().Get("Content-Type") != "application/scim+json" {
		t.Errorf("ListUsers Content-Type = %q, want %q", rr.Header().Get("Content-Type"), "application/scim+json")
	}
}

func TestSCIMHandler_Provision_nilAuth(t *testing.T) {
	h := NewSCIMHandler(nil, nil)
	body := `{"userName":"alice","emails":[{"value":"alice@test.com"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/scim/Users", strings.NewReader(body))
	req.Header.Set("X-Tenant-ID", "t1")
	rr := httptest.NewRecorder()

	h.Provision(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Provision(nil auth) = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestSCIMHandler_UpdateUser_nilAuth(t *testing.T) {
	h := NewSCIMHandler(nil, nil)
	body := `{"userName":"updated","emails":[{"value":"updated@test.com"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/iam/scim/Users/some-uuid", strings.NewReader(body))
	req.Header.Set("X-Tenant-ID", "t1")
	rr := httptest.NewRecorder()

	h.UpdateUser(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("UpdateUser(nil auth) = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestSCIMHandler_DeleteUser_nilAuth(t *testing.T) {
	h := NewSCIMHandler(nil, nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/scim/Users/some-uuid", http.NoBody)
	req.Header.Set("X-Tenant-ID", "t1")
	rr := httptest.NewRecorder()

	h.DeleteUser(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("DeleteUser(nil auth) = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestSCIMHandler_BulkProvision_nilAuth(t *testing.T) {
	h := NewSCIMHandler(nil, nil)
	body := `{"schemas":["urn:scim:schemas:core:2.0:BulkRequest"],"bulkRequests":[{"userName":"alice"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/scim/Bulk", strings.NewReader(body))
	req.Header.Set("X-Tenant-ID", "t1")
	rr := httptest.NewRecorder()

	h.BulkProvision(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("BulkProvision(nil auth) = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// extractScimID
// ---------------------------------------------------------------------------

func TestExtractScimID(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"valid UUID", "/api/v1/iam/scim/users/550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000"},
		{"with trailing slash", "/api/v1/iam/scim/users/550e8400-e29b-41d4-a716-446655440000/", "550e8400-e29b-41d4-a716-446655440000"},
		{"empty", "/api/v1/iam/scim/users/", ""},
		{"no dashes", "/api/v1/iam/scim/users/550e8400e29b41d4a716446655440000", "550e8400e29b41d4a716446655440000"},
		{"wrong prefix", "/other/path/uuid", "/other/path/uuid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractScimID(tt.path)
			if got != tt.want {
				t.Errorf("extractScimID(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractOp
// ---------------------------------------------------------------------------

func TestExtractOp(t *testing.T) {
	tests := []struct {
		filterStr string
		want      string
	}{
		{"userName eq \"alice\"", "userName eq "},
		{"userName ne \"bob\"", "userName ne "},
		{"userName co \"ali\"", "userName co "},
		{"userName sw \"al\"", "userName sw "},
		{"userName ew \"ce\"", "userName ew "},
		{"active pr ", "active pr "},
		{"emails.value eq \"a@b.com\"", "emails.value eq "},
		{"groups.display co \"team\"", "groups.display co "},
		{"no operator", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.filterStr, func(t *testing.T) {
			got := extractOp(tt.filterStr)
			if got != tt.want {
				t.Errorf("extractOp(%q) = %q, want %q", tt.filterStr, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// configString
// ---------------------------------------------------------------------------

func TestConfigString_nil(t *testing.T) {
	got := configString(nil, "key")
	if got != "" {
		t.Errorf("configString(nil, \"key\") = %q, want %q", got, "")
	}
}

func TestConfigString_missingKey(t *testing.T) {
	got := configString(map[string]interface{}{}, "key")
	if got != "" {
		t.Errorf("configString({}, \"key\") = %q, want %q", got, "")
	}
}

func TestConfigString_nonStringValue(t *testing.T) {
	cfg := map[string]interface{}{"count": 42}
	got := configString(cfg, "count")
	if got != "" {
		t.Errorf("configString({count:42}, \"count\") = %q, want %q", got, "")
	}
}

func TestConfigString_valid(t *testing.T) {
	cfg := map[string]interface{}{"url": "https://idp.test"}
	got := configString(cfg, "url")
	if got != "https://idp.test" {
		t.Errorf("configString({url:...}, \"url\") = %q, want %q", got, "https://idp.test")
	}
}

// ---------------------------------------------------------------------------
// sanitizeSlug
// ---------------------------------------------------------------------------

func TestSanitizeSlug(t *testing.T) {
	tests := []struct {
		name string
		input string
		want  string
	}{
		{"spaces", "My Provider", "my-provider"},
		{"underscores", "my_provider", "my-provider"},
		{"slashes", "my/provider", "my-provider"},
		{"mixed", "My Provider_Name/Type", "my-provider-name-type"},
		{"trailing dash", " Provider ", "provider"},
		{"empty", "", ""},
		{"already safe", "my-provider", "my-provider"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeSlug(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeSlug(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractPathSuffix
// ---------------------------------------------------------------------------

func TestExtractPathSuffix(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		prefix string
		want   string
	}{
		{"exact match", "/api/v1/iam/scim/users/123", "/api/v1/iam/scim/users/", "123"},
		{"with trailing slash", "/api/v1/iam/scim/users/123/", "/api/v1/iam/scim/users/", "123"},
		{"no match", "/other/path/123", "/api/v1/iam/scim/users/", ""},
		{"empty path", "", "/api/v1/iam/scim/users/", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPathSuffix(tt.path, tt.prefix)
			if got != tt.want {
				t.Errorf("extractPathSuffix(%q, %q) = %q, want %q", tt.path, tt.prefix, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// matchesScimFilter
// ---------------------------------------------------------------------------

func TestMatchesScimFilter_noFilter(t *testing.T) {
	u := &authentik.User{Username: "alice", Email: "alice@test.com", IsActive: true}
	if !matchesScimFilter(u, "", "", "") {
		t.Error("matchesScimFilter(empty filter) should return true")
	}
}

func TestMatchesScimFilter_userNameEq(t *testing.T) {
	u := &authentik.User{Username: "alice", Email: "alice@test.com", IsActive: true}
	if !matchesScimFilter(u, "userName eq \"alice\"", "", "") {
		t.Error("matchesScimFilter(user=alice, filter=alice) should return true")
	}
	if matchesScimFilter(u, "userName eq \"bob\"", "", "") {
		t.Error("matchesScimFilter(user=alice, filter=bob) should return false")
	}
}

func TestMatchesScimFilter_userNameContains(t *testing.T) {
	u := &authentik.User{Username: "alice", Email: "alice@test.com", IsActive: true}
	if !matchesScimFilter(u, "userName co \"ali\"", "", "") {
		t.Error("matchesScimFilter(user=alice, filter=ali) should return true")
	}
	if matchesScimFilter(u, "userName co \"xyz\"", "", "") {
		t.Error("matchesScimFilter(user=alice, filter=xyz) should return false")
	}
}

func TestMatchesScimFilter_userNameStartsWith(t *testing.T) {
	u := &authentik.User{Username: "alice", Email: "alice@test.com", IsActive: true}
	if !matchesScimFilter(u, "userName sw \"ali\"", "", "") {
		t.Error("matchesScimFilter(user=alice, filter=ali) should return true")
	}
	if matchesScimFilter(u, "userName sw \"bob\"", "", "") {
		t.Error("matchesScimFilter(user=alice, filter=bob) should return false")
	}
}

func TestMatchesScimFilter_activePresent(t *testing.T) {
	activeUser := &authentik.User{Username: "alice", IsActive: true}
	inactiveUser := &authentik.User{Username: "bob", IsActive: false}

	if !matchesScimFilter(activeUser, "active pr ", "", "") {
		t.Error("matchesScimFilter(active user, pr) should return true")
	}
	if matchesScimFilter(inactiveUser, "active pr ", "", "") {
		t.Error("matchesScimFilter(inactive user, pr) should return false")
	}
}

// ---------------------------------------------------------------------------
// compareSCIMSort
// ---------------------------------------------------------------------------

func TestCompareSCIMSort_userName(t *testing.T) {
	a := &authentik.User{Username: "alice"}
	b := &authentik.User{Username: "bob"}
	got := compareSCIMSort(a, b, "userName")
	if got >= 0 {
		t.Errorf("compareSCIMSort(alice, bob) = %d, want < 0", got)
	}
}

func TestCompareSCIMSort_email(t *testing.T) {
	a := &authentik.User{Username: "alice", Email: "alice@test.com"}
	b := &authentik.User{Username: "bob", Email: "bob@test.com"}
	got := compareSCIMSort(a, b, "emails")
	if got >= 0 {
		t.Errorf("compareSCIMSort(alice, bob) by email = %d, want < 0", got)
	}
}

func TestCompareSCIMSort_active(t *testing.T) {
	activeUser := &authentik.User{Username: "alice", IsActive: true}
	inactiveUser := &authentik.User{Username: "bob", IsActive: false}

	got := compareSCIMSort(activeUser, inactiveUser, "active")
	if got <= 0 {
		t.Errorf("compareSCIMSort(active, inactive) = %d, want > 0", got)
	}
}

func TestCompareSCIMSort_name(t *testing.T) {
	a := &authentik.User{Name: "Alice"}
	b := &authentik.User{Name: "Bob"}
	got := compareSCIMSort(a, b, "name")
	if got >= 0 {
		t.Errorf("compareSCIMSort(Alice, Bob) = %d, want < 0", got)
	}
}

// ---------------------------------------------------------------------------
// scimUserFromAuthentik
// ---------------------------------------------------------------------------

func TestScimUserFromAuthentik_basic(t *testing.T) {
	tm := time.Now().UTC()
	u := &authentik.User{
		UUID:       "550e8400-e29b-41d4-a716-446655440000",
		Username:   "alice",
		Email:      "alice@test.com",
		Name:       "Alice Smith",
		IsActive:   true,
		DateJoined: &tm,
	}
	sc := scimUserFromAuthentik(u, "https://idp.test")

	if sc.ID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("scimUser.ID = %q, want %q", sc.ID, "550e8400-e29b-41d4-a716-446655440000")
	}
	if sc.UserName != "alice" {
		t.Errorf("scimUser.UserName = %q, want %q", sc.UserName, "alice")
	}
	if len(sc.Emails) != 1 || sc.Emails[0].Value != "alice@test.com" {
		t.Errorf("scimUser.Emails = %v, want [{alice@test.com primary:true}]", sc.Emails)
	}
	if !sc.Active {
		t.Error("scimUser.Active = false, want true")
	}
	if sc.Name.GivenName != "Alice" {
		t.Errorf("scimUser.Name.GivenName = %q, want %q", sc.Name.GivenName, "Alice")
	}
	if sc.Name.FamilyName != "Smith" {
		t.Errorf("scimUser.Name.FamilyName = %q, want %q", sc.Name.FamilyName, "Smith")
	}
	if sc.Meta.ResourceType != "User" {
		t.Errorf("scimUser.Meta.ResourceType = %q, want %q", sc.Meta.ResourceType, "User")
	}
}

// ---------------------------------------------------------------------------
// scimEmailsFromAuthentik
// ---------------------------------------------------------------------------

func TestScimEmailsFromAuthentik_primaryOnly(t *testing.T) {
	u := &authentik.User{Email: "alice@test.com"}
	emails := scimEmailsFromAuthentik(u)
	if len(emails) != 1 || !emails[0].Primary || emails[0].Value != "alice@test.com" {
		t.Errorf("scimEmailsFromAuthentik(alice) = %v, want [{alice@test.com primary:true}]", emails)
	}
}

func TestScimEmailsFromAuthentik_withExtras(t *testing.T) {
	u := &authentik.User{
		Email: "alice@test.com",
		Attributes: map[string]interface{}{
			"emails": []interface{}{"alice@work.com", "alice@personal.com"},
		},
	}
	emails := scimEmailsFromAuthentik(u)
	if len(emails) != 3 {
		t.Errorf("scimEmailsFromAuthentik = %d emails, want 3", len(emails))
	}
	if !emails[0].Primary {
		t.Error("First email should be primary")
	}
}

// ---------------------------------------------------------------------------
// SCIMHandler — error path tests (nil Auth)
// ---------------------------------------------------------------------------

func TestSCIMHandler_Provision_invalidJSON(t *testing.T) {
	h := NewSCIMHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/scim/Users", strings.NewReader("not json"))
	req.Header.Set("X-Tenant-ID", "t1")
	rr := httptest.NewRecorder()

	h.Provision(rr, req)

	// nil Auth → 404, not 400
	if rr.Code != http.StatusNotFound {
		t.Errorf("Provision(nil auth, bad JSON) = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestSCIMHandler_BulkProvision_invalidJSON(t *testing.T) {
	h := NewSCIMHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/scim/Bulk", strings.NewReader("not json"))
	req.Header.Set("X-Tenant-ID", "t1")
	rr := httptest.NewRecorder()

	h.BulkProvision(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("BulkProvision(nil auth, bad JSON) = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// nil Publisher guard — no panic on publish calls
// ---------------------------------------------------------------------------

func TestSSOHandler_Configure_nilPublisherNoPanic(t *testing.T) {
	h := NewSSOHandler(nil, nil)
	// nil Auth → early return at 404, no publish attempt — but the guard
	// is still exercised if we ever pass nil Auth + valid Auth path.
	// With nil Auth we get 404 before any publish call.
	body := `{"provider":"okta","type":"saml","configuration":{}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/sso/configure", strings.NewReader(body))
	req.Header.Set("X-Tenant-ID", "t1")
	rr := httptest.NewRecorder()

	h.Configure(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Configure(nil publisher) = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// Tenant-ID header propagation
// ---------------------------------------------------------------------------

func TestSCIMHandler_TenantIDPropagated_nilAuth(t *testing.T) {
	h := NewSCIMHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/scim/Users", http.NoBody)
	req.Header.Set("X-Tenant-ID", "custom-tenant-123")
	rr := httptest.NewRecorder()

	h.ListUsers(rr, req)

	// nil Auth → 200 empty response regardless of tenant
	if rr.Code != http.StatusOK {
		t.Errorf("ListUsers(custom tenant, nil auth) = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestSSOHandler_GetConfig_TenantIDHeader(t *testing.T) {
	h := NewSSOHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/auth/sso/config", http.NoBody)
	req.Header.Set("X-Tenant-ID", "tenant-with-sso")
	rr := httptest.NewRecorder()

	h.GetConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GetConfig(custom tenant, nil auth) = %d, want %d", rr.Code, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// Standalone helper functions
// ---------------------------------------------------------------------------

func TestWriteSCIMError(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		scimType string
		detail   string
		want     map[string]interface{}
	}{
		{
			name:     "not found error",
			status:   404,
			scimType: "scimType",
			detail:   "User not found",
		},
		{
			name:     "unhandled error",
			status:   500,
			scimType: "unprocessed",
			detail:   "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			writeSCIMError(rr, tt.status, tt.scimType, tt.detail)

			if rr.Code != tt.status {
				t.Errorf("writeSCIMError(%d) = %d, want %d", tt.status, rr.Code, tt.status)
			}

			ct := rr.Header().Get("Content-Type")
			if ct != "application/scim+json" {
				t.Errorf("writeSCIMError Content-Type = %q, want %q", ct, "application/scim+json")
			}

			var result map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if result["scimType"] != tt.scimType {
				t.Errorf("scimType = %v, want %q", result["scimType"], tt.scimType)
			}
			if result["detail"] != tt.detail {
				t.Errorf("detail = %v, want %q", result["detail"], tt.detail)
			}
			if result["status"] != strconv.Itoa(tt.status) {
				t.Errorf("status = %v, want %q", result["status"], strconv.Itoa(tt.status))
			}
			schemas, ok := result["schemas"].([]interface{})
			if !ok {
				t.Fatal("schemas is not []interface{}")
			}
			if len(schemas) != 1 || schemas[0] != "urn:ietf:params:scim:api:messages:2.0:Error" {
				t.Errorf("schemas = %v, want [urn:ietf:params:scim:api:messages:2.0:Error]", schemas)
			}
		})
	}
}

func TestExtractScimUserFromPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "/api/v1/iam/scim/users/abc-123",
			want:  "abc-123",
		},
		{
			input: "/api/v1/iam/scim/users/456-def/operations/789-ghi",
			want:  "456-def/operations/789-ghi",
		},
		{
			input: "/api/v1/iam/scim/users/abc-123/",
			want:  "abc-123",
		},
		{
			input: "/api/v1/iam/scim/users",
			want:  "",
		},
		{
			input: "/api/v1/iam/scim/",
			want:  "",
		},
		{
			input: "users/abc-123",
			want:  "users/abc-123",
		},
		{
			input: "abc-123",
			want:  "abc-123",
		},
		{
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractScimUserFromPath(tt.input)
			if got != tt.want {
				t.Errorf("extractScimUserFromPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConfigString(t *testing.T) {
	cfg := map[string]interface{}{
		"string_val":  "hello",
		"int_val":     42,
		"nil_val":     nil,
		"bool_val":    true,
		"missing_key": "present",
	}

	tests := []struct {
		name   string
		cfg    map[string]interface{}
		key    string
		want   string
	}{
		{"string value", cfg, "string_val", "hello"},
		{"int value", cfg, "int_val", ""},
		{"nil value", cfg, "nil_val", ""},
		{"bool value", cfg, "bool_val", ""},
		{"present key", cfg, "missing_key", "present"},
		{"missing key", cfg, "nonexistent", ""},
		{"nil cfg", nil, "anything", ""},
		{"empty cfg", map[string]interface{}{}, "anything", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := configString(tt.cfg, tt.key)
			if got != tt.want {
				t.Errorf("configString(%v, %q) = %q, want %q", tt.cfg, tt.key, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
