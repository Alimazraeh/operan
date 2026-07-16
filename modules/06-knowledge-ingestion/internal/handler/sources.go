package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"context"

	"github.com/operan/modules/06-knowledge-ingestion/internal/ctxkeys"
	"github.com/operan/modules/06-knowledge-ingestion/internal/store"
)

// SourcesHandler handles ingestion source CRUD.
type SourcesHandler struct {
	store SourcesStore
}

// SourcesStore is the source persistence interface.
type SourcesStore interface {
	Create(ctx context.Context, src *store.IngestionSource) error
	GetByID(ctx context.Context, id string) (*store.IngestionSource, error)
	ListByTenant(ctx context.Context, tenantID string, sourceType *string, page, pageSize int) ([]store.IngestionSource, int, error)
	Update(ctx context.Context, src *store.IngestionSource) error
	Delete(ctx context.Context, id string) error
}

// NewSourcesHandler creates a new SourcesHandler.
func NewSourcesHandler(s SourcesStore) *SourcesHandler {
	return &SourcesHandler{store: s}
}

// ListSources handles GET /v1/sources
func (h *SourcesHandler) ListSources(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	sourceType := r.URL.Query().Get("source_type")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var st *string
	if sourceType != "" {
		st = &sourceType
	}

	sources, total, err := h.store.ListByTenant(r.Context(), tenantID, st, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sources: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"sources": sources,
		"page":    page,
		"page_size": pageSize,
		"total":   total,
	})
}

// CreateSource handles POST /v1/sources
func (h *SourcesHandler) CreateSource(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	var req struct {
		Name            string         `json:"name"`
		SourceType      string         `json:"source_type"`
		SourceURL       string         `json:"source_url"`
		FileType        string         `json:"file_type,omitempty"`
		NormalizeArabic bool           `json:"normalize_arabic,omitempty"`
		ChunkStrategy   string         `json:"chunk_strategy,omitempty"`
		ChunkSize       int            `json:"chunk_size,omitempty"`
		ChunkOverlap    int            `json:"chunk_overlap,omitempty"`
		Metadata        map[string]any `json:"metadata,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.SourceType == "" {
		writeError(w, http.StatusBadRequest, "source_type is required")
		return
	}
	if req.SourceURL == "" {
		writeError(w, http.StatusBadRequest, "source_url is required")
		return
	}

	src := &store.IngestionSource{
		TenantID:        tenantID,
		Name:            req.Name,
		SourceType:      req.SourceType,
		SourceURL:       req.SourceURL,
		FileType:        req.FileType,
		NormalizeArabic: req.NormalizeArabic,
		ChunkStrategy:   req.ChunkStrategy,
		ChunkSize:       req.ChunkSize,
		ChunkOverlap:    req.ChunkOverlap,
		Status:          "active",
		Metadata:        req.Metadata,
	}

	if src.ChunkStrategy == "" {
		src.ChunkStrategy = "adaptive"
	}
	if src.ChunkSize == 0 {
		src.ChunkSize = 512
	}
	if src.ChunkOverlap == 0 {
		src.ChunkOverlap = 50
	}

	if err := h.store.Create(r.Context(), src); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create source: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	WriteJSON(w, http.StatusCreated, src)
}

// GetSource handles GET /v1/sources/{id}
func (h *SourcesHandler) GetSource(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/sources/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing source id")
		return
	}

	src, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	WriteJSON(w, http.StatusOK, src)
}

// UpdateSource handles PATCH /v1/sources/{id}
func (h *SourcesHandler) UpdateSource(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/sources/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing source id")
		return
	}

	existing, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	var req struct {
		Name            string `json:"name,omitempty"`
		SourceType      string `json:"source_type,omitempty"`
		SourceURL       string `json:"source_url,omitempty"`
		FileType        string `json:"file_type,omitempty"`
		NormalizeArabic *bool  `json:"normalize_arabic,omitempty"`
		ChunkStrategy   string `json:"chunk_strategy,omitempty"`
		ChunkSize       int    `json:"chunk_size,omitempty"`
		ChunkOverlap    int    `json:"chunk_overlap,omitempty"`
		Status          string `json:"status,omitempty"`
		Metadata        map[string]any `json:"metadata,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.SourceType != "" {
		existing.SourceType = req.SourceType
	}
	if req.SourceURL != "" {
		existing.SourceURL = req.SourceURL
	}
	if req.FileType != "" {
		existing.FileType = req.FileType
	}
	if req.NormalizeArabic != nil {
		existing.NormalizeArabic = *req.NormalizeArabic
	}
	if req.ChunkStrategy != "" {
		existing.ChunkStrategy = req.ChunkStrategy
	}
	if req.ChunkSize > 0 {
		existing.ChunkSize = req.ChunkSize
	}
	if req.ChunkOverlap > 0 {
		existing.ChunkOverlap = req.ChunkOverlap
	}
	if req.Status != "" {
		existing.Status = req.Status
	}
	if req.Metadata != nil {
		existing.Metadata = req.Metadata
	}

	if err := h.store.Update(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update source: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, existing)
}

// DeleteSource handles DELETE /v1/sources/{id}
func (h *SourcesHandler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/sources/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing source id")
		return
	}

	if err := h.store.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}