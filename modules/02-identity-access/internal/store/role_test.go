package store

import (
	"testing"
	"time"

	"github.com/operan/modules/02-identity-access/internal/models"
)

func TestRoleStoreCreate(t *testing.T) {
	s := NewRoleStore()

	role := &models.Role{
		TenantID:    "tenant-1",
		Name:        "admin",
		Description: "Admin role",
		Permissions: []string{"*:*"},
	}

	if err := s.Create(role); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if role.ID == "" {
		t.Error("Create() should auto-generate role ID")
	}
	if role.CreatedAt.IsZero() {
		t.Error("Create() should set CreatedAt")
	}
}

func TestRoleStoreCreateDuplicate(t *testing.T) {
	s := NewRoleStore()

	role := &models.Role{
		TenantID: "tenant-1",
		Name:     "admin",
	}
	if err := s.Create(role); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	role2 := &models.Role{
		TenantID: "tenant-1",
		Name:     "admin",
	}
	err := s.Create(role2)
	if err == nil {
		t.Error("Create() should return error for duplicate role name")
	}
}

func TestRoleStoreCreateMissingTenant(t *testing.T) {
	s := NewRoleStore()
	role := &models.Role{Name: "admin"}
	if err := s.Create(role); err == nil {
		t.Error("Create() should error when tenant_id is empty")
	}
}

func TestRoleStoreGetByID(t *testing.T) {
	s := NewRoleStore()
	role := &models.Role{
		TenantID:    "tenant-1",
		Name:        "viewer",
		Permissions: []string{"documents:read"},
	}
	if err := s.Create(role); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := s.GetByID(role.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Name != "viewer" {
		t.Errorf("GetByID() name = %v, want viewer", got.Name)
	}
	if len(got.Permissions) != 1 || got.Permissions[0] != "documents:read" {
		t.Errorf("GetByID() permissions = %v, want [documents:read]", got.Permissions)
	}
}

func TestRoleStoreGetByIDNotFound(t *testing.T) {
	s := NewRoleStore()
	_, err := s.GetByID("nonexistent")
	if err != ErrRoleNotFound {
		t.Errorf("GetByID() error = %v, want ErrRoleNotFound", err)
	}
}

func TestRoleStoreListPagination(t *testing.T) {
	s := NewRoleStore()

	// Create 10 roles for tenant-1
	for i := 0; i < 10; i++ {
		role := &models.Role{
			TenantID: "tenant-1",
			Name:     "role-" + string(rune('a'+i)),
		}
		if err := s.Create(role); err != nil {
			t.Fatalf("Create(%d) error = %v", i, err)
		}
	}

	// Create 5 roles for tenant-2
	for i := 0; i < 5; i++ {
		role := &models.Role{
			TenantID: "tenant-2",
			Name:     "role-" + string(rune('a'+i)),
		}
		if err := s.Create(role); err != nil {
			t.Fatalf("Create tenant-2(%d) error = %v", i, err)
		}
	}

	// Test page 1, pageSize 3
	roles, total, err := s.List("tenant-1", 1, 3)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 10 {
		t.Errorf("List() total = %v, want 10", total)
	}
	if len(roles) != 3 {
		t.Errorf("List() len(roles) = %v, want 3", len(roles))
	}

	// Test page 2, pageSize 3
	roles, _, err = s.List("tenant-1", 2, 3)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(roles) != 3 {
		t.Errorf("List() page2 len = %v, want 3", len(roles))
	}

	// Test page 4, pageSize 3 (last page has 1)
	roles, _, err = s.List("tenant-1", 4, 3)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(roles) != 1 {
		t.Errorf("List() page4 len = %v, want 1", len(roles))
	}

	// Test page 5 (beyond total)
	roles, _, err = s.List("tenant-1", 5, 3)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("List() page5 len = %v, want 0", len(roles))
	}

	// Tenant isolation: tenant-2 should have 5 roles
	roles, total, err = s.List("tenant-2", 1, 100)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 5 {
		t.Errorf("List() tenant-2 total = %v, want 5", total)
	}
}

func TestRoleStoreUpdate(t *testing.T) {
	s := NewRoleStore()
	role := &models.Role{
		TenantID:    "tenant-1",
		Name:        "old-name",
		Description: "old desc",
	}
	if err := s.Create(role); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := s.Update(role.ID, "new-name", "new desc", []string{"documents:read"}, false)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got.Name != "new-name" {
		t.Errorf("Update() name = %v, want new-name", got.Name)
	}
	if got.Description != "new desc" {
		t.Errorf("Update() description = %v, want new desc", got.Description)
	}
	if len(got.Permissions) != 1 || got.Permissions[0] != "documents:read" {
		t.Errorf("Update() permissions = %v, want [documents:read]", got.Permissions)
	}
	if !got.UpdatedAt.After(role.CreatedAt) {
		t.Error("Update() should update UpdatedAt")
	}
}

func TestRoleStoreDelete(t *testing.T) {
	s := NewRoleStore()
	role := &models.Role{
		TenantID: "tenant-1",
		Name:     "to-delete",
	}
	if err := s.Create(role); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := s.Delete(role.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := s.GetByID(role.ID)
	if err != ErrRoleNotFound {
		t.Errorf("GetByID() after Delete() = %v, want ErrRoleNotFound", err)
	}
}

func TestRoleStoreDeleteNotFound(t *testing.T) {
	s := NewRoleStore()
	err := s.Delete("nonexistent")
	if err != ErrRoleNotFound {
		t.Errorf("Delete() error = %v, want ErrRoleNotFound", err)
	}
}

func TestRoleStoreGetByName(t *testing.T) {
	s := NewRoleStore()
	role := &models.Role{
		TenantID:    "tenant-1",
		Name:        "editor",
		Permissions: []string{"documents:write"},
	}
	if err := s.Create(role); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := s.GetByName("tenant-1", "editor")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	if got.ID != role.ID {
		t.Errorf("GetByName() ID = %v, want %v", got.ID, role.ID)
	}
}

func TestRoleStoreIsSystem(t *testing.T) {
	s := NewRoleStore()
	fixedTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	role := &models.Role{
		TenantID: "tenant-1",
		Name:     "system-role",
		IsSystem: true,
	}
	// Simulate the Create logic for IsSystem
	if role.IsSystem {
		role.CreatedAt = fixedTime
	}

	if err := s.Create(role); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if role.CreatedAt != fixedTime {
		t.Errorf("IsSystem role CreatedAt = %v, want %v", role.CreatedAt, fixedTime)
	}
}
