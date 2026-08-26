package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeployTemplatePath(t *testing.T) {
	var gotPath string
	var gotBody DeployRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(DeploymentResponse{
			ID: "dep-1", TemplateID: "it-medium-001", Status: "select",
			Environment: "production", DepartmentID: "dept-1",
		})
	}))
	defer srv.Close()

	c := &DepartmentsClient{BaseURL: srv.URL, Doer: NewDoer()}
	got, err := c.DeployTemplate(context.Background(), "admin-jwt", "smoke-tenant", "it-medium-001", DeployRequest{
		Environment: "production",
	})
	if err != nil {
		t.Fatalf("DeployTemplate: %v", err)
	}
	if gotPath != "/templates/it-medium-001/deploy" {
		t.Errorf("path = %q, want /templates/it-medium-001/deploy", gotPath)
	}
	if gotBody.Environment != "production" {
		t.Errorf("request body Environment = %q", gotBody.Environment)
	}
	// department_id must come back synchronously — restore relies on this
	// to bind seats without a separate lookup right after deploy.
	if got.DepartmentID != "dept-1" {
		t.Errorf("got.DepartmentID = %q, want dept-1", got.DepartmentID)
	}
}

func TestFindDepartmentMatchesTemplateAndName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(departmentListResponse{
			Data: []*DepartmentSummary{
				{ID: "d-1", Name: "IT Department", TemplateID: "other-template"},
				{ID: "d-2", Name: "Some Other Dept", TemplateID: "it-medium-001"},
				{ID: "d-3", Name: "IT Department", TemplateID: "it-medium-001"},
			},
		})
	}))
	defer srv.Close()

	c := &DepartmentsClient{BaseURL: srv.URL, Doer: NewDoer()}
	found, err := c.FindDepartment(context.Background(), "tok", "smoke-tenant", "it-medium-001", "IT Department")
	if err != nil {
		t.Fatalf("FindDepartment: %v", err)
	}
	if found == nil || found.ID != "d-3" {
		t.Fatalf("expected to find d-3 (matching both template and name), got %+v", found)
	}
}

func TestFindDepartmentEmptyNameMatchesAnyNameForThatTemplate(t *testing.T) {
	// A department deployed with no department_name override does NOT get
	// an empty Name on the server (M05 defaults it to the template's own
	// name) — so an empty wantName must match on template id alone, not
	// require an empty Name.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(departmentListResponse{
			Data: []*DepartmentSummary{
				{ID: "d-1", Name: "Unrelated Department", TemplateID: "other-template"},
				{ID: "d-2", Name: "IT Department", TemplateID: "it-medium-001"},
			},
		})
	}))
	defer srv.Close()

	c := &DepartmentsClient{BaseURL: srv.URL, Doer: NewDoer()}
	found, err := c.FindDepartment(context.Background(), "tok", "smoke-tenant", "it-medium-001", "")
	if err != nil {
		t.Fatalf("FindDepartment: %v", err)
	}
	if found == nil || found.ID != "d-2" {
		t.Fatalf("expected to find d-2 by template id alone, got %+v", found)
	}
}

func TestFindDepartmentNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(departmentListResponse{
			Data: []*DepartmentSummary{{ID: "d-1", Name: "Unrelated", TemplateID: "other"}},
		})
	}))
	defer srv.Close()

	c := &DepartmentsClient{BaseURL: srv.URL, Doer: NewDoer()}
	found, err := c.FindDepartment(context.Background(), "tok", "smoke-tenant", "it-medium-001", "IT Department")
	if err != nil {
		t.Fatalf("FindDepartment: unexpected error: %v", err)
	}
	if found != nil {
		t.Fatalf("expected nil, got %+v", found)
	}
}

func TestSetPositionHolderPathAndMethod(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody SetHolderRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(OrgChartResponse{RootPositionID: "pos-it-manager-01"})
	}))
	defer srv.Close()

	c := &DepartmentsClient{BaseURL: srv.URL, Doer: NewDoer()}
	_, err := c.SetPositionHolder(context.Background(), "admin-jwt", "smoke-tenant", "dept-1", "pos-it-manager-01", SetHolderRequest{
		HolderType: "human", HumanRef: "u-2",
	})
	if err != nil {
		t.Fatalf("SetPositionHolder: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/departments/dept-1/org-chart/pos-it-manager-01/holder" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody.HolderType != "human" || gotBody.HumanRef != "u-2" {
		t.Errorf("request body = %+v", gotBody)
	}
}

func TestSyncWorkflowsPath(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		_ = json.NewEncoder(w).Encode(SyncWorkflowsResponse{Changed: 4})
	}))
	defer srv.Close()

	c := &DepartmentsClient{BaseURL: srv.URL, Doer: NewDoer()}
	got, err := c.SyncWorkflows(context.Background(), "admin-jwt", "smoke-tenant", "dept-1")
	if err != nil {
		t.Fatalf("SyncWorkflows: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/departments/dept-1/services/sync-workflows" {
		t.Errorf("method/path = %s %s", gotMethod, gotPath)
	}
	if got.Changed != 4 {
		t.Errorf("got.Changed = %d, want 4", got.Changed)
	}
}

func TestCreateRequestPath(t *testing.T) {
	var gotPath string
	var gotBody CreateRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ServiceRequest{ID: "req-1", Status: "open", Title: gotBody.Title})
	}))
	defer srv.Close()

	c := &DepartmentsClient{BaseURL: srv.URL, Doer: NewDoer()}
	got, err := c.CreateRequest(context.Background(), "dana-jwt", "smoke-tenant", "dept-1", CreateRequestBody{
		ServiceID: "svc-1", Title: "Grant replay-test read access", Priority: "normal",
	})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}
	if gotPath != "/departments/dept-1/requests" {
		t.Errorf("path = %q", gotPath)
	}
	if got.ID != "req-1" || got.Title != "Grant replay-test read access" {
		t.Errorf("got = %+v", got)
	}
}

func TestGetRequestPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(ServiceRequest{ID: "req-1", Status: "completed"})
	}))
	defer srv.Close()

	c := &DepartmentsClient{BaseURL: srv.URL, Doer: NewDoer()}
	got, err := c.GetRequest(context.Background(), "tok", "smoke-tenant", "req-1")
	if err != nil {
		t.Fatalf("GetRequest: %v", err)
	}
	if gotPath != "/requests/req-1" {
		t.Errorf("path = %q, want /requests/req-1", gotPath)
	}
	if got.Status != "completed" {
		t.Errorf("got.Status = %q", got.Status)
	}
}

func TestGetDepartmentReturnsOrgChartAndServices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Department{
			ID: "dept-1", Status: "operational",
			OrgChart: []Position{{ID: "pos-it-manager-01", Title: "IT Manager", HolderType: "human", HumanRef: "u-2"}},
			Services: []ServiceOffering{{ID: "svc-1", Name: "Access Request"}},
		})
	}))
	defer srv.Close()

	c := &DepartmentsClient{BaseURL: srv.URL, Doer: NewDoer()}
	got, err := c.GetDepartment(context.Background(), "tok", "smoke-tenant", "dept-1")
	if err != nil {
		t.Fatalf("GetDepartment: %v", err)
	}
	if len(got.OrgChart) != 1 || got.OrgChart[0].ID != "pos-it-manager-01" {
		t.Errorf("got.OrgChart = %+v", got.OrgChart)
	}
	if len(got.Services) != 1 || got.Services[0].ID != "svc-1" {
		t.Errorf("got.Services = %+v", got.Services)
	}
}
