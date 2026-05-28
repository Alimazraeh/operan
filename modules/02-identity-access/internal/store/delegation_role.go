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

// ErrDelegationRoleNotFound is returned when a delegation role is not found.
var ErrDelegationRoleNotFound = errors.New("delegation role not found")

// DelegationRoleStore provides in-memory CRUD operations for delegation roles with tenant isolation.
type DelegationRoleStore struct {
	mu      sync.RWMutex
	roles   map[string]*models.DelegationRole // key: role ID
	byName  map[string]*models.DelegationRole // key: tenantID::name
}

// NewDelegationRoleStore creates a new in-memory delegation role store.
func NewDelegationRoleStore() *DelegationRoleStore {
	return &DelegationRoleStore{
		roles:  make(map[string]*models.DelegationRole),
		byName: make(map[string]*models.DelegationRole),
	}
}

// Create creates a new delegation role.
func (s *DelegationRoleStore) Create(role *models.DelegationRole) error {
	if role.ID == "" {
		role.ID = uuid.New().String()
	}
	if role.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if role.Name == "" {
		return fmt.Errorf("role name is required")
	}
	if role.Scope == "" {
		return fmt.Errorf("scope is required")
	}

	if _, exists := s.byName[role.TenantID+"::"+role.Name]; exists {
		return fmt.Errorf("delegation role with name %s already exists in tenant %s", role.Name, role.TenantID)
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

	// Store delegated_to as JSON
	if len(role.DelegatedToIDs) > 0 {
		data, err := json.Marshal(role.DelegatedToIDs)
		if err != nil {
			return fmt.Errorf("marshal delegated_to: %w", err)
		}
		role.DelegatedToJSON = string(data)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.roles[role.ID] = role
	s.byName[role.TenantID+"::"+role.Name] = role

	return nil
}

// GetByID retrieves a delegation role by ID.
func (s *DelegationRoleStore) GetByID(id string) (*models.DelegationRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	role, exists := s.roles[id]
	if !exists {
		return nil, ErrDelegationRoleNotFound
	}

	result := *role
	result.Permissions = unmarshalString(role.PermissionsJSON)
	result.DelegatedToIDs = unmarshalString(role.DelegatedToJSON)
	return &result, nil
}

// GetByName retrieves a delegation role by name within a tenant.
func (s *DelegationRoleStore) GetByName(tenantID, name string) (*models.DelegationRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	role, exists := s.byName[tenantID+"::"+name]
	if !exists {
		return nil, ErrDelegationRoleNotFound
	}

	result := *role
	result.Permissions = unmarshalString(role.PermissionsJSON)
	result.DelegatedToIDs = unmarshalString(role.DelegatedToJSON)
	return &result, nil
}

// List returns a paginated list of delegation roles for a tenant.
func (s *DelegationRoleStore) List(tenantID string, page, pageSize int) ([]models.DelegationRole, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []models.DelegationRole
	for _, role := range s.roles {
		if role.TenantID == tenantID {
			r := *role
			r.Permissions = unmarshalString(role.PermissionsJSON)
			r.DelegatedToIDs = unmarshalString(role.DelegatedToJSON)
			all = append(all, r)
		}
	}

	total := len(all)

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

// Update updates a delegation role.
func (s *DelegationRoleStore) Update(id string, name, description, scope string, permissions []string, maxDepth int) (*models.DelegationRole, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, exists := s.roles[id]
	if !exists {
		return nil, ErrDelegationRoleNotFound
	}

	if name != "" {
		if _, exists := s.byName[role.TenantID+"::"+name]; exists && name != role.Name {
			return nil, fmt.Errorf("delegation role with name %s already exists in tenant %s", name, role.TenantID)
		}
		delete(s.byName, role.TenantID+"::"+role.Name)
		role.Name = name
	}
	if description != "" {
		role.Description = description
	}
	if scope != "" {
		role.Scope = scope
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
	role.MaxDelegationDepth = maxDepth
	role.UpdatedAt = time.Now().UTC()

	result := *role
	result.Permissions = unmarshalString(role.PermissionsJSON)
	result.DelegatedToIDs = unmarshalString(role.DelegatedToJSON)
	return &result, nil
}

// Delete removes a delegation role.
func (s *DelegationRoleStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, exists := s.roles[id]
	if !exists {
		return ErrDelegationRoleNotFound
	}

	delete(s.byName, role.TenantID+"::"+role.Name)
	delete(s.roles, id)
	return nil
}

// GrantDelegation grants a delegation role to a user.
func (s *DelegationRoleStore) GrantDelegation(grant *models.DelegationGrant) error {
	if grant.ID == "" {
		grant.ID = uuid.New().String()
	}
	if grant.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if grant.DelegationRoleID == "" {
		return fmt.Errorf("delegation_role_id is required")
	}
	if grant.UserID == "" {
		return fmt.Errorf("user_id is required")
	}

	grant.GrantedAt = time.Now().UTC()
	grant.IsActive = true

	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure role exists
	role, exists := s.roles[grant.DelegationRoleID]
	if !exists {
		return ErrDelegationRoleNotFound
	}

	// Add user to role's delegated_to list
	role.DelegatedToIDs = append(role.DelegatedToIDs, grant.UserID)
	if len(role.DelegatedToIDs) > 0 {
		data, err := json.Marshal(role.DelegatedToIDs)
		if err != nil {
			return fmt.Errorf("marshal delegated_to: %w", err)
		}
		role.DelegatedToJSON = string(data)
	}

	return nil
}

// RevokeDelegation revokes a delegation grant from a user.
func (s *DelegationRoleStore) RevokeDelegation(roleID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, exists := s.roles[roleID]
	if !exists {
		return ErrDelegationRoleNotFound
	}

	// Remove user from delegated_to list
	newList := make([]string, 0, len(role.DelegatedToIDs))
	for _, id := range role.DelegatedToIDs {
		if id != userID {
			newList = append(newList, id)
		}
	}
	role.DelegatedToIDs = newList
	if len(newList) > 0 {
		data, err := json.Marshal(newList)
		if err != nil {
			return fmt.Errorf("marshal delegated_to: %w", err)
		}
		role.DelegatedToJSON = string(data)
	} else {
		role.DelegatedToJSON = "[]"
	}

	return nil
}

// ListDelegations returns all delegation grants for a tenant/user/role.
func (s *DelegationRoleStore) ListDelegations(tenantID, roleID, userID string) ([]models.DelegationGrant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// This is a simplified version - in production, you'd have a dedicated store for delegation grants
	// For now, we return empty list as the grants are tracked in the role itself
	return []models.DelegationGrant{}, nil
}
