package store

import (
	"fmt"
	"sync"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/models"
)

// ─────────────────────────── helpers ───────────────────────────

const (
	tenantA = "tenant-a"
	tenantB = "tenant-b"
)

func makeUser(tenantID, email, name string) *models.User {
	return &models.User{
		TenantID:    tenantID,
		Email:       email,
		DisplayName: name,
		Status:      "active",
	}
}

func makeServiceIdentity(tenantID, name string) *models.ServiceIdentity {
	return &models.ServiceIdentity{
		TenantID: tenantID,
		Name:     name,
		RoleIDs:  []string{"viewer"},
	}
}

func makeAuditEvent(tenantID, actorID, action, resourceType, resourceID string) *models.AuditEvent {
	return &models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "user",
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Result:       "success",
	}
}

func makeRole(tenantID, name string) *models.Role {
	return &models.Role{
		TenantID:    tenantID,
		Name:        name,
		Description: "test role",
		Permissions: []string{"read"},
	}
}

// ─────────────────────── UserStore ───────────────────────

func TestUserStore_CreateTenantIsolation(t *testing.T) {
	s := NewUserStore()

	alice := makeUser(tenantA, "alice@example.com", "Alice A")
	bob := makeUser(tenantB, "bob@example.com", "Bob B")

	if err := s.Create(alice); err != nil {
		t.Fatalf("Create(alice) error = %v", err)
	}
	if err := s.Create(bob); err != nil {
		t.Fatalf("Create(bob) error = %v", err)
	}

	// Both users should exist by their own IDs
	gotA, err := s.GetByID(alice.ID)
	if err != nil {
		t.Fatalf("GetByID(alice) error = %v", err)
	}
	if gotA.TenantID != tenantA {
		t.Errorf("GetByID(alice) tenant = %v, want %v", gotA.TenantID, tenantA)
	}

	gotB, err := s.GetByID(bob.ID)
	if err != nil {
		t.Fatalf("GetByID(bob) error = %v", err)
	}
	if gotB.TenantID != tenantB {
		t.Errorf("GetByID(bob) tenant = %v, want %v", gotB.TenantID, tenantB)
	}
}

func TestUserStore_ListTenantIsolation(t *testing.T) {
	s := NewUserStore()

	// 5 users in tenant A
	for i := 0; i < 5; i++ {
		user := makeUser(tenantA, fmt.Sprintf("user-a%d@example.com", i), fmt.Sprintf("User A%d", i))
		if err := s.Create(user); err != nil {
			t.Fatalf("Create(tenantA user %d) error = %v", i, err)
		}
	}

	// 3 users in tenant B
	for i := 0; i < 3; i++ {
		user := makeUser(tenantB, fmt.Sprintf("user-b%d@example.com", i), fmt.Sprintf("User B%d", i))
		if err := s.Create(user); err != nil {
			t.Fatalf("Create(tenantB user %d) error = %v", i, err)
		}
	}

	// List tenant A → expect 5
	usersA, totalA, err := s.List(tenantA, 1, 100)
	if err != nil {
		t.Fatalf("List(tenantA) error = %v", err)
	}
	if totalA != 5 {
		t.Errorf("List(tenantA) total = %v, want 5", totalA)
	}
	if len(usersA) != 5 {
		t.Fatalf("List(tenantA) returned %d users, want 5", len(usersA))
	}
	for _, u := range usersA {
		if u.TenantID != tenantA {
			t.Errorf("List(tenantA) returned user with tenant = %v, want %v", u.TenantID, tenantA)
		}
	}

	// List tenant B → expect 3
	usersB, totalB, err := s.List(tenantB, 1, 100)
	if err != nil {
		t.Fatalf("List(tenantB) error = %v", err)
	}
	if totalB != 3 {
		t.Errorf("List(tenantB) total = %v, want 3", totalB)
	}
	if len(usersB) != 3 {
		t.Fatalf("List(tenantB) returned %d users, want 3", len(usersB))
	}
	for _, u := range usersB {
		if u.TenantID != tenantB {
			t.Errorf("List(tenantB) returned user with tenant = %v, want %v", u.TenantID, tenantB)
		}
	}
}

