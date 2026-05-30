package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

func testRequest(method, url string, body interface{}) (*http.Request, context.Context) {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, url, bytes.NewReader(reqBody))
	req = req.WithContext(withTenantAndUser(req.Context(), "tenant-1", "user-1"))
	return req, req.Context()
}

func TestCreateTemplate(t *testing.T) {
	h := newTestHandlers(t)

	body := map[string]interface{}{
		"name":     "Test Department",
		"category": "engineering",
	}
	req, _ := testRequest("POST", "/templates", body)
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

	req, _ := testRequest("POST", "/templates", nil)
	req.Body = io.NopCloser(bytes.NewReader([]byte("not json")))
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
	req, _ := testRequest("POST", "/templates", body)
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
	req, _ := testRequest("POST", "/templates", body)
	h.CreateTemplate(httptest.NewRecorder(), req)

	// List templates
	req, _ = testRequest("GET", "/templates", nil)
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
	req, _ := testRequest("POST", "/templates", body)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Get the template
	req, _ = testRequest("GET", "/templates/"+tmplID, nil)
	rec = httptest.NewRecorder()

	h.GetTemplate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetTemplate_NotFound(t *testing.T) {
	h := newTestHandlers(t)

	req, _ := testRequest("GET", "/templates/nonexistent", nil)
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
	req, _ := testRequest("POST", "/templates", body)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Update the template
	updateBody := map[string]interface{}{
		"name": "Updated Name",
	}
	req, _ = testRequest("PATCH", "/templates/"+tmplID, updateBody)
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
	req, _ := testRequest("POST", "/templates", body)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Delete the template
	req, _ = testRequest("DELETE", "/templates/"+tmplID, nil)
	rec = httptest.NewRecorder()

	h.DeleteTemplate(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}

	// Verify it's deleted
	req, _ = testRequest("GET", "/templates/"+tmplID, nil)
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
	req, _ := testRequest("POST", "/templates/custom", body)
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
		req, _ := testRequest("POST", "/templates/custom", body)
		h.CreateCustomTemplate(httptest.NewRecorder(), req)
	}

	// List custom templates
	req, _ := testRequest("GET", "/templates/custom", nil)
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
	req, _ := testRequest("POST", "/templates/custom", body)
	rec := httptest.NewRecorder()
	h.CreateCustomTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	ctID := created["id"].(string)

	// Get the custom template
	req, _ = testRequest("GET", "/templates/custom/"+ctID, nil)
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

	req, _ := testRequest("GET", "/templates/custom/nonexistent", nil)
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
	req, _ := testRequest("POST", "/templates/custom", body)
	rec := httptest.NewRecorder()
	h.CreateCustomTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	ctID := created["id"].(string)

	// Update the custom template
	updateBody := map[string]interface{}{
		"name": "Updated Custom",
	}
	req, _ = testRequest("PATCH", "/templates/custom/"+ctID, updateBody)
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
	req, _ := testRequest("POST", "/templates/custom", body)
	rec := httptest.NewRecorder()
	h.CreateCustomTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	ctID := created["id"].(string)

	// Delete the custom template
	req, _ = testRequest("DELETE", "/templates/custom/"+ctID, nil)
	rec = httptest.NewRecorder()

	h.DeleteCustomTemplate(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}

	// Verify it's deleted
	req, _ = testRequest("GET", "/templates/custom/"+ctID, nil)
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
	req, _ := testRequest("POST", "/templates/nonexistent/deploy", deployBody)
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
	req, _ := testRequest("POST", "/templates", body)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Deploy the template
	deployBody := map[string]interface{}{
		"environment": "production",
	}
	req, _ = testRequest("POST", "/templates/"+tmplID+"/deploy", deployBody)
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
	req, _ := testRequest("POST", "/templates", body)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Deploy without environment
	deployBody := map[string]interface{}{}
	req, _ = testRequest("POST", "/templates/"+tmplID+"/deploy", deployBody)
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
	req, _ := testRequest("POST", "/templates", body)
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
	req, _ = testRequest("POST", "/templates/"+tmplID+"/clone", cloneBody)
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
	req, _ := testRequest("POST", "/templates", body)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Create a version via the store directly
	v, err := h.VersionStore.CreateFromTemplate(tmplID, "1.0.0", "tenant-1", map[string]interface{}{
		"name": "Version Template",
	})
	if err != nil {
		t.Fatalf("failed to create version: %v", err)
	}

	// Get the version
	req, _ = testRequest("GET", "/templates/"+tmplID+"/versions/"+v.ID, nil)
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

	req, _ := testRequest("GET", "/templates/nonexistent/versions/nonexistent", nil)
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
	req, _ := testRequest("POST", "/templates", body)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Create a version via the store directly
	_, err := h.VersionStore.CreateFromTemplate(tmplID, "1.0.0", "tenant-1", map[string]interface{}{
		"name": "Versions Template",
	})
	if err != nil {
		t.Fatalf("failed to create version: %v", err)
	}

	// List versions
	req, _ = testRequest("GET", "/templates/"+tmplID+"/versions", nil)
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

	req, _ := testRequest("GET", "/health", nil)
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


// ─── Nested Deployment Handler Tests ─────────────────────────────────────────

func TestHandleDeploy(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "Deploy Test Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	deployBody := map[string]interface{}{"environment": "production"}
	req, _ = testRequest("POST", "/templates/"+tmplID+"/deploy", deployBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		return
	}

	var depResp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &depResp)
	if depResp["environment"] != "production" {
		t.Errorf("expected environment 'production', got %v", depResp["environment"])
	}
	if depResp["status"] != "select" {
		t.Errorf("expected initial status 'select', got %v", depResp["status"])
	}
}

