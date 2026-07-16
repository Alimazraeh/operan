package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/06-knowledge-ingestion/internal/ctxkeys"
	"github.com/operan/modules/06-knowledge-ingestion/internal/store"
)

// mockWorker is a mock worker for handler tests.
type mockWorker struct {
	calledJobs []string
}

func (m *mockWorker) ProcessJob(ctx context.Context, jobID string, serviceToken string) {
	m.calledJobs = append(m.calledJobs, jobID)
}

func (m *mockWorker) CancelJob(jobID string) bool {
	for _, id := range m.calledJobs {
		if id == jobID {
			return true
		}
	}
	return false
}

// mockJobsStore is a minimal mock for ingest tests.
type mockJobsStore struct {
	lastJob *store.IngestionJob
}

func (m *mockJobsStore) Create(ctx context.Context, job *store.IngestionJob) error {
	m.lastJob = job
	return nil
}

func (m *mockJobsStore) GetByID(ctx context.Context, id string) (*store.IngestionJob, error) {
	if m.lastJob == nil {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	return m.lastJob, nil
}

func (m *mockJobsStore) UpdateStatus(ctx context.Context, id string, status string, updates map[string]any) error {
	return nil
}

func (m *mockJobsStore) ListPending(ctx context.Context) ([]*store.IngestionJob, error) {
	return nil, nil
}

func (m *mockJobsStore) ListStuck(ctx context.Context) ([]*store.IngestionJob, error) {
	return nil, nil
}

func (m *mockJobsStore) ListByTenant(ctx context.Context, tenantID string, statusFilter *string, sourceID *string, page, pageSize int) ([]store.IngestionJob, int, error) {
	return nil, 0, nil
}

// mockSourceStore is a minimal mock for sources tests.
type mockSourceStore struct {
	lastCreated *store.IngestionSource
}

func (m *mockSourceStore) Create(ctx context.Context, src *store.IngestionSource) error {
	m.lastCreated = src
	return nil
}

func (m *mockSourceStore) GetByID(ctx context.Context, id string) (*store.IngestionSource, error) {
	return &store.IngestionSource{ID: id, Name: "mock"}, nil
}

func (m *mockSourceStore) ListByTenant(ctx context.Context, tenantID string, sourceType *string, page, pageSize int) ([]store.IngestionSource, int, error) {
	return nil, 0, nil
}

func (m *mockSourceStore) Update(ctx context.Context, src *store.IngestionSource) error {
	return nil
}

func (m *mockSourceStore) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockSourceStore) GetByHash(ctx context.Context, tenantID, fileHash string) (*store.IngestionSource, error) {
	return nil, nil
}

