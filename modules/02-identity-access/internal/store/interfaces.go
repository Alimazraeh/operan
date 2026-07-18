// Package store defines interfaces that both in-memory and persistent store
// implementations satisfy. Handlers use these interfaces for type flexibility.
package store

import (
	"time"

	"github.com/operan/modules/02-identity-access/internal/models"
)

// UserStoreAPI defines the interface for user data access.
// Both in-memory (*UserStore) and persistent (*PersistentUserStore) implement this.
type UserStoreAPI interface {
	Create(user *models.User) error
	GetByID(id string) (*models.User, error)
	GetByTenantAndEmail(tenantID, email string) (*models.User, error)
	GetByActorID(tenantID, actorID string) (*models.User, error)
	List(tenantID string, page, pageSize int) ([]models.User, int, error)
	Update(id string, updates *models.UpdateUserRequest) (*models.User, error)
	Deactivate(id string) error
	SetRoles(id string, roles []string) error
	CountTotal() (int, error)
}

// AuditStoreAPI defines the interface for audit data access.
// Both in-memory (*AuditStore) and persistent (*PersistentAuditStore) implement this.
type AuditStoreAPI interface {
	Create(e *models.AuditEvent) error
	CreateWithTenant(e *models.AuditEvent, tenantID string) error
	List(tenantID, actorID, action string, from, to *time.Time, limit, offset int) ([]models.AuditEvent, int, error)
	GetByID(id string) (*models.AuditEvent, error)
	CountTotal() (int, error)
}