package store

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

// DepartmentStore provides tenant-isolated CRUD for deployed department instances.
type DepartmentStore struct {
	mu          sync.RWMutex
	departments map[string]*Department // id -> department
	byTenant    map[string][]string    // tenantID -> []departmentIDs
}

// NewDepartmentStore creates a new empty DepartmentStore.
func NewDepartmentStore() *DepartmentStore {
	return &DepartmentStore{
		departments: make(map[string]*Department),
		byTenant:    make(map[string][]string),
	}
}

// Create adds a new department. The ID and timestamps are auto-generated.
func (s *DepartmentStore) Create(d *Department) (*Department, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	if d.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	d.CreatedAt = timeNow()
	d.UpdatedAt = d.CreatedAt
	if d.Status == "" {
		d.Status = "provisioning"
	}

	s.departments[d.ID] = d
	s.byTenant[d.TenantID] = append(s.byTenant[d.TenantID], d.ID)
	cp := *d
	return &cp, nil
}

// GetByIDAndTenant retrieves a department by ID, verifying tenant ownership.
func (s *DepartmentStore) GetByIDAndTenant(id, tenantID string) (*Department, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.departments[id]
	if !ok || d.TenantID != tenantID {
		return nil, ErrNotFound
	}
	cp := *d
	return &cp, nil
}

// List returns departments for the given tenant with pagination and optional
// category/status filters.
func (s *DepartmentStore) List(tenantID string, page, pageSize int, filterCategory, filterStatus *string) ([]Department, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return nil, 0, false
	}

	var all []*Department
	for _, id := range ids {
		d, exists := s.departments[id]
		if !exists {
			continue
		}
		if filterCategory != nil && *filterCategory != "" && d.Category != *filterCategory {
			continue
		}
		if filterStatus != nil && *filterStatus != "" && d.Status != *filterStatus {
			continue
		}
		all = append(all, d)
	}

	total := len(all)
	hasMore := false
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	if end < total {
		hasMore = true
	}

	result := make([]Department, 0, end-start)
	for i := start; i < end; i++ {
		cp := *all[i]
		result = append(result, cp)
	}
	return result, total, hasMore
}

// UpdateByTenant partially updates a department with tenant verification.
// Operating-model sections are replaced wholesale when present in the patch.
func (s *DepartmentStore) UpdateByTenant(id, tenantID string, patch map[string]interface{}) (*Department, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.departments[id]
	if !ok || d.TenantID != tenantID {
		return nil, ErrNotFound
	}

	if v, exists := patch["name"]; exists {
		if str, ok := v.(string); ok && str != "" {
			d.Name = str
		}
	}
	if v, exists := patch["description"]; exists {
		if str, ok := v.(string); ok {
			d.Description = str
		}
	}
	if v, exists := patch["mission"]; exists {
		if str, ok := v.(string); ok {
			d.Mission = str
		}
	}
	if v, exists := patch["status"]; exists {
		if str, ok := v.(string); ok && str != "" {
			d.Status = str
		}
	}
	if v, exists := patch["environment"]; exists {
		if str, ok := v.(string); ok && str != "" {
			d.Environment = str
		}
	}
	if v, exists := patch["metadata"]; exists {
		if m, ok := v.(map[string]interface{}); ok {
			d.Metadata = m
		}
	}
	if v, exists := patch["business_logic"]; exists && v != nil {
		b, _ := json.Marshal(v)
		json.Unmarshal(b, &d.BusinessLogic)
	}
	remarshal := func(v interface{}, dst interface{}) {
		b, _ := json.Marshal(v)
		json.Unmarshal(b, dst)
	}
	if v, exists := patch["org_chart"]; exists {
		remarshal(v, &d.OrgChart)
	}
	if v, exists := patch["services"]; exists {
		remarshal(v, &d.Services)
	}
	if v, exists := patch["value_streams"]; exists {
		remarshal(v, &d.ValueStreams)
	}
	if v, exists := patch["risks"]; exists {
		remarshal(v, &d.Risks)
	}
	if v, exists := patch["quality_standards"]; exists {
		remarshal(v, &d.QualityStandards)
	}
	if v, exists := patch["compliance_controls"]; exists {
		remarshal(v, &d.ComplianceControls)
	}
	if v, exists := patch["governance_rules"]; exists {
		remarshal(v, &d.GovernanceRules)
	}
	if v, exists := patch["kpis"]; exists {
		remarshal(v, &d.KPIS)
	}
	if v, exists := patch["operational_policies"]; exists {
		remarshal(v, &d.OperationalPolicies)
	}

	d.UpdatedAt = timeNow()
	cp := *d
	return &cp, nil
}

// Replace overwrites a stored department wholesale (used by the deploy
// orchestrator, which owns the instance during provisioning). The stored
// value is a deep copy so the orchestrator can keep mutating its own
// instance without racing concurrent readers.
func (s *DepartmentStore) Replace(d *Department) (*Department, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur, ok := s.departments[d.ID]
	if !ok || cur.TenantID != d.TenantID {
		return nil, ErrNotFound
	}
	d.CreatedAt = cur.CreatedAt
	d.UpdatedAt = timeNow()

	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	var stored Department
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	s.departments[d.ID] = &stored
	cp := stored
	return &cp, nil
}

// UpdateStatus sets the department status.
func (s *DepartmentStore) UpdateStatus(id, tenantID, status string) (*Department, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.departments[id]
	if !ok || d.TenantID != tenantID {
		return nil, ErrNotFound
	}
	d.Status = status
	d.UpdatedAt = timeNow()
	cp := *d
	return &cp, nil
}

// Archive marks a department archived (soft delete).
func (s *DepartmentStore) Archive(id, tenantID string) (*Department, error) {
	return s.UpdateStatus(id, tenantID, "archived")
}

// ─── Persistence (snapshot Export/Import) ────────────────────────────────────

type departmentSnapshot struct {
	Departments map[string]*Department `json:"departments"`
	ByTenant    map[string][]string    `json:"by_tenant"`
}

// Export dumps the full store state as JSON.
func (s *DepartmentStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(departmentSnapshot{Departments: s.departments, ByTenant: s.byTenant})
}

// Import restores the full store state from JSON.
func (s *DepartmentStore) Import(data []byte) error {
	var snap departmentSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if snap.Departments != nil {
		s.departments = snap.Departments
	}
	if snap.ByTenant != nil {
		s.byTenant = snap.ByTenant
	}
	return nil
}
