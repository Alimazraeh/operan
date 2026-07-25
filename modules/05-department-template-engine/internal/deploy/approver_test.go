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

// The manual-handling gate raised for a service with no SOP names no approver,
// so it targeted the role department_head and sat in a queue nobody owns. When
// a real person holds the department's root seat, the gate must reach them by
// name — that is the same target, resolved, and it is the difference between an
// approval sitting in a role queue and one arriving in somebody's inbox.
func manualGateSOP() *store.WorkflowDefinition {
	return &store.WorkflowDefinition{
		ID: "manual-1", Name: "Manual handling",
		Steps: []store.WorkflowStep{{ID: "gate-manual", Type: "approval", Name: "Handle and sign off"}},
	}
}

func TestUnroutedGateGoesToTheDepartmentHeadWhoHoldsTheSeat(t *testing.T) {
	dept := &store.Department{
		ID: "d1", TenantID: "t1",
		OrgChart: []store.Position{
			{ID: "pos-mgr", Title: "IT Manager", AgentDefID: "it-manager-01",
				HolderType: "human", HumanRef: "user-dana"},
			{ID: "pos-sys", Title: "Systems Administrator", AgentDefID: "sys-admin-01",
				HolderType: "ai_agent", ReportsTo: "pos-mgr"},
		},
	}
	p := gateParams(t, manualGateSOP(), dept)
	if p["required_approver_user_id"] != "user-dana" {
		t.Fatalf("gate approver = %v, want user-dana (%+v)", p["required_approver_user_id"], p)
	}
	if p["required_approver_source"] != "department_head_seat" {
		t.Fatalf("approver source = %v, want department_head_seat", p["required_approver_source"])
	}
}

// With nobody holding the root seat the gate stays on its role target. A
// fabricated approver is worse than an unowned queue: it records a named person
// as responsible for something they have never seen.
func TestUnroutedGateWithNoHumanHeadNamesNobody(t *testing.T) {
	p := gateParams(t, manualGateSOP(), deptWith("ai_agent", ""))
	if _, present := p["required_approver_user_id"]; present {
		t.Fatalf("named an approver for an unbound department: %+v", p)
	}
}

// An approver the SOP actually names still wins; the seat fallback must not
// redirect a gate that was routed deliberately.
func TestDeclaredApproverBeatsTheDepartmentHeadFallback(t *testing.T) {
	dept := &store.Department{
		ID: "d1", TenantID: "t1",
		OrgChart: []store.Position{
			{ID: "pos-mgr", Title: "IT Manager", AgentDefID: "it-manager-01",
				HolderType: "human", HumanRef: "user-dana"},
			{ID: "pos-sec", Title: "Security Lead", AgentDefID: "sec-lead-01",
				HolderType: "human", HumanRef: "user-sam", ReportsTo: "pos-mgr"},
		},
	}
	p := gateParams(t, gateSOP("sec-lead-01"), dept)
	if p["required_approver_user_id"] != "user-sam" {
		t.Fatalf("approver = %v, want the declared user-sam", p["required_approver_user_id"])
	}
}

// A step that names an approver who cannot be resolved must stay unrouted. The
// department-head fallback exists for gates that name nobody; applying it here
// would record the IT manager as the signatory for a decision the SOP said
// belongs to someone else.
func TestUnresolvableDeclaredApproverIsNotReplacedByTheHead(t *testing.T) {
	p := gateParams(t, gateSOP("finance-manager-99"), deptWith("human", "user-dana"))
	if got, present := p["required_approver_user_id"]; present {
		t.Fatalf("substituted %v for an approver the SOP named but could not resolve", got)
	}
}

// A capability-bearing action node must carry whose authority the verb runs
// under: the named agent's seat, or the department root when the step names
// nobody. The funnel treats unknown authority as no authority, so a compile
// that drops the tier turns every write verb into a denial.
func TestCompileCarriesActorAuthorityOntoCapabilityNodes(t *testing.T) {
	dept := &store.Department{
		ID: "d1", TenantID: "t1",
		OrgChart: []store.Position{
			{ID: "pos-mgr", Title: "IT Manager", AgentDefID: "it-manager-01",
				HolderType: "human", HumanRef: "user-dana", AutonomyTier: "coordinate"},
			{ID: "pos-sys", Title: "Systems Administrator", AgentDefID: "sys-admin-01",
				HolderType: "ai_agent", AgentID: "agent-sys-live", ReportsTo: "pos-mgr", AutonomyTier: "execute"},
		},
	}
	wf := &store.WorkflowDefinition{
		ID: "wf-x", Name: "X",
		Steps: []store.WorkflowStep{
			{ID: "s1", Type: "tool_call", Name: "Execute Restore", Config: map[string]interface{}{
				"capability": "itops.backup.restore", "agent": "sys-admin-01",
				"inputs": map[string]interface{}{"request_ref": "{{request_id}}"},
			}},
			{ID: "s2", Type: "notification", Name: "Notify", Config: map[string]interface{}{
				"capability": "comms.message.send",
				"inputs":     map[string]interface{}{"channel": "email", "message": "done"},
			}},
		},
	}
	out := CompileWorkflowFor(wf, dept.ID, map[string]string{"sys-admin-01": "agent-sys-live"}, dept)
	nodes, _ := out.Graph["nodes"].([]clients.WorkflowNode)
	if len(nodes) != 2 {
		t.Fatalf("nodes = %d", len(nodes))
	}
	// s1 names sys-admin-01 → its seat, execute tier, live agent id.
	p1 := nodes[0].Parameters
	if p1["actor_position_id"] != "pos-sys" || p1["actor_autonomy_tier"] != "execute" || p1["actor_agent_id"] != "agent-sys-live" {
		t.Fatalf("s1 actor wrong: %v", p1)
	}
	// s2 names nobody → the department root coordinates it.
	p2 := nodes[1].Parameters
	if p2["actor_position_id"] != "pos-mgr" || p2["actor_autonomy_tier"] != "coordinate" {
		t.Fatalf("s2 actor wrong: %v", p2)
	}
}