// TestIngestHandler creates valid job
func TestIngestHandler_IngestSource_Valid(t *testing.T) {
	js := &mockJobsStore{}
	w := &mockWorker{}
	h := NewIngestHandler(js, w, "service-token")

	body := strings.NewReader(`{"source_id": "src-123"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest", body)
	ctx := context.WithValue(req.Context(), ctxkeys.TenantIDKey, "tenant-abc")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.IngestSource(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["status"] != "pending" {
		t.Errorf("expected status pending, got %v", resp["status"])
	}
	if len(js.lastJob.SourceID) == 0 {
		t.Error("expected job to have source_id")
	}
	if len(w.calledJobs) != 1 {
		t.Errorf("expected worker to be called once, got %d", len(w.calledJobs))
	}
}

// TestIngestHandler creates 400 for missing source_id
func TestIngestHandler_IngestSource_MissingSourceID(t *testing.T) {
	js := &mockJobsStore{}
	w := &mockWorker{}
	h := NewIngestHandler(js, w, "service-token")

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest", body)
	ctx := context.WithValue(req.Context(), ctxkeys.TenantIDKey, "tenant-abc")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.IngestSource(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestIngestHandler creates 400 for invalid JSON
func TestIngestHandler_IngestSource_InvalidJSON(t *testing.T) {
	js := &mockJobsStore{}
	w := &mockWorker{}
	h := NewIngestHandler(js, w, "service-token")

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest", body)
	ctx := context.WithValue(req.Context(), ctxkeys.TenantIDKey, "tenant-abc")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.IngestSource(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ==================== CancelJob Tests ====================

// TestJobsHandler_CancelJob_Success tests cancel endpoint returns 200.
func TestJobsHandler_CancelJob_Success(t *testing.T) {
	js := &mockJobsStore{
		lastJob: &store.IngestionJob{
			ID:     "job-123",
			Status: "extracting",
		},
	}
	w := &mockWorker{calledJobs: []string{"job-123"}}
	h := NewJobsHandler(js, w)

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/job-123/cancel", nil)
	ctx := context.WithValue(req.Context(), ctxkeys.TenantIDKey, "tenant-abc")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.CancelJob(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestJobsHandler_CancelJob_NotFound tests cancel returns 404.
func TestJobsHandler_CancelJob_NotFound(t *testing.T) {
	js := &mockJobsStore{}
	w := &mockWorker{}
	h := NewJobsHandler(js, w)

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/nonexistent/cancel", nil)
	ctx := context.WithValue(req.Context(), ctxkeys.TenantIDKey, "tenant-abc")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.CancelJob(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestJobsHandler_CancelJob_AlreadyDone tests cancel returns 400 for completed job.
func TestJobsHandler_CancelJob_AlreadyDone(t *testing.T) {
	js := &mockJobsStore{
		lastJob: &store.IngestionJob{
			ID:     "job-done",
			Status: "completed",
		},
	}
	w := &mockWorker{}
	h := NewJobsHandler(js, w)

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/job-done/cancel", nil)
	ctx := context.WithValue(req.Context(), ctxkeys.TenantIDKey, "tenant-abc")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.CancelJob(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for completed job, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ==================== Sources CRUD Tests ====================

// TestSourcesHandler_CreateSource_Valid tests source creation with valid body.
func TestSourcesHandler_CreateSource_Valid(t *testing.T) {
	ss := &mockSourceStore{}
	h := NewSourcesHandler(ss)

	body := strings.NewReader(`{
		"name": "Test PDF",
		"source_type": "file",
		"source_url": "https://example.com/doc.pdf",
		"file_type": "pdf"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sources", body)
	ctx := context.WithValue(req.Context(), ctxkeys.TenantIDKey, "tenant-abc")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.CreateSource(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestSourcesHandler_CreateSource_MissingName tests 400 for missing name.
func TestSourcesHandler_CreateSource_MissingName(t *testing.T) {
	ss := &mockSourceStore{}
	h := NewSourcesHandler(ss)

	body := strings.NewReader(`{
		"source_type": "file",
		"source_url": "https://example.com/doc.pdf"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sources", body)
	ctx := context.WithValue(req.Context(), ctxkeys.TenantIDKey, "tenant-abc")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.CreateSource(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestSourcesHandler_CreateSource_MissingSourceType tests 400 for missing source_type.
func TestSourcesHandler_CreateSource_MissingSourceType(t *testing.T) {
	ss := &mockSourceStore{}
	h := NewSourcesHandler(ss)

	body := strings.NewReader(`{
		"name": "Test",
		"source_url": "https://example.com/doc.pdf"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sources", body)
	ctx := context.WithValue(req.Context(), ctxkeys.TenantIDKey, "tenant-abc")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.CreateSource(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestSourcesHandler_CreateSource_MissingSourceURL tests 400 for missing source_url.
func TestSourcesHandler_CreateSource_MissingSourceURL(t *testing.T) {
	ss := &mockSourceStore{}
	h := NewSourcesHandler(ss)

	body := strings.NewReader(`{
		"name": "Test",
		"source_type": "file"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sources", body)
	ctx := context.WithValue(req.Context(), ctxkeys.TenantIDKey, "tenant-abc")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.CreateSource(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}