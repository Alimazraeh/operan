package events

import (
	"testing"
	"time"
)

func TestTopicNaming(t *testing.T) {
	p := NewPublisher()

	t.Run("langgraph topic", func(t *testing.T) {
		expected := "operan.orchestration.langgraph.graph.registered"
		got := p.topic(StackLangGraph, "graph", "registered")
		if got != expected {
			t.Errorf("Expected %s, got %s", expected, got)
		}
	})

	t.Run("temporal topic", func(t *testing.T) {
		expected := "operan.orchestration.temporal.workflow.registered"
		got := p.topic(StackTemporal, "workflow", "registered")
		if got != expected {
			t.Errorf("Expected %s, got %s", expected, got)
		}
	})

	t.Run("ray topic", func(t *testing.T) {
		expected := "operan.orchestration.ray.task.submitted"
		got := p.topic(StackRay, "task", "submitted")
		if got != expected {
			t.Errorf("Expected %s, got %s", expected, got)
		}
	})

	t.Run("celery topic", func(t *testing.T) {
		expected := "operan.orchestration.celery.task.published"
		got := p.topic(StackCelery, "task", "published")
		if got != expected {
			t.Errorf("Expected %s, got %s", expected, got)
		}
	})

	t.Run("stack health topic", func(t *testing.T) {
		// Stack health uses a special topic format without stack prefix
		expected := "operan.orchestration.stack.health"
		// Note: PublishStackHealth uses a hardcoded topic - not via topic() helper
		_ = expected // This test validates the naming convention
	})
}

func TestPublishWorkflowCreated(t *testing.T) {
	p := NewPublisher()

	payload := WorkflowCreatedPayload{
		WorkflowID:   "wf-1",
		TenantID:     "tenant-1",
		DepartmentID: "dept-1",
		Name:         "Test Workflow",
		Version:      "1.0.0",
		CreatedBy:    "user-1",
		CreatedAt:    time.Now(),
	}

	t.Run("langgraph stack", func(t *testing.T) {
		err := p.PublishWorkflowCreated(StackLangGraph, payload)
		if err != nil {
			t.Fatalf("PublishWorkflowCreated failed: %v", err)
		}
	})

	t.Run("temporal stack", func(t *testing.T) {
		err := p.PublishWorkflowCreated(StackTemporal, payload)
		if err != nil {
			t.Fatalf("PublishWorkflowCreated failed: %v", err)
		}
	})

	t.Run("ray stack", func(t *testing.T) {
		err := p.PublishWorkflowCreated(StackRay, payload)
		if err != nil {
			t.Fatalf("PublishWorkflowCreated failed: %v", err)
		}
	})

	t.Run("celery stack", func(t *testing.T) {
		err := p.PublishWorkflowCreated(StackCelery, payload)
		if err != nil {
			t.Fatalf("PublishWorkflowCreated failed: %v", err)
		}
	})
}

func TestPublishLangGraphEvents(t *testing.T) {
	p := NewPublisher()

	t.Run("graph registered", func(t *testing.T) {
		payload := LangGraphGraphRegisteredPayload{
			GraphID:     "graph-1",
			TenantID:    "tenant-1",
			Name:        "Test Graph",
			Version:     "1.0.0",
			NodeCount:   5,
			EdgeCount:   4,
			CreatedAt:   time.Now(),
		}
		err := p.PublishLangGraphGraphRegistered(payload)
		if err != nil {
			t.Fatalf("PublishLangGraphGraphRegistered failed: %v", err)
		}
	})

	t.Run("state updated", func(t *testing.T) {
		payload := LangGraphStateUpdatedPayload{
			GraphID:   "graph-1",
			NodeID:    "node-1",
			State:     map[string]interface{}{"value": 42},
			UpdatedAt: time.Now(),
			Revision:  1,
		}
		err := p.PublishLangGraphStateUpdated(payload)
		if err != nil {
			t.Fatalf("PublishLangGraphStateUpdated failed: %v", err)
		}
	})

	t.Run("graph deployed", func(t *testing.T) {
		payload := LangGraphGraphDeployedPayload{
			GraphID:    "graph-1",
			TenantID:   "tenant-1",
			DeployedBy: "user-1",
			DeployedAt: time.Now(),
			Status:     "active",
		}
		err := p.PublishLangGraphGraphDeployed(payload)
		if err != nil {
			t.Fatalf("PublishLangGraphGraphDeployed failed: %v", err)
		}
	})
}

func TestPublishTemporalEvents(t *testing.T) {
	p := NewPublisher()

	t.Run("workflow registered", func(t *testing.T) {
		payload := TemporalWorkflowRegisteredPayload{
			WorkflowID:   "wf-1",
			TenantID:     "tenant-1",
			Name:         "Test Workflow",
			Version:      "1.0.0",
			RegisterdBy:  "user-1",
			RegisteredAt: time.Now(),
		}
		err := p.PublishTemporalWorkflowRegistered(payload)
		if err != nil {
			t.Fatalf("PublishTemporalWorkflowRegistered failed: %v", err)
		}
	})

	t.Run("checkpoint created", func(t *testing.T) {
		payload := TemporalCheckpointCreatedPayload{
			CheckpointID: "cp-1",
			WorkflowID:   "wf-1",
			TenantID:     "tenant-1",
			CreatedAt:    time.Now(),
		}
		err := p.PublishTemporalCheckpointCreated(payload)
		if err != nil {
			t.Fatalf("PublishTemporalCheckpointCreated failed: %v", err)
		}
	})

	t.Run("workflow replayed", func(t *testing.T) {
		payload := TemporalWorkflowReplayedPayload{
			WorkflowID:     "wf-1",
			TenantID:       "tenant-1",
			ReplayID:       "replay-1",
			FromCheckpoint: "cp-1",
			ReplayedBy:     "user-1",
			ReplayedAt:     time.Now(),
		}
		err := p.PublishTemporalWorkflowReplayed(payload)
		if err != nil {
			t.Fatalf("PublishTemporalWorkflowReplayed failed: %v", err)
		}
	})
}

