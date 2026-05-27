package store

import (
	"testing"

	"github.com/operan/modules/02-identity-access/internal/models"
)

func TestServiceIdentityStoreCreate(t *testing.T) {
	s := NewServiceIdentityStore()

	identity := &models.ServiceIdentity{
		TenantID: "tenant-1",
		Name:     "my-service",
		Roles:    []string{"admin"},
	}

	if err := s.Create(identity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if identity.ID == "" {
		t.Error("Create() should auto-generate identity ID")
	}
	if identity.APIKeyID == "" {
		t.Error("Create() should auto-generate API key ID")
	}
	if len(identity.Roles) != 1 {
		t.Errorf("Create() roles = %v, want [admin]", identity.Roles)
	}
}

func TestServiceIdentityStoreCreateDuplicate(t *testing.T) {
	s := NewServiceIdentityStore()

	identity1 := &models.ServiceIdentity{
		TenantID: "tenant-1",
		Name:     "my-service",
		Roles:    []string{"admin"},
	}
	if err := s.Create(identity1); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	identity2 := &models.ServiceIdentity{
		TenantID: "tenant-1",
		Name:     "my-service",
		Roles:    []string{"viewer"},
	}
	err := s.Create(identity2)
	if err == nil {
		t.Error("Create() should return error for duplicate service name in same tenant")
	}
}

func TestServiceIdentityStoreCreateMissingFields(t *testing.T) {
	s := NewServiceIdentityStore()

	// Missing name
	identity1 := &models.ServiceIdentity{TenantID: "tenant-1"}
	if err := s.Create(identity1); err == nil {
		t.Error("Create() should error when name is empty")
	}

	// Missing tenant_id
	identity2 := &models.ServiceIdentity{Name: "service-1"}
	if err := s.Create(identity2); err == nil {
		t.Error("Create() should error when tenant_id is empty")
	}
}

func TestServiceIdentityStoreGetByID(t *testing.T) {
	s := NewServiceIdentityStore()

	identity := &models.ServiceIdentity{
		TenantID: "tenant-1",
		Name:     "get-service",
		Roles:    []string{"admin", "editor"},
	}

	if err := s.Create(identity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := s.GetByID(identity.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Name != "get-service" {
		t.Errorf("GetByID() name = %v, want get-service", got.Name)
	}
	if len(got.Roles) != 2 {
		t.Errorf("GetByID() roles = %v, want [admin editor]", got.Roles)
	}
}

func TestServiceIdentityStoreGetByIDNotFound(t *testing.T) {
	s := NewServiceIdentityStore()
	_, err := s.GetByID("nonexistent")
	if err != ErrServiceIdentityNotFound {
		t.Errorf("GetByID() error = %v, want ErrServiceIdentityNotFound", err)
	}
}

func TestServiceIdentityStoreGetByName(t *testing.T) {
	s := NewServiceIdentityStore()

	identity := &models.ServiceIdentity{
		TenantID: "tenant-1",
		Name:     "named-service",
		Roles:    []string{"viewer"},
	}

	if err := s.Create(identity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := s.GetByName("tenant-1", "named-service")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	if got.Name != "named-service" {
		t.Errorf("GetByName() name = %v, want named-service", got.Name)
	}
}

func TestServiceIdentityStoreGetByNameNotFound(t *testing.T) {
	s := NewServiceIdentityStore()
	_, err := s.GetByName("tenant-1", "nonexistent")
	if err != ErrServiceIdentityNotFound {
		t.Errorf("GetByName() error = %v, want ErrServiceIdentityNotFound", err)
	}
}

func TestServiceIdentityStoreList(t *testing.T) {
	s := NewServiceIdentityStore()

	// Create 4 services for tenant-1
	for i := 0; i < 4; i++ {
		identity := &models.ServiceIdentity{
			TenantID: "tenant-1",
			Name:     "service-" + string(rune('0'+i)),
			Roles:    []string{"admin"},
		}
		if err := s.Create(identity); err != nil {
			t.Fatalf("Create(%d) error = %v", i, err)
		}
	}

	// Create 2 services for tenant-2
	for i := 0; i < 2; i++ {
		identity := &models.ServiceIdentity{
			TenantID: "tenant-2",
			Name:     "tenant2-service-" + string(rune('a'+i)),
			Roles:    []string{"viewer"},
		}
		if err := s.Create(identity); err != nil {
			t.Fatalf("Create tenant-2(%d) error = %v", i, err)
		}
	}

	// List tenant-1 services
	services, err := s.List("tenant-1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(services) != 4 {
		t.Errorf("List() tenant-1 count = %v, want 4", len(services))
	}

	// List tenant-2 services
	services, err = s.List("tenant-2")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(services) != 2 {
		t.Errorf("List() tenant-2 count = %v, want 2", len(services))
	}
}

func TestServiceIdentityStoreListEmpty(t *testing.T) {
	s := NewServiceIdentityStore()

	services, err := s.List("nonexistent")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(services) != 0 {
		t.Errorf("List() count = %v, want 0", len(services))
	}
}

func TestServiceIdentityStoreUpdateLastUsed(t *testing.T) {
	s := NewServiceIdentityStore()

	identity := &models.ServiceIdentity{
		TenantID: "tenant-1",
		Name:     "last-used-service",
		Roles:    []string{"admin"},
	}

	if err := s.Create(identity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := s.UpdateLastUsed(identity.ID); err != nil {
		t.Fatalf("UpdateLastUsed() error = %v", err)
	}

	got, err := s.GetByID(identity.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.LastUsedAt == nil {
		t.Error("UpdateLastUsed() should set LastUsedAt")
	}
}

func TestServiceIdentityStoreUpdateLastUsedNotFound(t *testing.T) {
	s := NewServiceIdentityStore()
	err := s.UpdateLastUsed("nonexistent")
	if err != ErrServiceIdentityNotFound {
		t.Errorf("UpdateLastUsed() error = %v, want ErrServiceIdentityNotFound", err)
	}
}

func TestServiceIdentityStoreRevokeAPIKey(t *testing.T) {
	s := NewServiceIdentityStore()

	identity := &models.ServiceIdentity{
		TenantID: "tenant-1",
		Name:     "revoke-service",
		Roles:    []string{"admin"},
	}

	if err := s.Create(identity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if identity.APIKeyID == "" {
		t.Error("Create() should generate APIKeyID")
	}

	err := s.RevokeAPIKey(identity.ID)
	if err != nil {
		t.Fatalf("RevokeAPIKey() error = %v", err)
	}

	got, err := s.GetByID(identity.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.APIKeyID != "" {
		t.Errorf("RevokeAPIKey() APIKeyID = %v, want empty string", got.APIKeyID)
	}
}

func TestServiceIdentityStoreRevokeAPIKeyNotFound(t *testing.T) {
	s := NewServiceIdentityStore()
	err := s.RevokeAPIKey("nonexistent")
	if err != ErrServiceIdentityNotFound {
		t.Errorf("RevokeAPIKey() error = %v, want ErrServiceIdentityNotFound", err)
	}
}

func TestServiceIdentityStoreCreateStoresRolesJSON(t *testing.T) {
	s := NewServiceIdentityStore()

	identity := &models.ServiceIdentity{
		TenantID: "tenant-1",
		Name:     "json-roles-service",
		Roles:    []string{"admin", "editor", "viewer"},
	}

	if err := s.Create(identity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if identity.RolesJSON == "" {
		t.Error("Create() should store roles as JSON")
	}
}
