package events

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPublisher_PublishTemplateCreated(t *testing.T) {
	publisher := NewPublisher()

	payload := TemplateCreatedPayload{
		Event:           "template.created",
		TemplateID:      uuid.New().String(),
		Name:            "Test Template",
		Category:        "engineering",
		Version:         "1.0.0",
		CreatedAt:       time.Now(),
		TenantID:        "tenant-1",
		AgentsCount:     3,
		WorkflowsCount:  2,
		IntegrationsCount: 1,
	}

	err := publisher.PublishTemplateCreated(payload)
	if err != nil {
		t.Errorf("PublishTemplateCreated failed: %v", err)
	}
}

func TestPublisher_PublishTemplateUpdated(t *testing.T) {
	publisher := NewPublisher()

	payload := TemplateUpdatedPayload{
		Event:           "template.updated",
		TemplateID:      uuid.New().String(),
		Name:            "Updated Template",
		Category:        "engineering",
		Version:         "1.0.0",
		PreviousVersion: "1.0.0",
		ChangedFields:   []string{"name", "category"},
		UpdatedAt:       time.Now(),
		TenantID:        "tenant-1",
	}

	err := publisher.PublishTemplateUpdated(payload)
	if err != nil {
		t.Errorf("PublishTemplateUpdated failed: %v", err)
	}
}

func TestPublisher_PublishTemplateDeleted(t *testing.T) {
	publisher := NewPublisher()

	payload := TemplateDeletedPayload{
		Event:        "template.deleted",
		TemplateID:   uuid.New().String(),
		Name:         "Deleted Template",
		Category:     "engineering",
		DeletedAt:    time.Now(),
		TenantID:     "tenant-1",
	}

	err := publisher.PublishTemplateDeleted(payload)
	if err != nil {
		t.Errorf("PublishTemplateDeleted failed: %v", err)
	}
}

func TestPublisher_PublishTemplateVersioned(t *testing.T) {
	publisher := NewPublisher()

	payload := TemplateVersionedPayload{
		Event:           "template.versioned",
		TemplateID:      uuid.New().String(),
		Version:         "1.0.0",
		PreviousVersion: "1.0.0",
		Name:            "Versioned Template",
		Category:        "engineering",
		CreatedAt:       time.Now(),
		TenantID:        "tenant-1",
	}

	err := publisher.PublishTemplateVersioned(payload)
	if err != nil {
		t.Errorf("PublishTemplateVersioned failed: %v", err)
	}
}

func TestPublisher_PublishTemplateDeployed(t *testing.T) {
	publisher := NewPublisher()

	payload := TemplateDeployedPayload{
		Event:          "template.deployed",
		DeploymentID:   uuid.New().String(),
		TemplateID:     uuid.New().String(),
		Version:        "1.0.0",
		Environment:    "production",
		Status:         "select",
		DeployedAt:     time.Now(),
		TenantID:       "tenant-1",
		Configuration:  map[string]interface{}{"replicas": 3},
	}

	err := publisher.PublishTemplateDeployed(payload)
	if err != nil {
		t.Errorf("PublishTemplateDeployed failed: %v", err)
	}
}

func TestPublisher_PublishTemplateDeploymentFailed(t *testing.T) {
	publisher := NewPublisher()

	payload := TemplateDeploymentFailedPayload{
		Event:             "template.deployment_failed",
		DeploymentID:      uuid.New().String(),
		TemplateID:        uuid.New().String(),
		Version:           "1.0.0",
		Environment:       "production",
		DeploymentStage:   "provision_memory",
		ErrorMessage:      "insufficient resources",
		ErrorCode:         "RESOURCE_EXHAUSTED",
		FailedAt:          time.Now(),
		TenantID:          "tenant-1",
	}

	err := publisher.PublishTemplateDeploymentFailed(payload)
	if err != nil {
		t.Errorf("PublishTemplateDeploymentFailed failed: %v", err)
	}
}

func TestPublisher_PublishTemplateUndeployed(t *testing.T) {
	publisher := NewPublisher()

	payload := TemplateUndeployedPayload{
		Event:          "template.undeployed",
		DeploymentID:   uuid.New().String(),
		TemplateID:     uuid.New().String(),
		Version:        "1.0.0",
		Environment:    "production",
		UndeployedAt:   time.Now(),
		Reason:         "scaling down",
		TenantID:       "tenant-1",
	}

	err := publisher.PublishTemplateUndeployed(payload)
	if err != nil {
		t.Errorf("PublishTemplateUndeployed failed: %v", err)
	}
}

func TestPublisher_PublishTemplateCloned(t *testing.T) {
	publisher := NewPublisher()

	payload := TemplateClonedPayload{
		Event:            "template.cloned",
		SourceTemplateID: uuid.New().String(),
		ClonedTemplateID: uuid.New().String(),
		Name:             "Cloned Template",
		Category:         "engineering",
		CreatedAt:        time.Now(),
		TenantID:         "tenant-1",
	}

	err := publisher.PublishTemplateCloned(payload)
	if err != nil {
		t.Errorf("PublishTemplateCloned failed: %v", err)
	}
}