func TestUserStore_GetByTenantAndEmail(t *testing.T) {
	s := NewUserStore()

	sameTenant := makeUser(tenantA, "shared@example.com", "Same Tenant")
	diffTenant := makeUser(tenantB, "shared@example.com", "Diff Tenant")

	if err := s.Create(sameTenant); err != nil {
		t.Fatalf("Create(sameTenant) error = %v", err)
	}
	if err := s.Create(diffTenant); err != nil {
		t.Fatalf("Create(diffTenant) error = %v", err)
	}

	// Query with tenant A → should find sameTenant
	got, err := s.GetByTenantAndEmail(tenantA, "shared@example.com")
	if err != nil {
		t.Fatalf("GetByTenantAndEmail(tenantA, shared@example.com) error = %v", err)
	}
	if got.ID != sameTenant.ID {
		t.Errorf("GetByTenantAndEmail returned user with ID %v, want %v", got.ID, sameTenant.ID)
	}
	if got.TenantID != tenantA {
		t.Errorf("GetByTenantAndEmail tenant = %v, want %v", got.TenantID, tenantA)
	}

	// Query with tenant B → should find diffTenant
	got, err = s.GetByTenantAndEmail(tenantB, "shared@example.com")
	if err != nil {
		t.Fatalf("GetByTenantAndEmail(tenantB, shared@example.com) error = %v", err)
	}
	if got.ID != diffTenant.ID {
		t.Errorf("GetByTenantAndEmail returned user with ID %v, want %v", got.ID, diffTenant.ID)
	}
	if got.TenantID != tenantB {
		t.Errorf("GetByTenantAndEmail tenant = %v, want %v", got.TenantID, tenantB)
	}

	// Wrong email for same tenant → not found
	_, err = s.GetByTenantAndEmail(tenantA, "nobody@example.com")
	if err != ErrUserNotFound {
		t.Errorf("GetByTenantAndEmail(wrong email) error = %v, want ErrUserNotFound", err)
	}
}

func TestUserStore_UpdateCrossTenantBypass(t *testing.T) {
	// Update uses GetByID (tenant-blind). The store does NOT verify
	// that the updating caller is authorized for the user's tenant.
	// We document this behavior so that handler-level tenant checks
	// can be relied upon to fill the gap.
	s := NewUserStore()

	userA := makeUser(tenantA, "target@example.com", "Target User")
	if err := s.Create(userA); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Simulate a caller from tenant B trying to update user from tenant A.
	// Since Update() is called with just the user ID, it will succeed.
	// This is expected: the handler must enforce tenant ownership.
	newName := "Updated Name"
	updates := &models.UpdateUserRequest{
		DisplayName: &newName,
	}
	got, err := s.Update(userA.ID, updates)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got.DisplayName != "Updated Name" {
		t.Errorf("Update() display_name = %v, want Updated Name", got.DisplayName)
	}
	// NOTE: UserStore.Update does NOT reject cross-tenant updates.
	// This is by design — handler middleware must enforce tenant scoping.
}

func TestUserStore_DeactivateCrossTenantBypass(t *testing.T) {
	// Similar to Update: Deactivate works by ID without tenant verification.
	// Handler-level tenant checks are the guardrail.
	s := NewUserStore()

	userA := makeUser(tenantA, "deactivate-target@example.com", "Deactivate Target")
	if err := s.Create(userA); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err := s.Deactivate(userA.ID)
	if err != nil {
		t.Fatalf("Deactivate() error = %v", err)
	}

	got, err := s.GetByID(userA.ID)
	if err != nil {
		t.Fatalf("GetByID() after deactivate error = %v", err)
	}
	if got.Status != "deactivated" {
		t.Errorf("GetByID() status = %v, want deactivated", got.Status)
	}
}

func TestUserStore_EmptyTenantID(t *testing.T) {
	s := NewUserStore()

	// Create with empty tenant → should fail
	badUser := &models.User{
		Email:       "bad@example.com",
		DisplayName: "Bad User",
	}
	if err := s.Create(badUser); err == nil {
		t.Error("Create(empty tenant) should return error")
	}

	// List with empty tenant → should return empty (no users have empty TenantID)
	users, total, err := s.List("", 1, 100)
	if err != nil {
		t.Fatalf("List(\"\") error = %v", err)
	}
	if total != 0 {
		t.Errorf("List(\"\") total = %v, want 0", total)
	}
	if len(users) != 0 {
		t.Errorf("List(\"\") returned %d users, want 0", len(users))
	}
}

