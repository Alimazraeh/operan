// Package deploy runs the server-side department provisioning pipeline.
//
// A deploy materializes a Department instance from a Template blueprint and
// walks the stages select → configure → connect_data → provision_memory →
// deploy_swarm → operational, doing real work at each: validation, memory
// provisioning in Module 07, and agent registration in Module 04 (with
// department_id, reporting-derived escalation rules and KPI-derived
// objectives). Stage history, provisioned entities and events are recorded
// as it goes; any failure marks the deployment failed and the department
// degraded.
package deploy

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/operan/modules/05-department-template-engine/internal/clients"
	"github.com/operan/modules/05-department-template-engine/internal/events"
	"github.com/operan/modules/05-department-template-engine/internal/store"
	"github.com/operan/modules/05-department-template-engine/internal/validate"
)

// Orchestrator drives the provisioning pipeline for one deployment at a time.
type Orchestrator struct {
	Deployments   *store.DeploymentStore
	Departments   *store.DepartmentStore
	Publisher     *events.Publisher
	Registry      *clients.RegistryClient
	Memory        *clients.MemoryClient
	Orchestration *clients.OrchestrationClient
	// Timeout bounds the whole async pipeline.
	Timeout time.Duration
}

// MaterializeDepartment builds the Department instance from the template.
// Called synchronously by the deploy handler before responding 201.
func MaterializeDepartment(tmpl *store.Template, dep *store.TemplateDeployment, name, userID string) *store.Department {
	if name == "" {
		name = tmpl.Name
	}
	return &store.Department{
		TenantID:            dep.TenantID,
		Name:                name,
		Slug:                slugify(name),
		Category:            tmpl.Category,
		Description:         tmpl.Description,
		TemplateID:          tmpl.ID,
		TemplateVersion:     tmpl.Version,
		DeploymentID:        dep.ID,
		Status:              "provisioning",
		Mission:             missionOf(tmpl),
		BusinessLogic:       tmpl.BusinessLogic,
		OrgChart:            append([]store.Position(nil), tmpl.OrgChart...),
		Services:            servicesOf(tmpl),
		ValueStreams:        append([]store.ValueStream(nil), tmpl.ValueStreams...),
		Risks:               append([]store.RiskItem(nil), tmpl.Risks...),
		QualityStandards:    append([]store.QualityStandard(nil), tmpl.QualityStandards...),
		ComplianceControls:  append([]store.ComplianceControl(nil), tmpl.ComplianceControls...),
		GovernanceRules:     append([]store.GovernanceRule(nil), tmpl.GovernanceRules...),
		KPIS:                append([]store.KPIDefinition(nil), tmpl.KPIS...),
		OperationalPolicies: append([]store.OperationalPolicy(nil), tmpl.OperationalPolicies...),
		Environment:         dep.Environment,
		Metadata:            map[string]interface{}{"template_metadata": tmpl.Metadata},
		CreatedBy:           userID,
	}
}

// servicesOf returns the template's service portfolio; when a template
// defines none, the portfolio is derived from its workflows — each workflow
// is an SOP the department delivers, owned by the org chart's root position.
func servicesOf(t *store.Template) []store.ServiceOffering {
	if len(t.Services) > 0 {
		return append([]store.ServiceOffering(nil), t.Services...)
	}
	var rootPos string
	for _, p := range t.OrgChart {
		if p.ReportsTo == "" {
			rootPos = p.ID
			break
		}
	}
	var consumers []string
	if t.BusinessLogic != nil && len(t.BusinessLogic.Stakeholders) > 0 {
		consumers = append(consumers, t.BusinessLogic.Stakeholders...)
	} else {
		consumers = []string{"internal"}
	}
	out := make([]store.ServiceOffering, 0, len(t.Workflows))
	for _, wf := range t.Workflows {
		out = append(out, store.ServiceOffering{
			ID:                 "svc-" + wf.ID,
			Name:               wf.Name,
			Description:        wf.Description,
			OwnerPositionID:    rootPos,
			Consumers:          consumers,
			DeliveryWorkflowID: wf.ID,
			Status:             "active",
		})
	}
	return out
}

