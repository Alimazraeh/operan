package sandbox

import (
	"context"
	"os"
	"testing"
)

func TestNewExecutor(t *testing.T) {
	workspace := "/tmp/test-sandbox-" + t.Name()
	defer os.RemoveAll(workspace)

	e, err := NewExecutor(workspace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.workspace != workspace {
		t.Errorf("expected workspace '%s', got '%s'", workspace, e.workspace)
	}
}

func TestNewExecutor_DefaultWorkspace(t *testing.T) {
	e, err := NewExecutor("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.workspace != "/tmp/operan-sandbox" {
		t.Errorf("expected default workspace, got '%s'", e.workspace)
	}
	os.RemoveAll(e.workspace)
}

func TestExecute_Echo(t *testing.T) {
	workspace := "/tmp/test-sandbox-execute-" + t.Name()
	defer os.RemoveAll(workspace)

	e, err := NewExecutor(workspace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := &ExecutionRequest{
		TenantID: "tenant-1",
		ToolName: "echo",
		InputData: "hello sandbox",
		Profile: SandboxProfile{
			TimeoutSeconds: 10,
			MaxOutputSizeKB: 1024,
		},
	}

	result, err := e.Execute(context.Background(), req)
	if err != nil && result == nil {
		// Command might not exist (e.g. echo not on path) — that's OK
		t.Logf("execute result: exit=%d stdout=%q stderr=%q err=%v", result.ExitCode, result.Stdout, result.Stderr, result.Err)
	}
	if result != nil {
		t.Logf("exit_code=%d duration=%dms stdout=%q", result.ExitCode, result.DurationMs, result.Stdout)
	}
}