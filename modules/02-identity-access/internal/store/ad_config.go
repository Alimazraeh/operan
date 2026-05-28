package store

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// ErrADConfigNotFound is returned when an AD config is not found.
var ErrADConfigNotFound = errors.New("AD config not found")

// ADConfigStore provides in-memory CRUD operations for AD configurations with tenant isolation.
type ADConfigStore struct {
	mu       sync.RWMutex
	byTenant map[string]*models.ADConfig // key: tenantID
	byID     map[string]*models.ADConfig // key: config ID
}

// NewADConfigStore creates a new in-memory AD config store.
func NewADConfigStore() *ADConfigStore {
	return &ADConfigStore{
		byTenant: make(map[string]*models.ADConfig),
		byID:     make(map[string]*models.ADConfig),
	}
}

// Create creates a new AD configuration.
func (s *ADConfigStore) Create(config *models.ADConfig) error {
	if config.ID == "" {
		config.ID = uuid.New().String()
	}
	if config.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if config.DomainName == "" {
		return fmt.Errorf("domain_name is required")
	}
	if config.DomainController == "" {
		return fmt.Errorf("domain_controller is required")
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

// GetByTenant retrieves the AD configuration for a tenant.
func (s *ADConfigStore) GetByTenant(tenantID string) (*models.ADConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.byTenant[tenantID]
	if !exists {
		return nil, ErrADConfigNotFound
	}

	result := *config
	return &result, nil
}

// GetByID retrieves an AD configuration by ID.
func (s *ADConfigStore) GetByID(id string) (*models.ADConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.byID[id]
	if !exists {
		return nil, ErrADConfigNotFound
	}

	result := *config
	return &result, nil
}

// Update updates an AD configuration.
func (s *ADConfigStore) Update(tenantID string, displayName, domainName, domainController, bindDN, bindPassword, orgUnit string, configJSON string, enabled bool, status string) (*models.ADConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, exists := s.byTenant[tenantID]
	if !exists {
		return nil, ErrADConfigNotFound
	}

	if displayName != "" {
		config.DisplayName = displayName
	}
	if domainName != "" {
		config.DomainName = domainName
	}
	if domainController != "" {
		config.DomainController = domainController
	}
	if bindDN != "" {
		config.BindDN = bindDN
	}
	if bindPassword != "" {
		config.BindPassword = bindPassword
	}
	if orgUnit != "" {
		config.OrganizationUnit = orgUnit
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

// Delete removes an AD configuration.
func (s *ADConfigStore) Delete(tenantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, exists := s.byTenant[tenantID]
	if !exists {
		return ErrADConfigNotFound
	}

	delete(s.byTenant, tenantID)
	delete(s.byID, config.ID)
	return nil
}
