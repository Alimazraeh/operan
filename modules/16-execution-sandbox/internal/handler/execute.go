package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/operan/execution-sandbox/internal/ctxkeys"
	"github.com/operan/execution-sandbox/internal/events"
	"github.com/operan/execution-sandbox/internal/policies"
	"github.com/operan/execution-sandbox/internal/sandbox"
	"github.com/operan/execution-sandbox/internal/store"
)

// ExecuteHandler handles sandbox execution requests.
type ExecuteHandler struct {
	executor    *sandbox.Executor
	profileStore *store.ProfileStore
	instanceStore *store.InstanceStore
	policyClient *policies.PolicyClient
	eventPub     *events.Publisher
}

// NewExecuteHandler creates a new ExecuteHandler.
func NewExecuteHandler(executor *sandbox.Executor, profileStore *store.ProfileStore,
	instanceStore *store.InstanceStore, policyClient *policies.PolicyClient, eventPub *events.Publisher,
) *ExecuteHandler {
	return &ExecuteHandler{
		executor:     executor,
		profileStore: profileStore,
		instanceStore: instanceStore,
		policyClient: policyClient,
		eventPub:     eventPub,
	}
}

// Execute handles POST /v1/sandboxes/execute.
func (h *ExecuteHandler) Execute(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())

	var req struct {
		ProfileID  string                 `json:"profile_id"`
		ToolName   string                 `json:"tool_name"`
		InputData  string                 `json:"input_data"`
		ToolConfig map[string]interface{} `json:"tool_config,omitempty"`
		AgentID    string                 `json:"agent_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.ToolName == "" {
		http.Error(w, `{"error":"bad-request","message":"tool_name is required"}`, http.StatusBadRequest)
		return
	}
	if req.ProfileID == "" {
		http.Error(w, `{"error":"bad-request","message":"profile_id is required"}`, http.StatusBadRequest)
		return
	}

	profileID, err := uuid.Parse(req.ProfileID)
	if err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid profile_id"}`, http.StatusBadRequest)
		return
	}

	// Fetch profile
	profile, err := h.profileStore.GetByID(r.Context(), profileID)
	if err != nil {
		if err == store.ErrNotFound {
			http.Error(w, `{"error":"not-found","message":"profile not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if profile.TenantID != tenantID {
		http.Error(w, `{"error":"forbidden","message":"tenant mismatch"}`, http.StatusForbidden)
		return
	}
	if !profile.IsActive {
		http.Error(w, `{"error":"bad-request","message":"profile is inactive"}`, http.StatusBadRequest)
		return
	}

	// Check tool is allowed by profile
	if !sandbox.IsToolAllowed(sandbox.SandboxProfile{AllowedTools: profile.AllowedTools}, req.ToolName) {
		http.Error(w, `{"error":"forbidden","message":"tool not allowed in this profile"}`, http.StatusForbidden)
		return
	}

	// Policy check via M10. Sandboxed execution is a side effect on the
	// customer's infrastructure: if we cannot establish that it is permitted,
	// it does not run. Fail closed, never open.
	policyResult, err := h.policyClient.CanExecute(r.Context(), tenantID, req.AgentID, req.ToolName)
	if err != nil && policyResult == nil {
		policyResult = &policies.PolicyCheckResult{
			Allowed: false,
			Reason:  "policy engine did not answer: " + err.Error(),
		}
	}
	if policyResult == nil || !policyResult.Allowed {
		h.eventPub.Publish("operan.sandbox.policy_denied", tenantID, map[string]interface{}{
			"tenant_id": tenantID,
			"agent_id":  req.AgentID,
			"tool_name": req.ToolName,
			"reason":    policyResult.Reason,
		})
		// Create instance record with policy_denied status
		inst := &store.SandboxInstance{
			TenantID: tenantID,
			AgentID:  strPtr(req.AgentID),
			ProfileID: profileID,
			ToolName:  req.ToolName,
			InputData: strPtr(req.InputData),
			Status:    "policy_denied",
		}
		if err := h.instanceStore.Create(r.Context(), inst); err != nil {
			h.logError("failed to create policy_denied instance record:", err)
		}
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"instance_id": inst.ID,
			"status":      "policy_denied",
			"error":       policyResult.Reason,
		})
		return
	}

	// Create instance record
	inst := &store.SandboxInstance{
		TenantID:  tenantID,
		AgentID:   strPtr(req.AgentID),
		ProfileID: profileID,
		ToolName:  req.ToolName,
		InputData: strPtr(req.InputData),
		Status:    "running",
	}
	if err := h.instanceStore.Create(r.Context(), inst); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Publish execution_started event
	h.eventPub.Publish("operan.sandbox.execution_started", tenantID, map[string]interface{}{
		"tenant_id":  tenantID,
		"agent_id":   req.AgentID,
		"instance_id": inst.ID.String(),
		"tool_name":  req.ToolName,
		"profile_id": profileID.String(),
	})

	// Run execution
	sandboxProfile := sandbox.SandboxProfile{
		Name:            profile.Name,
		CPUCores:        profile.CPUCores,
		MemoryMB:        profile.MemoryMB,
		TimeoutSeconds:  profile.TimeoutSeconds,
		NetworkAccess:   profile.NetworkAccess,
		AllowedTools:    profile.AllowedTools,
		FilesystemAccess: profile.FilesystemAccess,
		MaxFileSizeMB:   profile.MaxFileSizeMB,
		MaxOutputSizeKB: profile.MaxOutputSizeKB,
	}

	// Start tracking
	now := time.Now()
	inst.StartedAt = &now

	reqCtx, cancel := context.WithTimeout(r.Context(), time.Duration(profile.TimeoutSeconds)*time.Second)
	defer cancel()

	execReq := &sandbox.ExecutionRequest{
		TenantID:   tenantID,
		AgentID:    req.AgentID,
		ToolName:   req.ToolName,
		InputData:  req.InputData,
		Profile:    sandboxProfile,
		ToolConfig: req.ToolConfig,
	}

	result, err := h.executor.Execute(reqCtx, execReq)
	if result == nil {
		result = &sandbox.Result{Err: err}
	}

	// Update instance record
	completedAt := time.Now()
	inst.CompletedAt = &completedAt
	inst.ExitCode = &result.ExitCode
	inst.Stdout = strPtr(result.Stdout)
	inst.Stderr = strPtr(result.Stderr)
	inst.CPUMs = &result.CPUMs
	inst.MemoryPeakMB = &result.MemoryPeakMB

	if err != nil {
		if result.Err != nil {
			inst.ErrorMessage = strPtr(result.Err.Error())
		}
		if result.DurationMs > 0 {
			// Was it a timeout?
			inst.Status = "timeout"
		} else {
			inst.Status = "failed"
		}
	} else {
		inst.Status = "completed"
	}

	if err := h.instanceStore.UpdateStatus(r.Context(), inst.ID, map[string]interface{}{
		"exit_code":      inst.ExitCode,
		"stdout":         inst.Stdout,
		"stderr":         inst.Stderr,
		"status":         inst.Status,
		"cpu_time_ms":    inst.CPUMs,
		"memory_peak_mb": inst.MemoryPeakMB,
		"error_message":  inst.ErrorMessage,
		"started_at":     inst.StartedAt,
		"completed_at":   inst.CompletedAt,
	}); err != nil {
		h.logError("failed to update instance status:", err)
	}

	// Publish completion or timeout event
	if inst.Status == "completed" {
		durationMs := int(completedAt.Sub(*inst.StartedAt).Milliseconds())
		h.eventPub.Publish("operan.sandbox.execution_completed", tenantID, map[string]interface{}{
			"tenant_id":     tenantID,
			"agent_id":      req.AgentID,
			"instance_id":   inst.ID.String(),
			"tool_name":     req.ToolName,
			"exit_code":     *inst.ExitCode,
			"cpu_time_ms":   *inst.CPUMs,
			"memory_peak_mb": *inst.MemoryPeakMB,
			"duration_ms":   durationMs,
		})
	} else if inst.Status == "timeout" {
		h.eventPub.Publish("operan.sandbox.execution_timeout", tenantID, map[string]interface{}{
			"tenant_id":         tenantID,
			"instance_id":       inst.ID.String(),
			"tool_name":         req.ToolName,
			"timeout_seconds":   profile.TimeoutSeconds,
		})
	} else {
		h.eventPub.Publish("operan.sandbox.execution_failed", tenantID, map[string]interface{}{
			"tenant_id":     tenantID,
			"instance_id":   inst.ID.String(),
			"tool_name":     req.ToolName,
			"error_message": result.Err.Error(),
			"exit_code":     result.ExitCode,
		})
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"instance_id": inst.ID,
		"status":      inst.Status,
		"exit_code":   inst.ExitCode,
		"stdout":      inst.Stdout,
		"stderr":      inst.Stderr,
	})
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (h *ExecuteHandler) logError(msg string, err error) {
	// Non-fatal logging — sandbox execution doesn't block on this
}