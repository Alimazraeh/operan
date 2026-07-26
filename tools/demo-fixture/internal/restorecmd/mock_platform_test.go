package restorecmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
)

// fakeIDCounter backs fakeID below — good enough uniqueness for a
// single-process test double; no need for a real UUID library dependency
// just to generate distinguishable mock ids.
var fakeIDCounter int64

func fakeID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, atomic.AddInt64(&fakeIDCounter, 1))
}

// mockPlatform is a small, stateful stand-in for M01/M02/M04/M05/M09,
// replicating just enough of each real store's documented behavior for
// idempotency to be meaningfully exercisable in tests:
//   - tenant create 409s on a repeat name (M01's real behavior)
//   - user create NEVER dedups on email — every call mints a new user (M02's
//     real behavior, per internal/store/user.go's doc trail)
//   - agent create 409s on a repeat id (M04's real behavior)
//   - department deploy NEVER dedups — every call mints a new department
//     (M05's real behavior, per internal/deploy/orchestrator.go)
//
// Call counters let tests assert not just "the second Provision call didn't
// error" but "the server-side create endpoint was only ever hit once" —
// the actual evidence of idempotency this work order asks for.
type mockPlatform struct {
	mu sync.Mutex

	tenants     map[string]mockTenant // by name
	users       []mockUser
	agents      map[string]mockAgent // by id
	departments []mockDepartment
	queue       map[string][]mockQueueItem // by user id
	approvals   map[string]*mockApproval   // by id

	CreateTenantCalls  int
	CreateUserCalls    int
	CreateAgentCalls   int
	DeployCalls        int
	SetHolderCalls     int
	SyncWorkflowCalls  int
	CreateRequestCalls int

	// requestStatusScript, if set, is consumed one entry per GetRequest
	// poll for the most recently created request, letting replay tests
	// drive a multi-poll state machine deterministically.
	requestStatusScript []string
	requestStatusIdx    int

	srv *httptest.Server
}

type mockTenant struct {
	ID, Name, Plan, Region, Isolation string
}
type mockUser struct {
	ID, Email, DisplayName string
	RoleIDs                []string
}
type mockAgent struct {
	ID, Name, Role string
}
type mockDepartment struct {
	ID, TemplateID, Name, Environment string
	OrgChart                          []mockPosition
	Services                          []mockService
}
type mockPosition struct {
	ID, HolderType, HumanRef, AgentID string
}
type mockService struct {
	ID, Name string
}
type mockQueueItem struct {
	ID, Title, Status string
}
type mockApproval struct {
	ID, Status string
}