func missionOf(t *store.Template) string {
	if t.BusinessLogic != nil && t.BusinessLogic.Purpose != "" {
		return t.BusinessLogic.Purpose
	}
	return t.Description
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// Run walks the pipeline asynchronously. auth/tenantID/userID are captured
// caller credentials (forwarded to Modules 04/07). Satisfies the handlers'
// DeployOrchestrator interface.
func (o *Orchestrator) Run(auth, tenantID, userID string, tmpl *store.Template, dep *store.TemplateDeployment, dept *store.Department) {
	timeout := o.Timeout
	if timeout <= 0 {
		// Memory provisioning triggers real embedding generation; give the
		// whole pipeline generous headroom.
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	caller := clients.Caller{Authorization: auth, TenantID: tenantID}

	fail := func(stage string, err error) {
		log.Printf("[DEPLOY] %s stage %s failed: %v", dep.ID, stage, err)
		o.recordStage(dep.ID, stage, "failed", err.Error())
		o.Deployments.Mutate(dep.ID, func(d *store.TemplateDeployment) {
			d.Status = "failed"
			d.ErrorMessage = fmt.Sprintf("%s: %v", stage, err)
		})
		o.Departments.UpdateStatus(dept.ID, tenantID, "degraded")
		o.Publisher.PublishDepartmentProvisioningFailed(events.DepartmentStagePayload{
			DepartmentID: dept.ID, DeploymentID: dep.ID, TenantID: tenantID,
			Stage: stage, Status: "failed", Detail: err.Error(), Timestamp: time.Now(),
		})
	}

	// ─── configure: validation + integrity + pre-flight ──────────────────
	o.beginStage(dep.ID, dept.ID, tenantID, "configure")
	if issues := validate.Errors(validate.Template(tmpl)); len(issues) > 0 {
		fail("configure", fmt.Errorf("template invalid: %s (%s)", issues[0].Message, issues[0].Path))
		return
	}
	if o.Registry != nil {
		if err := o.Registry.Ping(ctx, caller); err != nil {
			fail("configure", fmt.Errorf("registry pre-flight (deploy requires an admin token): %w", err))
			return
		}
	}
	o.completeStage(dep.ID, dept.ID, tenantID, "configure", fmt.Sprintf("validated: %d agents, %d services, %d positions", len(tmpl.Agents), len(tmpl.Services), len(tmpl.OrgChart)))

	// ─── connect_data: record integrations as pending connectors ─────────
	o.beginStage(dep.ID, dept.ID, tenantID, "connect_data")
	if len(tmpl.Integrations) > 0 {
		pending := make([]map[string]interface{}, 0, len(tmpl.Integrations))
		for _, ig := range tmpl.Integrations {
			pending = append(pending, map[string]interface{}{
				"id": ig.ID, "type": ig.Type, "name": ig.Name, "provider": ig.Provider, "status": "pending",
			})
		}
		dept.Metadata["pending_connectors"] = pending
		o.Departments.Replace(dept)
	}
	o.completeStage(dep.ID, dept.ID, tenantID, "connect_data", fmt.Sprintf("%d integrations recorded", len(tmpl.Integrations)))

	// ─── provision_memory: department charter + service descriptions ─────
	o.beginStage(dep.ID, dept.ID, tenantID, "provision_memory")
	if o.Memory != nil {
		items := memoryItems(dept, tmpl)
		ids, err := o.Memory.StoreVectors(ctx, caller, items)
		if err != nil {
			fail("provision_memory", err)
			return
		}
		dept.MemoryRefs = ids
		o.Departments.Replace(dept)
		o.Deployments.Mutate(dep.ID, func(d *store.TemplateDeployment) {
			if d.ProvisionedEntities == nil {
				d.ProvisionedEntities = map[string]interface{}{}
			}
			d.ProvisionedEntities["memory"] = ids
		})
	}
	o.completeStage(dep.ID, dept.ID, tenantID, "provision_memory", fmt.Sprintf("%d memory documents", len(dept.MemoryRefs)))

	// ─── deploy_swarm: register agents in Module 04 ──────────────────────
	o.beginStage(dep.ID, dept.ID, tenantID, "deploy_swarm")
	var provisioned []map[string]interface{}
	var firstErr error
	if o.Registry != nil {
		for i := range tmpl.Agents {
			ad := &tmpl.Agents[i]
			created, err := o.Registry.CreateAgent(ctx, caller, agentRequest(ad, tmpl, dept, tenantID))
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("agent %s: %w", ad.ID, err)
				}
				log.Printf("[DEPLOY] %s: agent %s failed: %v", dep.ID, ad.ID, err)
				continue
			}
			dept.AgentIDs = append(dept.AgentIDs, created.ID)
			// Backfill the org-chart position holding this agent def.
			for pi := range dept.OrgChart {
				if dept.OrgChart[pi].AgentDefID == ad.ID {
					dept.OrgChart[pi].AgentID = created.ID
				}
			}
			provisioned = append(provisioned, map[string]interface{}{
				"agent_def_id": ad.ID, "agent_id": created.ID, "name": created.Name,
			})
			o.Publisher.PublishDepartmentAgentProvisioned(events.DepartmentAgentPayload{
				DepartmentID: dept.ID, DeploymentID: dep.ID, TenantID: tenantID,
				AgentDefID: ad.ID, AgentID: created.ID, Name: created.Name, Role: created.Role,
				Timestamp: time.Now(),
			})
		}
	}
	o.Departments.Replace(dept)
	o.Deployments.Mutate(dep.ID, func(d *store.TemplateDeployment) {
		if d.ProvisionedEntities == nil {
			d.ProvisionedEntities = map[string]interface{}{}
		}
		d.ProvisionedEntities["agents"] = provisioned
		d.ProvisionedEntities["department_id"] = dept.ID
	})
	if firstErr != nil {
		fail("deploy_swarm", firstErr)
		return
	}
	o.completeStage(dep.ID, dept.ID, tenantID, "deploy_swarm", fmt.Sprintf("%d agents registered", len(provisioned)))

	// ─── deploy_workflows: compile template SOPs into Module 03 workflows ─
	if o.Orchestration != nil && len(tmpl.Workflows) > 0 {
		o.beginStage(dep.ID, dept.ID, tenantID, "deploy_workflows")
		// agent def id → live M04 agent id, resolved during deploy_swarm.
		agentByDef := map[string]string{}
		for pi := range dept.OrgChart {
			if dept.OrgChart[pi].AgentDefID != "" && dept.OrgChart[pi].AgentID != "" {
				agentByDef[dept.OrgChart[pi].AgentDefID] = dept.OrgChart[pi].AgentID
			}
		}
		var wfErr error
		for wi := range tmpl.Workflows {
			wf := &tmpl.Workflows[wi]
			created, err := o.Orchestration.CreateWorkflow(ctx, caller, CompileWorkflow(wf, dept.ID, agentByDef))
			if err != nil {
				wfErr = fmt.Errorf("workflow %s: %w", wf.ID, err)
				break
			}
			dept.WorkflowIDs = append(dept.WorkflowIDs, created.ID)
		}
		if wfErr != nil {
			fail("deploy_workflows", wfErr)
			return
		}
		o.Departments.Replace(dept)
		o.Deployments.Mutate(dep.ID, func(d *store.TemplateDeployment) {
			if d.ProvisionedEntities == nil {
				d.ProvisionedEntities = map[string]interface{}{}
			}
			d.ProvisionedEntities["workflows"] = dept.WorkflowIDs
		})
		o.completeStage(dep.ID, dept.ID, tenantID, "deploy_workflows",
			fmt.Sprintf("%d SOPs compiled into Module 03", len(dept.WorkflowIDs)))
	}

	// ─── operational ─────────────────────────────────────────────────────
	now := time.Now()
	o.Deployments.Mutate(dep.ID, func(d *store.TemplateDeployment) {
		d.Status = "operational"
		d.CompletedAt = &now
		d.Stages = append(d.Stages, store.StageRecord{
			Stage: "operational", Status: "completed", StartedAt: now, CompletedAt: &now,
		})
	})
	o.Departments.UpdateStatus(dept.ID, tenantID, "operational")
	o.Publisher.PublishDepartmentOperational(events.DepartmentLifecyclePayload{
		DepartmentID: dept.ID, TenantID: tenantID, TemplateID: tmpl.ID, DeploymentID: dep.ID,
		Name: dept.Name, Category: dept.Category, Status: "operational", Timestamp: now,
	})
	// Legacy event for existing Module 11 consumers.
	o.Publisher.PublishTemplateDeployed(events.TemplateDeployedPayload{
		Event: "template.deployed", DeploymentID: dep.ID, TemplateID: tmpl.ID,
		Version: dep.Version, Environment: dep.Environment, Status: "operational",
		DeployedAt: now, DeployedBy: userID, TenantID: tenantID,
	})
	log.Printf("[DEPLOY] %s operational: department %s (%d agents)", dep.ID, dept.ID, len(dept.AgentIDs))
}