func TestUserStore_ConcurrencyTenantIsolation(t *testing.T) {
	s := NewUserStore()

	var wg sync.WaitGroup

	// 50 goroutines write to tenant A
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			user := makeUser(tenantA, fmt.Sprintf("conc-a%d@example.com", idx), fmt.Sprintf("Conc A%d", idx))
			if err := s.Create(user); err != nil {
				t.Errorf("Create(conc tenantA %d) error = %v", idx, err)
			}
		}(i)
	}

	// 30 goroutines write to tenant B
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			user := makeUser(tenantB, fmt.Sprintf("conc-b%d@example.com", idx), fmt.Sprintf("Conc B%d", idx))
			if err := s.Create(user); err != nil {
				t.Errorf("Create(conc tenantB %d) error = %v", idx, err)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, total, err := s.List(tenantA, 1, 1000)
			if err != nil {
				t.Errorf("List(tenantA) concurrent error = %v", err)
				return
			}
			if total < 0 {
				t.Errorf("List(tenantA) concurrent total = %v, want >= 0", total)
			}
			_, total, err = s.List(tenantB, 1, 1000)
			if err != nil {
				t.Errorf("List(tenantB) concurrent error = %v", err)
				return
			}
			if total < 0 {
				t.Errorf("List(tenantB) concurrent total = %v, want >= 0", total)
			}
		}()
	}

	wg.Wait()

	// Final assertion
	usersA, totalA, err := s.List(tenantA, 1, 1000)
	if err != nil {
		t.Fatalf("List(tenantA) final error = %v", err)
	}
	if totalA != 50 {
		t.Errorf("List(tenantA) total = %v, want 50", totalA)
	}
	for _, u := range usersA {
		if u.TenantID != tenantA {
			t.Errorf("List(tenantA) found user with tenant = %v, want %v", u.TenantID, tenantA)
		}
	}

	usersB, totalB, err := s.List(tenantB, 1, 1000)
	if err != nil {
		t.Fatalf("List(tenantB) final error = %v", err)
	}
	if totalB != 30 {
		t.Errorf("List(tenantB) total = %v, want 30", totalB)
	}
	for _, u := range usersB {
		if u.TenantID != tenantB {
			t.Errorf("List(tenantB) found user with tenant = %v, want %v", u.TenantID, tenantB)
		}
	}
}

// ─────────────── ServiceIdentityStore ───────────────────────

func TestServiceIdentityStore_CreateTenantIsolation(t *testing.T) {
	s := NewServiceIdentityStore()

	identityA := makeServiceIdentity(tenantA, "my-service")
	identityB := makeServiceIdentity(tenantB, "my-service")

	if err := s.Create(identityA); err != nil {
		t.Fatalf("Create(tenantA) error = %v", err)
	}
	if err := s.Create(identityB); err != nil {
		t.Fatalf("Create(tenantB) error = %v", err)
	}

	// Same name in different tenants should both exist
	gotA, err := s.GetByName(tenantA, "my-service")
	if err != nil {
		t.Fatalf("GetByName(tenantA, my-service) error = %v", err)
	}
	if gotA.TenantID != tenantA {
		t.Errorf("GetByName(tenantA) tenant = %v, want %v", gotA.TenantID, tenantA)
	}

	gotB, err := s.GetByName(tenantB, "my-service")
	if err != nil {
		t.Fatalf("GetByName(tenantB, my-service) error = %v", err)
	}
	if gotB.TenantID != tenantB {
		t.Errorf("GetByName(tenantB) tenant = %v, want %v", gotB.TenantID, tenantB)
	}
}

func TestServiceIdentityStore_ListTenantIsolation(t *testing.T) {
	s := NewServiceIdentityStore()

	// 4 services for tenant A
	for i := 0; i < 4; i++ {
		identity := makeServiceIdentity(tenantA, fmt.Sprintf("svc-a%d", i))
		if err := s.Create(identity); err != nil {
			t.Fatalf("Create(tenantA svc %d) error = %v", i, err)
		}
	}

	// 3 services for tenant B
	for i := 0; i < 3; i++ {
		identity := makeServiceIdentity(tenantB, fmt.Sprintf("svc-b%d", i))
		if err := s.Create(identity); err != nil {
			t.Fatalf("Create(tenantB svc %d) error = %v", i, err)
		}
	}

	// List tenant A
	svcsA, err := s.List(tenantA)
	if err != nil {
		t.Fatalf("List(tenantA) error = %v", err)
	}
	if len(svcsA) != 4 {
		t.Fatalf("List(tenantA) count = %v, want 4", len(svcsA))
	}
	for _, svc := range svcsA {
		if svc.TenantID != tenantA {
			t.Errorf("List(tenantA) svc tenant = %v, want %v", svc.TenantID, tenantA)
		}
	}

	// List tenant B
	svcsB, err := s.List(tenantB)
	if err != nil {
		t.Fatalf("List(tenantB) error = %v", err)
	}
	if len(svcsB) != 3 {
		t.Fatalf("List(tenantB) count = %v, want 3", len(svcsB))
	}
	for _, svc := range svcsB {
		if svc.TenantID != tenantB {
			t.Errorf("List(tenantB) svc tenant = %v, want %v", svc.TenantID, tenantB)
		}
	}
}

