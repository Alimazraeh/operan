package workers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/operan/modules/06-knowledge-ingestion/internal/chunker"
	"github.com/operan/modules/06-knowledge-ingestion/internal/clients"
	"github.com/operan/modules/06-knowledge-ingestion/internal/store"
)

// mockJobStore implements JobStore for tests.
type mockJobStore struct {
	mu       sync.Mutex
	jobs     map[string]*store.IngestionJob
	pending  []*store.IngestionJob
	stuck    []*store.IngestionJob
}

func newMockJobStore() *mockJobStore {
	return &mockJobStore{
		jobs:    make(map[string]*store.IngestionJob),
		pending: make([]*store.IngestionJob, 0),
		stuck:   make([]*store.IngestionJob, 0),
	}
}

func (m *mockJobStore) Create(job *store.IngestionJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
	return nil
}

func (m *mockJobStore) GetByID(ctx context.Context, id string) (*store.IngestionJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	// Return a copy so the caller does not hold a pointer to shared data.
	jobCopy := *job
	return &jobCopy, nil
}

func (m *mockJobStore) UpdateStatus(ctx context.Context, id string, status string, updates map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job, ok := m.jobs[id]; ok {
		job.Status = status
		if tc, ok := updates["total_chunks"]; ok {
			job.TotalChunks = tc.(int)
		}
		if pc, ok := updates["processed_chunks"]; ok {
			job.ProcessedChunks = pc.(int)
		}
	}
	return nil
}

func (m *mockJobStore) ListPending(ctx context.Context) ([]*store.IngestionJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pending := make([]*store.IngestionJob, len(m.pending))
	copy(pending, m.pending)
	return pending, nil
}

func (m *mockJobStore) ListStuck(ctx context.Context) ([]*store.IngestionJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stuck := make([]*store.IngestionJob, len(m.stuck))
	copy(stuck, m.stuck)
	return stuck, nil
}

func (m *mockJobStore) ListByTenant(ctx context.Context, tenantID string, statusFilter *string, sourceID *string, page, pageSize int) ([]store.IngestionJob, int, error) {
	return nil, 0, nil
}

// mockSourceStore implements SourceStore for tests.
type mockSourceStore struct {
	sources map[string]*store.IngestionSource
}

func newMockSourceStore() *mockSourceStore {
	return &mockSourceStore{
		sources: make(map[string]*store.IngestionSource),
	}
}

func (m *mockSourceStore) GetByID(ctx context.Context, id string) (*store.IngestionSource, error) {
	src, ok := m.sources[id]
	if !ok {
		return nil, fmt.Errorf("source not found: %s", id)
	}
	return src, nil
}

// mockResultsStore implements ResultsStore for tests.
type mockResultsStore struct {
	mu      sync.Mutex
	results []store.IngestionResult
	hashes  map[string]bool
}

func newMockResultsStore() *mockResultsStore {
	return &mockResultsStore{
		results: make([]store.IngestionResult, 0),
		hashes:  make(map[string]bool),
	}
}

func (m *mockResultsStore) Create(ctx context.Context, result *store.IngestionResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = append(m.results, *result)
	return nil
}

func (m *mockResultsStore) ExistsByHash(ctx context.Context, tenantID, chunkHash string) (bool, error) {
	return m.hashes[chunkHash], nil
}

func (m *mockResultsStore) UpdateStatus(ctx context.Context, resultID, status string, errMsg *string) error {
	return nil
}

func (m *mockResultsStore) GetByJobID(ctx context.Context, jobID string) ([]store.IngestionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.results, nil
}

// mockM12Client implements M12Client for tests.
type mockM12Client struct {
	embedCount int
	lastError  error
}

func (m *mockM12Client) EmbedChunk(ctx context.Context, model, text, jwtToken string) ([]float64, int, error) {
	m.embedCount++
	if m.lastError != nil {
		return nil, 0, m.lastError
	}
	return []float64{0.1, 0.2, 0.3}, 5, nil
}

// mockM07Client implements M07Client for tests.
type mockM07Client struct {
	mu       sync.Mutex
	storeCount int
	lastError  error
}

func (m *mockM07Client) StoreVectors(ctx context.Context, jwtToken, tenantID string, items []clients.VectorItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storeCount++
	if m.lastError != nil {
		return m.lastError
	}
	return nil
}

// mockM19Client implements M19Client for tests.
type mockM19Client struct {
	normalizeCount int
	lastError      error
}

func (m *mockM19Client) NormalizeText(ctx context.Context, text string) (string, error) {
	m.normalizeCount++
	if m.lastError != nil {
		return text, m.lastError
	}
	return "normalized_" + text, nil
}

// mockBroker implements events.Broker for tests.
type mockBroker struct {
	mu       sync.Mutex
	publishCount int
}

func (m *mockBroker) Publish(ctx context.Context, topic string, key, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishCount++
	return nil
}

func (m *mockBroker) Close() error {
	return nil
}

// newTestServer returns a local HTTP server that serves the given content.
func newTestServer(content string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
}

