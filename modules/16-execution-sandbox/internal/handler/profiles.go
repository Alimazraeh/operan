package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/operan/execution-sandbox/internal/ctxkeys"
	"github.com/operan/execution-sandbox/internal/sandbox"
	"github.com/operan/execution-sandbox/internal/store"
)

// ProfileHandler handles sandbox profile HTTP requests.
type ProfileHandler struct {
	store *store.ProfileStore
}

// NewProfileHandler creates a new ProfileHandler.
func NewProfileHandler(store *store.ProfileStore) *ProfileHandler {
	return &ProfileHandler{store: store}
}

// ListProfiles handles GET /sandbox-profiles.
func (h *ProfileHandler) ListProfiles(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	profiles, total, err := h.store.List(r.Context(), tenantID, page, pageSize)
	if err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"profiles": profiles,
		"page":     page,
		"page_size": pageSize,
		"total":    total,
		"has_more": (page * pageSize) < total,
	})
}

// CreateProfile handles POST /sandbox-profiles.
func (h *ProfileHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	var req struct {
		Name            string   `json:"name"`
		Description     *string  `json:"description"`
		CPUCores        float64  `json:"cpu_cores"`
		MemoryMB        int      `json:"memory_mb"`
		TimeoutSeconds  int      `json:"timeout_seconds"`
		NetworkAccess   bool     `json:"network_access"`
		AllowedTools    []string `json:"allowed_tools"`
		FilesystemAccess bool    `json:"filesystem_access"`
		MaxFileSizeMB   int      `json:"max_file_size_mb"`
		MaxOutputSizeKB int      `json:"max_output_size_kb"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, `{"error":"bad-request","message":"name is required"}`, http.StatusBadRequest)
		return
	}
	if req.MemoryMB == 0 {
		req.MemoryMB = 256
	}
	if req.TimeoutSeconds == 0 {
		req.TimeoutSeconds = 60
	}
	if req.CPUCores == 0 {
		req.CPUCores = 0.5
	}
	if req.MaxFileSizeMB == 0 {
		req.MaxFileSizeMB = 50
	}
	if req.MaxOutputSizeKB == 0 {
		req.MaxOutputSizeKB = 1024
	}

	if err := sandbox.ValidateProfile(sandbox.SandboxProfile{Name: req.Name, MemoryMB: req.MemoryMB, TimeoutSeconds: req.TimeoutSeconds}); err != nil {
		http.Error(w, `{"error":"bad-request","message":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	profile := &store.SandboxProfile{
		TenantID:         tenantID,
		Name:             req.Name,
		Description:      req.Description,
		CPUCores:         req.CPUCores,
		MemoryMB:         req.MemoryMB,
		TimeoutSeconds:   req.TimeoutSeconds,
		NetworkAccess:    req.NetworkAccess,
		AllowedTools:     req.AllowedTools,
		FilesystemAccess: req.FilesystemAccess,
		MaxFileSizeMB:    req.MaxFileSizeMB,
		MaxOutputSizeKB:  req.MaxOutputSizeKB,
		IsActive:         true,
	}

	if err := h.store.Create(r.Context(), profile); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusCreated, profile)
}

// GetProfile handles GET /sandbox-profiles/{id}.
func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid profile ID"}`, http.StatusBadRequest)
		return
	}

	profile, err := h.store.GetByID(r.Context(), id)
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

	WriteJSON(w, http.StatusOK, profile)
}

// UpdateProfile handles PATCH /sandbox-profiles/{id}.
func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid profile ID"}`, http.StatusBadRequest)
		return
	}

	profile, err := h.store.GetByID(r.Context(), id)
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

	var req struct {
		Name            *string  `json:"name"`
		Description     *string  `json:"description"`
		CPUCores        *float64 `json:"cpu_cores"`
		MemoryMB        *int     `json:"memory_mb"`
		TimeoutSeconds  *int     `json:"timeout_seconds"`
		NetworkAccess   *bool    `json:"network_access"`
		AllowedTools    *[]string `json:"allowed_tools"`
		FilesystemAccess *bool   `json:"filesystem_access"`
		MaxFileSizeMB   *int     `json:"max_file_size_mb"`
		MaxOutputSizeKB *int     `json:"max_output_size_kb"`
		IsActive        *bool    `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if req.Name != nil {
		profile.Name = *req.Name
	}
	if req.Description != nil {
		profile.Description = req.Description
	}
	if req.CPUCores != nil {
		profile.CPUCores = *req.CPUCores
	}
	if req.MemoryMB != nil {
		profile.MemoryMB = *req.MemoryMB
	}
	if req.TimeoutSeconds != nil {
		profile.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.NetworkAccess != nil {
		profile.NetworkAccess = *req.NetworkAccess
	}
	if req.AllowedTools != nil {
		profile.AllowedTools = *req.AllowedTools
	}
	if req.FilesystemAccess != nil {
		profile.FilesystemAccess = *req.FilesystemAccess
	}
	if req.MaxFileSizeMB != nil {
		profile.MaxFileSizeMB = *req.MaxFileSizeMB
	}
	if req.MaxOutputSizeKB != nil {
		profile.MaxOutputSizeKB = *req.MaxOutputSizeKB
	}
	if req.IsActive != nil {
		profile.IsActive = *req.IsActive
	}

	if err := sandbox.ValidateProfile(sandbox.SandboxProfile{Name: profile.Name, MemoryMB: profile.MemoryMB, TimeoutSeconds: profile.TimeoutSeconds}); err != nil {
		http.Error(w, `{"error":"bad-request","message":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	if err := h.store.Update(r.Context(), profile); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, profile)
}

// DeleteProfile handles DELETE /sandbox-profiles/{id}.
func (h *ProfileHandler) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid profile ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.store.Delete(r.Context(), id, tenantID); err != nil {
		if err == store.ErrNotFound {
			http.Error(w, `{"error":"not-found","message":"profile not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}