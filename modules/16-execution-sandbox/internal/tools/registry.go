package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Tool defines an executable tool available in the sandbox.
type Tool struct {
	Name        string
	Description string
	Command     string
	Args        []string
}

// Registry is a registry of available sandbox tools.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a default tool registry.
func NewRegistry() *Registry {
	r := &Registry{tools: make(map[string]Tool)}
	r.registerDefaultTools()
	return r
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (*Tool, bool) {
	t, ok := r.tools[name]
	return &t, ok
}

// List returns all registered tool names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	return names
}

// Execute runs a tool with the given input and returns stdout/stderr/exitCode.
func (r *Registry) Execute(name string, inputData string) (stdout, stderr string, exitCode int, err error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Sprintf("unknown tool: %s", name), -1, fmt.Errorf("tool not found: %s", name)
	}

	cmd := exec.Command(t.Command)
	if len(t.Args) > 0 {
		cmd.Args = append([]string{t.Command}, t.Args...)
	}
	// For tools like bash -c, pass inputData as the command argument
	if len(t.Args) > 0 && t.Args[0] == "-c" && inputData != "" {
		cmd.Args = append(cmd.Args, inputData)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if inputData != "" {
		cmd.Stdin = strings.NewReader(inputData)
	}

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

func (r *Registry) registerDefaultTools() {
	r.tools["echo"] = Tool{
		Name:        "echo",
		Description: "Print text output",
		Command:     "echo",
	}
	r.tools["cat"] = Tool{
		Name:        "cat",
		Description: "Concatenate and print files",
		Command:     "cat",
	}
	r.tools["python3"] = Tool{
		Name:        "python3",
		Description: "Execute Python scripts",
		Command:     "python3",
	}
	r.tools["bash"] = Tool{
		Name:        "bash",
		Description: "Execute shell commands",
		Command:     "/bin/bash",
		Args:        []string{"-c"},
	}
	r.tools["ls"] = Tool{
		Name:        "ls",
		Description: "List directory contents",
		Command:     "ls",
	}
	r.tools["wc"] = Tool{
		Name:        "wc",
		Description: "Count lines, words, bytes",
		Command:     "wc",
	}
	r.tools["jq"] = Tool{
		Name:        "jq",
		Description: "JSON processor",
		Command:     "jq",
	}
}