func newMockPlatform() *mockPlatform {
	m := &mockPlatform{
		tenants:   map[string]mockTenant{},
		agents:    map[string]mockAgent{},
		queue:     map[string][]mockQueueItem{},
		approvals: map[string]*mockApproval{},
	}
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/iam/admin/login", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"token": "admin-jwt", "user_id": "admin-001", "email": "admin@operan"})
	})
	mux.HandleFunc("POST /api/v1/iam/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		defer m.mu.Unlock()
		for _, u := range m.users {
			if u.Email == body["email"] {
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"token": "user-jwt-" + u.ID, "user_id": u.ID, "email": u.Email, "roles": u.RoleIDs,
				})
				return
			}
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	})

	mux.HandleFunc("GET /v1/tenants", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		items := make([]map[string]interface{}, 0, len(m.tenants))
		for _, t := range m.tenants {
			items = append(items, map[string]interface{}{
				"id": t.ID, "name": t.Name, "plan": t.Plan, "region": t.Region, "isolation_level": t.Isolation,
			})
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"items": items, "total": len(items)})
	})
	mux.HandleFunc("POST /v1/tenants", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		defer m.mu.Unlock()
		m.CreateTenantCalls++
		if _, exists := m.tenants[body["name"]]; exists {
			writeJSON(w, http.StatusConflict, map[string]string{"error": fmt.Sprintf("tenant name %q already exists", body["name"])})
			return
		}
		t := mockTenant{ID: fakeID("tenant"), Name: body["name"], Plan: body["plan"], Region: body["region"], Isolation: body["isolation_level"]}
		m.tenants[body["name"]] = t
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"id": t.ID, "name": t.Name, "plan": t.Plan, "region": t.Region, "isolation_level": t.Isolation,
		})
	})

	mux.HandleFunc("GET /api/v1/iam/users", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		items := make([]map[string]interface{}, 0, len(m.users))
		for _, u := range m.users {
			items = append(items, map[string]interface{}{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "role_ids": u.RoleIDs, "status": "pending"})
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"users": items, "total": len(items)})
	})
	mux.HandleFunc("POST /api/v1/iam/users", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email       string   `json:"email"`
			DisplayName string   `json:"display_name"`
			RoleIDs     []string `json:"role_ids"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		defer m.mu.Unlock()
		m.CreateUserCalls++
		u := mockUser{ID: fakeID("user"), Email: body.Email, DisplayName: body.DisplayName, RoleIDs: body.RoleIDs}
		m.users = append(m.users, u) // no dedup — matches the real store
		writeJSON(w, http.StatusCreated, map[string]interface{}{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "role_ids": u.RoleIDs, "status": "pending"})
	})
	mux.HandleFunc("POST /api/v1/iam/users/{id}/password", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /registry/agents", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Role string `json:"role"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		defer m.mu.Unlock()
		m.CreateAgentCalls++
		if _, exists := m.agents[body.ID]; exists {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "agent already exists"})
			return
		}
		a := mockAgent{ID: body.ID, Name: body.Name, Role: body.Role}
		m.agents[body.ID] = a
		writeJSON(w, http.StatusCreated, map[string]interface{}{"id": a.ID, "name": a.Name, "role": a.Role, "status": "active"})
	})
	mux.HandleFunc("GET /registry/agents/{id}", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		a, ok := m.agents[r.PathValue("id")]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"id": a.ID, "name": a.Name, "role": a.Role, "status": "active"})
	})

	mux.HandleFunc("GET /departments", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		items := make([]map[string]interface{}, 0, len(m.departments))
		for _, d := range m.departments {
			items = append(items, map[string]interface{}{"id": d.ID, "name": d.Name, "template_id": d.TemplateID})
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "meta": map[string]interface{}{"total": len(items)}})
	})
	mux.HandleFunc("POST /templates/{id}/deploy", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Environment    string `json:"environment"`
			DepartmentName string `json:"department_name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		defer m.mu.Unlock()
		m.DeployCalls++
		name := body.DepartmentName
		if name == "" {
			name = "Default Department Name"
		}
		d := mockDepartment{ID: fakeID("dept"), TemplateID: r.PathValue("id"), Name: name, Environment: body.Environment,
			Services: []mockService{{ID: "svc-1", Name: "Access Request"}}}
		m.departments = append(m.departments, d)
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"id": fakeID("deployment"), "template_id": d.TemplateID, "status": "select", "environment": d.Environment, "department_id": d.ID,
		})
	})
	mux.HandleFunc("GET /departments/{id}", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		for _, d := range m.departments {
			if d.ID == r.PathValue("id") {
				oc := make([]map[string]interface{}, 0, len(d.OrgChart))
				for _, p := range d.OrgChart {
					oc = append(oc, map[string]interface{}{"id": p.ID, "holder_type": p.HolderType, "human_ref": p.HumanRef, "agent_id": p.AgentID})
				}
				svcs := make([]map[string]interface{}, 0, len(d.Services))
				for _, s := range d.Services {
					svcs = append(svcs, map[string]interface{}{"id": s.ID, "name": s.Name})
				}
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"id": d.ID, "name": d.Name, "template_id": d.TemplateID, "environment": d.Environment,
					"status": "operational", "org_chart": oc, "services": svcs,
				})
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})
	mux.HandleFunc("PUT /departments/{id}/org-chart/{posID}/holder", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			HolderType string `json:"holder_type"`
			HumanRef   string `json:"human_ref"`
			AgentID    string `json:"agent_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		defer m.mu.Unlock()
		m.SetHolderCalls++
		for i := range m.departments {
			if m.departments[i].ID != r.PathValue("id") {
				continue
			}
			found := false
			for j := range m.departments[i].OrgChart {
				if m.departments[i].OrgChart[j].ID == r.PathValue("posID") {
					m.departments[i].OrgChart[j] = mockPosition{ID: r.PathValue("posID"), HolderType: body.HolderType, HumanRef: body.HumanRef, AgentID: body.AgentID}
					found = true
				}
			}
			if !found {
				m.departments[i].OrgChart = append(m.departments[i].OrgChart, mockPosition{ID: r.PathValue("posID"), HolderType: body.HolderType, HumanRef: body.HumanRef, AgentID: body.AgentID})
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"root_position_id": "", "positions": []interface{}{}})
	})
	mux.HandleFunc("POST /departments/{id}/services/sync-workflows", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.SyncWorkflowCalls++
		writeJSON(w, http.StatusOK, map[string]interface{}{"template_version": "1.1.0", "changed": 0, "skipped": []string{}})
	})

	mux.HandleFunc("POST /departments/{id}/requests", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ServiceID string `json:"service_id"`
			Title     string `json:"title"`
			Priority  string `json:"priority"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		defer m.mu.Unlock()
		m.CreateRequestCalls++
		m.requestStatusIdx = 0
		status := "open"
		if len(m.requestStatusScript) > 0 {
			status = m.requestStatusScript[0]
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"id": "req-1", "service_id": body.ServiceID, "title": body.Title, "priority": body.Priority, "status": status,
		})
	})
	mux.HandleFunc("GET /requests/{id}", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		status := "completed"
		if len(m.requestStatusScript) > 0 {
			idx := m.requestStatusIdx
			if idx >= len(m.requestStatusScript) {
				idx = len(m.requestStatusScript) - 1
			}
			status = m.requestStatusScript[idx]
			m.requestStatusIdx++
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id": r.PathValue("id"), "title": "Grant replay-test read access", "status": status,
		})
	})

	mux.HandleFunc("GET /queue", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		m.mu.Lock()
		defer m.mu.Unlock()
		items := make([]map[string]interface{}, 0)
		for _, it := range m.queue[userID] {
			items = append(items, map[string]interface{}{"item_id": it.ID, "item_type": "approval", "title": it.Title, "status": it.Status})
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"items": items, "total": len(items)})
	})
	mux.HandleFunc("POST /approvals/{id}/approve", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		a, ok := m.approvals[r.PathValue("id")]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		a.Status = "approved"
		// Once approved, the next request poll should see it complete —
		// tests configure requestStatusScript to reflect that themselves.
		writeJSON(w, http.StatusOK, map[string]interface{}{"id": a.ID, "status": a.Status})
	})

	m.srv = httptest.NewServer(mux)
	return m
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (m *mockPlatform) URL() string { return m.srv.URL }
func (m *mockPlatform) Close()      { m.srv.Close() }

func (m *mockPlatform) seedApprovalForUser(userID, itemID, title string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queue[userID] = append(m.queue[userID], mockQueueItem{ID: itemID, Title: title, Status: "pending"})
	m.approvals[itemID] = &mockApproval{ID: itemID, Status: "pending"}
}