// beginStage records a running stage and advances the deployment status.
func (o *Orchestrator) beginStage(depID, deptID, tenantID, stage string) {
	o.recordStage(depID, stage, "running", "")
	o.Deployments.Mutate(depID, func(d *store.TemplateDeployment) { d.Status = stage })
	o.Publisher.PublishDepartmentStageAdvanced(events.DepartmentStagePayload{
		DepartmentID: deptID, DeploymentID: depID, TenantID: tenantID,
		Stage: stage, Status: "running", Timestamp: time.Now(),
	})
}

func (o *Orchestrator) completeStage(depID, deptID, tenantID, stage, detail string) {
	now := time.Now()
	o.Deployments.Mutate(depID, func(d *store.TemplateDeployment) {
		for i := len(d.Stages) - 1; i >= 0; i-- {
			if d.Stages[i].Stage == stage && d.Stages[i].Status == "running" {
				d.Stages[i].Status = "completed"
				d.Stages[i].Detail = detail
				d.Stages[i].CompletedAt = &now
				return
			}
		}
	})
	o.Publisher.PublishDepartmentStageAdvanced(events.DepartmentStagePayload{
		DepartmentID: deptID, DeploymentID: depID, TenantID: tenantID,
		Stage: stage, Status: "completed", Detail: detail, Timestamp: now,
	})
}

