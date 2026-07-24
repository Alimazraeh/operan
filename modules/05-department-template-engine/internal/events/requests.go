package events

import (
	"encoding/json"
	"time"
)

// Service-request lifecycle events. Topic family:
// operan.templates.request.<event>
const requestTopic = "operan.templates.request"

// RequestLifecyclePayload is the common payload for request events.
type RequestLifecyclePayload struct {
	RequestID    string    `json:"request_id"`
	TenantID     string    `json:"tenant_id"`
	DepartmentID string    `json:"department_id"`
	ServiceID    string    `json:"service_id,omitempty"`
	Title        string    `json:"title,omitempty"`
	Status       string    `json:"status"`
	Detail       string    `json:"detail,omitempty"`
	TokensUsed   int       `json:"tokens_used,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

func (p *Publisher) publishRequest(event string, payload RequestLifecyclePayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.publish(requestTopic+"."+event, data)
}

func (p *Publisher) PublishRequestCreated(payload RequestLifecyclePayload) error {
	return p.publishRequest("created", payload)
}

func (p *Publisher) PublishRequestDispatched(payload RequestLifecyclePayload) error {
	return p.publishRequest("dispatched", payload)
}

func (p *Publisher) PublishRequestAwaitingApproval(payload RequestLifecyclePayload) error {
	return p.publishRequest("awaiting_approval", payload)
}

func (p *Publisher) PublishRequestCompleted(payload RequestLifecyclePayload) error {
	return p.publishRequest("completed", payload)
}

func (p *Publisher) PublishRequestRejected(payload RequestLifecyclePayload) error {
	return p.publishRequest("rejected", payload)
}

func (p *Publisher) PublishRequestFailed(payload RequestLifecyclePayload) error {
	return p.publishRequest("failed", payload)
}

func (p *Publisher) PublishRequestSLABreached(payload RequestLifecyclePayload) error {
	return p.publishRequest("sla_breached", payload)
}