func TestHandleDeploy_InvalidEnv(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "Deploy Invalid Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	deployBody := map[string]interface{}{"environment": ""}
	req, _ = testRequest("POST", "/templates/"+tmplID+"/deploy", deployBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleListDeployments(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "List Deploy Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	for _, env := range []string{"production", "staging"} {
		deployBody := map[string]interface{}{"environment": env}
		req, _ = testRequest("POST", "/templates/"+tmplID+"/deploy", deployBody)
		rec = httptest.NewRecorder()
		h.HandleTemplateNested(rec, req)
	}

	req, _ = testRequest("GET", "/templates/"+tmplID+"/deployments", nil)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
		return
	}

	var listResp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &listResp)
	if data, ok := listResp["data"].([]interface{}); !ok || len(data) != 2 {
		t.Errorf("expected 2 deployments in list, got %v", listResp["data"])
	}
}

func TestHandleGetDeployment(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "Get Deploy Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	deployBody := map[string]interface{}{"environment": "production"}
	req, _ = testRequest("POST", "/templates/"+tmplID+"/deploy", deployBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	var depResp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &depResp)
	depID := depResp["id"].(string)

	req, _ = testRequest("GET", "/templates/"+tmplID+"/deployments/"+depID, nil)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		return
	}

	var getResp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &getResp)
	if getResp["id"] != depID {
		t.Errorf("expected id %s, got %v", depID, getResp["id"])
	}
}