func TestServiceIdentityStore_GetByNameCrossTenant(t *testing.T) {
	s := NewServiceIdentityStore()

	identityA := &models.ServiceIdentity{
		TenantID: tenantA,
		Name:     "shared-service",
		RoleIDs:  []string{"admin"},
	}
	identityB := &models.ServiceIdentity{
		TenantID: tenantB,
		Name:     "shared-service",
		RoleIDs:  []string{"viewer"},
	}

	if err := s.Create(identityA); err != nil {
		t.Fatalf("Create(tenantA) error = %v", err)
	}
	if err := s.Create(identityB); err != nil {
		t.Fatalf("Create(tenantB) error = %v", err)
	}

	// Each tenant should only see its own service identity
	gotA, err := s.GetByName(tenantA, "shared-service")
	if err != nil {
		t.Fatalf("GetByName(tenantA) error = %v", err)
	}
	if gotA.RoleIDs[0] != "admin" {
		t.Errorf("GetByName(tenantA) role = %v, want admin", gotA.RoleIDs[0])
	}

	gotB, err := s.GetByName(tenantB, "shared-service")
	if err != nil {
		t.Fatalf("GetByName(tenantB) error = %v", err)
	}
	if gotB.RoleIDs[0] != "viewer" {
		t.Errorf("GetByName(tenantB) role = %v, want viewer", gotB.RoleIDs[0])
	}
}

func TestServiceIdentityStore_EmptyTenantID(t *testing.T) {
	s := NewServiceIdentityStore()

	// Create with empty tenant → should fail
	badIdentity := &models.ServiceIdentity{
		Name:    "bad-service",
		RoleIDs: []string{"viewer"},
	}
	if err := s.Create(badIdentity); err == nil {
		t.Error("Create(empty tenant) should return error")
	}

	// List with empty tenant → should return empty
	svcs, err := s.List("")
	if err != nil {
		t.Fatalf("List(\"\") error = %v", err)
	}
	if len(svcs) != 0 {
		t.Errorf("List(\"\") returned %d services, want 0", len(svcs))
	}
}

func TestServiceIdentityStore_ConcurrencyTenantIsolation(t *testing.T) {
	s := NewServiceIdentityStore()

	var wg sync.WaitGroup

	// 25 goroutines write to tenant A
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			identity := makeServiceIdentity(tenantA, fmt.Sprintf("conc-svc-a%d", idx))
			if err := s.Create(identity); err != nil {
				t.Errorf("Create(conc tenantA svc %d) error = %v", idx, err)
			}
		}(i)
	}

	// 20 goroutines write to tenant B
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			identity := makeServiceIdentity(tenantB, fmt.Sprintf("conc-svc-b%d", idx))
			if err := s.Create(identity); err != nil {
				t.Errorf("Create(conc tenantB svc %d) error = %v", idx, err)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svcsA, err := s.List(tenantA)
			if err != nil {
				t.Errorf("List(tenantA) concurrent error = %v", err)
				return
			}
			if len(svcsA) < 0 {
				t.Errorf("List(tenantA) concurrent count = %v, want >= 0", len(svcsA))
			}
			svcsB, err := s.List(tenantB)
			if err != nil {
				t.Errorf("List(tenantB) concurrent error = %v", err)
				return
			}
			if len(svcsB) < 0 {
				t.Errorf("List(tenantB) concurrent count = %v, want >= 0", len(svcsB))
			}
		}()
	}

	wg.Wait()

	svcsA, err := s.List(tenantA)
	if err != nil {
		t.Fatalf("List(tenantA) final error = %v", err)
	}
	if len(svcsA) != 25 {
		t.Errorf("List(tenantA) count = %v, want 25", len(svcsA))
	}

	svcsB, err := s.List(tenantB)
	if err != nil {
		t.Fatalf("List(tenantB) final error = %v", err)
	}
	if len(svcsB) != 20 {
		t.Errorf("List(tenantB) count = %v, want 20", len(svcsB))
	}
}

// ─────────────────────── AuditStore ───────────────────────

