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

// ErrRoleNotFound is returned when a role is not found.
var ErrRoleNotFound = errors.New("role not found")

// RoleStore provides in-memory CRUD operations for roles with tenant isolation.
type RoleStore struct {
	mu      sync.RWMutex
	roles   map[string]*models.Role // key: role ID
	byName  map[string]*models.Role // key: tenantID::name
}

// NewRoleStore creates a new in-memory role store.
func NewRoleStore() *RoleStore {
	return &RoleStore{
		roles:  make(map[string]*models.Role),
		byName: make(map[string]*models.Role),
	}
}

// Create creates a new role.
func (s *RoleStore) Create(role *models.Role) error {
	if role.ID == "" {
		role.ID = uuid.New().String()
	}
	if role.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if role.Name == "" {
		return fmt.Errorf("role name is required")
	}

	// Check uniqueness
	if _, exists := s.byName[role.TenantID+"::"+role.Name]; exists {
		return fmt.Errorf("role with name %s already exists in tenant %s", role.Name, role.TenantID)
	}

	if role.CreatedAt.IsZero() {
		role.CreatedAt = time.Now().UTC()
	}

	// Store permissions as JSON
	if len(role.Permissions) > 0 {
		data, err := json.Marshal(role.Permissions)
		if err != nil {
			return fmt.Errorf("marshal permissions: %w", err)
		}
		role.PermissionsJSON = string(data)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.roles[role.ID] = role
	s.byName[role.TenantID+"::"+role.Name] = role

	return nil
}

// GetByID retrieves a role by ID.
func (s *RoleStore) GetByID(id string) (*models.Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	role, exists := s.roles[id]
	if !exists {
		return nil, ErrRoleNotFound
	}

	result := *role
	result.Permissions = unmarshalString(role.PermissionsJSON)
	return &result, nil
}

// GetByName retrieves a role by name within a tenant.
func (s *RoleStore) GetByName(tenantID, name string) (*models.Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	role, exists := s.byName[tenantID+"::"+name]
	if !exists {
		return nil, ErrRoleNotFound
	}

	result := *role
	result.Permissions = unmarshalString(role.PermissionsJSON)
	return &result, nil
}

// List returns a paginated list of roles for a tenant.
func (s *RoleStore) List(tenantID string, page, pageSize int) ([]models.Role, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []models.Role
	for _, role := range s.roles {
		if role.TenantID == tenantID {
			r := *role
			r.Permissions = unmarshalString(role.PermissionsJSON)
			all = append(all, r)
		}
	}

	total := len(all)

	// Calculate start/end indices for pagination
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return all[start:end], total, nil
}

// Update updates a role's fields.
func (s *RoleStore) Update(id string, name, description string, permissions []string, isSystem bool) (*models.Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, exists := s.roles[id]
	if !exists {
		return nil, ErrRoleNotFound
	}

	if name != "" {
		// Check uniqueness
		if _, exists := s.byName[role.TenantID+"::"+name]; exists && name != role.Name {
			return nil, fmt.Errorf("role with name %s already exists in tenant %s", name, role.TenantID)
		}
		// Remove old entry
		delete(s.byName, role.TenantID+"::"+role.Name)
		role.Name = name
	}
	if description != "" {
		role.Description = description
	}
	if permissions != nil {
		role.Permissions = permissions
		if len(permissions) > 0 {
			data, err := json.Marshal(permissions)
			if err != nil {
				return nil, fmt.Errorf("marshal permissions: %w", err)
			}
			role.PermissionsJSON = string(data)
		} else {
			role.PermissionsJSON = "[]"
		}
	}
	role.IsSystem = isSystem
	role.UpdatedAt = time.Now().UTC()

	result := *role
	result.Permissions = unmarshalString(role.PermissionsJSON)
	return &result, nil
}

// Delete removes a role.
func (s *RoleStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, exists := s.roles[id]
	if !exists {
		return ErrRoleNotFound
	}

	delete(s.byName, role.TenantID+"::"+role.Name)
	delete(s.roles, id)

	return nil
}

// unmarshalString converts a JSON string to a string slice.
func unmarshalString(jsonStr string) []string {
	if jsonStr == "" || jsonStr == "[]" {
		return []string{}
	}
	var result []string
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return []string{}
	}
	return result
}