func TestPublisher_Noop(t *testing.T) {
	publisher := NewPublisher()

	// Call all methods - should not error
	_ = publisher.PublishTemplateCreated(TemplateCreatedPayload{Event: "template.created", TemplateID: uuid.New().String(), Name: "Test", Category: "eng", Version: "1.0.0", CreatedAt: time.Now(), TenantID: "t1"})
	_ = publisher.PublishTemplateUpdated(TemplateUpdatedPayload{Event: "template.updated", TemplateID: uuid.New().String(), Version: "1.0.0", UpdatedAt: time.Now(), TenantID: "t1"})
	_ = publisher.PublishTemplateDeleted(TemplateDeletedPayload{Event: "template.deleted", TemplateID: uuid.New().String(), DeletedAt: time.Now(), TenantID: "t1"})
	_ = publisher.PublishTemplateVersioned(TemplateVersionedPayload{Event: "template.versioned", TemplateID: uuid.New().String(), Version: "1.0.0", CreatedAt: time.Now(), TenantID: "t1"})
	_ = publisher.PublishTemplateDeployed(TemplateDeployedPayload{Event: "template.deployed", DeploymentID: uuid.New().String(), TemplateID: uuid.New().String(), Version: "1.0.0", Environment: "prod", Status: "select", DeployedAt: time.Now(), TenantID: "t1"})
	_ = publisher.PublishTemplateDeploymentFailed(TemplateDeploymentFailedPayload{Event: "template.deployment_failed", DeploymentID: uuid.New().String(), TemplateID: uuid.New().String(), Version: "1.0.0", Environment: "prod", ErrorMessage: "error", FailedAt: time.Now(), TenantID: "t1"})
	_ = publisher.PublishTemplateUndeployed(TemplateUndeployedPayload{Event: "template.undeployed", DeploymentID: uuid.New().String(), TemplateID: uuid.New().String(), Version: "1.0.0", Environment: "prod", UndeployedAt: time.Now(), TenantID: "t1"})
	_ = publisher.PublishTemplateCloned(TemplateClonedPayload{Event: "template.cloned", SourceTemplateID: uuid.New().String(), ClonedTemplateID: uuid.New().String(), Name: "Clone", Category: "eng", CreatedAt: time.Now(), TenantID: "t1"})
}

func TestPublisher_Close(t *testing.T) {
	publisher := NewPublisher()

	err := publisher.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestNewPublisherWithBroker(t *testing.T) {
	publisher := NewPublisherWithBroker(&logBroker{})
	if publisher == nil {
		t.Fatal("expected non-nil publisher")
	}
}

func TestPublisher_SetBroker(t *testing.T) {
	publisher := NewPublisher()

	publisher.SetBroker(&logBroker{})
	// Should not panic
}

func TestPublisher_Close_NilBroker(t *testing.T) {
	publisher := &Publisher{} // no broker set

	err := publisher.Close()
	if err != nil {
		t.Errorf("Close with nil broker should return nil, got: %v", err)
	}
}

// errorBroker returns an error on Publish
type errorBroker struct{}

func (e *errorBroker) Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error {
	return fmt.Errorf("broker publish error")
}
func (e *errorBroker) Close() error { return nil }

func TestPublisher_publish_BrokerError(t *testing.T) {
	publisher := NewPublisherWithBroker(&errorBroker{})

	err := publisher.publish("test.topic", []byte(`{}`))
	if err == nil {
		t.Error("expected error from failed broker publish")
	}
}

func TestPublisher_publish_NilBroker(t *testing.T) {
	publisher := &Publisher{} // no broker set

	err := publisher.publish("test.topic", []byte(`{}`))
	if err == nil {
		t.Error("expected error when broker is nil")
	}
}

// ─── Custom template publish methods ─────────────────────────────────────────

func TestPublisher_PublishCustomTemplateCreated(t *testing.T) {
	publisher := NewPublisher()

	payload := CustomTemplateCreatedPayload{
		Event:            "custom_template.created",
		CustomTemplateID: uuid.New().String(),
		Name:             "Custom Onboarding Template",
		Category:         "hr",
		CreatedAt:        time.Now(),
		CreatedBy:        "user-1",
		TenantID:         "tenant-1",
		Metadata:         map[string]interface{}{"version": 2},
	}

	err := publisher.PublishCustomTemplateCreated(payload)
	if err != nil {
		t.Errorf("PublishCustomTemplateCreated failed: %v", err)
	}
}

func TestPublisher_PublishCustomTemplateUpdated(t *testing.T) {
	publisher := NewPublisher()

	payload := CustomTemplateUpdatedPayload{
		Event:            "custom_template.updated",
		CustomTemplateID: uuid.New().String(),
		Name:             "Updated Custom Template",
		Category:         "finance",
		Version:          "2.0.0",
		ChangedFields:    []string{"name", "category"},
		UpdatedAt:        time.Now(),
		UpdatedBy:        "user-2",
		TenantID:         "tenant-1",
	}

	err := publisher.PublishCustomTemplateUpdated(payload)
	if err != nil {
		t.Errorf("PublishCustomTemplateUpdated failed: %v", err)
	}
}

func TestPublisher_PublishCustomTemplateDeleted(t *testing.T) {
	publisher := NewPublisher()

	payload := CustomTemplateDeletedPayload{
		Event:            "custom_template.deleted",
		CustomTemplateID: uuid.New().String(),
		Name:             "Deleted Custom Template",
		Category:         "engineering",
		DeletedAt:        time.Now(),
		DeletedBy:        "user-3",
		TenantID:         "tenant-1",
	}

	err := publisher.PublishCustomTemplateDeleted(payload)
	if err != nil {
		t.Errorf("PublishCustomTemplateDeleted failed: %v", err)
	}
}

func TestPublisher_PublishCustomTemplateCloned(t *testing.T) {
	publisher := NewPublisher()

	payload := CustomTemplateClonedPayload{
		Event:            "custom_template.cloned",
		SourceTemplateID: uuid.New().String(),
		ClonedTemplateID: uuid.New().String(),
		Name:             "Cloned Custom Template",
		Category:         "sales",
		CreatedAt:        time.Now(),
		CreatedBy:        "user-4",
		TenantID:         "tenant-1",
	}

	err := publisher.PublishCustomTemplateCloned(payload)
	if err != nil {
		t.Errorf("PublishCustomTemplateCloned failed: %v", err)
	}
}
