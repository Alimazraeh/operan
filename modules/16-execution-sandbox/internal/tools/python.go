package tools

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// PythonExecutor runs Python code inside the sandbox workspace.
type PythonExecutor struct {
	workspace string
}

// NewPythonExecutor creates a new PythonExecutor.
func NewPythonExecutor(workspace string) *PythonExecutor {
	return &PythonExecutor{workspace: workspace}
}

// Execute runs Python code and returns the result.
func (e *PythonExecutor) Execute(code, inputData string) (stdout, stderr string, exitCode int, err error) {
	tmpDir := filepath.Join(e.workspace, "python_exec")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Sprintf("create tmp dir: %s", err), -1, err
	}

	f, err := os.CreateTemp(tmpDir, "sandbox_script_*.py")
	if err != nil {
		return "", fmt.Sprintf("create temp file: %s", err), -1, err
	}
	scriptPath := f.Name()
	defer os.Remove(scriptPath)

	if _, err := f.WriteString(code); err != nil {
		f.Close()
		os.Remove(scriptPath)
		return "", fmt.Sprintf("write code: %s", err), -1, err
	}
	f.Close()

	cmd := exec.Command("python3", scriptPath)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return stdoutBuf.String(), stderrBuf.String(), -1, err
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, nil
}