// waitForJob polls the job store until the job reaches a terminal status
// or the deadline is exceeded.
func waitForJob(t *testing.T, js *mockJobStore, jobID string, timeout time.Duration) (*store.IngestionJob, error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, err := js.GetByID(context.Background(), jobID)
		if err != nil {
			return nil, err
		}
		switch job.Status {
		case "completed", "failed", "cancelled":
			// Return a copy to avoid races when the caller reads the struct.
			jobCopy := *job
			return &jobCopy, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	job, err := js.GetByID(context.Background(), jobID)
	if err != nil {
		return nil, err
	}
	jobCopy := *job
	return &jobCopy, nil
}

// TestWorker_ProcessJob_ValidPipeline tests a full pipeline execution with a local test server.
func TestWorker_ProcessJob_ValidPipeline(t *testing.T) {
	// Start a local HTTP server that serves a known text file.
	server := newTestServer("This is a test document for ingestion. It has multiple sentences. Each sentence provides content for chunking and embedding.")
	defer server.Close()

	jobStore := newMockJobStore()
	sourceStore := newMockSourceStore()
	resultsStore := newMockResultsStore()
	m12Client := &mockM12Client{}
	m07Client := &mockM07Client{}
	m19Client := &mockM19Client{}
	broker := &mockBroker{}
	chunkerEngine := chunker.NewAdaptiveChunker()

	source := &store.IngestionSource{
		ID:            "src-123",
		Name:          "Test Document",
		SourceURL:     server.URL,
		FileType:      "txt",
		ChunkStrategy: "fixed",
		ChunkSize:     100,
	}
	sourceStore.sources[source.ID] = source

	job := &store.IngestionJob{
		ID:       "job-123",
		TenantID: "tenant-001",
		SourceID: source.ID,
		Status:   "pending",
	}
	jobStore.Create(job)

	worker := NewWorker(
		sourceStore,
		jobStore,
		resultsStore,
		m12Client,
		m07Client,
		m19Client,
		broker,
		chunkerEngine,
		"service-token",
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	worker.ProcessJob(ctx, job.ID, "service-token")

	// Wait for the job to complete.
	updatedJob, err := waitForJob(t, jobStore, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}
	if updatedJob.Status != "completed" {
		t.Errorf("expected job status completed, got %s", updatedJob.Status)
	}
	// M07 embeds server-side now — M12 must NOT be called by ingestion.
	if m12Client.embedCount != 0 {
		t.Error("M12 must not be called: M07 embeds content server-side")
	}
	if m07Client.storeCount == 0 {
		t.Error("expected M07 client to be called for vector storage")
	}
	resultsStore.mu.Lock()
	if len(resultsStore.results) == 0 {
		t.Error("expected at least one result to be stored")
	}
	resultsStore.mu.Unlock()
}

// TestWorker_ProcessJob_EmptyText tests a pipeline with empty extracted text.
func TestWorker_ProcessJob_EmptyText(t *testing.T) {
	server := newTestServer("")
	defer server.Close()

	jobStore := newMockJobStore()
	sourceStore := newMockSourceStore()
	resultsStore := newMockResultsStore()
	m12Client := &mockM12Client{}
	m07Client := &mockM07Client{}
	m19Client := &mockM19Client{}
	broker := &mockBroker{}
	chunkerEngine := chunker.NewAdaptiveChunker()

	source := &store.IngestionSource{
		ID:       "src-456",
		Name:     "Empty Document",
		SourceURL: server.URL,
		FileType: "txt",
	}
	sourceStore.sources[source.ID] = source

	job := &store.IngestionJob{
		ID:       "job-456",
		TenantID: "tenant-001",
		SourceID: source.ID,
		Status:   "pending",
	}
	jobStore.Create(job)

	worker := NewWorker(
		sourceStore,
		jobStore,
		resultsStore,
		m12Client,
		m07Client,
		m19Client,
		broker,
		chunkerEngine,
		"service-token",
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	worker.ProcessJob(ctx, job.ID, "service-token")

	// Job should complete even with empty text (no embeddings needed).
	updatedJob, err := waitForJob(t, jobStore, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}
	if updatedJob.Status != "completed" {
		t.Errorf("expected job status completed for empty text, got %s", updatedJob.Status)
	}
}

// TestWorker_ProcessJob_Normalization tests that M19 normalization is applied when enabled.
func TestWorker_ProcessJob_Normalization(t *testing.T) {
	server := newTestServer("Arabic نص test")
	defer server.Close()

	jobStore := newMockJobStore()
	sourceStore := newMockSourceStore()
	resultsStore := newMockResultsStore()
	m12Client := &mockM12Client{}
	m07Client := &mockM07Client{}
	m19Client := &mockM19Client{}
	broker := &mockBroker{}
	chunkerEngine := chunker.NewAdaptiveChunker()

	source := &store.IngestionSource{
		ID:            "src-789",
		Name:          "Arabic Doc",
		SourceURL:     server.URL,
		FileType:      "txt",
		NormalizeArabic: true,
		ChunkStrategy: "fixed",
		ChunkSize:     100,
	}
	sourceStore.sources[source.ID] = source

	job := &store.IngestionJob{
		ID:       "job-789",
		TenantID: "tenant-001",
		SourceID: source.ID,
		Status:   "pending",
	}
	jobStore.Create(job)

	worker := NewWorker(
		sourceStore,
		jobStore,
		resultsStore,
		m12Client,
		m07Client,
		m19Client,
		broker,
		chunkerEngine,
		"service-token",
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	worker.ProcessJob(ctx, job.ID, "service-token")

	updatedJob, err := waitForJob(t, jobStore, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}
	if updatedJob.Status != "completed" {
		t.Errorf("expected job status completed, got %s", updatedJob.Status)
	}
	if m19Client.normalizeCount == 0 {
		t.Error("expected M19 client to be called for normalization")
	}
}

// TestWorker_CancelJob tests job cancellation.
func TestWorker_CancelJob(t *testing.T) {
	worker := NewWorker(
		nil, nil, nil, nil, nil, nil, nil, nil, "service-token", nil)

	// Cancel a non-existent job returns false.
	if worker.CancelJob("non-existent") {
		t.Error("expected CancelJob to return false for non-existent job")
	}
}

// TestNewWorker_InitializesCancellers tests that the worker initializes properly.
func TestNewWorker_InitializesCancellers(t *testing.T) {
	worker := NewWorker(nil, nil, nil, nil, nil, nil, nil, nil, "service-token", nil)
	if worker == nil {
		t.Fatal("expected worker to be non-nil")
	}
}

// TestWorker_Start_RecoversPendingJobs tests that Start() recovers pending jobs.
func TestWorker_Start_RecoversPendingJobs(t *testing.T) {
	server := newTestServer("recover me")
	defer server.Close()

	jobStore := newMockJobStore()
	sourceStore := newMockSourceStore()
	resultsStore := newMockResultsStore()
	m12Client := &mockM12Client{}
	m07Client := &mockM07Client{}
	m19Client := &mockM19Client{}
	broker := &mockBroker{}
	chunkerEngine := chunker.NewAdaptiveChunker()

	source := &store.IngestionSource{
		ID:       "src-recover",
		Name:     "Recovery Test",
		SourceURL: server.URL,
		FileType: "txt",
	}
	sourceStore.sources[source.ID] = source

	// Pre-seed a job in "extracting" status (should be recovered).
	job := &store.IngestionJob{
		ID:       "job-recover",
		TenantID: "tenant-001",
		SourceID: source.ID,
		Status:   "extracting",
	}
	jobStore.Create(job)
	jobStore.pending = append(jobStore.pending, job)

	worker := NewWorker(
		sourceStore,
		jobStore,
		resultsStore,
		m12Client,
		m07Client,
		m19Client,
		broker,
		chunkerEngine,
		"service-token",
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	worker.Start(ctx)

	// Give the recovery goroutine time to finish.
	updatedJob, err := waitForJob(t, jobStore, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}
	if updatedJob.Status != "completed" {
		t.Errorf("expected recovered job to complete, got %s", updatedJob.Status)
	}
}

// TestWorker_ProcessJob_EmbeddingError tests that embed failures don't crash the pipeline.
func TestWorker_ProcessJob_EmbeddingError(t *testing.T) {
	server := newTestServer("test content")
	defer server.Close()

	jobStore := newMockJobStore()
	sourceStore := newMockSourceStore()
	resultsStore := newMockResultsStore()
	m12Client := &mockM12Client{lastError: fmt.Errorf("embedding service unavailable")}
	m07Client := &mockM07Client{}
	m19Client := &mockM19Client{}
	broker := &mockBroker{}
	chunkerEngine := chunker.NewAdaptiveChunker()

	source := &store.IngestionSource{
		ID:       "src-embed-fail",
		Name:     "Embed Fail Test",
		SourceURL: server.URL,
		FileType: "txt",
		ChunkStrategy: "fixed",
		ChunkSize: 100,
	}
	sourceStore.sources[source.ID] = source

	job := &store.IngestionJob{
		ID:       "job-embed-fail",
		TenantID: "tenant-001",
		SourceID: source.ID,
		Status:   "pending",
	}
	jobStore.Create(job)

	worker := NewWorker(
		sourceStore,
		jobStore,
		resultsStore,
		m12Client,
		m07Client,
		m19Client,
		broker,
		chunkerEngine,
		"service-token",
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	worker.ProcessJob(ctx, job.ID, "service-token")

	// Job should complete (chunks recorded as failed, but job itself succeeds).
	updatedJob, err := waitForJob(t, jobStore, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}
	// The pipeline records failed chunks but doesn't fail the whole job.
	if updatedJob.Status != "completed" {
		t.Errorf("expected job to complete with failed chunks, got %s", updatedJob.Status)
	}
	// Chunks always ship to M07 now (it embeds server-side), even when the
	// legacy embedding path would have failed.
	if m07Client.storeCount == 0 {
		t.Error("expected M07 client to be called")
	}
}