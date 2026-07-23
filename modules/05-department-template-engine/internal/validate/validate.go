// Package validate checks templates against the canonical operating-model
// contract: required fields, enum values, org-chart integrity (single root,
// acyclic reporting lines), and cross-reference resolution (services →
// workflows/KPIs/positions, value streams → KPIs, compliance → governance
// rules). It is the single validation code path shared by catalog loading,
// POST /templates/{id}/validate, and the deploy orchestrator's configure stage.
package validate

import (
	"fmt"

	"github.com/operan/modules/05-department-template-engine/internal/store"
)

// Issue is one validation finding. Errors block deploys; warnings do not.
type Issue struct {
	Path     string `json:"path"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // error, warning
}

func errf(path, format string, args ...interface{}) Issue {
	return Issue{Path: path, Message: fmt.Sprintf(format, args...), Severity: "error"}
}

func warnf(path, format string, args ...interface{}) Issue {
	return Issue{Path: path, Message: fmt.Sprintf(format, args...), Severity: "warning"}
}

var validAutonomyTiers = map[string]bool{
	"": true, "recommend": true, "analyze": true, "coordinate": true, "draft": true, "execute": true,
}
var validRoleTypes = map[string]bool{
	"director": true, "manager": true, "specialist": true, "coordinator": true, "analyst": true, "support": true,
}
var validHolderTypes = map[string]bool{"ai_agent": true, "human": true, "vacant": true}
var validSeverities = map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
var validLikelihoods = map[string]bool{
	"rare": true, "unlikely": true, "possible": true, "likely": true, "almost_certain": true,
}

// Template validates a template's operating model. Only "error" severity
// issues should block deployment.
func Template(t *store.Template) []Issue {
	var issues []Issue

	if t.Name == "" {
		issues = append(issues, errf("name", "template name is required"))
	}
	if t.Category == "" {
		issues = append(issues, errf("category", "template category is required"))
	}

	// ─── Index the referenceable entities ────────────────────────────────
	agentIDs := map[string]bool{}
	for i, a := range t.Agents {
		path := fmt.Sprintf("agents[%d]", i)
		if a.ID == "" {
			issues = append(issues, errf(path, "agent id is required"))
			continue
		}
		if agentIDs[a.ID] {
			issues = append(issues, errf(path, "duplicate agent id %q", a.ID))
		}
		agentIDs[a.ID] = true
		if a.Role == "" {
			issues = append(issues, errf(path+".role", "agent role is required"))
		}
		if !validAutonomyTiers[a.AutonomyTier] {
			issues = append(issues, errf(path+".autonomy_tier", "invalid autonomy tier %q", a.AutonomyTier))
		}
	}
	workflowIDs := map[string]bool{}
	for _, w := range t.Workflows {
		workflowIDs[w.ID] = true
	}
	kpiIDs := map[string]bool{}
	for _, k := range t.KPIS {
		kpiIDs[k.ID] = true
	}
	ruleIDs := map[string]bool{}
	for _, g := range t.GovernanceRules {
		ruleIDs[g.ID] = true
	}
	serviceIDs := map[string]bool{}
	for _, s := range t.Services {
		serviceIDs[s.ID] = true
	}
	positionIDs := map[string]bool{}
	for _, p := range t.OrgChart {
		positionIDs[p.ID] = true
	}

	// ─── Agent reporting lines (blueprint chain of command) ──────────────
	for i, a := range t.Agents {
		if a.ReportsTo != "" && !agentIDs[a.ReportsTo] {
			issues = append(issues, errf(fmt.Sprintf("agents[%d].reports_to", i),
				"references unknown agent %q", a.ReportsTo))
		}
		for _, svc := range a.Services {
			if !serviceIDs[svc] {
				issues = append(issues, warnf(fmt.Sprintf("agents[%d].services", i),
					"references unknown service %q", svc))
			}
		}
	}
	issues = append(issues, checkAgentCycles(t.Agents)...)

	// ─── Org chart integrity ─────────────────────────────────────────────
	if len(t.OrgChart) > 0 {
		roots := 0
		for i, p := range t.OrgChart {
			path := fmt.Sprintf("org_chart[%d]", i)
			if p.ID == "" {
				issues = append(issues, errf(path, "position id is required"))
				continue
			}
			if p.Title == "" {
				issues = append(issues, errf(path+".title", "position title is required"))
			}
			if !validRoleTypes[p.RoleType] {
				issues = append(issues, errf(path+".role_type", "invalid role type %q", p.RoleType))
			}
			if !validHolderTypes[p.HolderType] {
				issues = append(issues, errf(path+".holder_type", "invalid holder type %q", p.HolderType))
			}
			if p.ReportsTo == "" {
				roots++
			} else if !positionIDs[p.ReportsTo] {
				issues = append(issues, errf(path+".reports_to", "references unknown position %q", p.ReportsTo))
			}
			if p.EscalatesTo != "" && !positionIDs[p.EscalatesTo] {
				issues = append(issues, errf(path+".escalates_to", "references unknown position %q", p.EscalatesTo))
			}
			if p.HolderType == "ai_agent" && p.AgentDefID != "" && !agentIDs[p.AgentDefID] {
				issues = append(issues, errf(path+".agent_def_id", "references unknown agent %q", p.AgentDefID))
			}
			if !validAutonomyTiers[p.AutonomyTier] {
				issues = append(issues, errf(path+".autonomy_tier", "invalid autonomy tier %q", p.AutonomyTier))
			}
		}
		if roots == 0 {
			issues = append(issues, errf("org_chart", "no root position (every position has reports_to — cycle or misconfiguration)"))
		}
		if roots > 1 {
			issues = append(issues, warnf("org_chart", "%d root positions (expected exactly 1)", roots))
		}
		issues = append(issues, checkPositionCycles(t.OrgChart)...)
	}

	// ─── Service portfolio references ────────────────────────────────────
	for i, s := range t.Services {
		path := fmt.Sprintf("services[%d]", i)
		if s.ID == "" || s.Name == "" {
			issues = append(issues, errf(path, "service id and name are required"))
			continue
		}
		if s.OwnerPositionID != "" && !positionIDs[s.OwnerPositionID] {
			issues = append(issues, errf(path+".owner_position_id", "references unknown position %q", s.OwnerPositionID))
		}
		if s.OwnerAgentDefID != "" && !agentIDs[s.OwnerAgentDefID] {
			issues = append(issues, errf(path+".owner_agent_def_id", "references unknown agent %q", s.OwnerAgentDefID))
		}
		if s.DeliveryWorkflowID != "" && !workflowIDs[s.DeliveryWorkflowID] {
			issues = append(issues, errf(path+".delivery_workflow_id", "references unknown workflow %q", s.DeliveryWorkflowID))
		}
		for _, ref := range s.KPIRefs {
			if !kpiIDs[ref] {
				issues = append(issues, errf(path+".kpi_refs", "references unknown KPI %q", ref))
			}
		}
	}

	// ─── Value streams ───────────────────────────────────────────────────
	for i, vs := range t.ValueStreams {
		path := fmt.Sprintf("value_streams[%d]", i)
		if vs.ID == "" || vs.Name == "" {
			issues = append(issues, errf(path, "value stream id and name are required"))
			continue
		}
		if len(vs.Stages) == 0 {
			issues = append(issues, errf(path+".stages", "value stream has no stages"))
		}
		for j, st := range vs.Stages {
			sp := fmt.Sprintf("%s.stages[%d]", path, j)
			if st.OwnerPositionID != "" && !positionIDs[st.OwnerPositionID] {
				issues = append(issues, errf(sp+".owner_position_id", "references unknown position %q", st.OwnerPositionID))
			}
			if st.WorkflowRef != "" && !workflowIDs[st.WorkflowRef] {
				issues = append(issues, errf(sp+".workflow_ref", "references unknown workflow %q", st.WorkflowRef))
			}
		}
		for _, ref := range vs.ValueMetricKPIRefs {
			if !kpiIDs[ref] {
				issues = append(issues, errf(path+".value_metric_kpi_refs", "references unknown KPI %q", ref))
			}
		}
	}

	// ─── Risks / quality / compliance ────────────────────────────────────
	for i, rk := range t.Risks {
		path := fmt.Sprintf("risks[%d]", i)
		if rk.ID == "" || rk.Name == "" {
			issues = append(issues, errf(path, "risk id and name are required"))
			continue
		}
		if !validSeverities[rk.Severity] {
			issues = append(issues, errf(path+".severity", "invalid severity %q", rk.Severity))
		}
		if !validLikelihoods[rk.Likelihood] {
			issues = append(issues, errf(path+".likelihood", "invalid likelihood %q", rk.Likelihood))
		}
		if rk.OwnerPositionID != "" && !positionIDs[rk.OwnerPositionID] {
			issues = append(issues, errf(path+".owner_position_id", "references unknown position %q", rk.OwnerPositionID))
		}
		if rk.ServiceRef != "" && !serviceIDs[rk.ServiceRef] {
			issues = append(issues, errf(path+".service_ref", "references unknown service %q", rk.ServiceRef))
		}
		if rk.AgentDefID != "" && !agentIDs[rk.AgentDefID] {
			issues = append(issues, errf(path+".agent_def_id", "references unknown agent %q", rk.AgentDefID))
		}
	}
	for i, q := range t.QualityStandards {
		path := fmt.Sprintf("quality_standards[%d]", i)
		if q.ID == "" || q.Name == "" || q.Target == "" {
			issues = append(issues, errf(path, "quality standard id, name and target are required"))
			continue
		}
		if q.MeasureKPIRef != "" && !kpiIDs[q.MeasureKPIRef] {
			issues = append(issues, errf(path+".measure_kpi_ref", "references unknown KPI %q", q.MeasureKPIRef))
		}
		if q.ServiceRef != "" && !serviceIDs[q.ServiceRef] {
			issues = append(issues, errf(path+".service_ref", "references unknown service %q", q.ServiceRef))
		}
	}
	for i, c := range t.ComplianceControls {
		path := fmt.Sprintf("compliance_controls[%d]", i)
		if c.ID == "" || c.Framework == "" || c.Name == "" {
			issues = append(issues, errf(path, "compliance control id, framework and name are required"))
			continue
		}
		for _, ref := range c.GovernanceRuleRefs {
			if !ruleIDs[ref] {
				issues = append(issues, errf(path+".governance_rule_refs", "references unknown governance rule %q", ref))
			}
		}
		for _, id := range c.AgentDefIDs {
			if !agentIDs[id] {
				issues = append(issues, errf(path+".agent_def_ids", "references unknown agent %q", id))
			}
		}
	}

	return issues
}

// Errors filters issues down to blocking errors.
func Errors(issues []Issue) []Issue {
	var out []Issue
	for _, i := range issues {
		if i.Severity == "error" {
			out = append(out, i)
		}
	}
	return out
}

// checkPositionCycles detects reports_to cycles among positions.
func checkPositionCycles(positions []store.Position) []Issue {
	parent := map[string]string{}
	for _, p := range positions {
		if p.ID != "" {
			parent[p.ID] = p.ReportsTo
		}
	}
	var issues []Issue
	for id := range parent {
		seen := map[string]bool{}
		cur := id
		for cur != "" {
			if seen[cur] {
				issues = append(issues, errf("org_chart", "reporting cycle involving position %q", cur))
				return issues // one report is enough
			}
			seen[cur] = true
			cur = parent[cur]
		}
	}
	return issues
}

// checkAgentCycles detects reports_to cycles among template agents.
func checkAgentCycles(agents []store.AgentDefinition) []Issue {
	parent := map[string]string{}
	for _, a := range agents {
		if a.ID != "" {
			parent[a.ID] = a.ReportsTo
		}
	}
	var issues []Issue
	for id := range parent {
		seen := map[string]bool{}
		cur := id
		for cur != "" {
			if seen[cur] {
				issues = append(issues, errf("agents", "reporting cycle involving agent %q", cur))
				return issues
			}
			seen[cur] = true
			cur = parent[cur]
		}
	}
	return issues
}
