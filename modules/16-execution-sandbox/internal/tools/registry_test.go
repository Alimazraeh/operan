package tools

import (
	"strings"
	"testing"
)

func TestRegistry_Get_Found(t *testing.T) {
	r := NewRegistry()
	t2, ok := r.Get("echo")
	if !ok {
		t.Fatal("expected echo to be found")
	}
	if t2.Name != "echo" {
		t.Errorf("expected name 'echo', got '%s'", t2.Name)
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("nonexistent-tool")
	if ok {
		t.Error("expected nonexistent tool to not be found")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	tools := r.List()
	if len(tools) < 5 {
		t.Errorf("expected at least 5 tools, got %d", len(tools))
	}
	// Verify some expected tools
	found := make(map[string]bool)
	for _, name := range tools {
		found[name] = true
	}
	if !found["echo"] || !found["bash"] || !found["python3"] {
		t.Errorf("missing expected tools: %v", found)
	}
}

func TestRegistry_Execute_Echo(t *testing.T) {
	r := NewRegistry()
	stdout, _, exitCode, err := r.Execute("echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	// echo runs without args, outputs nothing; the input is passed to stdin
	// The echo tool just runs "echo" with no arguments
	if stdout != "" {
		t.Logf("echo output: %q", stdout)
	}
}

func TestRegistry_Execute_EchoWithInput(t *testing.T) {
	// Test bash tool which accepts commands via stdin
	r := NewRegistry()
	stdout, _, exitCode, err := r.Execute("bash", "echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("expected stdout to contain 'hello', got '%s'", stdout)
	}
}

func TestRegistry_Execute_UnknownTool(t *testing.T) {
	r := NewRegistry()
	_, stderr, exitCode, err := r.Execute("nonexistent", "input")
	if err == nil {
		t.Error("expected error for unknown tool")
	}
	if exitCode != -1 {
		t.Errorf("expected exit code -1, got %d", exitCode)
	}
	if stderr == "" {
		t.Error("expected non-empty stderr for unknown tool")
	}
}

func TestRegistry_Execute_Bash(t *testing.T) {
	// bash tool runs commands from stdin via "bash -c"
	r := NewRegistry()
	stdout, _, exitCode, err := r.Execute("bash", "echo success")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "success") {
		t.Errorf("expected stdout to contain 'success', got '%s'", stdout)
	}
}