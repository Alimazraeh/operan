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

// ErrUserNotFound is returned when a user is not found.
var ErrUserNotFound = errors.New("user not found")

// UserStore provides in-memory CRUD operations for users with tenant isolation.
type UserStore struct {
	mu    sync.RWMutex
	users map[string]*models.User // key: user ID
}

// NewUserStore creates a new in-memory user store.
func NewUserStore() *UserStore {
	return &UserStore{
		users: make(map[string]*models.User),
	}
}

// Create creates a new user with a unique UUID and tenant_id.
func (s *UserStore) Create(user *models.User) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	if user.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if user.Email == "" {
		return fmt.Errorf("email is required")
	}
	if user.DisplayName == "" {
		return fmt.Errorf("display_name is required")
	}
	if user.Status == "" {
		user.Status = "pending"
	}
	if user.MFAEnabled {
		user.AuthenticationMethod = "mfa"
	} else {
		user.AuthenticationMethod = "password"
	}
	user.CreatedAt = time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[user.ID]; exists {
		return fmt.Errorf("user with id %s already exists", user.ID)
	}

	// Store roles as JSON
	if len(user.RoleIDs) > 0 {
		data, err := json.Marshal(user.RoleIDs)
		if err != nil {
			return fmt.Errorf("marshal roles: %w", err)
		}
		user.RolesJSON = string(data)
	}

	s.users[user.ID] = user
	return nil
}

// GetByID retrieves a user by ID.
func (s *UserStore) GetByID(id string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, ErrUserNotFound
	}

	// Return a copy
	copy := *user
	copy.RoleIDs = unmarshalRoleIDs(user.RolesJSON)
	return &copy, nil
}

// GetByTenantAndEmail retrieves a user by tenant and email.
func (s *UserStore) GetByTenantAndEmail(tenantID, email string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if user.TenantID == tenantID && user.Email == email {
			copy := *user
			copy.RoleIDs = unmarshalRoleIDs(user.RolesJSON)
			return &copy, nil
		}
	}

	return nil, ErrUserNotFound
}

// GetByActorID retrieves a user by tenant and actor ID (email).
func (s *UserStore) GetByActorID(tenantID, actorID string) (*models.User, error) {
	return s.GetByTenantAndEmail(tenantID, actorID)
}

// List returns all users for a tenant with pagination.
func (s *UserStore) List(tenantID string, page, pageSize int) ([]models.User, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	var allUsers []*models.User
	for _, user := range s.users {
		if user.TenantID == tenantID {
			allUsers = append(allUsers, user)
		}
	}

	total := len(allUsers)
	start := (page - 1) * pageSize
	if start > total {
		return []models.User{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	result := make([]models.User, 0, end-start)
	for _, user := range allUsers[start:end] {
		copy := *user
		copy.RoleIDs = unmarshalRoleIDs(user.RolesJSON)
		result = append(result, copy)
	}

	return result, total, nil
}

// Update updates a user's fields.
func (s *UserStore) Update(id string, updates *models.UpdateUserRequest) (*models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[id]
	if !exists {
		return nil, ErrUserNotFound
	}

	if updates.DisplayName != nil {
		user.DisplayName = *updates.DisplayName
	}
	if updates.RoleIDs != nil {
		user.RoleIDs = updates.RoleIDs
		if len(user.RoleIDs) > 0 {
			data, err := json.Marshal(user.RoleIDs)
			if err != nil {
				return nil, fmt.Errorf("marshal roles: %w", err)
			}
			user.RolesJSON = string(data)
		}
	}
	if updates.MFAEnabled != nil {
		user.MFAEnabled = *updates.MFAEnabled
		if *updates.MFAEnabled {
			user.AuthenticationMethod = "mfa"
		}
	}
	if updates.Status != nil {
		user.Status = *updates.Status
	}

	user.UpdatedAt = time.Now().UTC()

	result := *user
	result.RoleIDs = unmarshalRoleIDs(user.RolesJSON)
	return &result, nil
}

// Deactivate soft-deletes a user.
func (s *UserStore) Deactivate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[id]
	if !exists {
		return ErrUserNotFound
	}

	user.Status = "deactivated"
	user.UpdatedAt = time.Now().UTC()
	return nil
}

// SetRoles replaces all roles for a user.
func (s *UserStore) SetRoles(id string, roles []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[id]
	if !exists {
		return ErrUserNotFound
	}

	user.RoleIDs = roles
	if len(roles) > 0 {
		data, err := json.Marshal(roles)
		if err != nil {
			return fmt.Errorf("marshal roles: %w", err)
		}
		user.RolesJSON = string(data)
	} else {
		user.RolesJSON = "[]"
	}

	return nil
}

// unmarshalRoleIDs converts the JSON roles field to a string slice.
func unmarshalRoleIDs(jsonStr string) []string {
	if jsonStr == "" || jsonStr == "[]" {
		return []string{}
	}
	var roles []string
	if err := json.Unmarshal([]byte(jsonStr), &roles); err != nil {
		return []string{}
	}
	return roles
}
