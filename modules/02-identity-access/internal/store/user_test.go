package store

import (
	"testing"

	"github.com/operan/modules/02-identity-access/internal/models"
)

func TestUserStoreCreate(t *testing.T) {
	s := NewUserStore()

	user := &models.User{
		TenantID:    "tenant-1",
		Email:       "test@example.com",
		DisplayName: "Test User",
		Status:      "active",
		Roles:       []string{"role-1"},
	}

	if err := s.Create(user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if user.ID == "" {
		t.Error("Create() should auto-generate user ID")
	}
	if user.CreatedAt.IsZero() {
		t.Error("Create() should set CreatedAt")
	}
	if user.AuthenticationMethod != "password" {
		t.Errorf("Create() auth method = %v, want password", user.AuthenticationMethod)
	}
}

func TestUserStoreCreateMFA(t *testing.T) {
	s := NewUserStore()

	user := &models.User{
		TenantID:    "tenant-1",
		Email:       "mfa@example.com",
		DisplayName: "MFA User",
		MFAEnabled:  true,
	}

	if err := s.Create(user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if user.AuthenticationMethod != "mfa" {
		t.Errorf("Create() MFA auth method = %v, want mfa", user.AuthenticationMethod)
	}
}

func TestUserStoreCreateMissingFields(t *testing.T) {
	s := NewUserStore()

	// Missing email
	user1 := &models.User{TenantID: "tenant-1", DisplayName: "Test"}
	if err := s.Create(user1); err == nil {
		t.Error("Create() should error when email is empty")
	}

	// Missing display_name
	user2 := &models.User{TenantID: "tenant-1", Email: "test@example.com"}
	if err := s.Create(user2); err == nil {
		t.Error("Create() should error when display_name is empty")
	}

	// Missing tenant_id
	user3 := &models.User{Email: "test@example.com", DisplayName: "Test"}
	if err := s.Create(user3); err == nil {
		t.Error("Create() should error when tenant_id is empty")
	}
}

func TestUserStoreGetByID(t *testing.T) {
	s := NewUserStore()

	user := &models.User{
		TenantID:    "tenant-1",
		Email:       "get@example.com",
		DisplayName: "Get User",
		Roles:       []string{"admin", "viewer"},
	}
	if err := s.Create(user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := s.GetByID(user.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Email != "get@example.com" {
		t.Errorf("GetByID() email = %v, want get@example.com", got.Email)
	}
	if len(got.Roles) != 2 {
		t.Errorf("GetByID() roles = %v, want [admin viewer]", got.Roles)
	}
}

func TestUserStoreGetByIDNotFound(t *testing.T) {
	s := NewUserStore()
	_, err := s.GetByID("nonexistent")
	if err != ErrUserNotFound {
		t.Errorf("GetByID() error = %v, want ErrUserNotFound", err)
	}
}

func TestUserStoreListPagination(t *testing.T) {
	s := NewUserStore()

	// Create 7 users for tenant-1
	for i := 0; i < 7; i++ {
		user := &models.User{
			TenantID:    "tenant-1",
			Email:       "user" + string(rune('0'+i)) + "@example.com",
			DisplayName: "User " + string(rune('0'+i)),
			Status:      "active",
		}
		if err := s.Create(user); err != nil {
			t.Fatalf("Create(%d) error = %v", i, err)
		}
	}

	// Create 3 users for tenant-2
	for i := 0; i < 3; i++ {
		user := &models.User{
			TenantID:    "tenant-2",
			Email:       "tenant2-" + string(rune('0'+i)) + "@example.com",
			DisplayName: "Tenant2 User " + string(rune('0'+i)),
			Status:      "active",
		}
		if err := s.Create(user); err != nil {
			t.Fatalf("Create tenant-2(%d) error = %v", i, err)
		}
	}

	// Test page 1, pageSize 3
	users, total, err := s.List("tenant-1", 1, 3)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 7 {
		t.Errorf("List() total = %v, want 7", total)
	}
	if len(users) != 3 {
		t.Errorf("List() len(users) = %v, want 3", len(users))
	}

	// Test page 3, pageSize 3 (last page has 1)
	users, _, err = s.List("tenant-1", 3, 3)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(users) != 1 {
		t.Errorf("List() page3 len = %v, want 1", len(users))
	}

	// Test page 4 (beyond total)
	users, _, err = s.List("tenant-1", 4, 3)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(users) != 0 {
		t.Errorf("List() page4 len = %v, want 0", len(users))
	}

	// Tenant isolation
	users, total, err = s.List("tenant-2", 1, 100)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 3 {
		t.Errorf("List() tenant-2 total = %v, want 3", total)
	}
}

func TestUserStoreUpdate(t *testing.T) {
	s := NewUserStore()

	user := &models.User{
		TenantID:    "tenant-1",
		Email:       "update@example.com",
		DisplayName: "Old Name",
		Status:      "active",
	}
	if err := s.Create(user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	newName := "New Name"
	newStatus := "suspended"
	updates := &models.UpdateUserRequest{
		DisplayName: &newName,
		Status:      &newStatus,
	}

	got, err := s.Update(user.ID, updates)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got.DisplayName != "New Name" {
		t.Errorf("Update() display_name = %v, want New Name", got.DisplayName)
	}
	if got.Status != "suspended" {
		t.Errorf("Update() status = %v, want suspended", got.Status)
	}
}

func TestUserStoreDeactivate(t *testing.T) {
	s := NewUserStore()

	user := &models.User{
		TenantID:    "tenant-1",
		Email:       "deactivate@example.com",
		DisplayName: "Deactivate User",
		Status:      "active",
	}
	if err := s.Create(user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := s.Deactivate(user.ID); err != nil {
		t.Fatalf("Deactivate() error = %v", err)
	}

	got, err := s.GetByID(user.ID)
	if err != nil {
		t.Fatalf("GetByID() after deactivate error = %v", err)
	}
	if got.Status != "deactivated" {
		t.Errorf("GetByID() status = %v, want deactivated", got.Status)
	}
}

func TestUserStoreDeactivateNotFound(t *testing.T) {
	s := NewUserStore()
	err := s.Deactivate("nonexistent")
	if err != ErrUserNotFound {
		t.Errorf("Deactivate() error = %v, want ErrUserNotFound", err)
	}
}

func TestUserStoreSetRoles(t *testing.T) {
	s := NewUserStore()

	user := &models.User{
		TenantID:    "tenant-1",
		Email:       "roles@example.com",
		DisplayName: "Roles User",
	}
	if err := s.Create(user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := s.SetRoles(user.ID, []string{"admin", "editor", "viewer"}); err != nil {
		t.Fatalf("SetRoles() error = %v", err)
	}

	got, err := s.GetByID(user.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if len(got.Roles) != 3 {
		t.Errorf("GetByID() roles count = %v, want 3", len(got.Roles))
	}
}

func TestUserStoreGetByTenantAndEmail(t *testing.T) {
	s := NewUserStore()

	user := &models.User{
		TenantID:    "tenant-1",
		Email:       "find@example.com",
		DisplayName: "Find User",
	}
	if err := s.Create(user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := s.GetByTenantAndEmail("tenant-1", "find@example.com")
	if err != nil {
		t.Fatalf("GetByTenantAndEmail() error = %v", err)
	}
	if got.Email != "find@example.com" {
		t.Errorf("GetByTenantAndEmail() email = %v, want find@example.com", got.Email)
	}
}

func TestUserStoreGetByTenantAndEmailNotFound(t *testing.T) {
	s := NewUserStore()
	_, err := s.GetByTenantAndEmail("tenant-1", "nonexistent@example.com")
	if err != ErrUserNotFound {
		t.Errorf("GetByTenantAndEmail() error = %v, want ErrUserNotFound", err)
	}
}

func TestUserStoreIsActive(t *testing.T) {
	user := &models.User{Status: "active"}
	if !user.IsActive() {
		t.Error("IsActive() should return true for active user")
	}

	user.Status = "deactivated"
	if user.IsActive() {
		t.Error("IsActive() should return false for deactivated user")
	}
}

func TestUserStoreValidateUpdateNil(t *testing.T) {
	updates := &models.UpdateUserRequest{}
	err := updates.Validate()
	if err == nil {
		t.Error("Validate() should error when all fields are nil")
	}
}

func TestUserStoreUpdateInvalidatesNil(t *testing.T) {
	updates := &models.UpdateUserRequest{}
	err := updates.Validate()
	if err == nil {
		t.Error("Validate() should error when all fields are nil")
	}
}

func TestUserStoreUpdateSetRoles(t *testing.T) {
	s := NewUserStore()

	user := &models.User{
		TenantID:    "tenant-1",
		Email:       "setroles@example.com",
		DisplayName: "Set Roles User",
	}
	if err := s.Create(user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := s.SetRoles(user.ID, []string{"admin", "editor"}); err != nil {
		t.Fatalf("SetRoles() error = %v", err)
	}

	got, err := s.GetByID(user.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if len(got.Roles) != 2 {
		t.Errorf("GetByID() roles count = %v, want 2", len(got.Roles))
	}
}

func TestUserStoreUpdateWithMFA(t *testing.T) {
	s := NewUserStore()

	user := &models.User{
		TenantID:    "tenant-1",
		Email:       "mfa-update@example.com",
		DisplayName: "MFA Update User",
		MFAEnabled:  false,
	}
	if err := s.Create(user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	enableMFA := true
	updates := &models.UpdateUserRequest{
		MFAEnabled: &enableMFA,
	}

	got, err := s.Update(user.ID, updates)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !got.MFAEnabled {
		t.Error("Update() should enable MFA")
	}
	if got.AuthenticationMethod != "mfa" {
		t.Errorf("Update() auth method = %v, want mfa", got.AuthenticationMethod)
	}
}

func TestUserStoreCreateDefaultStatus(t *testing.T) {
	s := NewUserStore()

	user := &models.User{
		TenantID:    "tenant-1",
		Email:       "default-status@example.com",
		DisplayName: "Default Status User",
	}
	if err := s.Create(user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if user.Status != "pending" {
		t.Errorf("Create() default status = %v, want pending", user.Status)
	}
}

func TestUserStoreListInvalidPageSize(t *testing.T) {
	s := NewUserStore()

	// Create 3 users
	for i := 0; i < 3; i++ {
		user := &models.User{
			TenantID:    "tenant-1",
			Email:       "page" + string(rune('0'+i)) + "@example.com",
			DisplayName: "Page User " + string(rune('0'+i)),
		}
		if err := s.Create(user); err != nil {
			t.Fatalf("Create(%d) error = %v", i, err)
		}
	}

	// pageSize > 100 should be clamped to 50
	users, total, err := s.List("tenant-1", 1, 200)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 3 {
		t.Errorf("List() total = %v, want 3", total)
	}
	if len(users) != 3 {
		t.Errorf("List() len(users) = %v, want 3 (clamped)", len(users))
	}
}

func TestUserStoreListInvalidPage(t *testing.T) {
	s := NewUserStore()

	user := &models.User{
		TenantID:    "tenant-1",
		Email:       "invalid-page@example.com",
		DisplayName: "Invalid Page User",
	}
	if err := s.Create(user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// page < 1 should default to 1
	users, total, err := s.List("tenant-1", 0, 10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 1 {
		t.Errorf("List() total = %v, want 1", total)
	}
	if len(users) != 1 {
		t.Errorf("List() len(users) = %v, want 1", len(users))
	}
}
