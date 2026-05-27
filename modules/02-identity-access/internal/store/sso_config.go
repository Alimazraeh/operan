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

// ErrSSOConfigNotFound is returned when an SSO config is not found.
var ErrSSOConfigNotFound = errors.New("SSO config not found")

// SSOConfigStore provides in-memory CRUD operations for SSO configurations with tenant isolation.
type SSOConfigStore struct {
	mu       sync.RWMutex
	byTenant map[string]*models.SSOConfig // key: tenantID
}

// NewSSOConfigStore creates a new in-memory SSO config store.
func NewSSOConfigStore() *SSOConfigStore {
	return &SSOConfigStore{
		byTenant: make(map[string]*models.SSOConfig),
	}
}

// Create creates a new SSO configuration.
func (s *SSOConfigStore) Create(config *models.SSOConfig) error {
	if config.ID == "" {
		config.ID = uuid.New().String()
	}
	if config.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if config.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if config.Type == "" {
		return fmt.Errorf("type is required")
	}

	// Store configuration as JSON
	if config.Configuration != nil {
		data, err := json.Marshal(config.Configuration)
		if err != nil {
			return fmt.Errorf("marshal configuration: %w", err)
		}
		config.ConfigJSON = string(data)
	}

	if config.Status == "" {
		config.Status = "configured"
	}

	config.CreatedAt = time.Now().UTC()
	config.UpdatedAt = time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.byTenant[config.TenantID] = config
	return nil
}

// GetByTenant retrieves the SSO configuration for a tenant.
func (s *SSOConfigStore) GetByTenant(tenantID string) (*models.SSOConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.byTenant[tenantID]
	if !exists {
		return nil, ErrSSOConfigNotFound
	}

	result := *config
	result.Configuration = unmarshalMap(config.ConfigJSON)
	return &result, nil
}

// GetByID retrieves an SSO configuration by ID.
func (s *SSOConfigStore) GetByID(id string) (*models.SSOConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, config := range s.byTenant {
		if config.ID == id {
			result := *config
			result.Configuration = unmarshalMap(config.ConfigJSON)
			return &result, nil
		}
	}

	return nil, ErrSSOConfigNotFound
}

// Update updates an SSO configuration.
func (s *SSOConfigStore) Update(tenantID string, provider, ssoType string, configuration map[string]interface{}, status string) (*models.SSOConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, exists := s.byTenant[tenantID]
	if !exists {
		return nil, ErrSSOConfigNotFound
	}

	if provider != "" {
		config.Provider = provider
	}
	if ssoType != "" {
		config.Type = ssoType
	}
	if configuration != nil {
		config.Configuration = configuration
		data, err := json.Marshal(configuration)
		if err != nil {
			return nil, fmt.Errorf("marshal configuration: %w", err)
		}
		config.ConfigJSON = string(data)
	}
	if status != "" {
		config.Status = status
	}

	config.UpdatedAt = time.Now().UTC()

	result := *config
	result.Configuration = unmarshalMap(config.ConfigJSON)
	return &result, nil
}

// Delete removes an SSO configuration.
func (s *SSOConfigStore) Delete(tenantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byTenant[tenantID]; !exists {
		return ErrSSOConfigNotFound
	}

	delete(s.byTenant, tenantID)
	return nil
}

// unmarshalMap converts a JSON string to a map.
func unmarshalMap(jsonStr string) map[string]interface{} {
	if jsonStr == "" || jsonStr == "{}" {
		return map[string]interface{}{}
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return map[string]interface{}{}
	}
	return result
}
