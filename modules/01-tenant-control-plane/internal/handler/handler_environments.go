package handler

import (
	"encoding/json"
	"net/http"

	"github.com/operan/modules/01-tenant-control-plane/internal/middleware"
	"github.com/operan/modules/01-tenant-control-plane/internal/store"
)

// ─── Environment request/response types ──────────────────────────────────────

// EnvironmentCreateRequest for creating a new environment.
type EnvironmentCreateRequest struct {
	Name            string                         `json:"name"`
	Type            string                         `json:"type"`
	IsolationLevel  string                         `json:"isolation_level,omitempty"`
	IsolationConfig *EnvironmentIsolationConfigReq `json:"isolation_config,omitempty"`
	NetworkConfig   map[string]any                 `json:"network_config,omitempty"`
	CreatedBy       string                         `json:"created_by,omitempty"`
	Notes           string                         `json:"notes,omitempty"`
}

// EnvironmentIsolationConfigReq defines the request schema for isolation config.
type EnvironmentIsolationConfigReq struct {
	DataIsolation    bool                     `json:"data_isolation,omitempty"`
	NetworkIsolation bool                     `json:"network_isolation,omitempty"`
	ResourceQuota    EnvironmentResourceQuotaReq `json:"resource_quota"`
	BackupPolicy     EnvironmentBackupPolicyReq `json:"backup_policy"`
	ComplianceLevel  string                   `json:"compliance_level,omitempty"`
	Metadata         map[string]any           `json:"metadata,omitempty"`
}

// EnvironmentResourceQuotaReq defines resource limits for an environment (request).
type EnvironmentResourceQuotaReq struct {
	MaxCPUs        int     `json:"max_cpus,omitempty"`
	MaxMemoryGB    float64 `json:"max_memory_gb,omitempty"`
	MaxStorageGB   float64 `json:"max_storage_gb,omitempty"`
	MaxConnections int     `json:"max_connections,omitempty"`
	MaxDeployments int     `json:"max_deployments,omitempty"`
	MaxAgents      int     `json:"max_agents,omitempty"`
}

// EnvironmentBackupPolicyReq defines backup configuration (request).
type EnvironmentBackupPolicyReq struct {
	Enabled           bool   `json:"enabled,omitempty"`
	Frequency         string `json:"frequency,omitempty"`
	RetentionDays     int    `json:"retention_days,omitempty"`
	EncryptionEnabled bool   `json:"encryption_enabled,omitempty"`
	BackupRegion      string `json:"backup_region,omitempty"`
}

// EnvironmentPatchRequest for partial environment updates.
type EnvironmentPatchRequest struct {
	Name            string                            `json:"name,omitempty"`
	Notes           string                            `json:"notes,omitempty"`
	IsolationLevel  string                            `json:"isolation_level,omitempty"`
	IsolationConfig *EnvironmentIsolationConfigReq    `json:"isolation_config,omitempty"`
}

// EnvironmentResponse represents an environment in API responses.
type EnvironmentResponse struct {
	ID              string                                  `json:"id"`
	TenantID        string                                  `json:"tenant_id"`
	Name            string                                  `json:"name"`
	Type            string                                  `json:"type"`
	State           string                                  `json:"state"`
	IsolationLevel  string                                  `json:"isolation_level"`
	IsolationConfig EnvironmentIsolationConfigResponse      `json:"isolation_config"`
	ResourceIDs     []string                                `json:"resource_ids,omitempty"`
	NetworkConfig   map[string]any                          `json:"network_config,omitempty"`
	CreatedBy       string                                  `json:"created_by,omitempty"`
	Notes           string                                  `json:"notes,omitempty"`
	CreatedAt       string                                  `json:"created_at"`
	UpdatedAt       string                                  `json:"updated_at"`
	ActivatedAt     *string                                 `json:"activated_at,omitempty"`
	DeactivatedAt   *string                                 `json:"deactivated_at,omitempty"`
}

