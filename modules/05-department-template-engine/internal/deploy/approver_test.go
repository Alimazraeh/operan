package deploy

import (
	"testing"

	"github.com/operan/modules/05-department-template-engine/internal/clients"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

func gateSOP(requiredBy interface{}) *store.WorkflowDefinition {
	return &store.WorkflowDefinition{
		ID: "wf-1", Name: "Change Management",
		Steps: []store.WorkflowStep{
			{ID: "s1", Type: "agent_call", Name: "Draft", Config: map[string]interface{}{"agent": "sys-admin-01"}},
			{ID: "s2", Type: "approval", Name: "CAB Review", Config: map[string]interface{}{"required_by": requiredBy}},
		},
	}
}

func deptWith(holderType, humanRef string) *store.Department {
	return &store.Department{
		ID: "d1", TenantID: "t1",
		OrgChart: []store.Position{
			{ID: "pos-mgr", Title: "IT Manager", AgentDefID: "it-manager-01",
				HolderType: holderType, HumanRef: humanRef},
		},
	}
}

// gateParams returns the compiled parameters of the first human_gate node.
func gateParams(t *testing.T, wf *store.WorkflowDefinition, dept *store.Department) map[string]interface{} {
	t.Helper()
	out := CompileWorkflowFor(wf, "d1", map[string]string{}, dept)
	nodes, ok := out.Graph["nodes"].([]clients.WorkflowNode)
	if !ok {
		t.Fatalf("graph nodes have unexpected type %T", out.Graph["nodes"])
	}
	for _, n := range nodes {
		if n.Type == "human_gate" {
			return n.Parameters
		}
	}
	t.Fatal("no human_gate node compiled")
	return nil
}

// A gate step names its approver as an agent-definition id; when a real person
// holds the seat carrying that definition, the compiled node must carry them.
func TestCompileResolvesGateApproverFromTheOrgChart(t *testing.T) {
	p := gateParams(t, gateSOP("it-manager-01"), deptWith("human", "user-dana"))
	if p["required_approver_user_id"] != "user-dana" {
		t.Errorf("approver = %v, want user-dana", p["required_approver_user_id"])
	}
	if p["required_approver_agent_def_id"] != "it-manager-01" {
		t.Errorf("provenance lost: %v", p["required_approver_agent_def_id"])
	}
}

// The catalogue also spells required_by as a list.
func TestCompileResolvesApproverFromAList(t *testing.T) {
	p := gateParams(t, gateSOP([]interface{}{"nobody-01", "it-manager-01"}),
		deptWith("human", "user-dana"))
	if p["required_approver_user_id"] != "user-dana" {
		t.Errorf("approver = %v, want user-dana", p["required_approver_user_id"])
	}
}

// An agent-held or vacant seat must resolve to nothing, so the orchestrator
// falls back to a role target rather than inventing an approver. 171 of 172
// seats across the catalogue are agent-held, so this is the common path.
func TestCompileLeavesApproverUnsetWhenNoHumanHoldsTheSeat(t *testing.T) {
	for _, dept := range []*store.Department{
		deptWith("ai_agent", ""),
		deptWith("vacant", ""),
		deptWith("human", ""), // bound to nobody
		nil,                   // no department in hand
	} {
		p := gateParams(t, gateSOP("it-manager-01"), dept)
		if _, present := p["required_approver_user_id"]; present {
			t.Errorf("an approver was invented for a seat nobody holds: %v", p)
		}
	}
}

// A gate naming a definition no seat carries resolves to nothing.
func TestCompileIgnoresUnknownRequiredBy(t *testing.T) {
	p := gateParams(t, gateSOP("finance-manager-99"), deptWith("human", "user-dana"))
	if _, present := p["required_approver_user_id"]; present {
		t.Errorf("unknown required_by produced an approver: %v", p)
	}
}
