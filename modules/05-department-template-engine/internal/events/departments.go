package events

import (
	"encoding/json"
	"time"
)

// Department lifecycle events, published by the deploy orchestrator and the
// department handlers. Topic format follows the module convention:
// operan.templates.department.{event}
const departmentTopic = "operan.templates.department"

// DepartmentLifecyclePayload is shared by created/operational/updated/archived.
type DepartmentLifecyclePayload struct {
	Event        string    `json:"event"`
	DepartmentID string    `json:"department_id"`
	TenantID     string    `json:"tenant_id"`
	TemplateID   string    `json:"template_id,omitempty"`
	DeploymentID string    `json:"deployment_id,omitempty"`
	Name         string    `json:"name,omitempty"`
	Category     string    `json:"category,omitempty"`
	Status       string    `json:"status,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// DepartmentStagePayload reports provisioning stage transitions.
type DepartmentStagePayload struct {
	Event        string    `json:"event"`
	DepartmentID string    `json:"department_id"`
	DeploymentID string    `json:"deployment_id"`
	TenantID     string    `json:"tenant_id"`
	Stage        string    `json:"stage"`
	Status       string    `json:"status"` // running, completed, failed
	Detail       string    `json:"detail,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// DepartmentAgentPayload reports one agent provisioned during deploy_swarm.
type DepartmentAgentPayload struct {
	Event        string    `json:"event"`
	DepartmentID string    `json:"department_id"`
	DeploymentID string    `json:"deployment_id,omitempty"`
	TenantID     string    `json:"tenant_id"`
	AgentDefID   string    `json:"agent_def_id"`
	AgentID      string    `json:"agent_id"`
	Name         string    `json:"name,omitempty"`
	Role         string    `json:"role,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

func (p *Publisher) PublishDepartmentCreated(payload DepartmentLifecyclePayload) error {
	payload.Event = "department.created"
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.publish(departmentTopic+".created", data)
}

func (p *Publisher) PublishDepartmentStageAdvanced(payload DepartmentStagePayload) error {
	payload.Event = "department.stage_advanced"
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.publish(departmentTopic+".stage_advanced", data)
}

func (p *Publisher) PublishDepartmentAgentProvisioned(payload DepartmentAgentPayload) error {
	payload.Event = "department.agent_provisioned"
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.publish(departmentTopic+".agent_provisioned", data)
}

func (p *Publisher) PublishDepartmentOperational(payload DepartmentLifecyclePayload) error {
	payload.Event = "department.operational"
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.publish(departmentTopic+".operational", data)
}

func (p *Publisher) PublishDepartmentProvisioningFailed(payload DepartmentStagePayload) error {
	payload.Event = "department.provisioning_failed"
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.publish(departmentTopic+".provisioning_failed", data)
}

func (p *Publisher) PublishDepartmentUpdated(payload DepartmentLifecyclePayload) error {
	payload.Event = "department.updated"
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.publish(departmentTopic+".updated", data)
}

func (p *Publisher) PublishDepartmentArchived(payload DepartmentLifecyclePayload) error {
	payload.Event = "department.archived"
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.publish(departmentTopic+".archived", data)
}
