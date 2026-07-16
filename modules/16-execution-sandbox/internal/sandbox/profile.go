package sandbox

import (
	"fmt"
)

// ValidateProfile checks that a profile has valid resource limits.
func ValidateProfile(p SandboxProfile) error {
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if p.MemoryMB < 16 {
		return fmt.Errorf("memory_mb must be at least 16")
	}
	if p.MemoryMB > 65536 {
		return fmt.Errorf("memory_mb must not exceed 65536")
	}
	if p.TimeoutSeconds < 1 {
		return fmt.Errorf("timeout_seconds must be at least 1")
	}
	if p.TimeoutSeconds > 3600 {
		return fmt.Errorf("timeout_seconds must not exceed 3600")
	}
	if p.MaxFileSizeMB < 1 {
		return fmt.Errorf("max_file_size_mb must be at least 1")
	}
	if p.MaxOutputSizeKB < 1 {
		return fmt.Errorf("max_output_size_kb must be at least 1")
	}
	return nil
}

// IsToolAllowed checks if a tool is in the profile's allowed list.
func IsToolAllowed(profile SandboxProfile, toolName string) bool {
	if len(profile.AllowedTools) == 0 {
		// Empty allowed_tools means all tools are allowed (permissive mode)
		return true
	}
	for _, t := range profile.AllowedTools {
		if t == toolName {
			return true
		}
	}
	return false
}