// EnvironmentIsolationConfigResponse represents isolation config in API responses.
type EnvironmentIsolationConfigResponse struct {
	DataIsolation    bool                          `json:"data_isolation"`
	NetworkIsolation bool                          `json:"network_isolation"`
	ResourceQuota    EnvironmentResourceQuotaResp  `json:"resource_quota"`
	BackupPolicy     EnvironmentBackupPolicyResp   `json:"backup_policy"`
	ComplianceLevel  string                        `json:"compliance_level,omitempty"`
	Metadata         map[string]any                `json:"metadata,omitempty"`
}

// EnvironmentResourceQuotaResp represents resource quota in API responses.
type EnvironmentResourceQuotaResp struct {
	MaxCPUs        int     `json:"max_cpus"`
	MaxMemoryGB    float64 `json:"max_memory_gb"`
	MaxStorageGB   float64 `json:"max_storage_gb"`
	MaxConnections int     `json:"max_connections"`
	MaxDeployments int     `json:"max_deployments"`
	MaxAgents      int     `json:"max_agents"`
}

// EnvironmentBackupPolicyResp represents backup policy in API responses.
type EnvironmentBackupPolicyResp struct {
	Enabled           bool   `json:"enabled"`
	Frequency         string `json:"frequency"`
	RetentionDays     int    `json:"retention_days"`
	EncryptionEnabled bool   `json:"encryption_enabled"`
	BackupRegion      string `json:"backup_region,omitempty"`
}

// ─── Environment handlers ────────────────────────────────────────────────────

// ListEnvironments handles GET /v1/tenants/{id}/environments.
func ListEnvironments(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		page, pageSize := paginationFrom(r)
		environments, total, _ := h.EnvironmentStore.ListByTenant(tenantID, page, pageSize)

		items := make([]*EnvironmentResponse, 0, len(environments))
		for _, e := range environments {
			items = append(items, environmentToResponse(e))
		}

		h.WriteJSON(w, http.StatusOK, &EnvironmentListResponse{
			Items:    items,
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		})
	}
}

// CreateEnvironment handles POST /v1/tenants/{id}/environments.
func CreateEnvironment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		var req EnvironmentCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "Invalid request body")
			return
		}

		if req.Name == "" {
				h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "Name is required")
			return
		}

		e := &store.Environment{
			TenantID:       tenantID,
			Name:           req.Name,
			Type:           store.EnvironmentType(req.Type),
			State:          store.EnvironmentStateCreating,
			IsolationLevel: store.EnvironmentIsolationLevel(req.IsolationLevel),
			IsolationConfig: store.EnvironmentIsolationConfig{
				DataIsolation:   true,
				NetworkIsolation: true,
				ResourceQuota:   store.EnvironmentResourceQuota{
					MaxCPUs:        4,
					MaxMemoryGB:    16,
					MaxStorageGB:   100,
					MaxConnections: 100,
					MaxDeployments: 10,
					MaxAgents:      5,
				},
			},
			NetworkConfig: req.NetworkConfig,
			CreatedBy:     req.CreatedBy,
			Notes:         req.Notes,
		}

		if req.IsolationConfig != nil {
			e.IsolationConfig.DataIsolation = req.IsolationConfig.DataIsolation
			e.IsolationConfig.NetworkIsolation = req.IsolationConfig.NetworkIsolation
			if req.IsolationConfig.ResourceQuota.MaxCPUs > 0 {
				e.IsolationConfig.ResourceQuota.MaxCPUs = req.IsolationConfig.ResourceQuota.MaxCPUs
			}
			if req.IsolationConfig.ResourceQuota.MaxMemoryGB > 0 {
				e.IsolationConfig.ResourceQuota.MaxMemoryGB = req.IsolationConfig.ResourceQuota.MaxMemoryGB
			}
			if req.IsolationConfig.ResourceQuota.MaxStorageGB > 0 {
				e.IsolationConfig.ResourceQuota.MaxStorageGB = req.IsolationConfig.ResourceQuota.MaxStorageGB
			}
			if req.IsolationConfig.ResourceQuota.MaxConnections > 0 {
				e.IsolationConfig.ResourceQuota.MaxConnections = req.IsolationConfig.ResourceQuota.MaxConnections
			}
			if req.IsolationConfig.ResourceQuota.MaxDeployments > 0 {
				e.IsolationConfig.ResourceQuota.MaxDeployments = req.IsolationConfig.ResourceQuota.MaxDeployments
			}
			if req.IsolationConfig.ResourceQuota.MaxAgents > 0 {
				e.IsolationConfig.ResourceQuota.MaxAgents = req.IsolationConfig.ResourceQuota.MaxAgents
			}
			e.IsolationConfig.BackupPolicy = store.EnvironmentBackupPolicy{
				Enabled:           req.IsolationConfig.BackupPolicy.Enabled,
				Frequency:         req.IsolationConfig.BackupPolicy.Frequency,
				RetentionDays:     req.IsolationConfig.BackupPolicy.RetentionDays,
				EncryptionEnabled: req.IsolationConfig.BackupPolicy.EncryptionEnabled,
				BackupRegion:      req.IsolationConfig.BackupPolicy.BackupRegion,
			}
			if req.IsolationConfig.ComplianceLevel != "" {
				e.IsolationConfig.ComplianceLevel = req.IsolationConfig.ComplianceLevel
			}
			if req.IsolationConfig.Metadata != nil {
				e.IsolationConfig.Metadata = req.IsolationConfig.Metadata
			}
		}

		if e.IsolationLevel == "" {
			e.IsolationLevel = store.IsolationLevelLogical
		}

		created, err := h.EnvironmentStore.Create(e)
		if err != nil {
				h.WriteError(w, http.StatusInternalServerError, 500, "internal error", err.Error())
			return
		}

		w.WriteHeader(http.StatusCreated)
		h.WriteJSON(w, http.StatusCreated, environmentToResponse(created))
	}
}