func TestAuditStore_CreateTenantIsolation(t *testing.T) {
	s := NewAuditStore()

	eventA := makeAuditEvent(tenantA, "actor-1", "user.login", "user", "user-100")
	eventB := makeAuditEvent(tenantB, "actor-2", "user.login", "user", "user-200")

	if err := s.Create(eventA); err != nil {
		t.Fatalf("Create(eventA) error = %v", err)
	}
	if err := s.Create(eventB); err != nil {
		t.Fatalf("Create(eventB) error = %v", err)
	}

	// List tenant A → expect 1 event
	eventsA, totalA, err := s.List(tenantA, "", "", nil, nil, 100, 0)
	if err != nil {
		t.Fatalf("List(tenantA) error = %v", err)
	}
	if totalA != 1 {
		t.Errorf("List(tenantA) total = %v, want 1", totalA)
	}
	if len(eventsA) != 1 {
		t.Fatalf("List(tenantA) returned %d events, want 1", len(eventsA))
	}
	if eventsA[0].TenantID != tenantA {
		t.Errorf("List(tenantA) event tenant = %v, want %v", eventsA[0].TenantID, tenantA)
	}

	// List tenant B → expect 1 event
	eventsB, totalB, err := s.List(tenantB, "", "", nil, nil, 100, 0)
	if err != nil {
		t.Fatalf("List(tenantB) error = %v", err)
	}
	if totalB != 1 {
		t.Errorf("List(tenantB) total = %v, want 1", totalB)
	}
	if len(eventsB) != 1 {
		t.Fatalf("List(tenantB) returned %d events, want 1", len(eventsB))
	}
	if eventsB[0].TenantID != tenantB {
		t.Errorf("List(tenantB) event tenant = %v, want %v", eventsB[0].TenantID, tenantB)
	}
}

func TestAuditStore_CreateWithTenant(t *testing.T) {
	s := NewAuditStore()

	// Create an event with tenantA in it
	event := makeAuditEvent(tenantA, "actor-1", "user.delete", "user", "user-100")

	// Call CreateWithTenant with tenantB — tenant from parameter should override
	if err := s.CreateWithTenant(event, tenantB); err != nil {
		t.Fatalf("CreateWithTenant(tenantB) error = %v", err)
	}

	// The event should now have tenantB, NOT tenantA
	if event.TenantID != tenantB {
		t.Errorf("event.TenantID = %v, want %v (overridden by CreateWithTenant)", event.TenantID, tenantB)
	}

	// Listing tenantA should NOT find this event
	_, totalA, err := s.List(tenantA, "", "", nil, nil, 100, 0)
	if err != nil {
		t.Fatalf("List(tenantA) error = %v", err)
	}
	if totalA != 0 {
		t.Errorf("List(tenantA) total = %v, want 0 (CreateWithTenant overrides tenant)", totalA)
	}

	// Listing tenantB SHOULD find this event
	eventsB, totalB, err := s.List(tenantB, "", "", nil, nil, 100, 0)
	if err != nil {
		t.Fatalf("List(tenantB) error = %v", err)
	}
	if totalB != 1 {
		t.Errorf("List(tenantB) total = %v, want 1 (CreateWithTenant overrides tenant)", totalB)
	}
	if len(eventsB) != 1 || eventsB[0].TenantID != tenantB {
		t.Errorf("CreateWithTenant event not in tenantB list")
	}
}

func TestAuditStore_EmptyTenantID(t *testing.T) {
	s := NewAuditStore()

	// CreateWithTenant with empty tenant → should fail
	event := makeAuditEvent(tenantA, "actor-1", "test", "resource", "res-1")
	if err := s.CreateWithTenant(event, ""); err == nil {
		t.Error("CreateWithTenant(empty tenant) should return error")
	}

	// Create with empty tenant → should fail
	event2 := makeAuditEvent("", "actor-1", "test", "resource", "res-1")
	if err := s.Create(event2); err == nil {
		t.Error("Create(empty tenant) should return error")
	}
}

func TestAuditStore_CrossTenantNoLeak(t *testing.T) {
	s := NewAuditStore()

	// Create 5 events for tenant A
	for i := 0; i < 5; i++ {
		event := makeAuditEvent(tenantA, fmt.Sprintf("actor-a%d", i), "user.login", "user", fmt.Sprintf("user-a%d", i))
		if err := s.Create(event); err != nil {
			t.Fatalf("Create(tenantA event %d) error = %v", i, err)
		}
	}

	// Create 3 events for tenant B
	for i := 0; i < 3; i++ {
		event := makeAuditEvent(tenantB, fmt.Sprintf("actor-b%d", i), "user.login", "user", fmt.Sprintf("user-b%d", i))
		if err := s.Create(event); err != nil {
			t.Fatalf("Create(tenantB event %d) error = %v", i, err)
		}
	}

	// Verify tenant A list has no tenant B events
	eventsA, totalA, err := s.List(tenantA, "", "", nil, nil, 100, 0)
	if err != nil {
		t.Fatalf("List(tenantA) error = %v", err)
	}
	if totalA != 5 {
		t.Errorf("List(tenantA) total = %v, want 5", totalA)
	}
	for _, e := range eventsA {
		if e.TenantID != tenantA {
			t.Errorf("List(tenantA) found event with tenant = %v, want %v", e.TenantID, tenantA)
		}
	}

	// Verify tenant B list has no tenant A events
	eventsB, totalB, err := s.List(tenantB, "", "", nil, nil, 100, 0)
	if err != nil {
		t.Fatalf("List(tenantB) error = %v", err)
	}
	if totalB != 3 {
		t.Errorf("List(tenantB) total = %v, want 3", totalB)
	}
	for _, e := range eventsB {
		if e.TenantID != tenantB {
			t.Errorf("List(tenantB) found event with tenant = %v, want %v", e.TenantID, tenantB)
		}
	}
}

