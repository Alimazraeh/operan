package events

import (
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
