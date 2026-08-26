package apiclient

import (
	"context"
	"fmt"
	"net/http"
)

// IAMClient talks to Module 02 (identity-access). Base path confirmed as
// literal "/api/v1/iam" (modules/02-identity-access/cmd/identity-access/main.go:87-165).
// The admin-login and per-user-login routes bypass the JWT/tenant
// middleware chain entirely (main.go:704-714) — every other route requires
// both an Authorization: Bearer token and an X-Tenant-ID header.
type IAMClient struct {
	BaseURL string // e.g. http://identity-access.operan.svc.cluster.local:8002
	Doer    *Doer
}

// AdminLoginResponse mirrors handler_admin_login.go's AdminLoginResponse.
// UserID is always "admin-001" and Email always "admin@operan" — the admin
// login is a shared bootstrap credential, not a per-user account.
type AdminLoginResponse struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// AdminLogin calls POST /api/v1/iam/admin/login. This is the bootstrap
// credential restore uses to create the tenant, users, agents and
// department — every one of those calls needs an "admin" role, and there is
// no per-user account with that role until this login mints one.
//
// adminPassword is never logged or embedded in this tool; it is supplied by
// the caller (flag or environment variable, resolved in cmd/demo-fixture).
func (c *IAMClient) AdminLogin(ctx context.Context, adminPassword, tenant string) (*AdminLoginResponse, error) {
	var out AdminLoginResponse
	req := map[string]string{"password": adminPassword, "tenant": tenant}
	_, err := c.Doer.Call(ctx, http.MethodPost, c.BaseURL+"/api/v1/iam/admin/login", "", "", req, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// LoginResponse mirrors the raw map handler_auth_login.go's Login handler
// writes (handler_auth_login.go:81-89).
type LoginResponse struct {
	Token       string   `json:"token"`
	UserID      string   `json:"user_id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	Tenant      string   `json:"tenant"`
	Roles       []string `json:"roles"`
	ExpiresAt   string   `json:"expires_at"`
}

// Login calls POST /api/v1/iam/auth/login — real per-user login, used by
// the replay step to act (and be attributed) as the fixture user who is
// expected to approve the demonstration request's gate.
func (c *IAMClient) Login(ctx context.Context, email, password, tenant string) (*LoginResponse, error) {
	var out LoginResponse
	req := map[string]string{"email": email, "password": password, "tenant": tenant}
	_, err := c.Doer.Call(ctx, http.MethodPost, c.BaseURL+"/api/v1/iam/auth/login", "", "", req, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateUserRequest mirrors models.CreateUserRequest
// (internal/models/models.go:38-44). Note there is no password field here —
// M02 has no such field on user creation; a user is created "pending" and
// only becomes usable after a separate SetPassword call.
type CreateUserRequest struct {
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	RoleIDs     []string `json:"role_ids,omitempty"`
	MFAEnabled  *bool    `json:"mfa_enabled,omitempty"`
}

// User mirrors models.User (internal/models/models.go:17-35) — only the
// fields this tool reads. PasswordHash is json:"-" on the server side and
// never appears on the wire, so it has no field here at all.
type User struct {
	ID          string   `json:"id"`
	TenantID    string   `json:"tenant_id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	Status      string   `json:"status"`
	RoleIDs     []string `json:"role_ids"`
}

// CreateUser calls POST /api/v1/iam/users. The local in-memory UserStore
// backing this handler does not check email uniqueness — only ID
// uniqueness, and the handler always mints a fresh ID on this path
// (internal/store/user.go:31-72) — so calling this twice with the same
// email creates two users. FindOrCreateUser in restorecmd guards against
// that by listing and matching on email first; this method is the raw,
// non-idempotent primitive.
func (c *IAMClient) CreateUser(ctx context.Context, token, tenantID string, req CreateUserRequest) (*User, error) {
	var out User
	_, err := c.Doer.Call(ctx, http.MethodPost, c.BaseURL+"/api/v1/iam/users", token, tenantID, req, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type userListResponse struct {
	Users      []*User `json:"users"`
	Total      int     `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
	TotalPages int     `json:"total_pages"`
}

// ListUsers calls GET /api/v1/iam/users?page=&page_size=.
func (c *IAMClient) ListUsers(ctx context.Context, token, tenantID string, page, pageSize int) ([]*User, int, error) {
	url := fmt.Sprintf("%s/api/v1/iam/users?page=%d&page_size=%d", c.BaseURL, page, pageSize)
	var out userListResponse
	_, err := c.Doer.Call(ctx, http.MethodGet, url, token, tenantID, nil, &out)
	if err != nil {
		return nil, 0, err
	}
	return out.Users, out.Total, nil
}

// FindUserByEmail pages through every user in the tenant looking for an
// exact email match — see CreateUser's doc comment for why the server
// cannot be trusted to reject a duplicate itself.
func (c *IAMClient) FindUserByEmail(ctx context.Context, token, tenantID, email string) (*User, error) {
	const pageSize = 50
	const maxPages = 200
	for page := 1; page <= maxPages; page++ {
		items, total, err := c.ListUsers(ctx, token, tenantID, page, pageSize)
		if err != nil {
			return nil, err
		}
		for _, u := range items {
			if u.Email == email {
				return u, nil
			}
		}
		if page*pageSize >= total || len(items) == 0 {
			break
		}
	}
	return nil, nil
}

// SetPassword calls POST /api/v1/iam/users/{id}/password. This is naturally
// idempotent — setting the same password twice leaves the same state — so
// restore calls it unconditionally whenever a password was supplied,
// whether the user was just created or already existed.
func (c *IAMClient) SetPassword(ctx context.Context, token, tenantID, userID, password string) error {
	req := map[string]string{"password": password}
	_, err := c.Doer.Call(ctx, http.MethodPost, c.BaseURL+"/api/v1/iam/users/"+userID+"/password", token, tenantID, req, nil)
	return err
}
