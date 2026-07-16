package store

import (
	"time"

	"github.com/google/uuid"
)

// SandboxProfile represents a row in the sandbox_profiles table.
type SandboxProfile struct {
	ID              uuid.UUID   `json:"id" db:"id" dbjson:"id"`
	TenantID        string      `json:"tenant_id" db:"tenant_id"`
	Name            string      `json:"name" db:"name"`
	Description     *string     `json:"description,omitempty" db:"description"`
	CPUCores        float64     `json:"cpu_cores" db:"cpu_cores"`
	MemoryMB        int         `json:"memory_mb" db:"memory_mb"`
	TimeoutSeconds  int         `json:"timeout_seconds" db:"timeout_seconds"`
	NetworkAccess   bool        `json:"network_access" db:"network_access"`
	AllowedTools    []string    `json:"allowed_tools" db:"allowed_tools"`
	FilesystemAccess bool       `json:"filesystem_access" db:"filesystem_access"`
	MaxFileSizeMB   int         `json:"max_file_size_mb" db:"max_file_size_mb"`
	MaxOutputSizeKB int         `json:"max_output_size_kb" db:"max_output_size_kb"`
	IsActive        bool        `json:"is_active" db:"is_active"`
	CreatedAt       time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at" db:"updated_at"`
}

// SandboxInstance represents a row in the sandbox_instances table.
type SandboxInstance struct {
	ID            uuid.UUID  `json:"id" db:"id" dbjson:"id"`
	TenantID      string     `json:"tenant_id" db:"tenant_id"`
	AgentID       *string    `json:"agent_id,omitempty" db:"agent_id"`
	ProfileID     uuid.UUID  `json:"profile_id" db:"profile_id"`
	ToolName      string     `json:"tool_name" db:"tool_name"`
	InputData     *string    `json:"input_data,omitempty" db:"input_data"`
	ExitCode      *int       `json:"exit_code,omitempty" db:"exit_code"`
	Stdout        *string    `json:"stdout,omitempty" db:"stdout"`
	Stderr        *string    `json:"stderr,omitempty" db:"stderr"`
	Status        string     `json:"status" db:"status"`
	CPUMs         *int       `json:"cpu_time_ms,omitempty" db:"cpu_time_ms"`
	MemoryPeakMB  *int       `json:"memory_peak_mb,omitempty" db:"memory_peak_mb"`
	ErrorMessage  *string    `json:"error_message,omitempty" db:"error_message"`
	StartedAt     *time.Time `json:"started_at,omitempty" db:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}