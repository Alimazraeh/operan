package store

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

const testTenantID = "tenant-1"

// ─── TemplateStore Tests ─────────────────────────────────────────────────────

func TestTemplateStore_Create_GetByID(t *testing.T) {
	store := NewTemplateStore()

	tmpl := &Template{
		TenantID: testTenantID,
		Name:     "Test Template",
		Category: "engineering",
	}

	created, err := store.Create(tmpl)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if created.ID == "" {
		t.Fatal("expected template ID to be set after Create")
	}

	byID, err := store.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if byID.Name != "Test Template" {
		t.Errorf("expected name 'Test Template', got %s", byID.Name)
	}
}

func TestTemplateStore_List(t *testing.T) {
	store := NewTemplateStore()

	// Create 5 templates
	for i := 0; i < 5; i++ {
		name := "Template " + string(rune('A'+i))
		cat := "engineering"
		if i%2 == 0 {
			cat = "sales"
		}
		tmpl := &Template{
			TenantID: testTenantID,
			Name:     name,
			Category: cat,
		}
		if _, err := store.Create(tmpl); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// List all
	list, total, hasMore := store.List(testTenantID, 1, 100, nil)
	if len(list) != 5 {
		t.Errorf("expected 5 templates, got %d", len(list))
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if hasMore {
		t.Error("expected hasMore false")
	}

	// Pagination
	list, total, hasMore = store.List(testTenantID, 1, 2, nil)
	if len(list) != 2 {
		t.Errorf("expected 2 templates on page 1, got %d", len(list))
	}
	if !hasMore {
		t.Error("expected hasMore true")
	}

	list, _, hasMore = store.List(testTenantID, 2, 2, nil)
	if len(list) != 2 {
		t.Errorf("expected 2 templates on page 2, got %d", len(list))
	}

	list, _, hasMore = store.List(testTenantID, 3, 2, nil)
	if len(list) != 1 {
		t.Errorf("expected 1 template on page 3, got %d", len(list))
	}
	if hasMore {
		t.Error("expected hasMore false on last page")
	}
}

func TestTemplateStore_List_CategoryFilter(t *testing.T) {
	store := NewTemplateStore()

	// Create mixed templates
	for _, cat := range []string{"engineering", "sales", "engineering"} {
		tmpl := &Template{
			TenantID: testTenantID,
			Name:     "Template for " + cat,
			Category: cat,
		}
		if _, err := store.Create(tmpl); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	engFilter := "engineering"
	list, total, hasMore := store.List(testTenantID, 1, 100, &engFilter)
	if len(list) != 2 {
		t.Errorf("expected 2 engineering templates, got %d", len(list))
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if hasMore {
		t.Error("expected hasMore false")
	}
}

func TestTemplateStore_Update(t *testing.T) {
	store := NewTemplateStore()

	tmpl, err := store.Create(&Template{
		TenantID: testTenantID,
		Name:     "Original",
		Category: "engineering",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updates := map[string]interface{}{
		"name": "Updated",
	}

	updated, err := store.UpdateByTenant(tmpl.ID, testTenantID, updates)
	if err != nil {
		t.Fatalf("UpdateByTenant failed: %v", err)
	}

	if updated.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %s", updated.Name)
	}
	if updated.Category != "engineering" {
		t.Error("expected category to remain 'engineering' after partial update")
	}
}

func TestTemplateStore_Delete(t *testing.T) {
	store := NewTemplateStore()

	tmpl, err := store.Create(&Template{
		TenantID: testTenantID,
		Name:     "ToDelete",
		Category: "engineering",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := store.Delete(tmpl.ID, testTenantID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.GetByID(tmpl.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestTemplateStore_GetByID_NotFound(t *testing.T) {
	store := NewTemplateStore()

	_, err := store.GetByID("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent template, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTemplateStore_Delete_NotFound(t *testing.T) {
	store := NewTemplateStore()

	err := store.Delete("nonexistent-id", testTenantID)
	if err == nil {
		t.Fatal("expected error for nonexistent template, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ─── CustomTemplateStore Tests ───────────────────────────────────────────────

func TestCustomTemplateStore_Create_Get(t *testing.T) {
	store := NewCustomTemplateStore()

	ct, err := store.Create(&CustomTemplate{
		TenantID: testTenantID,
		Name:     "Custom Template",
		Category: "sales",
		Content:  map[string]interface{}{"field": "value"},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	byID, err := store.GetByID(ct.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if byID.Name != "Custom Template" {
		t.Errorf("expected name 'Custom Template', got %s", byID.Name)
	}
}

func TestCustomTemplateStore_Delete(t *testing.T) {
	store := NewCustomTemplateStore()

	ct, err := store.Create(&CustomTemplate{
		TenantID: testTenantID,
		Name:     "ToDelete",
		Category: "sales",
		Content:  map[string]interface{}{"delete": true},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := store.Delete(ct.ID, testTenantID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.GetByID(ct.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestCustomTemplateStore_Update(t *testing.T) {
	store := NewCustomTemplateStore()

	ct, err := store.Create(&CustomTemplate{
		TenantID: testTenantID,
		Name:     "Original",
		Category: "sales",
		Content:  map[string]interface{}{"before": true},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updates := map[string]interface{}{
		"name": "Updated",
	}

	updated, err := store.UpdateByTenant(ct.ID, testTenantID, updates)
	if err != nil {
		t.Fatalf("UpdateByTenant failed: %v", err)
	}

	if updated.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %s", updated.Name)
	}
}

func TestCustomTemplateStore_List(t *testing.T) {
	store := NewCustomTemplateStore()

	for i := 0; i < 3; i++ {
		ct := &CustomTemplate{
			TenantID: testTenantID,
			Name:     "Custom " + string(rune('A'+i)),
			Category: "sales",
			Content:  map[string]interface{}{"index": i},
		}
		if _, err := store.Create(ct); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	list, total, hasMore := store.List(testTenantID, 1, 100, nil)
	if len(list) != 3 {
		t.Errorf("expected 3 custom templates, got %d", len(list))
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if hasMore {
		t.Error("expected hasMore false")
	}
}

func TestCustomTemplateStore_Get_NotFound(t *testing.T) {
	store := NewCustomTemplateStore()

	_, err := store.GetByID("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent template, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ─── DeploymentStore Tests ───────────────────────────────────────────────────

func TestDeploymentStore_Create_ListByTemplate(t *testing.T) {
	store := NewDeploymentStore()

	tmplID1 := uuid.New().String()
	tmplID2 := uuid.New().String()

	dep1, err := store.Create(&TemplateDeployment{
		TenantID:  testTenantID,
		TemplateID: tmplID1,
		Environment: "production",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	dep2, err := store.Create(&TemplateDeployment{
		TenantID:  testTenantID,
		TemplateID: tmplID2,
		Environment: "staging",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	_ = dep2

	// Second deployment for same template
	_, err = store.Create(&TemplateDeployment{
		TenantID:  testTenantID,
		TemplateID: tmplID1,
		Environment: "development",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// List deployments for tmplID1
	list, total, hasMore := store.ListByTemplate(tmplID1, testTenantID, 1, 100)
	if len(list) != 2 {
		t.Errorf("expected 2 deployments for template %s, got %d", tmplID1, len(list))
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if hasMore {
		t.Error("expected hasMore false")
	}

	// Verify dep1 is in the list
	found := false
	for _, d := range list {
		if d.ID == dep1.ID && d.Environment == "production" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected deployment in production environment in list")
	}

	// List deployments for tmplID2
	list, total, _ = store.ListByTemplate(tmplID2, testTenantID, 1, 100)
	if len(list) != 1 {
		t.Errorf("expected 1 deployment for template %s, got %d", tmplID2, len(list))
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}

func TestDeploymentStore_UpdateStatus(t *testing.T) {
	store := NewDeploymentStore()

	dep, err := store.Create(&TemplateDeployment{
		TenantID:  testTenantID,
		TemplateID: uuid.New().String(),
		Environment: "production",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updated, err := store.UpdateStatus(dep.ID, "deployed", "user-1")
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	if updated.Status != "deployed" {
		t.Errorf("expected status 'deployed', got %s", updated.Status)
	}
	if updated.DeployedBy != "user-1" {
		t.Errorf("expected DeployedBy 'user-1', got %s", updated.DeployedBy)
	}

	// Update to rolled_back
	updated, err = store.UpdateStatus(dep.ID, "rolled_back", "user-1")
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	if updated.Status != "rolled_back" {
		t.Errorf("expected status 'rolled_back', got %s", updated.Status)
	}
}

func TestDeploymentStore_Get_NotFound(t *testing.T) {
	store := NewDeploymentStore()

	_, err := store.GetByID("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent deployment, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeploymentStore_ListByTemplate_Empty(t *testing.T) {
	store := NewDeploymentStore()

	list, total, hasMore := store.ListByTemplate("nonexistent-template", testTenantID, 1, 100)
	if len(list) != 0 {
		t.Errorf("expected 0 deployments, got %d", len(list))
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
	if hasMore {
		t.Error("expected hasMore false")
	}
}

// ─── VersionStore Tests ──────────────────────────────────────────────────────

func TestVersionStore_Create_ListByTemplate(t *testing.T) {
	store := NewVersionStore()

	tmplID := uuid.New().String()

	v1, err := store.Create(&TemplateVersion{
		TenantID: testTenantID,
		TemplateID: tmplID,
		Version:    "1.0.0",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	v2, err := store.Create(&TemplateVersion{
		TenantID: testTenantID,
		TemplateID: tmplID,
		Version:    "2.0.0",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	list := store.ListByTemplate(tmplID, testTenantID)
	if len(list) != 2 {
		t.Errorf("expected 2 versions, got %d", len(list))
	}

	// Verify order (should be in creation order)
	if list[0].ID != v1.ID {
		t.Error("expected v1 to be first in list")
	}
	if list[1].ID != v2.ID {
		t.Error("expected v2 to be second in list")
	}
}

func TestVersionStore_GetByVersion(t *testing.T) {
	store := NewVersionStore()

	tmplID := uuid.New().String()

	_, err := store.Create(&TemplateVersion{
		TenantID: testTenantID,
		TemplateID: tmplID,
		Version:    "1.0.0",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	v, err := store.GetByVersion(tmplID, "1.0.0")
	if err != nil {
		t.Fatalf("GetByVersion failed: %v", err)
	}

	if v.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", v.Version)
	}
}

func TestVersionStore_GetByVersion_NotFound(t *testing.T) {
	store := NewVersionStore()

	tmplID := uuid.New().String()

	_, err := store.Create(&TemplateVersion{
		TenantID: testTenantID,
		TemplateID: tmplID,
		Version:    "1.0.0",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = store.GetByVersion(tmplID, "2.0.0")
	if err == nil {
		t.Fatal("expected error for nonexistent version, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestVersionStore_CreateFromTemplate(t *testing.T) {
	store := NewVersionStore()

	tmplID := uuid.New().String()
	templateData := map[string]interface{}{
		"name":     "Test Template",
		"category": "engineering",
	}

	v, err := store.CreateFromTemplate(tmplID, "1.0.0", testTenantID, templateData)
	if err != nil {
		t.Fatalf("CreateFromTemplate failed: %v", err)
	}

	if v.TemplateID != tmplID {
		t.Errorf("expected templateID '%s', got %s", tmplID, v.TemplateID)
	}
	if v.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", v.Version)
	}
	if v.Snapshot == nil {
		t.Fatal("expected snapshot to be set")
	}
	if v.Snapshot["name"] != "Test Template" {
		t.Errorf("expected category 'engineering' in snapshot, got %v", v.Snapshot["category"])
	}
}

func TestVersionStore_ListByTemplate_Empty(t *testing.T) {
	store := NewVersionStore()

	list := store.ListByTemplate("nonexistent-template", testTenantID)
	if list != nil && len(list) != 0 {
		t.Errorf("expected empty list for nonexistent template, got %v", list)
	}
}

func TestVersionStore_GetByID_NotFound(t *testing.T) {
	store := NewVersionStore()

	_, err := store.GetByID("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent version, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ─── GetByIDAndTenant Tests ──────────────────────────────────────────────────

func TestTemplateStore_GetByIDAndTenant_Success(t *testing.T) {
	store := NewTemplateStore()

	tmpl, err := store.Create(&Template{
		TenantID: testTenantID,
		Name:     "Cross-Tenant Test",
		Category: "engineering",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Correct tenant should find it
	found, err := store.GetByIDAndTenant(tmpl.ID, testTenantID)
	if err != nil {
		t.Fatalf("GetByIDAndTenant failed: %v", err)
	}
	if found.Name != "Cross-Tenant Test" {
		t.Errorf("expected name 'Cross-Tenant Test', got %s", found.Name)
	}

	// Different tenant should NOT find it
	_, err = store.GetByIDAndTenant(tmpl.ID, "tenant-2")
	if err == nil {
		t.Fatal("expected error for wrong tenant, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for wrong tenant, got %v", err)
	}
}

func TestTemplateStore_GetByIDAndTenant_NotFound(t *testing.T) {
	store := NewTemplateStore()

	_, err := store.GetByIDAndTenant("nonexistent-id", testTenantID)
	if err == nil {
		t.Fatal("expected error for nonexistent template, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCustomTemplateStore_GetByIDAndTenant_Success(t *testing.T) {
	store := NewCustomTemplateStore()

	ct, err := store.Create(&CustomTemplate{
		TenantID: testTenantID,
		Name:     "Custom Cross-Tenant",
		Category: "sales",
		Content:  map[string]interface{}{"key": "value"},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	found, err := store.GetByIDAndTenant(ct.ID, testTenantID)
	if err != nil {
		t.Fatalf("GetByIDAndTenant failed: %v", err)
	}
	if found.Name != "Custom Cross-Tenant" {
		t.Errorf("expected name 'Custom Cross-Tenant', got %s", found.Name)
	}

	_, err = store.GetByIDAndTenant(ct.ID, "tenant-2")
	if err == nil {
		t.Fatal("expected error for wrong tenant, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for wrong tenant, got %v", err)
	}
}

func TestCustomTemplateStore_GetByIDAndTenant_NotFound(t *testing.T) {
	store := NewCustomTemplateStore()

	_, err := store.GetByIDAndTenant("nonexistent-id", testTenantID)
	if err == nil {
		t.Fatal("expected error for nonexistent template, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeploymentStore_GetByIDAndTenant_Success(t *testing.T) {
	store := NewDeploymentStore()

	dep, err := store.Create(&TemplateDeployment{
		TenantID:  testTenantID,
		TemplateID: uuid.New().String(),
		Environment: "production",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	found, err := store.GetByIDAndTenant(dep.ID, testTenantID)
	if err != nil {
		t.Fatalf("GetByIDAndTenant failed: %v", err)
	}
	if found.Environment != "production" {
		t.Errorf("expected environment 'production', got %s", found.Environment)
	}

	_, err = store.GetByIDAndTenant(dep.ID, "tenant-2")
	if err == nil {
		t.Fatal("expected error for wrong tenant, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for wrong tenant, got %v", err)
	}
}

func TestDeploymentStore_GetByIDAndTenant_NotFound(t *testing.T) {
	store := NewDeploymentStore()

	_, err := store.GetByIDAndTenant("nonexistent-id", testTenantID)
	if err == nil {
		t.Fatal("expected error for nonexistent deployment, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeploymentStore_Delete(t *testing.T) {
	store := NewDeploymentStore()

	dep, err := store.Create(&TemplateDeployment{
		TenantID:  testTenantID,
		TemplateID: uuid.New().String(),
		Environment: "staging",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := store.Delete(dep.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.GetByID(dep.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}

	_, err = store.GetByIDAndTenant(dep.ID, testTenantID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestDeploymentStore_Delete_NotFound(t *testing.T) {
	store := NewDeploymentStore()

	err := store.Delete("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent deployment, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestVersionStore_GetByIDAndTenant_Success(t *testing.T) {
	store := NewVersionStore()

	tmplID := uuid.New().String()

	v, err := store.Create(&TemplateVersion{
		TenantID: testTenantID,
		TemplateID: tmplID,
		Version:    "1.0.0",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	found, err := store.GetByIDAndTenant(v.ID, testTenantID)
	if err != nil {
		t.Fatalf("GetByIDAndTenant failed: %v", err)
	}
	if found.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", found.Version)
	}

	_, err = store.GetByIDAndTenant(v.ID, "tenant-2")
	if err == nil {
		t.Fatal("expected error for wrong tenant, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for wrong tenant, got %v", err)
	}
}

func TestVersionStore_GetByIDAndTenant_NotFound(t *testing.T) {
	store := NewVersionStore()

	_, err := store.GetByIDAndTenant("nonexistent-id", testTenantID)
	if err == nil {
		t.Fatal("expected error for nonexistent version, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ─── Tenant Isolation Tests ──────────────────────────────────────────────────

func TestTenantIsolation_TemplateStore(t *testing.T) {
	store := NewTemplateStore()

	t1, err := store.Create(&Template{
		TenantID: "t1",
		Name:     "T1 Template",
		Category: "engineering",
	})
	if err != nil {
		t.Fatalf("Create t1 failed: %v", err)
	}

	t2, err := store.Create(&Template{
		TenantID: "t2",
		Name:     "T2 Template",
		Category: "sales",
	})
	if err != nil {
		t.Fatalf("Create t2 failed: %v", err)
	}

	// T1 can only see T1's template
	found1, err := store.GetByIDAndTenant(t1.ID, "t1")
	if err != nil {
		t.Fatalf("GetByIDAndTenant t1 as t1 failed: %v", err)
	}
	if found1.Name != "T1 Template" {
		t.Errorf("expected 'T1 Template', got %s", found1.Name)
	}

	// T1 cannot see T2's template
	_, err = store.GetByIDAndTenant(t2.ID, "t1")
	if err == nil {
		t.Fatal("expected error when T1 tries to access T2's template, got nil")
	}

	// T2 can see T2's template
	found2, err := store.GetByIDAndTenant(t2.ID, "t2")
	if err != nil {
		t.Fatalf("GetByIDAndTenant t2 as t2 failed: %v", err)
	}
	if found2.Name != "T2 Template" {
		t.Errorf("expected 'T2 Template', got %s", found2.Name)
	}
}

func TestTenantIsolation_DeploymentStore(t *testing.T) {
	store := NewDeploymentStore()

	_, err := store.Create(&TemplateDeployment{
		TenantID:  "t1",
		TemplateID: uuid.New().String(),
		Environment: "production",
	})
	if err != nil {
		t.Fatalf("Create t1 deployment failed: %v", err)
	}

	dep2, err := store.Create(&TemplateDeployment{
		TenantID:  "t2",
		TemplateID: uuid.New().String(),
		Environment: "staging",
	})
	if err != nil {
		t.Fatalf("Create t2 deployment failed: %v", err)
	}

	// T1 cannot access T2's deployment
	_, err = store.GetByIDAndTenant(dep2.ID, "t1")
	if err == nil {
		t.Fatal("expected error when T1 tries to access T2's deployment, got nil")
	}

	// T2 can access T2's deployment
	found, err := store.GetByIDAndTenant(dep2.ID, "t2")
	if err != nil {
		t.Fatalf("GetByIDAndTenant t2 deployment failed: %v", err)
	}
	if found.Environment != "staging" {
		t.Errorf("expected 'staging', got %s", found.Environment)
	}
}
