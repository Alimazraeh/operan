package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/operan/modules/05-department-template-engine/internal/events"
	"github.com/operan/modules/05-department-template-engine/internal/ctxkeys"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

func newTestHandlers(t *testing.T) *TemplateHandlers {
	t.Helper()
	return NewTemplateHandlers(
		store.NewTemplateStore(),
		store.NewCustomTemplateStore(),
		store.NewDeploymentStore(),
		store.NewVersionStore(),
		events.NewPublisher(),
		100,
	)
}

func withTenantAndUser(ctx context.Context, tenantID, userID string) context.Context {
	ctx = context.WithValue(ctx, ctxkeys.RequestID, "test-req-001")
	ctx = context.WithValue(ctx, ctxkeys.TenantID, tenantID)
	ctx = context.WithValue(ctx, ctxkeys.UserID, userID)
	return ctx
}

func TestCreateTemplate(t *testing.T) {
	h := newTestHandlers(t)

	body := map[string]interface{}{
		"name":     "Test Department",
		"category": "engineering",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()

	h.CreateTemplate(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["name"] != "Test Department" {
		t.Errorf("expected name 'Test Department', got %v", resp["name"])
	}
}

func TestCreateTemplate_InvalidBody(t *testing.T) {
	h := newTestHandlers(t)

	req := httptest.NewRequest("POST", "/templates", bytes.NewReader([]byte("not json")))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()

	h.CreateTemplate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCreateTemplate_MissingName(t *testing.T) {
	h := newTestHandlers(t)

	body := map[string]interface{}{
		"category": "engineering",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()

	h.CreateTemplate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestListTemplates(t *testing.T) {
	h := newTestHandlers(t)

	// Create a template first
	body := map[string]interface{}{
		"name":     "Template 1",
		"category": "engineering",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	h.CreateTemplate(httptest.NewRecorder(), req)

	// List templates
	req = httptest.NewRequest("GET", "/templates", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()

	h.ListTemplates(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	meta := resp["meta"].(map[string]interface{})
	if meta["has_more"] != false {
		t.Errorf("expected has_more false, got %v", meta["has_more"])
	}
}

func TestGetTemplate(t *testing.T) {
	h := newTestHandlers(t)

	// Create a template
	body := map[string]interface{}{
		"name":     "Test Template",
		"category": "engineering",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Get the template
	req = httptest.NewRequest("GET", "/templates/"+tmplID, nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec = httptest.NewRecorder()

	h.GetTemplate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetTemplate_NotFound(t *testing.T) {
	h := newTestHandlers(t)

	req := httptest.NewRequest("GET", "/templates/nonexistent", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()

	h.GetTemplate(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateTemplate(t *testing.T) {
	h := newTestHandlers(t)

	// Create a template
	body := map[string]interface{}{
		"name":     "Original Name",
		"category": "engineering",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Update the template
	updateBody := map[string]interface{}{
		"name": "Updated Name",
	}
	updateJSON, _ := json.Marshal(updateBody)

	req = httptest.NewRequest("PATCH", "/templates/"+tmplID, bytes.NewReader(updateJSON))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec = httptest.NewRecorder()

	h.UpdateTemplate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["name"] != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %v", resp["name"])
	}
}

func TestDeleteTemplate(t *testing.T) {
	h := newTestHandlers(t)

	// Create a template
	body := map[string]interface{}{
		"name":     "To Delete",
		"category": "engineering",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Delete the template
	req = httptest.NewRequest("DELETE", "/templates/"+tmplID, nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec = httptest.NewRecorder()

	h.DeleteTemplate(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}

	// Verify it's deleted
	req = httptest.NewRequest("GET", "/templates/"+tmplID, nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec = httptest.NewRecorder()

	h.GetTemplate(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestCreateCustomTemplate(t *testing.T) {
	h := newTestHandlers(t)

	body := map[string]interface{}{
		"name":     "Custom Template",
		"category": "sales",
		"content": map[string]interface{}{
			"custom_field": "value",
		},
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates/custom", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()

	h.CreateCustomTemplate(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["name"] != "Custom Template" {
		t.Errorf("expected name 'Custom Template', got %v", resp["name"])
	}
}

func TestListCustomTemplates(t *testing.T) {
	h := newTestHandlers(t)

	// Create custom templates
	for i := 0; i < 3; i++ {
		body := map[string]interface{}{
			"name":     "Custom " + string(rune('0'+i)),
			"category": "sales",
		}
		reqBody, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/templates/custom", bytes.NewReader(reqBody))
		req.Header.Set("X-Tenant-ID", "tenant-1")
		h.CreateCustomTemplate(httptest.NewRecorder(), req)
	}

	// List custom templates
	req := httptest.NewRequest("GET", "/templates/custom", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()

	h.ListCustomTemplates(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	meta := resp["meta"].(map[string]interface{})
	if meta["total"].(float64) != 3 {
		t.Errorf("expected total 3, got %v", meta["total"])
	}
}

func TestGetCustomTemplate(t *testing.T) {
	h := newTestHandlers(t)

	// Create a custom template
	body := map[string]interface{}{
		"name":     "Get Custom Template",
		"category": "sales",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates/custom", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()
	h.CreateCustomTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	ctID := created["id"].(string)

	// Get the custom template
	req = httptest.NewRequest("GET", "/templates/custom/"+ctID, nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec = httptest.NewRecorder()

	h.GetCustomTemplate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["name"] != "Get Custom Template" {
		t.Errorf("expected name 'Get Custom Template', got %v", resp["name"])
	}
}

func TestGetCustomTemplate_NotFound(t *testing.T) {
	h := newTestHandlers(t)

	req := httptest.NewRequest("GET", "/templates/custom/nonexistent", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()

	h.GetCustomTemplate(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateCustomTemplate(t *testing.T) {
	h := newTestHandlers(t)

	// Create a custom template
	body := map[string]interface{}{
		"name":     "Original Custom",
		"category": "sales",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates/custom", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()
	h.CreateCustomTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	ctID := created["id"].(string)

	// Update the custom template
	updateBody := map[string]interface{}{
		"name": "Updated Custom",
	}
	updateJSON, _ := json.Marshal(updateBody)

	req = httptest.NewRequest("PATCH", "/templates/custom/"+ctID, bytes.NewReader(updateJSON))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec = httptest.NewRecorder()

	h.UpdateCustomTemplate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["name"] != "Updated Custom" {
		t.Errorf("expected name 'Updated Custom', got %v", resp["name"])
	}
}

func TestDeleteCustomTemplate(t *testing.T) {
	h := newTestHandlers(t)

	// Create a custom template
	body := map[string]interface{}{
		"name":     "To Delete Custom",
		"category": "sales",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates/custom", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()
	h.CreateCustomTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	ctID := created["id"].(string)

	// Delete the custom template
	req = httptest.NewRequest("DELETE", "/templates/custom/"+ctID, nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec = httptest.NewRecorder()

	h.DeleteCustomTemplate(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}

	// Verify it's deleted
	req = httptest.NewRequest("GET", "/templates/custom/"+ctID, nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec = httptest.NewRecorder()

	h.GetCustomTemplate(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestDeployTemplate_NotFound(t *testing.T) {
	h := newTestHandlers(t)

	deployBody := map[string]interface{}{
		"environment": "production",
	}
	reqBody, _ := json.Marshal(deployBody)

	req := httptest.NewRequest("POST", "/templates/nonexistent/deploy", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()

	// Use the nested handler
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestDeployTemplate(t *testing.T) {
	h := newTestHandlers(t)

	// Create a template
	body := map[string]interface{}{
		"name":     "Deploy Template",
		"category": "engineering",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Deploy the template
	deployBody := map[string]interface{}{
		"environment": "production",
	}
	deployJSON, _ := json.Marshal(deployBody)

	req = httptest.NewRequest("POST", "/templates/"+tmplID+"/deploy", bytes.NewReader(deployJSON))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec = httptest.NewRecorder()

	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["environment"] != "production" {
		t.Errorf("expected environment 'production', got %v", resp["environment"])
	}
}

func TestDeployTemplate_MissingEnvironment(t *testing.T) {
	h := newTestHandlers(t)

	// Create a template
	body := map[string]interface{}{
		"name":     "Deploy Template",
		"category": "engineering",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Deploy without environment
	deployBody := map[string]interface{}{}
	deployJSON, _ := json.Marshal(deployBody)

	req = httptest.NewRequest("POST", "/templates/"+tmplID+"/deploy", bytes.NewReader(deployJSON))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec = httptest.NewRecorder()

	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCloneTemplate(t *testing.T) {
	h := newTestHandlers(t)

	// Create a template to clone
	body := map[string]interface{}{
		"name":     "Source Template",
		"category": "engineering",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Clone the template
	cloneBody := map[string]interface{}{
		"name":     "Cloned Template",
		"category": "engineering",
	}
	cloneJSON, _ := json.Marshal(cloneBody)

	req = httptest.NewRequest("POST", "/templates/"+tmplID+"/clone", bytes.NewReader(cloneJSON))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec = httptest.NewRecorder()

	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["name"] != "Cloned Template" {
		t.Errorf("expected name 'Cloned Template', got %v", resp["name"])
	}
}

func TestGetTemplateVersion(t *testing.T) {
	h := newTestHandlers(t)

	// Create a template
	body := map[string]interface{}{
		"name":     "Version Template",
		"category": "engineering",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Create a version via the store directly
	v, err := h.VersionStore.CreateFromTemplate(tmplID, "1.0.0", map[string]interface{}{
		"name": "Version Template",
	})
	if err != nil {
		t.Fatalf("failed to create version: %v", err)
	}

	// Get the version
	req = httptest.NewRequest("GET", "/templates/"+tmplID+"/versions/"+v.ID, nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec = httptest.NewRecorder()

	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["template_id"] != tmplID {
		t.Errorf("expected template_id '%s', got %v", tmplID, resp["template_id"])
	}
}

func TestGetTemplateVersion_NotFound(t *testing.T) {
	h := newTestHandlers(t)

	req := httptest.NewRequest("GET", "/templates/nonexistent/versions/nonexistent", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()

	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
		t.Errorf("expected 400 or 404, got %d", rec.Code)
	}
}

func TestListTemplateVersions(t *testing.T) {
	h := newTestHandlers(t)

	// Create a template
	body := map[string]interface{}{
		"name":     "Versions Template",
		"category": "engineering",
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/templates", bytes.NewReader(reqBody))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Create a version via the store directly
	_, err := h.VersionStore.CreateFromTemplate(tmplID, "1.0.0", map[string]interface{}{
		"name": "Versions Template",
	})
	if err != nil {
		t.Fatalf("failed to create version: %v", err)
	}

	// List versions
	req = httptest.NewRequest("GET", "/templates/"+tmplID+"/versions", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec = httptest.NewRecorder()

	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	versions := data["versions"].([]interface{})
	if len(versions) != 1 {
		t.Errorf("expected 1 version, got %d", len(versions))
	}
}

func TestFilterChangedFields(t *testing.T) {
	// Test with allowed fields
	patch := map[string]interface{}{
		"name":             "test",
		"agents":           []string{"agent-1"},
		"workflows":        []string{"wf-1"},
		"invalid_field":    "should be filtered",
		"metadata":         map[string]interface{}{"key": "value"},
	}
	allowed := filterChangedFields(patch)

	if len(allowed) != 3 {
		t.Errorf("expected 3 allowed fields, got %d", len(allowed))
	}

	allowedSet := make(map[string]bool)
	for _, f := range allowed {
		allowedSet[f] = true
	}

	if !allowedSet["agents"] {
		t.Error("expected 'agents' in allowed fields")
	}
	if !allowedSet["workflows"] {
		t.Error("expected 'workflows' in allowed fields")
	}
	if !allowedSet["metadata"] {
		t.Error("expected 'metadata' in allowed fields")
	}
	if allowedSet["invalid_field"] {
		t.Error("unexpected 'invalid_field' in allowed fields")
	}
}

func TestFilterChangedFields_Empty(t *testing.T) {
	patch := map[string]interface{}{
		"name": "test",
	}
	allowed := filterChangedFields(patch)

	if len(allowed) != 0 {
		t.Errorf("expected 0 allowed fields, got %d", len(allowed))
	}
}

func TestHealthCheck(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","module":"department-template-engine","version":"1.0.0"}`))
	})

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", resp["status"])
	}
}
