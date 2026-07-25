package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// UserStore is a persistent user store backed by PostgreSQL.
type UserStore struct {
	pool *pgxpool.Pool
}

// NewUserStore creates a new persistent user store.
func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

// Create persists a new user to the database.
func (s *UserStore) Create(ctx context.Context, user *models.User) error {
	if user.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if user.Email == "" {
		return fmt.Errorf("email is required")
	}

	rolesJSON, _ := json.Marshal(user.RoleIDs)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, tenant_id, email, display_name, status, authentication_method, mfa_enabled, role_ids, roles_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, email) DO NOTHING
	`, user.ID, user.TenantID, user.Email, user.DisplayName, user.Status,
		user.AuthenticationMethod, user.MFAEnabled, rolesJSON, user.RolesJSON)

	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetByID retrieves a user by ID.
func (s *UserStore) GetByID(ctx context.Context, id string) (*models.User, error) {
	var user models.User
	var rolesJSON string

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, email, display_name, status, authentication_method, mfa_enabled, role_ids, roles_json, created_at, updated_at, last_login_at
		FROM users WHERE id = $1
	`, id).Scan(
		&user.ID, &user.TenantID, &user.Email, &user.DisplayName, &user.Status,
		&user.AuthenticationMethod, &user.MFAEnabled, &rolesJSON, &user.RolesJSON,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by ID: %w", err)
	}

	// Parse roles JSON
	if err := json.Unmarshal([]byte(rolesJSON), &user.RoleIDs); err != nil {
		return nil, fmt.Errorf("parse roles JSON: %w", err)
	}

	return &user, nil
}

// GetByTenantAndEmail retrieves a user by tenant and email.
func (s *UserStore) GetByTenantAndEmail(ctx context.Context, tenantID, email string) (*models.User, error) {
	var user models.User
	var rolesJSON string

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, email, display_name, status, authentication_method, mfa_enabled, role_ids, roles_json, created_at, updated_at, last_login_at
		FROM users WHERE tenant_id = $1 AND email = $2
	`, tenantID, email).Scan(
		&user.ID, &user.TenantID, &user.Email, &user.DisplayName, &user.Status,
		&user.AuthenticationMethod, &user.MFAEnabled, &rolesJSON, &rolesJSON,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by tenant/email: %w", err)
	}

	if err := json.Unmarshal([]byte(rolesJSON), &user.RoleIDs); err != nil {
		return nil, fmt.Errorf("parse roles JSON: %w", err)
	}

	return &user, nil
}

// List returns paginated users for a tenant.
func (s *UserStore) List(ctx context.Context, tenantID string, page, pageSize int) ([]models.User, int, error) {
	offset := (page - 1) * pageSize

	var users []models.User
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, email, display_name, status, authentication_method, mfa_enabled, role_ids, roles_json, created_at, updated_at, last_login_at
		FROM users WHERE tenant_id = $1 ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, tenantID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var user models.User
		var rolesJSON string
		if err := rows.Scan(
			&user.ID, &user.TenantID, &user.Email, &user.DisplayName, &user.Status,
			&user.AuthenticationMethod, &user.MFAEnabled, &rolesJSON, &user.RolesJSON,
			&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		if err := json.Unmarshal([]byte(rolesJSON), &user.RoleIDs); err != nil {
			return nil, 0, fmt.Errorf("parse roles: %w", err)
		}
		users = append(users, user)
	}

	// Get total count
	var total int
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE tenant_id = $1", tenantID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	return users, total, nil
}

// Update updates a user's fields.
func (s *UserStore) Update(ctx context.Context, id string, updates *models.UpdateUserRequest) (*models.User, error) {
	var user models.User

	query := `UPDATE users SET`
	args := []interface{}{}
	argIdx := 2

	if updates.DisplayName != nil {
		query += fmt.Sprintf(" display_name=$%d,", argIdx)
		args = append(args, *updates.DisplayName)
		argIdx++
	}
	if updates.Status != nil {
		query += fmt.Sprintf(" status=$%d,", argIdx)
		args = append(args, *updates.Status)
		argIdx++
	}

	query += fmt.Sprintf(" updated_at=NOW() WHERE id=$1 RETURNING id, tenant_id, email, display_name, status, authentication_method, mfa_enabled, role_ids, roles_json, created_at, updated_at, last_login_at")
	args = append([]interface{}{id}, args...)

	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&user.ID, &user.TenantID, &user.Email, &user.DisplayName, &user.Status,
		&user.AuthenticationMethod, &user.MFAEnabled, &user.RolesJSON, &user.RolesJSON,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	return &user, nil
}

// Deactivate soft-deletes a user.
func (s *UserStore) Deactivate(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET status='deactivated', updated_at=NOW() WHERE id=$1
	`, id)
	if err != nil {
		return fmt.Errorf("deactivate user: %w", err)
	}
	return nil
}

// SetRoles updates a user's roles.
func (s *UserStore) SetRoles(ctx context.Context, id string, roleIDs []string) error {
	rolesJSON, _ := json.Marshal(roleIDs)
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET role_ids=$1, roles_json=$1, updated_at=NOW() WHERE id=$2
	`, rolesJSON, id)
	if err != nil {
		return fmt.Errorf("set user roles: %w", err)
	}
	return nil
}

// GetByActorID is an alias for GetByTenantAndEmail (uses tenant_id + email).
func (s *UserStore) GetByActorID(ctx context.Context, tenantID, actorID string) (*models.User, error) {
	return s.GetByTenantAndEmail(ctx, tenantID, actorID)
}

// CountTotal returns the total number of users in the database.
func (s *UserStore) CountTotal(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}
// SetPassword stores a bcrypt hash for a user and stamps when it changed.
func (s *UserStore) SetPassword(ctx context.Context, userID, hash string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE users SET password_hash = $2, password_set_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, userID, hash)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("set password: no such user %s", userID)
	}
	return nil
}

// LoadAll returns every user across all tenants, including password hashes.
// It exists so the process can rehydrate its in-memory view at boot: without
// it, rows written to Postgres are invisible after a restart because every
// read is served from memory.
func (s *UserStore) LoadAll(ctx context.Context) ([]models.User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, email, display_name, status, authentication_method,
		       mfa_enabled, roles_json, created_at, updated_at, last_login_at,
		       COALESCE(password_hash, ''), password_set_at
		FROM users ORDER BY created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("load users: %w", err)
	}
	defer rows.Close()

	var out []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.TenantID, &u.Email, &u.DisplayName, &u.Status,
			&u.AuthenticationMethod, &u.MFAEnabled, &u.RolesJSON,
			&u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt,
			&u.PasswordHash, &u.PasswordSetAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		if u.RolesJSON != "" {
			_ = json.Unmarshal([]byte(u.RolesJSON), &u.RoleIDs)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