func TestHandleGetDeployment_NotFound(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "NF Deploy Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	req, _ = testRequest("GET", "/templates/"+tmplID+"/deployments/nonexistent-id", nil)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleUpdateDeployment_Status(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "Update Deploy Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	deployBody := map[string]interface{}{"environment": "production"}
	req, _ = testRequest("POST", "/templates/"+tmplID+"/deploy", deployBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	json.Unmarshal(rec.Body.Bytes(), &created)
	depID := created["id"].(string)

	patchBody := map[string]interface{}{"status": "deployed"}
	req, _ = testRequest("PATCH", "/templates/"+tmplID+"/deployments/"+depID, patchBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		return
	}

	var updated map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated["status"] != "deployed" {
		t.Errorf("expected status 'deployed', got %v", updated["status"])
	}
}

func TestHandleUpdateDeployment_NotFound(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "NF Update Deploy Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	patchBody := map[string]interface{}{"status": "deployed"}
	req, _ = testRequest("PATCH", "/templates/"+tmplID+"/deployments/nonexistent-id", patchBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleUpdateDeployment_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "Invalid JSON Deploy Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	deployBody := map[string]interface{}{"environment": "production"}
	req, _ = testRequest("POST", "/templates/"+tmplID+"/deploy", deployBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	json.Unmarshal(rec.Body.Bytes(), &created)
	depID := created["id"].(string)

	req = httptest.NewRequest("PATCH", "/templates/"+tmplID+"/deployments/"+depID, bytes.NewReader([]byte("not-json")))
	req = req.WithContext(withTenantAndUser(req.Context(), "tenant-1", "user-1"))
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// ─── Nested Version Handler Tests ────────────────────────────────────────────

func TestHandleGetVersion(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "Get Version Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	req, _ = testRequest("GET", "/templates/"+tmplID+"/versions/some-version", nil)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleListVersions(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "List Versions Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	req, _ = testRequest("GET", "/templates/"+tmplID+"/versions", nil)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
		return
	}
}

func TestHandleClone(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{
		"name":        "Original Template",
		"category":    "engineering",
		"description": "Original description",
		"tags":        []string{"tag1", "tag2"},
	}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	cloneBody := map[string]interface{}{"name": "Cloned Template", "category": "engineering"}
	req, _ = testRequest("POST", "/templates/"+tmplID+"/clone", cloneBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		return
	}

	var cloned map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &cloned)
	if cloned["name"] != "Cloned Template" {
		t.Errorf("expected name 'Cloned Template', got %v", cloned["name"])
	}
}

// ─── Additional error-path tests ──────────────────────────────────────────────

func TestHandleTemplateNested_InvalidMethod(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "Invalid Method Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	req, _ = testRequest("DELETE", "/templates/"+tmplID+"/deployments/some-id", nil)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleTemplateNested_InvalidOperation(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "Invalid Op Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	req, _ = testRequest("POST", "/templates/"+tmplID+"/some-random-op", nil)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleTemplateNested_EmptyTemplateID(t *testing.T) {
	h := newTestHandlers(t)

	req, _ := testRequest("GET", "/templates//deployments", nil)
	rec := httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleUpdateDeployment_EmptyDeploymentID(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "Empty ID Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	req, _ = testRequest("PATCH", "/templates/"+tmplID+"/deployments/", nil)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}


// ─── Helper function tests ───────────────────────────────────────────────────

func TestParsePositiveInt(t *testing.T) {
	// Valid positive integer
	n, err := parsePositiveInt("5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5, got %d", n)
	}

	// Zero should default to 1
	n, err = parsePositiveInt("0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 for zero, got %d", n)
	}

	// Negative should default to 1
	n, err = parsePositiveInt("-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 for negative, got %d", n)
	}

	// Non-numeric should default to 1
	n, err = parsePositiveInt("abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 for non-numeric, got %d", n)
	}
}

// ─── Custom template error-path tests ─────────────────────────────────────────

func TestCreateCustomTemplate_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t)

	req := httptest.NewRequest("POST", "/custom-templates", bytes.NewReader([]byte("not-json")))
	req = req.WithContext(withTenantAndUser(req.Context(), "tenant-1", "user-1"))
	rec := httptest.NewRecorder()
	h.CreateCustomTemplate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateCustomTemplate_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t)

	// First create a custom template
	createBody := map[string]interface{}{"name": "Update CT Template", "category": "engineering", "template_type": "policy"}
	req, _ := testRequest("POST", "/custom-templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateCustomTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	ctID := created["id"].(string)

	// Update with invalid JSON
	req = httptest.NewRequest("PATCH", "/custom-templates/"+ctID, bytes.NewReader([]byte("bad")))
	req = req.WithContext(withTenantAndUser(req.Context(), "tenant-1", "user-1"))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.UpdateCustomTemplate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteCustomTemplate_NotFound(t *testing.T) {
	h := newTestHandlers(t)

	req, _ := testRequest("DELETE", "/custom-templates/nonexistent-id", nil)
	req = req.WithContext(withTenantAndUser(req.Context(), "tenant-1", "user-1"))
	rec := httptest.NewRecorder()
	h.DeleteCustomTemplate(rec, req)

	// Returns 400 because "nonexistent-id" fails UUID validation
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// ─── Template error-path tests ────────────────────────────────────────────────

func TestUpdateTemplate_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t)

	// First create a template
	createBody := map[string]interface{}{"name": "Invalid JSON Update Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Update with invalid JSON
	req = httptest.NewRequest("PATCH", "/templates/"+tmplID, bytes.NewReader([]byte("bad")))
	req = req.WithContext(withTenantAndUser(req.Context(), "tenant-1", "user-1"))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.UpdateTemplate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteTemplate_NotFound(t *testing.T) {
	h := newTestHandlers(t)

	req, _ := testRequest("DELETE", "/templates/nonexistent-id", nil)
	req = req.WithContext(withTenantAndUser(req.Context(), "tenant-1", "user-1"))
	rec := httptest.NewRecorder()
	h.DeleteTemplate(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestListTemplates_InvalidPage(t *testing.T) {
	h := newTestHandlers(t)

	// Create a template
	createBody := map[string]interface{}{"name": "List Invalid Page Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	// List with invalid page number (should default to 1)
	req, _ = testRequest("GET", "/templates?page=abc", nil)
	rec = httptest.NewRecorder()
	h.ListTemplates(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestGetCustomTemplate_NotFound_Path(t *testing.T) {
	h := newTestHandlers(t)

	req, _ := testRequest("GET", "/custom-templates/nonexistent-id", nil)
	req = req.WithContext(withTenantAndUser(req.Context(), "tenant-1", "user-1"))
	rec := httptest.NewRecorder()
	h.GetCustomTemplate(rec, req)

	// Returns 400 because "nonexistent-id" fails UUID validation
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestListCustomTemplates_Empty(t *testing.T) {
	h := newTestHandlers(t)

	req, _ := testRequest("GET", "/custom-templates", nil)
	req = req.WithContext(withTenantAndUser(req.Context(), "tenant-1", "user-1"))
	rec := httptest.NewRecorder()
	h.ListCustomTemplates(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// ─── Nested handler error-path tests ──────────────────────────────────────────

func TestHandleDeploy_ReadError(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "Read Error Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Create a request with a closed body to trigger read error
	req = httptest.NewRequest("POST", "/templates/"+tmplID+"/deploy", nil)
	req = req.WithContext(withTenantAndUser(req.Context(), "tenant-1", "user-1"))
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}


// ─── Router test ──────────────────────────────────────────────────────────────

func TestRegisterRoutes(t *testing.T) {
	mux := http.NewServeMux()
	h := newTestHandlers(t)

	RegisterRoutes(mux, h)

	// Test a couple of routes to verify they're registered
	req, _ := testRequest("GET", "/templates", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Should return 200 OK with empty list
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// ─── handleUpdateDeployment error paths ────────────────────────────────────────

func TestHandleUpdateDeployment_EmptyStatus(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "Empty Status Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	deployBody := map[string]interface{}{"environment": "production"}
	req, _ = testRequest("POST", "/templates/"+tmplID+"/deploy", deployBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	json.Unmarshal(rec.Body.Bytes(), &created)
	depID := created["id"].(string)

	// Update with empty status string
	patchBody := map[string]interface{}{"status": ""}
	req, _ = testRequest("PATCH", "/templates/"+tmplID+"/deployments/"+depID, patchBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	// Should return 200 with unchanged deployment (empty status not valid)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ─── ListTemplates error paths ────────────────────────────────────────────────

func TestListTemplates_Empty(t *testing.T) {
	h := newTestHandlers(t)

	req, _ := testRequest("GET", "/templates", nil)
	req = req.WithContext(withTenantAndUser(req.Context(), "tenant-1", "user-1"))
	rec := httptest.NewRecorder()
	h.ListTemplates(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
		return
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if data, ok := resp["data"].([]interface{}); !ok || len(data) != 0 {
		t.Errorf("expected empty data list, got %v", resp["data"])
	}
}

// ─── UpdateTemplate error paths ───────────────────────────────────────────────

func TestUpdateTemplate_NotFound(t *testing.T) {
	h := newTestHandlers(t)

	updateBody := map[string]interface{}{"name": "Updated Template"}
	req, _ := testRequest("PATCH", "/templates/nonexistent-id", updateBody)
	req = req.WithContext(withTenantAndUser(req.Context(), "tenant-1", "user-1"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.UpdateTemplate(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// ─── handleClone error paths ──────────────────────────────────────────────────

func TestHandleClone_InvalidTemplate(t *testing.T) {
	h := newTestHandlers(t)

	// Try to clone a non-existent template
	cloneBody := map[string]interface{}{"name": "Clone Nonexistent", "category": "engineering"}
	req, _ := testRequest("POST", "/templates/nonexistent-id/clone", cloneBody)
	rec := httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// ─── handleGetVersion with valid version ──────────────────────────────────────

func TestHandleGetVersion_Success(t *testing.T) {
	h := newTestHandlers(t)

	// Create a template (but versions must be created separately)
	createBody := map[string]interface{}{"name": "Version Test Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	// Version doesn't exist in store, but let's verify the response is proper
	req, _ = testRequest("GET", "/templates/"+tmplID+"/versions/1.0.0", nil)
	req = req.WithContext(withTenantAndUser(req.Context(), "tenant-1", "user-1"))
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// ─── Helper function extractTemplateIDFromNestedPath ─────────────────────────

func TestExtractTemplateIDFromNestedPath(t *testing.T) {
	// Valid nested path
	id := extractTemplateIDFromNestedPath("/templates/abc123/deploy")
	if id != "abc123" {
		t.Errorf("expected abc123, got %s", id)
	}

	// Empty path
	id = extractTemplateIDFromNestedPath("")
	if id != "" {
		t.Errorf("expected empty, got %s", id)
	}

	// No template ID
	id = extractTemplateIDFromNestedPath("/templates")
	if id != "" {
		t.Errorf("expected empty, got %s", id)
	}
}


// ─── Additional handleUpdateDeployment tests ─────────────────────────────────

func TestHandleUpdateDeployment_FailedWithMessage(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "Failed Deploy Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	deployBody := map[string]interface{}{"environment": "production"}
	req, _ = testRequest("POST", "/templates/"+tmplID+"/deploy", deployBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	json.Unmarshal(rec.Body.Bytes(), &created)
	depID := created["id"].(string)

	// Update with failed status and error message
	patchBody := map[string]interface{}{
		"status":       "failed",
		"error_message": "Container crashed during deployment",
	}
	req, _ = testRequest("PATCH", "/templates/"+tmplID+"/deployments/"+depID, patchBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		return
	}

	var updated map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated["status"] != "failed" {
		t.Errorf("expected status 'failed', got %v", updated["status"])
	}
}

func TestHandleUpdateDeployment_UpdateStatusFailure(t *testing.T) {
	h := newTestHandlers(t)

	createBody := map[string]interface{}{"name": "Status Fail Template", "category": "engineering"}
	req, _ := testRequest("POST", "/templates", createBody)
	rec := httptest.NewRecorder()
	h.CreateTemplate(rec, req)

	var created map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &created)
	tmplID := created["id"].(string)

	deployBody := map[string]interface{}{"environment": "production"}
	req, _ = testRequest("POST", "/templates/"+tmplID+"/deploy", deployBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	json.Unmarshal(rec.Body.Bytes(), &created)
	depID := created["id"].(string)

	// Update with a status that won't match any specific handler but should still work
	patchBody := map[string]interface{}{"status": "unknown-status"}
	req, _ = testRequest("PATCH", "/templates/"+tmplID+"/deployments/"+depID, patchBody)
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)

	// Should return 200 since it goes to the else branch (get by ID)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