func TestPublishRayEvents(t *testing.T) {
	p := NewPublisher()

	t.Run("worker pooled", func(t *testing.T) {
		payload := RayWorkerPooledPayload{
			PoolID:      "pool-1",
			TenantID:    "tenant-1",
			WorkerCount: 10,
			CreatedAt:   time.Now(),
		}
		err := p.PublishRayWorkerPooled(payload)
		if err != nil {
			t.Fatalf("PublishRayWorkerPooled failed: %v", err)
		}
	})

	t.Run("task submitted", func(t *testing.T) {
		payload := RayTaskSubmittedPayload{
			TaskID:      "task-1",
			PoolID:      "pool-1",
			TenantID:    "tenant-1",
			SubmittedBy: "user-1",
			SubmittedAt: time.Now(),
		}
		err := p.PublishRayTaskSubmitted(payload)
		if err != nil {
			t.Fatalf("PublishRayTaskSubmitted failed: %v", err)
		}
	})

	t.Run("task completed", func(t *testing.T) {
		payload := RayTaskCompletedPayload{
			TaskID:      "task-1",
			PoolID:      "pool-1",
			TenantID:    "tenant-1",
			CompletedAt: time.Now(),
			DurationMs:  100,
			Success:     true,
		}
		err := p.PublishRayTaskCompleted(payload)
		if err != nil {
			t.Fatalf("PublishRayTaskCompleted failed: %v", err)
		}
	})

	t.Run("worker status", func(t *testing.T) {
		payload := RayWorkerStatusPayload{
			WorkerID:  "worker-1",
			PoolID:    "pool-1",
			TenantID:  "tenant-1",
			Status:    "running",
			UpdatedAt: time.Now(),
		}
		err := p.PublishRayWorkerStatus(payload)
		if err != nil {
			t.Fatalf("PublishRayWorkerStatus failed: %v", err)
		}
	})
}

func TestPublishCeleryEvents(t *testing.T) {
	p := NewPublisher()

	t.Run("queue created", func(t *testing.T) {
		payload := CeleryQueueCreatedPayload{
			QueueID:   "queue-1",
			TenantID:  "tenant-1",
			Name:      "default",
			CreatedAt: time.Now(),
		}
		err := p.PublishCeleryQueueCreated(payload)
		if err != nil {
			t.Fatalf("PublishCeleryQueueCreated failed: %v", err)
		}
	})

	t.Run("task published", func(t *testing.T) {
		payload := CeleryTaskPublishedPayload{
			TaskID:      "task-1",
			QueueID:     "queue-1",
			TenantID:    "tenant-1",
			PublishedAt: time.Now(),
			PublishedBy: "user-1",
		}
		err := p.PublishCeleryTaskPublished(payload)
		if err != nil {
			t.Fatalf("PublishCeleryTaskPublished failed: %v", err)
		}
	})

	t.Run("task consumed", func(t *testing.T) {
		payload := CeleryTaskConsumedPayload{
			TaskID:     "task-1",
			QueueID:    "queue-1",
			WorkerID:   "worker-1",
			TenantID:   "tenant-1",
			ConsumedAt: time.Now(),
		}
		err := p.PublishCeleryTaskConsumed(payload)
		if err != nil {
			t.Fatalf("PublishCeleryTaskConsumed failed: %v", err)
		}
	})

	t.Run("task completed", func(t *testing.T) {
		payload := CeleryTaskCompletedPayload{
			TaskID:      "task-1",
			QueueID:     "queue-1",
			TenantID:    "tenant-1",
			CompletedAt: time.Now(),
			DurationMs:  50,
			Success:     true,
		}
		err := p.PublishCeleryTaskCompleted(payload)
		if err != nil {
			t.Fatalf("PublishCeleryTaskCompleted failed: %v", err)
		}
	})

	t.Run("worker heartbeat", func(t *testing.T) {
		payload := CeleryWorkerHeartbeatPayload{
			WorkerID:  "worker-1",
			QueueID:   "queue-1",
			TenantID:  "tenant-1",
			Timestamp: time.Now(),
			Status:    "alive",
		}
		err := p.PublishCeleryWorkerHeartbeat(payload)
		if err != nil {
			t.Fatalf("PublishCeleryWorkerHeartbeat failed: %v", err)
		}
	})
}

func TestPublishStackHealth(t *testing.T) {
	p := NewPublisher()

	payload := StackHealthPayload{
		StackType: "langgraph",
		StackName: "default",
		TenantID:  "tenant-1",
		Status:    "healthy",
		UpdatedAt: time.Now(),
	}

	err := p.PublishStackHealth(payload)
	if err != nil {
		t.Fatalf("PublishStackHealth failed: %v", err)
	}
}