// GetEnvironment handles GET /v1/tenants/{id}/environments/{env_id}.
func GetEnvironment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		envID, ok := extractPathParam(r, "env_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "env_id is required")
			return
		}

		e, err := h.EnvironmentStore.GetByIDAndTenant(envID, tenantID)
		if err != nil {
				h.WriteError(w, http.StatusNotFound, 404, "environment not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, environmentToResponse(e))
	}
}

// PatchEnvironment handles PATCH /v1/tenants/{id}/environments/{env_id}.
func PatchEnvironment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envID, ok := extractPathParam(r, "env_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "env_id is required")
			return
		}

		var req EnvironmentPatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "Invalid request body")
			return
		}

		storeReq := store.EnvironmentPatchRequest{
			Name:  req.Name,
			Notes: req.Notes,
		}
		if req.IsolationLevel != "" {
			storeReq.IsolationLevel = store.EnvironmentIsolationLevel(req.IsolationLevel)
		}
		if req.IsolationConfig != nil {
			storeReq.IsolationConfig = store.EnvironmentIsolationConfig{
				DataIsolation:    req.IsolationConfig.DataIsolation,
				NetworkIsolation: req.IsolationConfig.NetworkIsolation,
				ResourceQuota: store.EnvironmentResourceQuota{
					MaxCPUs:        req.IsolationConfig.ResourceQuota.MaxCPUs,
					MaxMemoryGB:    req.IsolationConfig.ResourceQuota.MaxMemoryGB,
					MaxStorageGB:   req.IsolationConfig.ResourceQuota.MaxStorageGB,
					MaxConnections: req.IsolationConfig.ResourceQuota.MaxConnections,
					MaxDeployments: req.IsolationConfig.ResourceQuota.MaxDeployments,
					MaxAgents:      req.IsolationConfig.ResourceQuota.MaxAgents,
				},
				BackupPolicy: store.EnvironmentBackupPolicy{
					Enabled:           req.IsolationConfig.BackupPolicy.Enabled,
					Frequency:         req.IsolationConfig.BackupPolicy.Frequency,
					RetentionDays:     req.IsolationConfig.BackupPolicy.RetentionDays,
					EncryptionEnabled: req.IsolationConfig.BackupPolicy.EncryptionEnabled,
					BackupRegion:      req.IsolationConfig.BackupPolicy.BackupRegion,
				},
				ComplianceLevel: req.IsolationConfig.ComplianceLevel,
				Metadata:        req.IsolationConfig.Metadata,
			}
		}

		e, err := h.EnvironmentStore.Patch(envID, storeReq)
		if err != nil {
				h.WriteError(w, http.StatusNotFound, 404, "environment not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, environmentToResponse(e))
	}
}

