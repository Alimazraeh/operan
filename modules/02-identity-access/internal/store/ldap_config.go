package store

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// ErrLDAPConfigNotFound is returned when an LDAP config is not found.
var ErrLDAPConfigNotFound = errors.New("LDAP config not found")

// LDAPConfigStore provides in-memory CRUD operations for LDAP configurations with tenant isolation.
type LDAPConfigStore struct {
	mu      sync.RWMutex
	byTenant map[string]*models.LDAPConfig // key: tenantID
	byID     map[string]*models.LDAPConfig // key: config ID
}

// NewLDAPConfigStore creates a new in-memory LDAP config store.
func NewLDAPConfigStore() *LDAPConfigStore {
	return &LDAPConfigStore{
		byTenant: make(map[string]*models.LDAPConfig),
		byID:     make(map[string]*models.LDAPConfig),
	}
}

// Create creates a new LDAP configuration.
func (s *LDAPConfigStore) Create(config *models.LDAPConfig) error {
	if config.ID == "" {
		config.ID = uuid.New().String()
	}
	if config.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if config.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if config.URL == "" {
		return fmt.Errorf("url is required")
	}

	if config.Status == "" {
		config.Status = "configured"
	}

	config.CreatedAt = time.Now().UTC()
	config.UpdatedAt = time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.byTenant[config.TenantID] = config
	s.byID[config.ID] = config
	return nil
}

// GetByTenant retrieves the LDAP configuration for a tenant.
func (s *LDAPConfigStore) GetByTenant(tenantID string) (*models.LDAPConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.byTenant[tenantID]
	if !exists {
		return nil, ErrLDAPConfigNotFound
	}

	result := *config
	return &result, nil
}

// GetByID retrieves an LDAP configuration by ID.
func (s *LDAPConfigStore) GetByID(id string) (*models.LDAPConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.byID[id]
	if !exists {
		return nil, ErrLDAPConfigNotFound
	}

	result := *config
	return &result, nil
}

// Update updates an LDAP configuration.
func (s *LDAPConfigStore) Update(tenantID string, displayName, url, baseDN, bindDN, bindPassword, searchScope, userFilter, groupFilter string, configJSON string, enabled bool, status string) (*models.LDAPConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, exists := s.byTenant[tenantID]
	if !exists {
		return nil, ErrLDAPConfigNotFound
	}

	if displayName != "" {
		config.DisplayName = displayName
	}
	if url != "" {
		config.URL = url
	}
	if baseDN != "" {
		config.BaseDN = baseDN
	}
	if bindDN != "" {
		config.BindDN = bindDN
	}
	if bindPassword != "" {
		config.BindPassword = bindPassword
	}
	if searchScope != "" {
		config.SearchScope = searchScope
	}
	if userFilter != "" {
		config.UserFilter = userFilter
	}
	if groupFilter != "" {
		config.GroupFilter = groupFilter
	}
	if configJSON != "" {
		config.ConfigJSON = configJSON
	}
	config.Enabled = enabled
	if status != "" {
		config.Status = status
	}

	config.UpdatedAt = time.Now().UTC()

	result := *config
	return &result, nil
}

// Delete removes an LDAP configuration.
func (s *LDAPConfigStore) Delete(tenantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, exists := s.byTenant[tenantID]
	if !exists {
		return ErrLDAPConfigNotFound
	}

	delete(s.byTenant, tenantID)
	delete(s.byID, config.ID)
	return nil
}

// List returns all LDAP configurations for a tenant (for internal use).
func (s *LDAPConfigStore) List() []*models.LDAPConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*models.LDAPConfig, 0, len(s.byID))
	for _, config := range s.byID {
		c := *config
		result = append(result, &c)
	}
	return result
}