func TestAuditStore_CreateWithTenantEmptyOverride(t *testing.T) {
	s := NewAuditStore()

	event := makeAuditEvent(tenantA, "actor-1", "user.login", "user", "user-100")

	// Try to override with empty tenant → should fail
	err := s.CreateWithTenant(event, "")
	if err == nil {
		t.Error("CreateWithTenant(empty tenant) should return error")
	}
	if event.TenantID != tenantA {
		t.Error("event.TenantID should remain unchanged after failed CreateWithTenant")
	}
}

func TestAuditStore_ConcurrencyTenantIsolation(t *testing.T) {
	s := NewAuditStore()

	var wg sync.WaitGroup

	// 30 goroutines write events to tenant A
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := makeAuditEvent(tenantA, fmt.Sprintf("conc-actor-a%d", idx), "user.login", "user", fmt.Sprintf("conc-user-a%d", idx))
			if err := s.Create(event); err != nil {
				t.Errorf("Create(conc tenantA event %d) error = %v", idx, err)
			}
		}(i)
	}

	// 20 goroutines write events to tenant B
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := makeAuditEvent(tenantB, fmt.Sprintf("conc-actor-b%d", idx), "user.login", "user", fmt.Sprintf("conc-user-b%d", idx))
			if err := s.Create(event); err != nil {
				t.Errorf("Create(conc tenantB event %d) error = %v", idx, err)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, total, err := s.List(tenantA, "", "", nil, nil, 1000, 0)
			if err != nil {
				t.Errorf("List(tenantA) concurrent error = %v", err)
				return
			}
			if total < 0 {
				t.Errorf("List(tenantA) concurrent total = %v, want >= 0", total)
			}
			_, total, err = s.List(tenantB, "", "", nil, nil, 1000, 0)
			if err != nil {
				t.Errorf("List(tenantB) concurrent error = %v", err)
				return
			}
			if total < 0 {
				t.Errorf("List(tenantB) concurrent total = %v, want >= 0", total)
			}
		}()
	}

	wg.Wait()

	_, totalA, err := s.List(tenantA, "", "", nil, nil, 1000, 0)
	if err != nil {
		t.Fatalf("List(tenantA) final error = %v", err)
	}
	if totalA != 30 {
		t.Errorf("List(tenantA) total = %v, want 30", totalA)
	}

	_, totalB, err := s.List(tenantB, "", "", nil, nil, 1000, 0)
	if err != nil {
		t.Fatalf("List(tenantB) final error = %v", err)
	}
	if totalB != 20 {
		t.Errorf("List(tenantB) total = %v, want 20", totalB)
	}
}

// ─────────────────────── RoleStore ───────────────────────

func TestRoleStore_CreateTenantIsolation(t *testing.T) {
	s := NewRoleStore()

	roleA := makeRole(tenantA, "admin")
	roleB := makeRole(tenantB, "admin")

	if err := s.Create(roleA); err != nil {
		t.Fatalf("Create(tenantA) error = %v", err)
	}
	if err := s.Create(roleB); err != nil {
		t.Fatalf("Create(tenantB) error = %v", err)
	}

	// Same name in different tenants should both exist
	gotA, err := s.GetByName(tenantA, "admin")
	if err != nil {
		t.Fatalf("GetByName(tenantA, admin) error = %v", err)
	}
	if gotA.TenantID != tenantA {
		t.Errorf("GetByName(tenantA) tenant = %v, want %v", gotA.TenantID, tenantA)
	}

	gotB, err := s.GetByName(tenantB, "admin")
	if err != nil {
		t.Fatalf("GetByName(tenantB, admin) error = %v", err)
	}
	if gotB.TenantID != tenantB {
		t.Errorf("GetByName(tenantB) tenant = %v, want %v", gotB.TenantID, tenantB)
	}
}

