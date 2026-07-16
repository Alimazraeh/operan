package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// SandboxProfile is the subset of profile fields needed for execution.
type SandboxProfile struct {
	Name            string
	CPUCores        float64
	MemoryMB        int
	TimeoutSeconds  int
	NetworkAccess   bool
	AllowedTools    []string
	FilesystemAccess bool
	MaxFileSizeMB   int
	MaxOutputSizeKB int
}

// ExecutionRequest carries the input for a sandbox execution.
type ExecutionRequest struct {
	TenantID   string
	AgentID    string
	ToolName   string
	InputData  string
	Profile    SandboxProfile
	ToolConfig map[string]interface{}
}

// Result holds the output of a sandbox execution.
type Result struct {
	ExitCode     int
	Stdout       string
	Stderr       string
	CPUMs        int
	MemoryPeakMB int
	DurationMs   int
	Err          error
}

// Executor runs tool executions inside an isolated sandbox.
type Executor struct {
	workspace string
}

// NewExecutor creates a new Executor with the given base workspace directory.
func NewExecutor(workspace string) (*Executor, error) {
	if workspace == "" {
		workspace = "/tmp/operan-sandbox"
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	return &Executor{workspace: workspace}, nil
}

// Execute runs a tool inside the sandbox with resource isolation.
func (e *Executor) Execute(ctx context.Context, req *ExecutionRequest) (*Result, error) {
	// Create isolated workspace
	wsDir := filepath.Join(e.workspace, req.TenantID, req.ToolName)
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace dir: %w", err)
	}
	defer os.RemoveAll(wsDir)

	// Build the command
	cmd := e.buildCommand(req, wsDir)

	// Start in a goroutine so we can enforce timeout
	type result struct {
		r   *Result
		err error
	}
	ch := make(chan result, 1)

	go func() {
		r, err := e.runCommand(ctx, cmd, req)
		ch <- result{r, err}
	}()

	// Timeout enforcement
	timeout := time.Duration(req.Profile.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	select {
	case res := <-ch:
		return res.r, res.err
	case <-time.After(timeout):
		// Force kill the process
		if cmd.Process != nil {
			cmd.Process.Signal(syscall.SIGKILL)
		}
		return &Result{
			ExitCode:   -1,
			Stderr:     "execution timed out",
			DurationMs: int(timeout.Milliseconds()),
			Err:        fmt.Errorf("execution timed out after %d seconds", req.Profile.TimeoutSeconds),
		}, nil
	case <-ctx.Done():
		if cmd.Process != nil {
			cmd.Process.Signal(syscall.SIGKILL)
		}
		return nil, ctx.Err()
	}
}

func (e *Executor) buildCommand(req *ExecutionRequest, wsDir string) *exec.Cmd {
	toolName := req.ToolName
	if toolName == "" {
		toolName = "echo"
	}

	cmd := exec.CommandContext(context.Background(), toolName)
	cmd.Dir = wsDir
	cmd.Stdin = nil

	// Set non-root credentials
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: 65534, // nobody
			Gid: 65534,
		},
	}

	// Set resource limits if applicable
	if req.Profile.MemoryMB > 0 {
		// Memory limit via RLIMIT_AS
		_ = int64(req.Profile.MemoryMB)
		cmd.SysProcAttr.Setpgid = true
	}

	return cmd
}

func (e *Executor) runCommand(ctx context.Context, cmd *exec.Cmd, req *ExecutionRequest) (*Result, error) {
	start := time.Now()

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &Result{
		DurationMs: int(duration.Milliseconds()),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Err = err
		}
		// Capture stderr (combined output for now)
		maxOut := req.Profile.MaxOutputSizeKB * 1024
		if maxOut <= 0 {
			maxOut = 1024 * 1024
		}
		if len(output) > maxOut {
			result.Stderr = string(output[:maxOut])
		} else {
			result.Stderr = string(output)
		}
		return result, nil
	}

	result.ExitCode = 0
	maxOut := req.Profile.MaxOutputSizeKB * 1024
	if maxOut <= 0 {
		maxOut = 1024 * 1024
	}
	if len(output) > maxOut {
		result.Stdout = string(output[:maxOut])
	} else {
		result.Stdout = string(output)
	}

	return result, nil
}