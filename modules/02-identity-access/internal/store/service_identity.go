package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// ErrServiceIdentityNotFound is returned when a service identity is not found.
var ErrServiceIdentityNotFound = errors.New("service identity not found")

// ServiceIdentityStore provides in-memory CRUD operations for service identities with tenant isolation.
type ServiceIdentityStore struct {
	mu   sync.RWMutex
	idByTenantAndName map[string]*models.ServiceIdentity // key: tenantID::name
	idByID            map[string]*models.ServiceIdentity   // key: service identity ID
}

// NewServiceIdentityStore creates a new in-memory service identity store.
func NewServiceIdentityStore() *ServiceIdentityStore {
	return &ServiceIdentityStore{
		idByTenantAndName: make(map[string]*models.ServiceIdentity),
		idByID:            make(map[string]*models.ServiceIdentity),
	}
}

// Create creates a new service identity.
func (s *ServiceIdentityStore) Create(identity *models.ServiceIdentity) error {
	if identity.ID == "" {
		identity.ID = uuid.New().String()
	}
	if identity.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if identity.Name == "" {
		return fmt.Errorf("name is required")
	}

	// Generate API key ID if not provided
	if identity.APIKeyID == "" {
		identity.APIKeyID = "sk_" + uuid.New().String()
	}

	// Check uniqueness within tenant
	if _, exists := s.idByTenantAndName[identity.TenantID+"::"+identity.Name]; exists {
		return fmt.Errorf("service identity with name %s already exists in tenant %s", identity.Name, identity.TenantID)
	}

	// Store roles as JSON
	if len(identity.Roles) > 0 {
		data, err := json.Marshal(identity.Roles)
		if err != nil {
			return fmt.Errorf("marshal roles: %w", err)
		}
		identity.RolesJSON = string(data)
	}

	// Store metadata as JSON
	if identity.Metadata != "" {
		// Validate it's valid JSON
		var j interface{}
		if err := json.Unmarshal([]byte(identity.Metadata), &j); err != nil {
			return fmt.Errorf("metadata must be valid JSON: %w", err)
		}
	}

	identity.CreatedAt = time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.idByID[identity.ID] = identity
	s.idByTenantAndName[identity.TenantID+"::"+identity.Name] = identity

	return nil
}

// GetByID retrieves a service identity by ID.
func (s *ServiceIdentityStore) GetByID(id string) (*models.ServiceIdentity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	identity, exists := s.idByID[id]
	if !exists {
		return nil, ErrServiceIdentityNotFound
	}

	result := *identity
	result.Roles = unmarshalString(identity.RolesJSON)
	return &result, nil
}

// GetByName retrieves a service identity by name within a tenant.
func (s *ServiceIdentityStore) GetByName(tenantID, name string) (*models.ServiceIdentity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	identity, exists := s.idByTenantAndName[tenantID+"::"+name]
	if !exists {
		return nil, ErrServiceIdentityNotFound
	}

	result := *identity
	result.Roles = unmarshalString(identity.RolesJSON)
	return &result, nil
}

// List returns all service identities for a tenant.
func (s *ServiceIdentityStore) List(tenantID string) ([]models.ServiceIdentity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []models.ServiceIdentity
	for _, identity := range s.idByID {
		if identity.TenantID == tenantID {
			r := *identity
			r.Roles = unmarshalString(identity.RolesJSON)
			result = append(result, r)
		}
	}

	return result, nil
}

// UpdateLastUsed updates the last used timestamp.
func (s *ServiceIdentityStore) UpdateLastUsed(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	identity, exists := s.idByID[id]
	if !exists {
		return ErrServiceIdentityNotFound
	}

	now := time.Now().UTC()
	identity.LastUsedAt = &now
	return nil
}

// RevokeAPIKey revokes the API key by setting it to empty.
func (s *ServiceIdentityStore) RevokeAPIKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	identity, exists := s.idByID[id]
	if !exists {
		return ErrServiceIdentityNotFound
	}

	identity.APIKeyID = ""
	return nil
}