func TestRoleStore_ListTenantIsolation(t *testing.T) {
	s := NewRoleStore()

	// 5 roles for tenant A
	for i := 0; i < 5; i++ {
		role := makeRole(tenantA, fmt.Sprintf("role-a%d", i))
		if err := s.Create(role); err != nil {
			t.Fatalf("Create(tenantA role %d) error = %v", i, err)
		}
	}

	// 3 roles for tenant B
	for i := 0; i < 3; i++ {
		role := makeRole(tenantB, fmt.Sprintf("role-b%d", i))
		if err := s.Create(role); err != nil {
			t.Fatalf("Create(tenantB role %d) error = %v", i, err)
		}
	}

	rolesA, totalA, err := s.List(tenantA, 1, 100)
	if err != nil {
		t.Fatalf("List(tenantA) error = %v", err)
	}
	if totalA != 5 {
		t.Errorf("List(tenantA) total = %v, want 5", totalA)
	}
	if len(rolesA) != 5 {
		t.Fatalf("List(tenantA) returned %d roles, want 5", len(rolesA))
	}
	for _, r := range rolesA {
		if r.TenantID != tenantA {
			t.Errorf("List(tenantA) role tenant = %v, want %v", r.TenantID, tenantA)
		}
	}

	rolesB, totalB, err := s.List(tenantB, 1, 100)
	if err != nil {
		t.Fatalf("List(tenantB) error = %v", err)
	}
	if totalB != 3 {
		t.Errorf("List(tenantB) total = %v, want 3", totalB)
	}
	if len(rolesB) != 3 {
		t.Fatalf("List(tenantB) returned %d roles, want 3", len(rolesB))
	}
	for _, r := range rolesB {
		if r.TenantID != tenantB {
			t.Errorf("List(tenantB) role tenant = %v, want %v", r.TenantID, tenantB)
		}
	}
}

func TestRoleStore_GetByIDGlobal(t *testing.T) {
	// GetByID is global — it does NOT filter by tenant, which is
	// intentional because role IDs are unique within the store.
	// A caller that knows a role ID can retrieve it regardless of tenant.
	s := NewRoleStore()

	roleA := makeRole(tenantA, "global-role")
	if err := s.Create(roleA); err != nil {
		t.Fatalf("Create(tenantA) error = %v", err)
	}

	// GetByID should work for a role in a different tenant than the caller
	// This is by design: the role itself carries the tenant, so callers
	// can validate tenant ownership at the handler level.
	got, err := s.GetByID(roleA.ID)
	if err != nil {
		t.Fatalf("GetByID(roleA.ID) error = %v", err)
	}
	if got.TenantID != tenantA {
		t.Errorf("GetByID tenant = %v, want %v", got.TenantID, tenantA)
	}
}

func TestRoleStore_EmptyTenantID(t *testing.T) {
	s := NewRoleStore()

	// Create with empty tenant → should fail
	badRole := &models.Role{
		Name:        "bad-role",
		Description: "bad",
		Permissions: []string{"read"},
	}
	if err := s.Create(badRole); err == nil {
		t.Error("Create(empty tenant) should return error")
	}

	// List with empty tenant → should return empty
	roles, total, err := s.List("", 1, 100)
	if err != nil {
		t.Fatalf("List(\"\") error = %v", err)
	}
	if total != 0 {
		t.Errorf("List(\"\") total = %v, want 0", total)
	}
	if len(roles) != 0 {
		t.Errorf("List(\"\") returned %d roles, want 0", len(roles))
	}
}

func TestRoleStore_ConcurrencyTenantIsolation(t *testing.T) {
	s := NewRoleStore()

	var wg sync.WaitGroup

	// 20 goroutines write roles to tenant A
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			role := makeRole(tenantA, fmt.Sprintf("conc-role-a%d", idx))
			if err := s.Create(role); err != nil {
				t.Errorf("Create(conc tenantA role %d) error = %v", idx, err)
			}
		}(i)
	}

	// 15 goroutines write roles to tenant B
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			role := makeRole(tenantB, fmt.Sprintf("conc-role-b%d", idx))
			if err := s.Create(role); err != nil {
				t.Errorf("Create(conc tenantB role %d) error = %v", idx, err)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, total, err := s.List(tenantA, 1, 1000)
			if err != nil {
				t.Errorf("List(tenantA) concurrent error = %v", err)
				return
			}
			if total < 0 {
				t.Errorf("List(tenantA) concurrent total = %v, want >= 0", total)
			}
			_, total, err = s.List(tenantB, 1, 1000)
			if err != nil {
				t.Errorf("List(tenantB) concurrent error = %v", err)
				return
			}
			if total < 0 {
				t.Errorf("List(tenantB) concurrent total = %v, want >= 0", total)
			}
		}()
	}

	wg.Wait()

	_, totalA, err := s.List(tenantA, 1, 1000)
	if err != nil {
		t.Fatalf("List(tenantA) final error = %v", err)
	}
	if totalA != 20 {
		t.Errorf("List(tenantA) total = %v, want 20", totalA)
	}

	_, totalB, err := s.List(tenantB, 1, 1000)
	if err != nil {
		t.Fatalf("List(tenantB) final error = %v", err)
	}
	if totalB != 15 {
		t.Errorf("List(tenantB) total = %v, want 15", totalB)
	}
}

