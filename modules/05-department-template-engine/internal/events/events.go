// Package events publishes AsyncAPI events for Module 05 template lifecycle operations.
// Events use the topic format: operan/events/template.{event}
// Topics: operan/events/template.created, operan/events/template.updated,
//         operan/events/template.deployed, operan/events/template.deployment_failed,
//         operan/events/template.undeployed, operan/events/template.deleted,
//         operan/events/template.versioned, operan/events/template.cloned
// Custom template topics: operan/events/custom_template.created,
//         operan/events/custom_template.updated, operan/events/custom_template.deleted,
//         operan/events/custom_template.cloned
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// ─── Broker interface ────────────────────────────────────────────────────────

// Broker is the interface for event publishing.
type Broker interface {
	Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error
	Close() error
}

// logBroker is the default no-op broker that logs events instead of publishing.
type logBroker struct{}

func (l *logBroker) Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error {
	log.Printf("[EVENT] %s: %s", topic, string(value))
	return nil
}

func (l *logBroker) Close() error {
	return nil
}

// ─── Publisher ───────────────────────────────────────────────────────────────

// Publisher handles publishing template lifecycle events.
type Publisher struct {
	broker Broker
}

// NewPublisher creates a new event publisher with a log-only broker.
func NewPublisher() *Publisher {
	return &Publisher{broker: &logBroker{}}
}

// NewPublisherWithBroker creates a publisher backed by a real broker.
func NewPublisherWithBroker(broker Broker) *Publisher {
	return &Publisher{broker: broker}
}

// SetBroker replaces the underlying broker.
func (p *Publisher) SetBroker(broker Broker) {
	p.broker = broker
}

// Close gracefully shuts down the broker.
func (p *Publisher) Close() error {
	if p.broker != nil {
		return p.broker.Close()
	}
	return nil
}

func (p *Publisher) publish(topic string, data []byte) error {
	if p.broker == nil {
		return fmt.Errorf("broker not set")
	}
	return p.broker.Publish(context.Background(), topic, nil, data, nil)
}

// ─── Event payloads ──────────────────────────────────────────────────────────