func (o *Orchestrator) recordStage(depID, stage, status, detail string) {
	o.Deployments.Mutate(depID, func(d *store.TemplateDeployment) {
		if status == "running" {
			d.Stages = append(d.Stages, store.StageRecord{
				Stage: stage, Status: status, Detail: detail, StartedAt: time.Now(),
			})
			return
		}
		for i := len(d.Stages) - 1; i >= 0; i-- {
			if d.Stages[i].Stage == stage {
				now := time.Now()
				d.Stages[i].Status = status
				d.Stages[i].Detail = detail
				d.Stages[i].CompletedAt = &now
				return
			}
		}
	})
}

// agentRequest builds the Module 04 registration for one template agent:
// department linkage, chain-of-command-derived escalation rules, and
// KPI/service-derived objectives.
// agentIDFor derives a stable UUID for one agent definition within one
// department. UUIDv5 over the department id and definition id: same inputs,
// same id, and distinct departments deploying the same template still get
// distinct agents.
func agentIDFor(departmentID, agentDefID string) string {
	return uuid.NewSHA1(agentNamespace, []byte(departmentID+"/"+agentDefID)).String()
}

// agentNamespace scopes the derived ids so they cannot collide with UUIDv5 ids
// derived elsewhere for other purposes.
var agentNamespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("operan/department-agent"))

