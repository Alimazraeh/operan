package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PgxPool is the interface wrapping pgxpool.Pool for testability.
type PgxPool interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Close()
}

var ErrNotFound = errors.New("record not found")

// ProfileStore handles sandbox profile CRUD operations.
type ProfileStore struct {
	pool PgxPool
}

// NewProfileStore creates a new ProfileStore.
func NewProfileStore(pool PgxPool) *ProfileStore {
	return &ProfileStore{pool: pool}
}

// Create inserts a new sandbox profile.
func (s *ProfileStore) Create(ctx context.Context, p *SandboxProfile) error {
	query := `
		INSERT INTO sandbox_profiles (tenant_id, name, description, cpu_cores, memory_mb,
			timeout_seconds, network_access, allowed_tools, filesystem_access,
			max_file_size_mb, max_output_size_kb, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at`

	var id uuid.UUID
	var createdAt, updatedAt time.Time
	err := s.pool.QueryRow(ctx, query,
		p.TenantID, p.Name, p.Description, p.CPUCores, p.MemoryMB,
		p.TimeoutSeconds, p.NetworkAccess, p.AllowedTools, p.FilesystemAccess,
		p.MaxFileSizeMB, p.MaxOutputSizeKB, p.IsActive,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return err
	}
	p.ID = id
	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt
	return nil
}

// GetByID retrieves a profile by ID.
func (s *ProfileStore) GetByID(ctx context.Context, id uuid.UUID) (*SandboxProfile, error) {
	return s.getByField(ctx, "id", id)
}

// GetByName retrieves a profile by name for a tenant.
func (s *ProfileStore) GetByName(ctx context.Context, tenantID, name string) (*SandboxProfile, error) {
	return s.getByField(ctx, "name", name, tenantID)
}

func (s *ProfileStore) getByField(ctx context.Context, col string, vals ...interface{}) (*SandboxProfile, error) {
	var query string
	var args []interface{}

	if col == "name" {
		query = `SELECT id, tenant_id, name, description, cpu_cores, memory_mb,
			timeout_seconds, network_access, allowed_tools, filesystem_access,
			max_file_size_mb, max_output_size_kb, is_active, created_at, updated_at
			FROM sandbox_profiles WHERE name = $1 AND tenant_id = $2`
		args = append(args, vals[0], vals[1])
	} else {
		query = `SELECT id, tenant_id, name, description, cpu_cores, memory_mb,
			timeout_seconds, network_access, allowed_tools, filesystem_access,
			max_file_size_mb, max_output_size_kb, is_active, created_at, updated_at
			FROM sandbox_profiles WHERE id = $1`
		args = append(args, vals[0])
	}

	p := &SandboxProfile{}
	var desc *string
	var tools []string
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&p.ID, &p.TenantID, &p.Name, &desc, &p.CPUCores, &p.MemoryMB,
		&p.TimeoutSeconds, &p.NetworkAccess, &tools, &p.FilesystemAccess,
		&p.MaxFileSizeMB, &p.MaxOutputSizeKB, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.Description = desc
	p.AllowedTools = tools
	return p, nil
}

// Update modifies a profile.
func (s *ProfileStore) Update(ctx context.Context, p *SandboxProfile) error {
	query := `
		UPDATE sandbox_profiles SET name = $1, description = $2, cpu_cores = $3,
			memory_mb = $4, timeout_seconds = $5, network_access = $6,
			allowed_tools = $7, filesystem_access = $8, max_file_size_mb = $9,
			max_output_size_kb = $10, is_active = $11, updated_at = NOW()
		WHERE id = $12 AND tenant_id = $13
		RETURNING updated_at`

	var updatedAt time.Time
	err := s.pool.QueryRow(ctx, query,
		p.Name, p.Description, p.CPUCores, p.MemoryMB, p.TimeoutSeconds,
		p.NetworkAccess, p.AllowedTools, p.FilesystemAccess, p.MaxFileSizeMB,
		p.MaxOutputSizeKB, p.IsActive, p.ID, p.TenantID,
	).Scan(&updatedAt)
	if err == pgx.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	p.UpdatedAt = updatedAt
	return nil
}

// Delete removes a profile by ID.
func (s *ProfileStore) Delete(ctx context.Context, id uuid.UUID, tenantID string) error {
	_, err := s.pool.Exec(ctx,
		"DELETE FROM sandbox_profiles WHERE id = $1 AND tenant_id = $2", id, tenantID)
	if err != nil {
		return err
	}
	return nil
}

// List returns paginated profiles for a tenant.
func (s *ProfileStore) List(ctx context.Context, tenantID string, page, pageSize int) ([]SandboxProfile, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := `
		SELECT id, tenant_id, name, description, cpu_cores, memory_mb,
			timeout_seconds, network_access, allowed_tools, filesystem_access,
			max_file_size_mb, max_output_size_kb, is_active, created_at, updated_at
		FROM sandbox_profiles
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := s.pool.Query(ctx, query, tenantID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var profiles []SandboxProfile
	for rows.Next() {
		var p SandboxProfile
		var desc *string
		var tools []string
		err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &desc, &p.CPUCores, &p.MemoryMB,
			&p.TimeoutSeconds, &p.NetworkAccess, &tools, &p.FilesystemAccess,
			&p.MaxFileSizeMB, &p.MaxOutputSizeKB, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("scan profile: %w", err)
		}
		p.Description = desc
		p.AllowedTools = tools
		profiles = append(profiles, p)
	}

	countRows, err := s.pool.Query(ctx,
		"SELECT COUNT(*) FROM sandbox_profiles WHERE tenant_id = $1", tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer countRows.Close()
	var total int
	if countRows.Next() {
		countRows.Scan(&total)
	}

	return profiles, total, nil
}