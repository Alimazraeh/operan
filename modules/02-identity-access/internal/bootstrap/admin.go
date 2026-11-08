// Package bootstrap provisions a tenant's first administrator so that real
// per-user sign-in works on a fresh deployment.
//
// Why this exists: M02 has had per-user login since V007 — bcrypt hash in
// users.password_hash, POST /api/v1/iam/auth/login, and the portal already
// calls it whenever the email field is filled. But users are per-tenant and
// nothing created one when a tenant was provisioned, so every new workspace
// had no account to sign in with and fell back to the shared admin bootstrap.
// That is the whole reason signing in looked like a choice between convenient
// and secure.
package bootstrap

import (
	"errors"
	"fmt"
	"strings"

	"github.com/operan/modules/02-identity-access/internal/auth"
	"github.com/operan/modules/02-identity-access/internal/models"
	"github.com/operan/modules/02-identity-access/internal/store"
)

// AdminRole is the role the bootstrap administrator is granted.
//
// It must be a role the rest of the platform actually knows. The portal derives
// what a person may see from a fixed vocabulary — platform_admin, executive,
// department_head, supervisor, employee (web/app/js/perm.js ROLE_PERMISSIONS) —
// and silently grants nothing for a role outside it. Seeding "admin" instead of
// "platform_admin" therefore produced an administrator who could sign in and
// then saw a navigation stripped to the four ungated views, with no error
// anywhere to say why.
const AdminRole = "platform_admin"

// Outcome says what EnsureAdminUser did, so the caller can log it honestly.
type Outcome int

const (
	// AlreadyPresent means an account with that email already exists in the
	// tenant. Its credential is left untouched.
	AlreadyPresent Outcome = iota
	// Created means the account was created and its password set.
	Created
)

// ErrNoPasswordSetter is returned when the store cannot hold a credential, so
// the account would exist but never be able to log in.
var ErrNoPasswordSetter = errors.New("user store cannot set a password")

// The two user stores spell the same capability differently — the persistent
// one has SetPassword, the in-memory one SetPasswordHash — so accept either
// rather than quietly failing against one of them.
type passwordSetter interface {
	SetPassword(id, hash string) error
}

type passwordHashSetter interface {
	SetPasswordHash(id, hash string) error
}

// setCredential stores hash against the user by whichever setter the store has.
func setCredential(users store.UserStoreAPI, userID, hash string) error {
	switch s := users.(type) {
	case passwordSetter:
		return s.SetPassword(userID, hash)
	case passwordHashSetter:
		return s.SetPasswordHash(userID, hash)
	default:
		return ErrNoPasswordSetter
	}
}

// EnsureAdminUser makes sure tenantID has an administrator with this email and
// password, and reports which it did.
//
// It is deliberately idempotent in the conservative direction: if the account
// already exists it is left exactly as it is. Re-asserting the configured
// password on every boot would silently undo a rotation and quietly re-enable
// an account someone had deactivated.
func EnsureAdminUser(users store.UserStoreAPI, tenantID, email, password string) (Outcome, error) {
	tenantID = strings.TrimSpace(tenantID)
	email = strings.ToLower(strings.TrimSpace(email))
	if tenantID == "" || email == "" || password == "" {
		return AlreadyPresent, errors.New("tenant, email and password are all required")
	}

	if existing, err := users.GetByTenantAndEmail(tenantID, email); err == nil && existing != nil {
		return AlreadyPresent, nil
	}

	// Hash before creating anything: a rejected password should leave no
	// half-provisioned account behind.
	hash, err := auth.HashPassword(password)
	if err != nil {
		return AlreadyPresent, fmt.Errorf("bootstrap admin password: %w", err)
	}

	user := &models.User{
		TenantID:    tenantID,
		Email:       email,
		DisplayName: "Platform Administrator",
		RoleIDs:     []string{AdminRole},
	}
	if err := users.Create(user); err != nil {
		return AlreadyPresent, fmt.Errorf("create bootstrap admin: %w", err)
	}

	// Create leaves a new account "pending"; setting the credential promotes it
	// to active, which is what makes it able to sign in.
	if err := setCredential(users, user.ID, hash); err != nil {
		return AlreadyPresent, fmt.Errorf("set bootstrap admin password: %w", err)
	}

	// Roles are stored separately from the row Create writes.
	if err := users.SetRoles(user.ID, []string{AdminRole}); err != nil {
		return Created, fmt.Errorf("grant bootstrap admin role: %w", err)
	}

	return Created, nil
}
