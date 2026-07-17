package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/operan/modules/06-knowledge-ingestion/internal/chunker"
	"github.com/operan/modules/06-knowledge-ingestion/internal/clients"
	"github.com/operan/modules/06-knowledge-ingestion/internal/extract"
	"github.com/operan/modules/06-knowledge-ingestion/internal/events"
	"github.com/operan/modules/06-knowledge-ingestion/internal/store"
)

// JobStore provides job status updates for the worker.
type JobStore interface {
	GetByID(ctx context.Context, id string) (*store.IngestionJob, error)
	UpdateStatus(ctx context.Context, id string, status string, updates map[string]any) error
	ListPending(ctx context.Context) ([]*store.IngestionJob, error)
	ListStuck(ctx context.Context) ([]*store.IngestionJob, error)
}

// ResultsStore provides chunk result persistence for the worker.
type ResultsStore interface {
	Create(ctx context.Context, result *store.IngestionResult) error
	ExistsByHash(ctx context.Context, tenantID, chunkHash string) (bool, error)
}

// SourceStore provides source lookups for the worker.
type SourceStore interface {
	GetByID(ctx context.Context, id string) (*store.IngestionSource, error)
}

// Worker processes ingestion jobs asynchronously.
type Worker struct {
	sourcesStore SourceStore
	jobsStore    JobStore
	resultsStore ResultsStore
	m12Client    M12Client
	m07Client    M07Client
	m19Client    M19Client
	eventBroker  events.Broker
	chunker      chunker.Chunker
	serviceToken string
	logger       *log.Logger
	mu           sync.Mutex
	cancellers   map[string]context.CancelFunc
}

// NewWorker creates a new Worker.
func NewWorker(
	sourcesStore SourceStore,
	jobsStore JobStore,
	resultsStore ResultsStore,
	m12Client M12Client,
	m07Client M07Client,
	m19Client M19Client,
	broker events.Broker,
	chunker chunker.Chunker,
	serviceToken string,
	logger *log.Logger,
) *Worker {
	return &Worker{
		sourcesStore: sourcesStore,
		jobsStore:    jobsStore,
		resultsStore: resultsStore,
		m12Client:    m12Client,
		m07Client:    m07Client,
		m19Client:    m19Client,
		eventBroker:  broker,
		chunker:      chunker,
		serviceToken: serviceToken,
		logger:       logger,
		cancellers:   make(map[string]context.CancelFunc),
	}
}

// M12Client is the embedding client interface.
type M12Client interface {
	EmbedChunk(ctx context.Context, model, text, jwtToken string) ([]float64, int, error)
}

// M07Client is the vector store client interface.
type M07Client interface {
	StoreVectors(ctx context.Context, namespace string, vectors []clients.Vector) error
}

// M19Client is the normalization client interface.
type M19Client interface {
	NormalizeText(ctx context.Context, text string) (string, error)
}

// jobLogger returns a safe logger (no-op if nil).
func (w *Worker) jobLogger(msg string, args ...any) {
	if w.logger != nil {
		w.logger.Printf("worker: "+msg, args...)
	}
}

// Start scans for pending/stuck jobs and starts the recovery poller.
func (w *Worker) Start(ctx context.Context) {
	w.jobLogger("worker: scanning for pending jobs to recover...")

	// Recover pending/extracting/embedding jobs from DB.
	pendingJobs, err := w.jobsStore.ListPending(ctx)
	if err != nil {
		w.jobLogger("worker: failed to list pending jobs: %v", err)
	} else {
		for _, job := range pendingJobs {
			w.jobLogger("worker: recovering job %s (status=%s)", job.ID, job.Status)
			w.ProcessJob(ctx, job.ID, w.serviceToken)
		}
		w.jobLogger("worker: recovered %d jobs", len(pendingJobs))
	}

	// Start stuck-job poller in background.
	go w.pollStuckJobs(ctx)

	w.jobLogger("worker: started")
}

// pollStuckJobs periodically scans for jobs stuck for >30 minutes.
func (w *Worker) pollStuckJobs(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stuckJobs, err := w.jobsStore.ListStuck(ctx)
			if err != nil {
				w.jobLogger("worker: failed to list stuck jobs: %v", err)
				continue
			}
			for _, job := range stuckJobs {
				w.jobLogger("worker: requeueing stuck job %s", job.ID)
				w.ProcessJob(ctx, job.ID, w.serviceToken)
			}
		}
	}
}

