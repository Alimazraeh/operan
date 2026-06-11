package store

import "encoding/json"

// Export/Import implement the persist.Store contract: full-state JSON
// snapshots used for restart persistence. Approval/Escalation/Intervention
// TenantID fields are json:"-" in API responses, so snapshots wrap each
// item with its tenant; indexes are rebuilt on import.

type persistedApproval struct {
	TenantID string   `json:"tenant_id"`
	Item     Approval `json:"item"`
}

// Export dumps all approvals as JSON.
func (s *ApprovalStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]persistedApproval, 0, len(s.approvals))
	for _, a := range s.approvals {
		items = append(items, persistedApproval{TenantID: a.TenantID, Item: *a})
	}
	return json.Marshal(items)
}

// Import replaces the store contents from a JSON snapshot.
func (s *ApprovalStore) Import(data []byte) error {
	var items []persistedApproval
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvals = make(map[string]*Approval, len(items))
	s.byTenant = make(map[string][]string)
	s.byRequest = make(map[string]string)
	for i := range items {
		a := items[i].Item
		a.TenantID = items[i].TenantID
		s.approvals[a.ID] = &a
		s.byTenant[a.TenantID] = append(s.byTenant[a.TenantID], a.ID)
		s.byRequest[a.TenantID+"|"+a.RequestID] = a.ID
	}
	return nil
}

type persistedEscalation struct {
	TenantID string     `json:"tenant_id"`
	Item     Escalation `json:"item"`
}

// Export dumps all escalations as JSON.
func (s *EscalationStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]persistedEscalation, 0, len(s.escalations))
	for _, e := range s.escalations {
		items = append(items, persistedEscalation{TenantID: e.TenantID, Item: *e})
	}
	return json.Marshal(items)
}

// Import replaces the store contents from a JSON snapshot.
func (s *EscalationStore) Import(data []byte) error {
	var items []persistedEscalation
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.escalations = make(map[string]*Escalation, len(items))
	s.byTenant = make(map[string][]string)
	for i := range items {
		e := items[i].Item
		e.TenantID = items[i].TenantID
		s.escalations[e.ID] = &e
		s.byTenant[e.TenantID] = append(s.byTenant[e.TenantID], e.ID)
	}
	return nil
}

type persistedIntervention struct {
	TenantID string       `json:"tenant_id"`
	Item     Intervention `json:"item"`
}

// Export dumps all interventions as JSON.
func (s *InterventionStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]persistedIntervention, 0, len(s.interventions))
	for _, iv := range s.interventions {
		items = append(items, persistedIntervention{TenantID: iv.TenantID, Item: *iv})
	}
	return json.Marshal(items)
}

// Import replaces the store contents from a JSON snapshot.
func (s *InterventionStore) Import(data []byte) error {
	var items []persistedIntervention
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interventions = make(map[string]*Intervention, len(items))
	s.byTenant = make(map[string][]string)
	for i := range items {
		iv := items[i].Item
		iv.TenantID = items[i].TenantID
		s.interventions[iv.ID] = &iv
		s.byTenant[iv.TenantID] = append(s.byTenant[iv.TenantID], iv.ID)
	}
	return nil
}

type persistedAnswer struct {
	Key  string     `json:"key"` // tenantID|requestID
	Item HitlAnswer `json:"item"`
}

// Export dumps all HITL answers as JSON.
func (s *HitlStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]persistedAnswer, 0, len(s.answers))
	for k, a := range s.answers {
		items = append(items, persistedAnswer{Key: k, Item: *a})
	}
	return json.Marshal(items)
}

// Import replaces the store contents from a JSON snapshot.
func (s *HitlStore) Import(data []byte) error {
	var items []persistedAnswer
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.answers = make(map[string]*HitlAnswer, len(items))
	for i := range items {
		a := items[i].Item
		s.answers[items[i].Key] = &a
	}
	return nil
}