// DeleteEnvironment handles DELETE /v1/tenants/{id}/environments/{env_id}.
func DeleteEnvironment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envID, ok := extractPathParam(r, "env_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "env_id is required")
			return
		}

		if err := h.EnvironmentStore.Delete(envID); err != nil {
				h.WriteError(w, http.StatusNotFound, 404, "environment not found", err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ActivateEnvironment handles POST /v1/tenants/{id}/environments/{env_id}/activate.
func ActivateEnvironment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envID, ok := extractPathParam(r, "env_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "env_id is required")
			return
		}

		e, err := h.EnvironmentStore.Activate(envID)
		if err != nil {
				h.WriteError(w, http.StatusNotFound, 404, "environment not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, environmentToResponse(e))
	}
}

// DeactivateEnvironment handles POST /v1/tenants/{id}/environments/{env_id}/deactivate.
func DeactivateEnvironment(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envID, ok := extractPathParam(r, "env_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "env_id is required")
			return
		}

		e, err := h.EnvironmentStore.Deactivate(envID)
		if err != nil {
				h.WriteError(w, http.StatusNotFound, 404, "environment not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, environmentToResponse(e))
	}
}

// GetEnvironmentIsolationConfig handles GET /v1/tenants/{id}/environments/{env_id}/isolation-config.
func GetEnvironmentIsolationConfig(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		envID, ok := extractPathParam(r, "env_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "env_id is required")
			return
		}

		e, err := h.EnvironmentStore.GetByIDAndTenant(envID, tenantID)
		if err != nil {
				h.WriteError(w, http.StatusNotFound, 404, "environment not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, map[string]any{
			"isolation_level":     string(e.IsolationLevel),
			"isolation_config":    environmentIsolationConfigToMap(&e.IsolationConfig),
		})
	}
}

// UpdateEnvironmentIsolationConfig handles PUT /v1/tenants/{id}/environments/{env_id}/isolation-config.
func UpdateEnvironmentIsolationConfig(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envID, ok := extractPathParam(r, "env_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "env_id is required")
			return
		}

		var req EnvironmentIsolationConfigReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "Invalid request body")
			return
		}

		e, err := h.EnvironmentStore.Patch(envID, store.EnvironmentPatchRequest{
			IsolationConfig: store.EnvironmentIsolationConfig{
				DataIsolation:    req.DataIsolation,
				NetworkIsolation: req.NetworkIsolation,
				ResourceQuota: store.EnvironmentResourceQuota{
					MaxCPUs:        req.ResourceQuota.MaxCPUs,
					MaxMemoryGB:    req.ResourceQuota.MaxMemoryGB,
					MaxStorageGB:   req.ResourceQuota.MaxStorageGB,
					MaxConnections: req.ResourceQuota.MaxConnections,
					MaxDeployments: req.ResourceQuota.MaxDeployments,
					MaxAgents:      req.ResourceQuota.MaxAgents,
				},
				BackupPolicy: store.EnvironmentBackupPolicy{
					Enabled:           req.BackupPolicy.Enabled,
					Frequency:         req.BackupPolicy.Frequency,
					RetentionDays:     req.BackupPolicy.RetentionDays,
					EncryptionEnabled: req.BackupPolicy.EncryptionEnabled,
					BackupRegion:      req.BackupPolicy.BackupRegion,
				},
				ComplianceLevel: req.ComplianceLevel,
				Metadata:        req.Metadata,
			},
		})
		if err != nil {
				h.WriteError(w, http.StatusNotFound, 404, "environment not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, environmentIsolationConfigToMap(&e.IsolationConfig))
	}
}

// environmentToResponse converts a store.Environment to API response.
func environmentToResponse(e *store.Environment) *EnvironmentResponse {
	activatedAt := ""
	if e.ActivatedAt != nil {
		s := e.ActivatedAt.Format("2006-01-02T15:04:05Z")
		activatedAt = s
	}
	deactivatedAt := ""
	if e.DeactivatedAt != nil {
		s := e.DeactivatedAt.Format("2006-01-02T15:04:05Z")
		deactivatedAt = s
	}

	return &EnvironmentResponse{
		ID:             e.ID,
		TenantID:       e.TenantID,
		Name:           e.Name,
		Type:           string(e.Type),
		State:          string(e.State),
		IsolationLevel: string(e.IsolationLevel),
		IsolationConfig: EnvironmentIsolationConfigResponse{
			DataIsolation:    e.IsolationConfig.DataIsolation,
			NetworkIsolation: e.IsolationConfig.NetworkIsolation,
			ResourceQuota: EnvironmentResourceQuotaResp{
				MaxCPUs:        e.IsolationConfig.ResourceQuota.MaxCPUs,
				MaxMemoryGB:    e.IsolationConfig.ResourceQuota.MaxMemoryGB,
				MaxStorageGB:   e.IsolationConfig.ResourceQuota.MaxStorageGB,
				MaxConnections: e.IsolationConfig.ResourceQuota.MaxConnections,
				MaxDeployments: e.IsolationConfig.ResourceQuota.MaxDeployments,
				MaxAgents:      e.IsolationConfig.ResourceQuota.MaxAgents,
			},
			BackupPolicy: EnvironmentBackupPolicyResp{
				Enabled:           e.IsolationConfig.BackupPolicy.Enabled,
				Frequency:         e.IsolationConfig.BackupPolicy.Frequency,
				RetentionDays:     e.IsolationConfig.BackupPolicy.RetentionDays,
				EncryptionEnabled: e.IsolationConfig.BackupPolicy.EncryptionEnabled,
				BackupRegion:      e.IsolationConfig.BackupPolicy.BackupRegion,
			},
			ComplianceLevel: e.IsolationConfig.ComplianceLevel,
			Metadata:        e.IsolationConfig.Metadata,
		},
		ResourceIDs:   e.Resources,
		NetworkConfig: e.NetworkConfig,
		CreatedBy:     e.CreatedBy,
		Notes:         e.Notes,
		CreatedAt:     e.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     e.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		ActivatedAt:   &activatedAt,
		DeactivatedAt: &deactivatedAt,
	}
}

// environmentIsolationConfigToMap converts an EnvironmentIsolationConfig to a map.
func environmentIsolationConfigToMap(cfg *store.EnvironmentIsolationConfig) map[string]any {
	return map[string]any{
		"data_isolation":     cfg.DataIsolation,
		"network_isolation":  cfg.NetworkIsolation,
		"resource_quota": map[string]any{
			"max_cpus":         cfg.ResourceQuota.MaxCPUs,
			"max_memory_gb":    cfg.ResourceQuota.MaxMemoryGB,
			"max_storage_gb":   cfg.ResourceQuota.MaxStorageGB,
			"max_connections":  cfg.ResourceQuota.MaxConnections,
			"max_deployments":  cfg.ResourceQuota.MaxDeployments,
			"max_agents":       cfg.ResourceQuota.MaxAgents,
		},
		"backup_policy": map[string]any{
			"enabled":            cfg.BackupPolicy.Enabled,
			"frequency":          cfg.BackupPolicy.Frequency,
			"retention_days":     cfg.BackupPolicy.RetentionDays,
			"encryption_enabled": cfg.BackupPolicy.EncryptionEnabled,
			"backup_region":      cfg.BackupPolicy.BackupRegion,
		},
		"compliance_level": cfg.ComplianceLevel,
		"metadata":         cfg.Metadata,
	}
}