func agentRequest(ad *store.AgentDefinition, tmpl *store.Template, dept *store.Department, tenantID string) clients.CreateAgentRequest {
	req := clients.CreateAgentRequest{
		// Derive the agent's id from the department and its definition rather
		// than letting the registry mint one. The registry lost every agent
		// once, and because the ids were random there was no way to restore
		// them: the org chart's agent_id pointed at a record that could not be
		// recreated under the same identity. A derived id makes the same
		// department deploy to the same agents every time, so registering
		// again either restores or conflicts — never silently duplicates.
		ID:           agentIDFor(dept.ID, ad.ID),
		Name:         nameOf(ad),
		Role:         ad.Role,
		Description:  ad.Description,
		TenantID:     tenantID,
		DepartmentID: &dept.ID,
		Capabilities: ad.Capabilities,
		Tools:        ad.ToolRequirements,
	}

	// Chain of command → escalation rules Module 04 understands as strings.
	if ad.ReportsTo != "" {
		req.EscalationRules = append(req.EscalationRules, "reports_to:agent-def:"+ad.ReportsTo)
	}
	for _, e := range ad.EscalationPath {
		req.EscalationRules = append(req.EscalationRules, "escalate_to:"+e)
	}
	if ad.AutonomyTier != "" {
		req.EscalationRules = append(req.EscalationRules, "autonomy_tier:"+ad.AutonomyTier)
	}

	// Services owned → objectives (measured by the service's KPIs).
	kpiName := map[string]string{}
	for _, k := range tmpl.KPIS {
		kpiName[k.ID] = k.Name
	}
	for _, svcID := range ad.Services {
		for _, svc := range tmpl.Services {
			if svc.ID != svcID {
				continue
			}
			obj := struct {
				Description string  `json:"description"`
				Metric      string  `json:"metric,omitempty"`
				Weight      float64 `json:"weight,omitempty"`
			}{Description: "Deliver service: " + svc.Name, Weight: 1}
			if len(svc.KPIRefs) > 0 {
				obj.Metric = kpiName[svc.KPIRefs[0]]
			}
			req.Objectives = append(req.Objectives, obj)
		}
	}
	return req
}

func nameOf(ad *store.AgentDefinition) string {
	if ad.Name != "" {
		return ad.Name
	}
	return ad.Role
}

// memoryItems builds the department's provisioned memory: the charter document
// plus one document per service so agents can recall the portfolio.
func memoryItems(dept *store.Department, tmpl *store.Template) []clients.VectorItem {
	charter := fmt.Sprintf("Department charter: %s (%s). Mission: %s.", dept.Name, dept.Category, dept.Mission)
	if dept.BusinessLogic != nil {
		if dept.BusinessLogic.ValueProposition != "" {
			charter += " Value proposition: " + dept.BusinessLogic.ValueProposition + "."
		}
		if len(dept.BusinessLogic.Activities) > 0 {
			charter += " Core activities: " + strings.Join(dept.BusinessLogic.Activities, "; ") + "."
		}
	}
	items := []clients.VectorItem{{
		DocumentID:      "dept-" + dept.ID + "-charter",
		EmbeddingType:   "department",
		SemanticContent: charter,
		Metadata: map[string]interface{}{
			"department_id": dept.ID, "kind": "charter", "category": dept.Category,
		},
	}}
	for _, svc := range dept.Services {
		content := fmt.Sprintf("Service: %s — %s.", svc.Name, svc.Description)
		if svc.SLA != nil {
			content += fmt.Sprintf(" SLA: response %s, resolution %s, coverage %s.",
				svc.SLA.ResponseTime, svc.SLA.ResolutionTime, svc.SLA.Coverage)
		}
		if len(svc.Consumers) > 0 {
			content += " Consumers: " + strings.Join(svc.Consumers, ", ") + "."
		}
		items = append(items, clients.VectorItem{
			DocumentID:      "dept-" + dept.ID + "-svc-" + svc.ID,
			EmbeddingType:   "department",
			SemanticContent: content,
			Metadata: map[string]interface{}{
				"department_id": dept.ID, "kind": "service", "service_id": svc.ID,
			},
		})
	}
	return items
}

// compileWorkflow translates a template SOP (WorkflowDefinition) into a
// Module 03 workflow-create request. Steps become nodes; sequential edges
// mirror authoring order; agent references resolve to live M04 agent ids.
// approverByDef maps an agent-definition id to the platform user who holds the
// seat carrying it. Template approval steps name their approver as
// config.required_by = <agent_def_id>; the org chart says which Position
// carries that definition, and Position.human_ref says who sits in it. Doing
// this at compile time keeps the resolution where the org chart lives — the
// orchestrator never needs to reach back into this module at run time.
func approverByDef(dept *store.Department) map[string]string {
	out := map[string]string{}
	if dept == nil {
		return out
	}
	for _, p := range dept.OrgChart {
		if p.AgentDefID != "" && p.HolderType == "human" && p.HumanRef != "" {
			out[p.AgentDefID] = p.HumanRef
		}
	}
	return out
}

