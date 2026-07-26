package handlers

import (
	"testing"

	"github.com/operan/modules/05-department-template-engine/internal/store"
)

// TestIsOperational covers the mechanical rule from WO-2: a template is
// operational if at least one step in at least one of its workflows carries
// config.capability. Everything else — including a template with workflows
// whose steps simply don't happen to bind a capability — is an outline.
func TestIsOperational(t *testing.T) {
	tests := []struct {
		name string
		tmpl store.Template
		want bool
	}{
		{
			name: "no workflows at all",
			tmpl: store.Template{},
			want: false,
		},
		{
			name: "workflow with steps but zero capability-bearing steps", // the boundary case WO-2 requires
			tmpl: store.Template{
				Workflows: []store.WorkflowDefinition{
					{
						ID:   "wf-1",
						Name: "Onboard",
						Steps: []store.WorkflowStep{
							{ID: "s1", Type: "agent_call", Config: map[string]interface{}{"agent": "hr-lead"}},
							{ID: "s2", Type: "human_gate", Config: nil},
							{ID: "s3", Type: "notification", Config: map[string]interface{}{"template": "welcome"}},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "single step in single workflow carries a capability",
			tmpl: store.Template{
				Workflows: []store.WorkflowDefinition{
					{
						ID: "wf-1",
						Steps: []store.WorkflowStep{
							{ID: "s1", Type: "tool_call", Config: map[string]interface{}{"capability": "itsm.ticket.assign"}},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "capability appears only on the last step of the last workflow",
			tmpl: store.Template{
				Workflows: []store.WorkflowDefinition{
					{ID: "wf-1", Steps: []store.WorkflowStep{{ID: "a", Config: map[string]interface{}{"agent": "x"}}}},
					{ID: "wf-2", Steps: []store.WorkflowStep{
						{ID: "b", Config: map[string]interface{}{"agent": "y"}},
						{ID: "c", Config: map[string]interface{}{"capability": "comms.message.send"}},
					}},
				},
			},
			want: true,
		},
		{
			name: "empty-string capability value still counts as carrying the key",
			tmpl: store.Template{
				Workflows: []store.WorkflowDefinition{
					{ID: "wf-1", Steps: []store.WorkflowStep{{ID: "a", Config: map[string]interface{}{"capability": ""}}}},
				},
			},
			// Matches the existing presence-only check in
			// internal/deploy/orchestrator.go:609 (`_, hasCap := s.Config["capability"]`)
			// rather than inventing a stricter rule for this new call site.
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOperational(tt.tmpl); got != tt.want {
				t.Errorf("isOperational() = %v, want %v", got, tt.want)
			}
		})
	}
}
