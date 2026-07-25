package store

import (
	"context"
	"log"

	"github.com/operan/modules/04-agent-registry/internal/database"
)

// Durability is attached to the in-memory stores rather than wrapping them.
//
// The alternative — a PersistentAgentStore implementing an interface the
// handlers accept — would mean introducing interfaces for four stores and
// touching every handler signature to fix a bug about restarts. A write-through
// sink is the smaller change and leaves reads on the memory fast path, which is
// what the stores were built for.
type sink struct{ db *database.AgentStore }

// ─── Agents ─────────────────────────────────────────────────────────────────

// Persist attaches a durable backing store. Called once at boot; when it is not
// called the store behaves exactly as before, in memory only.
func (s *AgentStore) Persist(db *database.AgentStore) { s.sink = &sink{db: db} }

// save writes an agent through to the database. A failed write is loud: the
// agent is in memory and will answer requests until the next restart, at which
// point it silently disappears — which is the whole failure this file exists to
// stop, so it must not pass unremarked.
func (s *AgentStore) save(ctx context.Context, a *Agent) {
	if s.sink == nil {
		return
	}
	detail, err := database.MarshalJSONB(agentDetail{
		Objectives:         a.Objectives,
		MemoryAccess:       a.MemoryAccess,
		EscalationRules:    a.EscalationRules,
		GovernancePolicies: a.GovernancePolicies,
		SupportedLanguages: a.SupportedLanguages,
		Capabilities:       a.Capabilities,
		Tools:              a.Tools,
		RuntimeConstraints: a.RuntimeConstraints,
		CostProfile:        a.CostProfile,
		ExecutionBudget:    a.ExecutionBudget,
		AccessControl:      a.AccessControl,
	})
	if err != nil {
		log.Printf("[REGISTRY] agent %s not persisted (detail encode failed: %v) — it will be lost on restart", a.ID, err)
		return
	}
	row := database.AgentRow{
		ID: a.ID, TenantID: a.TenantID, Name: a.Name, Role: a.Role,
		Description: a.Description, DepartmentID: a.DepartmentID,
		Status: string(a.Status), CurrentVersionID: a.CurrentVersionID,
		Detail: detail, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
	if err := s.sink.db.UpsertAgent(ctx, row); err != nil {
		log.Printf("[REGISTRY] agent %s not persisted (%v) — it will be lost on restart", a.ID, err)
	}
}

// agentDetail is the JSON document in registry_agents.detail. Field names are
// pinned so a rename in the API type cannot silently orphan stored data.
type agentDetail struct {
	Objectives         []Objective         `json:"objectives,omitempty"`
	MemoryAccess       *MemoryAccess       `json:"memory_access,omitempty"`
	EscalationRules    []string            `json:"escalation_rules,omitempty"`
	GovernancePolicies []string            `json:"governance_policies,omitempty"`
	SupportedLanguages []string            `json:"supported_languages,omitempty"`
	Capabilities       []string            `json:"capabilities,omitempty"`
	Tools              []string            `json:"tools,omitempty"`
	RuntimeConstraints *RuntimeConstraints `json:"runtime_constraints,omitempty"`
	CostProfile        *CostProfile        `json:"cost_profile,omitempty"`
	ExecutionBudget    *ExecutionBudget    `json:"execution_budget,omitempty"`
	AccessControl      *AccessControl      `json:"access_control,omitempty"`
}

// HydrateAgents loads persisted agents into memory. Without this the rows are
// invisible after a restart and persistence is write-only.
func (s *AgentStore) HydrateAgents(ctx context.Context, db *database.AgentStore) (int, error) {
	rows, err := db.LoadAgents(ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		var d agentDetail
		if len(r.Detail) > 0 {
			if err := unmarshalJSON(r.Detail, &d); err != nil {
				// One unreadable row must not stop the other agents loading.
				log.Printf("[REGISTRY] agent %s has unreadable detail (%v) — loaded without it", r.ID, err)
			}
		}
		a := &Agent{
			ID: r.ID, TenantID: r.TenantID, Name: r.Name, Role: r.Role,
			Description: r.Description, DepartmentID: r.DepartmentID,
			Status: AgentStatus(r.Status), CurrentVersionID: r.CurrentVersionID,
			Objectives: d.Objectives, Capabilities: orEmptySlice(d.Capabilities),
			Tools: orEmptySlice(d.Tools), MemoryAccess: d.MemoryAccess,
			EscalationRules:    orEmptySlice(d.EscalationRules),
			GovernancePolicies: orEmptySlice(d.GovernancePolicies),
			SupportedLanguages: d.SupportedLanguages,
			RuntimeConstraints: d.RuntimeConstraints, CostProfile: d.CostProfile,
			ExecutionBudget: d.ExecutionBudget, AccessControl: d.AccessControl,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		s.agents[a.ID] = a
		if s.byTenant[a.TenantID] == nil {
			s.byTenant[a.TenantID] = make(map[string]*Agent)
		}
		s.byTenant[a.TenantID][a.ID] = a
	}
	return len(rows), nil
}

// ─── Versions ───────────────────────────────────────────────────────────────

// Persist attaches a durable backing store to the version store.
func (s *VersionStore) Persist(db *database.AgentStore) { s.sink = &sink{db: db} }

func (s *VersionStore) save(ctx context.Context, v *AgentVersion) {
	if s.sink == nil {
		return
	}
	modelCfg, err := database.MarshalJSONB(v.ModelConfig)
	if err != nil {
		log.Printf("[REGISTRY] version %s not persisted (model_config encode failed: %v)", v.ID, err)
		return
	}
	promoted, err := database.MarshalJSONB(v.PromotedTo)
	if err != nil {
		log.Printf("[REGISTRY] version %s not persisted (promoted_to encode failed: %v)", v.ID, err)
		return
	}
	row := database.VersionRow{
		ID: v.ID, AgentID: v.AgentID, TenantID: v.TenantID, Version: v.Version,
		Status: string(v.Status), Description: v.Description,
		ChangeSummary: v.ChangeSummary, DiffFromPrevious: v.DiffFromPrevious,
		PromptTemplateRef: v.PromptTemplateRef, CreatedBy: v.CreatedBy,
		ModelConfig: modelCfg, PromotedTo: promoted,
		CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
	if err := s.sink.db.UpsertVersion(ctx, row); err != nil {
		log.Printf("[REGISTRY] version %s not persisted (%v) — it will be lost on restart", v.ID, err)
	}
}

// HydrateVersions loads persisted versions into memory.
func (s *VersionStore) HydrateVersions(ctx context.Context, db *database.AgentStore) (int, error) {
	rows, err := db.LoadVersions(ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		v := &AgentVersion{
			ID: r.ID, AgentID: r.AgentID, TenantID: r.TenantID, Version: r.Version,
			Status: VersionStatus(r.Status), Description: r.Description,
			ChangeSummary: r.ChangeSummary, DiffFromPrevious: r.DiffFromPrevious,
			PromptTemplateRef: r.PromptTemplateRef, CreatedBy: r.CreatedBy,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		if len(r.ModelConfig) > 0 {
			_ = unmarshalJSON(r.ModelConfig, &v.ModelConfig)
		}
		if len(r.PromotedTo) > 0 {
			_ = unmarshalJSON(r.PromotedTo, &v.PromotedTo)
		}
		s.versions[v.ID] = v
		if s.byTenant[v.TenantID] == nil {
			s.byTenant[v.TenantID] = make(map[string]*AgentVersion)
		}
		s.byTenant[v.TenantID][v.ID] = v
		if s.byAgent[v.AgentID] == nil {
			s.byAgent[v.AgentID] = make(map[string]*AgentVersion)
		}
		s.byAgent[v.AgentID][v.ID] = v
	}
	return len(rows), nil
}

// orEmptySlice keeps a nil slice out of the API response, where the contract
// says these fields are arrays.
func orEmptySlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