// ─────────────────────── Mixed Store Isolation ───────────────────────

func TestMixedStore_CrossTenantIsolation(t *testing.T) {
	// Verify that across all stores, tenant boundaries are strict.
	userStore := NewUserStore()
	svcStore := NewServiceIdentityStore()
	auditStore := NewAuditStore()
	roleStore := NewRoleStore()

	// Create one of each for tenant A
	userA := makeUser(tenantA, "mixed-a@example.com", "Mixed A")
	if err := userStore.Create(userA); err != nil {
		t.Fatalf("Create user error = %v", err)
	}

	svcA := makeServiceIdentity(tenantA, "mixed-svc")
	if err := svcStore.Create(svcA); err != nil {
		t.Fatalf("Create service error = %v", err)
	}

	eventA := makeAuditEvent(tenantA, "mixed-actor", "user.login", "user", "mixed-user")
	if err := auditStore.Create(eventA); err != nil {
		t.Fatalf("Create audit event error = %v", err)
	}

	roleA := makeRole(tenantA, "mixed-role")
	if err := roleStore.Create(roleA); err != nil {
		t.Fatalf("Create role error = %v", err)
	}

	// Create one of each for tenant B
	userB := makeUser(tenantB, "mixed-b@example.com", "Mixed B")
	if err := userStore.Create(userB); err != nil {
		t.Fatalf("Create user error = %v", err)
	}

	svcB := makeServiceIdentity(tenantB, "mixed-svc")
	if err := svcStore.Create(svcB); err != nil {
		t.Fatalf("Create service error = %v", err)
	}

	eventB := makeAuditEvent(tenantB, "mixed-actor", "user.login", "user", "mixed-user")
	if err := auditStore.Create(eventB); err != nil {
		t.Fatalf("Create audit event error = %v", err)
	}

	roleB := makeRole(tenantB, "mixed-role")
	if err := roleStore.Create(roleB); err != nil {
		t.Fatalf("Create role error = %v", err)
	}

	// Verify each store's tenant isolation
	usersA, totalA, _ := userStore.List(tenantA, 1, 100)
	if len(usersA) != 1 || totalA != 1 {
		t.Errorf("UserStore: tenantA count = %d (total=%d), want 1", len(usersA), totalA)
	}

	usersB, totalB, _ := userStore.List(tenantB, 1, 100)
	if len(usersB) != 1 || totalB != 1 {
		t.Errorf("UserStore: tenantB count = %d (total=%d), want 1", len(usersB), totalB)
	}

	svcsA, _ := svcStore.List(tenantA)
	if len(svcsA) != 1 {
		t.Errorf("ServiceIdentityStore: tenantA count = %d, want 1", len(svcsA))
	}

	svcsB, _ := svcStore.List(tenantB)
	if len(svcsB) != 1 {
		t.Errorf("ServiceIdentityStore: tenantB count = %d, want 1", len(svcsB))
	}

	eventsA, totalA, _ := auditStore.List(tenantA, "", "", nil, nil, 100, 0)
	if len(eventsA) != 1 || totalA != 1 {
		t.Errorf("AuditStore: tenantA count = %d (total=%d), want 1", len(eventsA), totalA)
	}

	eventsB, totalB, _ := auditStore.List(tenantB, "", "", nil, nil, 100, 0)
	if len(eventsB) != 1 || totalB != 1 {
		t.Errorf("AuditStore: tenantB count = %d (total=%d), want 1", len(eventsB), totalB)
	}

	rolesA, totalA, _ := roleStore.List(tenantA, 1, 100)
	if len(rolesA) != 1 || totalA != 1 {
		t.Errorf("RoleStore: tenantA count = %d (total=%d), want 1", len(rolesA), totalA)
	}

	rolesB, totalB, _ := roleStore.List(tenantB, 1, 100)
	if len(rolesB) != 1 || totalB != 1 {
		t.Errorf("RoleStore: tenantB count = %d (total=%d), want 1", len(rolesB), totalB)
	}
}