// TemplateCreatedPayload matches the AsyncAPI TemplateCreatedPayload schema.
type TemplateCreatedPayload struct {
	Event             string                 `json:"event"`
	TemplateID        string                 `json:"template_id"`
	Name              string                 `json:"name"`
	Category          string                 `json:"category"`
	Version           string                 `json:"version"`
	Description       string                 `json:"description,omitempty"`
	AgentsCount       int                    `json:"agents_count,omitempty"`
	WorkflowsCount    int                    `json:"workflows_count,omitempty"`
	IntegrationsCount int                    `json:"integrations_count,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	CreatedBy         string                 `json:"created_by,omitempty"`
	TenantID          string                 `json:"tenant_id"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// TemplateUpdatedPayload matches the AsyncAPI TemplateUpdatedPayload schema.
type TemplateUpdatedPayload struct {
	Event           string    `json:"event"`
	TemplateID      string    `json:"template_id"`
	Name            string    `json:"name,omitempty"`
	Category        string    `json:"category,omitempty"`
	Version         string    `json:"version"`
	PreviousVersion string    `json:"previous_version,omitempty"`
	ChangedFields   []string  `json:"changed_fields,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
	UpdatedBy       string    `json:"updated_by,omitempty"`
	TenantID        string    `json:"tenant_id"`
}

// TemplateDeployedPayload matches the AsyncAPI TemplateDeployedPayload schema.
type TemplateDeployedPayload struct {
	Event               string                 `json:"event"`
	DeploymentID        string                 `json:"deployment_id"`
	TemplateID          string                 `json:"template_id"`
	Version             string                 `json:"version"`
	Environment         string                 `json:"environment"`
	Status              string                 `json:"status"`
	Configuration       map[string]interface{} `json:"configuration,omitempty"`
	ProvisionedEntities map[string]interface{} `json:"provisioned_entities,omitempty"`
	DeployedAt          time.Time              `json:"deployed_at"`
	DeployedBy          string                 `json:"deployed_by,omitempty"`
	TenantID            string                 `json:"tenant_id"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// TemplateDeploymentFailedPayload matches the AsyncAPI TemplateDeploymentFailedPayload schema.
type TemplateDeploymentFailedPayload struct {
	Event           string    `json:"event"`
	DeploymentID    string    `json:"deployment_id"`
	TemplateID      string    `json:"template_id"`
	Version         string    `json:"version"`
	Environment     string    `json:"environment"`
	DeploymentStage string    `json:"deployment_stage,omitempty"`
	ErrorMessage    string    `json:"error_message"`
	ErrorCode       string    `json:"error_code,omitempty"`
	FailedAt        time.Time `json:"failed_at"`
	FailedBy        string    `json:"failed_by,omitempty"`
	TenantID        string    `json:"tenant_id"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// TemplateUndeployedPayload matches the AsyncAPI TemplateUndeployedPayload schema.
type TemplateUndeployedPayload struct {
	Event        string    `json:"event"`
	DeploymentID string    `json:"deployment_id"`
	TemplateID   string    `json:"template_id"`
	Version      string    `json:"version"`
	Environment  string    `json:"environment"`
	UndeployedAt time.Time `json:"undeployed_at"`
	UndeployedBy string    `json:"undeployed_by,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	TenantID     string    `json:"tenant_id"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TemplateDeletedPayload matches the AsyncAPI TemplateDeletedPayload schema.
type TemplateDeletedPayload struct {
	Event      string    `json:"event"`
	TemplateID string    `json:"template_id"`
	Name       string    `json:"name,omitempty"`
	Category   string    `json:"category,omitempty"`
	DeletedAt  time.Time `json:"deleted_at"`
	DeletedBy  string    `json:"deleted_by,omitempty"`
	TenantID   string    `json:"tenant_id"`
}

// TemplateVersionedPayload matches the AsyncAPI TemplateVersionedPayload schema.
type TemplateVersionedPayload struct {
	Event           string    `json:"event"`
	TemplateID      string    `json:"template_id"`
	Version         string    `json:"version"`
	PreviousVersion string    `json:"previous_version,omitempty"`
	Name            string    `json:"name,omitempty"`
	Category        string    `json:"category,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	CreatedBy       string    `json:"created_by,omitempty"`
	TenantID        string    `json:"tenant_id"`
}

// TemplateClonedPayload matches the AsyncAPI TemplateClonedPayload schema.
type TemplateClonedPayload struct {
	Event          string    `json:"event"`
	SourceTemplateID string   `json:"source_template_id"`
	ClonedTemplateID string   `json:"cloned_template_id"`
	Name           string    `json:"name,omitempty"`
	Category       string    `json:"category,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	CreatedBy      string    `json:"created_by,omitempty"`
	TenantID       string    `json:"tenant_id"`
}

// ─── Custom Template Event payloads ─────────────────────────────────────────

// CustomTemplateCreatedPayload matches the AsyncAPI custom_template.created schema.
type CustomTemplateCreatedPayload struct {
	Event            string                 `json:"event"`
	CustomTemplateID string                 `json:"custom_template_id"`
	Name             string                 `json:"name,omitempty"`
	Category         string                 `json:"category,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	CreatedBy        string                 `json:"created_by,omitempty"`
	TenantID         string                 `json:"tenant_id"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// CustomTemplateUpdatedPayload matches the AsyncAPI custom_template.updated schema.
type CustomTemplateUpdatedPayload struct {
	Event            string                 `json:"event"`
	CustomTemplateID string                 `json:"custom_template_id"`
	Name             string                 `json:"name,omitempty"`
	Category         string                 `json:"category,omitempty"`
	Version          string                 `json:"version,omitempty"`
	ChangedFields    []string               `json:"changed_fields,omitempty"`
	UpdatedAt        time.Time              `json:"updated_at"`
	UpdatedBy        string                 `json:"updated_by,omitempty"`
	TenantID         string                 `json:"tenant_id"`
}

// CustomTemplateDeletedPayload matches the AsyncAPI custom_template.deleted schema.
type CustomTemplateDeletedPayload struct {
	Event            string    `json:"event"`
	CustomTemplateID string    `json:"custom_template_id"`
	Name             string    `json:"name,omitempty"`
	Category         string    `json:"category,omitempty"`
	DeletedAt        time.Time `json:"deleted_at"`
	DeletedBy        string    `json:"deleted_by,omitempty"`
	TenantID         string    `json:"tenant_id"`
}

// CustomTemplateClonedPayload matches the AsyncAPI custom_template.cloned schema.
type CustomTemplateClonedPayload struct {
	Event              string    `json:"event"`
	SourceTemplateID   string    `json:"source_template_id"`
	ClonedTemplateID   string    `json:"cloned_template_id"`
	Name               string    `json:"name,omitempty"`
	Category           string    `json:"category,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	CreatedBy          string    `json:"created_by,omitempty"`
	TenantID           string    `json:"tenant_id"`
}

// ─── Publish methods ─────────────────────────────────────────────────────────

const templateTopic = "operan/events/template"
const customTemplateTopic = "operan/events/custom_template"

// PublishTemplateCreated emits operan/events/template.created.
func (p *Publisher) PublishTemplateCreated(payload TemplateCreatedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal template.created event: %w", err)
	}
	return p.publish(templateTopic+".created", data)
}

// PublishTemplateUpdated emits operan/events/template.updated.
func (p *Publisher) PublishTemplateUpdated(payload TemplateUpdatedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal template.updated event: %w", err)
	}
	return p.publish(templateTopic+".updated", data)
}

// PublishTemplateDeployed emits operan/events/template.deployed.
func (p *Publisher) PublishTemplateDeployed(payload TemplateDeployedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal template.deployed event: %w", err)
	}
	return p.publish(templateTopic+".deployed", data)
}

// PublishTemplateDeploymentFailed emits operan/events/template.deployment_failed.
func (p *Publisher) PublishTemplateDeploymentFailed(payload TemplateDeploymentFailedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal template.deployment_failed event: %w", err)
	}
	return p.publish(templateTopic+".deployment_failed", data)
}

// PublishTemplateUndeployed emits operan/events/template.undeployed.
func (p *Publisher) PublishTemplateUndeployed(payload TemplateUndeployedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal template.undeployed event: %w", err)
	}
	return p.publish(templateTopic+".undeployed", data)
}

// PublishTemplateDeleted emits operan/events/template.deleted.
func (p *Publisher) PublishTemplateDeleted(payload TemplateDeletedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal template.deleted event: %w", err)
	}
	return p.publish(templateTopic+".deleted", data)
}

// PublishTemplateVersioned emits operan/events/template.versioned.
func (p *Publisher) PublishTemplateVersioned(payload TemplateVersionedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal template.versioned event: %w", err)
	}
	return p.publish(templateTopic+".versioned", data)
}

// PublishTemplateCloned emits operan/events/template.cloned.
func (p *Publisher) PublishTemplateCloned(payload TemplateClonedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal template.cloned event: %w", err)
	}
	return p.publish(templateTopic+".cloned", data)
}

// PublishCustomTemplateCreated emits operan/events/custom_template.created.
func (p *Publisher) PublishCustomTemplateCreated(payload CustomTemplateCreatedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal custom_template.created event: %w", err)
	}
	return p.publish(customTemplateTopic+".created", data)
}

// PublishCustomTemplateUpdated emits operan/events/custom_template.updated.
func (p *Publisher) PublishCustomTemplateUpdated(payload CustomTemplateUpdatedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal custom_template.updated event: %w", err)
	}
	return p.publish(customTemplateTopic+".updated", data)
}

// PublishCustomTemplateDeleted emits operan/events/custom_template.deleted.
func (p *Publisher) PublishCustomTemplateDeleted(payload CustomTemplateDeletedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal custom_template.deleted event: %w", err)
	}
	return p.publish(customTemplateTopic+".deleted", data)
}

// PublishCustomTemplateCloned emits operan/events/custom_template.cloned.
func (p *Publisher) PublishCustomTemplateCloned(payload CustomTemplateClonedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal custom_template.cloned event: %w", err)
	}
	return p.publish(customTemplateTopic+".cloned", data)
}
