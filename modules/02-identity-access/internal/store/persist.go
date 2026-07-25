// Package store adds database persistence to the in-memory store layer.
// When a PostgreSQL DATABASE_URL is configured, all writes are also persisted
// to the database while keeping the in-memory stores as the fast path.
package store

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/operan/modules/02-identity-access/internal/database"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// PersistentUserStore wraps an in-memory UserStore with PostgreSQL persistence.
type PersistentUserStore struct {
	mem  *UserStore
	pool *pgxpool.Pool
}

// NewPersistentUserStore creates a user store that syncs to PostgreSQL.
func NewPersistentUserStore(mem *UserStore, pool *pgxpool.Pool) *PersistentUserStore {
	return &PersistentUserStore{
		mem:  mem,
		pool: pool,
	}
}

// db returns a lazily-initialized database store.
func (s *PersistentUserStore) db() *database.UserStore {
	// We use a sync.Once pattern via a simple closure to avoid re-creating.
	// For simplicity, we just create it on first access.
	return database.NewUserStore(s.pool)
}

// Create persists to both in-memory and database.
func (s *PersistentUserStore) Create(u *models.User) error {
	if err := s.mem.Create(u); err != nil {
		return err
	}
	if s.pool != nil {
		if err := s.db().Create(context.Background(), u); err != nil {
			log.Printf("[WARN] database write failed for user %s: %v", u.ID, err)
		}
	}
	return nil
}

// GetByID reads from in-memory store.
func (s *PersistentUserStore) GetByID(id string) (*models.User, error) {
	return s.mem.GetByID(id)
}

// GetByTenantAndEmail reads from in-memory store.
func (s *PersistentUserStore) GetByTenantAndEmail(tenantID, email string) (*models.User, error) {
	return s.mem.GetByTenantAndEmail(tenantID, email)
}

// List reads from in-memory store.
func (s *PersistentUserStore) List(tenantID string, page, pageSize int) ([]models.User, int, error) {
	return s.mem.List(tenantID, page, pageSize)
}

// Update persists to both in-memory and database.
func (s *PersistentUserStore) Update(id string, updates *models.UpdateUserRequest) (*models.User, error) {
	result, err := s.mem.Update(id, updates)
	if err != nil {
		return nil, err
	}
	if s.pool != nil {
		if _, err := s.db().Update(context.Background(), id, updates); err != nil {
			log.Printf("[WARN] database update failed for user %s: %v", id, err)
		}
	}
	return result, nil
}

// Deactivate persists to both in-memory and database.
func (s *PersistentUserStore) Deactivate(id string) error {
	if err := s.mem.Deactivate(id); err != nil {
		return err
	}
	if s.pool != nil {
		if err := s.db().Deactivate(context.Background(), id); err != nil {
			log.Printf("[WARN] database deactivate failed for user %s: %v", id, err)
		}
	}
	return nil
}

// SetRoles persists to both in-memory and database.
func (s *PersistentUserStore) SetRoles(id string, roles []string) error {
	if err := s.mem.SetRoles(id, roles); err != nil {
		return err
	}
	if s.pool != nil {
		if err := s.db().SetRoles(context.Background(), id, roles); err != nil {
			log.Printf("[WARN] database setRoles failed for user %s: %v", id, err)
		}
	}
	return nil
}

// GetByActorID is an alias for GetByTenantAndEmail.
func (s *PersistentUserStore) GetByActorID(tenantID, actorID string) (*models.User, error) {
	return s.mem.GetByActorID(tenantID, actorID)
}

// CountTotal queries the database for the authoritative count.
func (s *PersistentUserStore) CountTotal() (int, error) {
	if s.pool != nil {
		return s.db().CountTotal(context.Background())
	}
	// No database available — count total users via handler's fallback behavior.
	// In practice, this is used only by the tenant bootstrap endpoint to check
	// if a default tenant should be created. Without a database, we return 0.
	return 0, nil
}

// PersistentAuditStore wraps an in-memory AuditStore with PostgreSQL persistence.
type PersistentAuditStore struct {
	mem  *AuditStore
	pool *pgxpool.Pool
}

// NewPersistentAuditStore creates an audit store that syncs to PostgreSQL.
func NewPersistentAuditStore(mem *AuditStore, pool *pgxpool.Pool) *PersistentAuditStore {
	return &PersistentAuditStore{
		mem:  mem,
		pool: pool,
	}
}

// db returns a lazily-initialized database audit store.
func (s *PersistentAuditStore) db() *database.AuditStore {
	return database.NewAuditStore(s.pool)
}

// Create persists to both in-memory and database.
func (s *PersistentAuditStore) Create(e *models.AuditEvent) error {
	if err := s.mem.Create(e); err != nil {
		return err
	}
	if s.pool != nil {
		if err := s.db().Create(context.Background(), e); err != nil {
			log.Printf("[WARN] database write failed for audit event: %v", err)
		}
	}
	return nil
}

// CreateWithTenant persists with tenant enforcement.
func (s *PersistentAuditStore) CreateWithTenant(e *models.AuditEvent, tenantID string) error {
	if err := s.mem.CreateWithTenant(e, tenantID); err != nil {
		return err
	}
	if s.pool != nil {
		if err := s.db().CreateWithTenant(context.Background(), e, tenantID); err != nil {
			log.Printf("[WARN] database write failed for audit event: %v", err)
		}
	}
	return nil
}

// List reads from in-memory store (paginated).
func (s *PersistentAuditStore) List(tenantID, actorID, action string, from, to *time.Time, limit, offset int) ([]models.AuditEvent, int, error) {
	return s.mem.List(tenantID, actorID, action, from, to, limit, offset)
}

// GetByID reads from in-memory store.
func (s *PersistentAuditStore) GetByID(id string) (*models.AuditEvent, error) {
	return s.mem.GetByID(id)
}

// CountTotal is not supported for audit — returns 0.
func (s *PersistentAuditStore) CountTotal() (int, error) {
	return 0, nil
}

// SetPassword records a credential in memory and persists it.
func (s *PersistentUserStore) SetPassword(id, hash string) error {
	if err := s.mem.SetPasswordHash(id, hash); err != nil {
		return err
	}
	if s.pool != nil {
		if err := s.db().SetPassword(context.Background(), id, hash); err != nil {
			// A credential that is not durable would silently stop working at
			// the next restart, so this is a failure, not a warning.
			return err
		}
	}
	return nil
}

// Hydrate loads persisted users into the in-memory view.
//
// Every read in this store is served from memory, so without this the rows
// written to PostgreSQL are invisible after a restart — persistence would be
// write-only. Called once at boot.
func (s *PersistentUserStore) Hydrate(ctx context.Context) (int, error) {
	if s.pool == nil {
		return 0, nil
	}
	users, err := s.db().LoadAll(ctx)
	if err != nil {
		return 0, err
	}
	for i := range users {
		s.mem.Put(&users[i])
	}
	return len(users), nil
}