// requiredByOf reads the approver a step names, tolerating both the singular
// and list spellings the catalogue uses.
func requiredByOf(cfg map[string]interface{}) []string {
	var out []string
	switch v := cfg["required_by"].(type) {
	case string:
		if v != "" {
			out = append(out, v)
		}
	case []interface{}:
		for _, it := range v {
			if s, ok := it.(string); ok && s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func CompileWorkflow(wf *store.WorkflowDefinition, departmentID string, agentByDef map[string]string) clients.WorkflowCreateRequest {
	return CompileWorkflowFor(wf, departmentID, agentByDef, nil)
}

// CompileWorkflowFor is CompileWorkflow with the department in hand, so gate
// steps can carry the real human who must sign them off.
func CompileWorkflowFor(wf *store.WorkflowDefinition, departmentID string, agentByDef map[string]string, dept *store.Department) clients.WorkflowCreateRequest {
	humanByDef := approverByDef(dept)
	nodes := make([]clients.WorkflowNode, 0, len(wf.Steps))
	edges := make([]clients.WorkflowEdge, 0, len(wf.Steps))
	for i, s := range wf.Steps {
		n := clients.WorkflowNode{
			ID:     s.ID,
			Type:   nodeTypeFor(s.Type),
			Action: s.Name,
		}
		if n.Action == "" {
			n.Action = s.ID
		}
		if s.TimeoutSeconds > 0 {
			n.TimeoutMs = s.TimeoutSeconds * 1000
		}
		// Resolve the acting agent when the step names one.
		if def, ok := s.Config["agent_id"].(string); ok {
			if live, ok := agentByDef[def]; ok {
				n.AgentID = live
			}
		} else if def, ok := s.Config["agent"].(string); ok {
			if live, ok := agentByDef[def]; ok {
				n.AgentID = live
			}
		}
		if len(s.Config) > 0 {
			n.Parameters = s.Config
		}
		// Route the gate to the person who holds the seat that owns it. When
		// nobody is bound the orchestrator falls back to a role target — never
		// to a fabricated user.
		if n.Type == "human_gate" {
			for _, def := range requiredByOf(s.Config) {
				if user, ok := humanByDef[def]; ok {
					if n.Parameters == nil {
						n.Parameters = map[string]interface{}{}
					}
					n.Parameters["required_approver_user_id"] = user
					n.Parameters["required_approver_agent_def_id"] = def
					break
				}
			}
		}
		if i+1 < len(wf.Steps) {
			n.OnSuccess = wf.Steps[i+1].ID
			edges = append(edges, clients.WorkflowEdge{From: s.ID, To: wf.Steps[i+1].ID})
		}
		nodes = append(nodes, n)
	}
	return clients.WorkflowCreateRequest{
		DepartmentID: departmentID,
		Name:         wf.Name,
		Description:  wf.Description,
		Graph: map[string]interface{}{
			"nodes": nodes,
			"edges": edges,
		},
	}
}

// nodeTypeFor maps template step types onto Module 03's node-type enum.
//
// "transformation" is agent work under another name: 78 of the 80 such steps
// in the catalogue carry {agent, task} in their config, exactly like
// agent_call, and the agent-node executor resolves config.agent already.
// Compiling them to "action" sent them to the pass-through instead — 80 steps
// that reported success while doing nothing.
//
// tool_call and notification remain "action" until the capability layer binds
// them to a real provider; they pass through and say so, rather than pretending.
func nodeTypeFor(stepType string) string {
	switch stepType {
	case "agent_call", "transformation":
		return "agent"
	case "approval", "human_gate":
		return "human_gate"
	case "conditional":
		return "condition"
	default: // api_call, data_fetch, notification, tool_call
		return "action"
	}
}
