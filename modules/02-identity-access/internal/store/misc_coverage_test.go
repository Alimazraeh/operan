package store

import (
	"testing"

	"github.com/operan/modules/02-identity-access/internal/models"
)

func TestAuditStore_GetByID(t *testing.T) {
	s := NewAuditStore()
	evt := &models.AuditEvent{
		TenantID:     "tenant-1",
		ActorID:      "user-1",
		ActorType:    "user",
		Action:       "login",
		ResourceType: "session",
		ResourceID:   "sess-1",
		Result:       "success",
		Details:      map[string]interface{}{"ip": "10.0.0.1"},
	}
	if err := s.Create(evt); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := s.GetByID(evt.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Action != "login" || got.Details["ip"] != "10.0.0.1" {
		t.Errorf("GetByID() roundtrip mismatch: %+v", got)
	}

	if _, err := s.GetByID("does-not-exist"); err == nil {
		t.Error("GetByID() should error for missing event")
	}
}

func TestParseFilter(t *testing.T) {
	if got := ParseFilter(""); len(got) != 0 {
		t.Errorf("ParseFilter(empty) = %v, want empty", got)
	}

	got := ParseFilter("action=login&result=success&malformed")
	if got["action"] != "login" || got["result"] != "success" {
		t.Errorf("ParseFilter() = %v", got)
	}
	if _, ok := got["malformed"]; ok {
		t.Error("ParseFilter() should skip pairs without '='")
	}
}

func TestUserStore_GetByActorID(t *testing.T) {
	s := NewUserStore()
	user := &models.User{
		TenantID:    "tenant-1",
		Email:       "actor@example.com",
		DisplayName: "Actor",
		Status:      "active",
	}
	if err := s.Create(user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// GetByActorID is an alias for GetByTenantAndEmail.
	got, err := s.GetByActorID("tenant-1", "actor@example.com")
	if err != nil {
		t.Fatalf("GetByActorID() error = %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("GetByActorID() returned wrong user: %s", got.ID)
	}

	if _, err := s.GetByActorID("tenant-1", "missing@example.com"); err == nil {
		t.Error("GetByActorID() should error for unknown actor")
	}
}