// RegisterCancel registers a cancel function for a job.
func (w *Worker) RegisterCancel(jobID string, cancel context.CancelFunc) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cancellers[jobID] = cancel
}

// CancelJob cancels a running job by jobID.
func (w *Worker) CancelJob(jobID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	cancel, ok := w.cancellers[jobID]
	if !ok {
		return false
	}
	cancel()
	delete(w.cancellers, jobID)
	return true
}

// ProcessJob starts processing an ingestion job.
func (w *Worker) ProcessJob(ctx context.Context, jobID string, jwtToken string) {
	w.mu.Lock()
	jobCtx, jobCancel := context.WithCancel(ctx)
	w.cancellers[jobID] = jobCancel
	w.mu.Unlock()

	go func() {
		w.runJob(jobCtx, jobID, jwtToken)
		w.mu.Lock()
		delete(w.cancellers, jobID)
		w.mu.Unlock()
	}()
}

func (w *Worker) runJob(parentCtx context.Context, jobID, jwtToken string) {
	job, err := w.jobsStore.GetByID(parentCtx, jobID)
	if err != nil {
		w.jobLogger("worker job %s: get failed: %v", jobID, err)
		return
	}

	source, err := w.sourcesStore.GetByID(parentCtx, job.SourceID)
	if err != nil {
		w.failJob(parentCtx, jobID, "source lookup failed: "+err.Error())
		return
	}

	extractCtx, extractCancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer extractCancel()

	// Phase 1: Extract
	if err := w.jobsStore.UpdateStatus(extractCtx, jobID, "extracting", nil); err != nil {
		w.failJob(extractCtx, jobID, "update status: "+err.Error())
		return
	}

	result, err := w.extract(extractCtx, source)
	if err != nil {
		w.failJob(extractCtx, jobID, "extract failed: "+err.Error())
		return
	}

	// Phase 2: Normalize (optional)
	text := result.Text
	if source.NormalizeArabic && w.m19Client != nil {
		normalizeCtx, normalizeCancel := context.WithTimeout(parentCtx, 2*time.Minute)
		text, _ = w.m19Client.NormalizeText(normalizeCtx, text)
		normalizeCancel()
	}

	// Phase 3: Chunk
	chunkCtx, chunkCancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer chunkCancel()

	if err := w.jobsStore.UpdateStatus(chunkCtx, jobID, "chunking", nil); err != nil {
		w.failJob(chunkCtx, jobID, "update status: "+err.Error())
		return
	}

	chunks := w.chunker.Chunk(text, chunker.ChunkOptions{
		Strategy:   source.ChunkStrategy,
		ChunkSize:  source.ChunkSize,
		ChunkOverlap: source.ChunkOverlap,
	})

	if err := w.jobsStore.UpdateStatus(chunkCtx, jobID, "chunking", map[string]any{
		"total_chunks": len(chunks),
	}); err != nil {
		w.failJob(chunkCtx, jobID, "update status: "+err.Error())
		return
	}

	// Phase 4 & 5: Embed + Store + Record
	embedCtx, embedCancel := context.WithTimeout(parentCtx, 30*time.Minute)
	defer embedCancel()

	vectors := make([]clients.Vector, 0, len(chunks))

	for i, chunk := range chunks {
		select {
		case <-parentCtx.Done():
			w.failJob(parentCtx, jobID, "job cancelled")
			return
		default:
		}

		chunkText := strings.TrimSpace(chunk.Text)
		if chunkText == "" {
			continue
		}

		chunkHash := sha256.Sum256([]byte(chunkText))
		hashHex := hex.EncodeToString(chunkHash[:])

		dedup, err := w.resultsStore.ExistsByHash(parentCtx, job.TenantID, hashHex)
		if err != nil {
			w.jobLogger("worker job %s chunk %d: dedup check failed: %v", jobID, i, err)
			continue
		}
		if dedup {
			w.jobLogger("worker job %s chunk %d: duplicate chunk skipped", jobID, i)
			if err := w.jobsStore.UpdateStatus(parentCtx, jobID, "chunking", map[string]any{
				"processed_chunks": i + 1,
			}); err != nil {
				w.jobLogger("worker job %s: update processed_chunks failed: %v", jobID, err)
			}
			continue
		}

		// Embed
		vector, _, err := w.m12Client.EmbedChunk(embedCtx, "text-embedding-3-small", chunkText, jwtToken)
		if err != nil {
			w.jobLogger("worker job %s chunk %d: embed failed: %v", jobID, i, err)
			// Record as failed chunk
			errMsg := err.Error()
			_ = w.resultsStore.Create(embedCtx, &store.IngestionResult{
				TenantID:     job.TenantID,
				JobID:        jobID,
				SourceID:     source.ID,
				ChunkIndex:   i,
				ChunkHash:    hashHex,
				ChunkText:    chunkText,
				ChunkMetadata: map[string]any{
					"source":      source.Name,
					"language":    "auto",
					"word_count":  len(strings.Fields(chunkText)),
				},
				EmbeddingModel: "text-embedding-3-small",
				Status:         "failed",
				ErrorMessage:   errMsg,
			})
			continue
		}

		vectorDim := len(vector)
		vectorID := fmt.Sprintf("%s-%d", jobID, i)

		vectors = append(vectors, clients.Vector{
			ID: vectorID,
			Vector: vector,
			Metadata: map[string]any{
				"chunk_index": i,
				"job_id":      jobID,
				"source_id":   source.ID,
				"chunk_hash":  hashHex,
				"source":      source.Name,
				"language":    "auto",
				"word_count":  len(strings.Fields(chunkText)),
			},
		})

		// Record result
		chunkMeta := map[string]any{
			"source":     source.Name,
			"language":   "auto",
			"word_count": len(strings.Fields(chunkText)),
		}
		for k, v := range result.Meta {
			chunkMeta[k] = v
		}

		if err := w.resultsStore.Create(embedCtx, &store.IngestionResult{
			TenantID:       job.TenantID,
			JobID:           jobID,
			SourceID:        source.ID,
			ChunkIndex:      i,
			ChunkHash:       hashHex,
			ChunkText:       chunkText,
			ChunkMetadata:   chunkMeta,
			EmbeddingModel:  "text-embedding-3-small",
			VectorDim:       vectorDim,
			Status:          "pending",
		}); err != nil {
			w.jobLogger("worker job %s chunk %d: store result failed: %v", jobID, i, err)
		}

		// Publish event
		w.eventBroker.Publish(embedCtx, "operan.knowledge.chunk_created", []byte(jobID), []byte(fmt.Sprintf(`{"chunk_index":%d}`, i)))
		w.eventBroker.Publish(embedCtx, "operan.knowledge.embedding_stored", []byte(jobID), []byte(fmt.Sprintf(`{"chunk_index":%d,"model":"text-embedding-3-small","vector_dim":%d}`, i, vectorDim)))

		// Update processed count
		if err := w.jobsStore.UpdateStatus(embedCtx, jobID, "chunking", map[string]any{
			"processed_chunks": i + 1,
		}); err != nil {
			w.jobLogger("worker job %s: update processed_chunks failed: %v", jobID, err)
		}
	}

	// Phase 6: Store vectors in M07
	if len(vectors) > 0 {
		storeCtx, storeCancel := context.WithTimeout(parentCtx, 5*time.Minute)
		defer storeCancel()

		if err := w.m07Client.StoreVectors(storeCtx, source.Name, vectors); err != nil {
			w.jobLogger("worker job %s: store vectors failed: %v", jobID, err)
		}
	}

	// Mark complete
	completeCtx, completeCancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer completeCancel()

	now := time.Now()
	if err := w.jobsStore.UpdateStatus(completeCtx, jobID, "completed", map[string]any{
		"total_chunks":    len(chunks),
		"processed_chunks": len(chunks),
		"completed_at":     now,
	}); err != nil {
		w.jobLogger("worker job %s: mark completed failed: %v", jobID, err)
	}

	w.jobLogger("worker job %s: completed (%d chunks)", jobID, len(chunks))
}

func (w *Worker) extract(ctx context.Context, source *store.IngestionSource) (*extract.ExtractResult, error) {
	extSource := extract.Source{
		URL:      source.SourceURL,
		Filename: source.Name,
	}

	extractor := extract.NewExtractor(source.FileType)
	if extractor == nil {
		return nil, fmt.Errorf("no extractor for file type: %s", source.FileType)
	}

	result, err := extractor.Extract(ctx, extSource)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (w *Worker) failJob(ctx context.Context, jobID, errMsg string) {
	now := time.Now()
	_ = w.jobsStore.UpdateStatus(ctx, jobID, "failed", map[string]any{
		"error_message": errMsg,
		"completed_at":  now,
	})
	w.jobLogger("worker job %s: failed: %s", jobID, errMsg)
}