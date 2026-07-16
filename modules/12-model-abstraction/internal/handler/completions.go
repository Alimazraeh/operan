package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/operan/model-abstraction/internal/adapters"
	"github.com/operan/model-abstraction/internal/config"
	"github.com/operan/model-abstraction/internal/ctxkeys"
	"github.com/operan/model-abstraction/internal/events"
	"github.com/operan/model-abstraction/internal/store"
)

// ModelRegistryStore provides model registry operations.
type ModelRegistryStore interface {
	GetByName(ctx context.Context, tenantID, modelName string) (*store.ModelRegistry, error)
	Create(ctx context.Context, model *store.ModelRegistry) error
	GetByID(ctx context.Context, id string) (*store.ModelRegistry, error)
	ListByTenant(ctx context.Context, tenantID string, providerID *string, page, pageSize int) ([]store.ModelRegistry, int, error)
	Update(ctx context.Context, model *store.ModelRegistry) error
	SetDefault(ctx context.Context, tenantID, modelID string) error
}

// ModelProvidersStore provides provider operations.
type ModelProvidersStore interface {
	Create(ctx context.Context, p *store.ModelProvider) error
	GetByID(ctx context.Context, id string) (*store.ModelProvider, error)
	ListByTenant(ctx context.Context, tenantID string, page, pageSize int) ([]store.ModelProvider, int, error)
	SoftDelete(ctx context.Context, id string) error
	Update(ctx context.Context, p *store.ModelProvider) error
	ActiveByTenant(ctx context.Context, tenantID string) ([]store.ModelProvider, error)
}

// ModelCallsStore provides call recording.
type ModelCallsStore interface {
	Create(ctx context.Context, call *store.ModelCall) error
}

// CompletionHandler handles POST /v1/models/completions.
type CompletionHandler struct {
	registry   ModelRegistryStore
	providers  ModelProvidersStore
	cfg        *config.Config
	broker     *events.Broker
	callsStore ModelCallsStore
	logger     *log.Logger
}

// NewCompletionHandler creates a new completion handler.
func NewCompletionHandler(registry ModelRegistryStore, providers ModelProvidersStore,
	cfg *config.Config, broker *events.Broker, callsStore ModelCallsStore, logger *log.Logger) *CompletionHandler {
	return &CompletionHandler{
		registry:   registry,
		providers:  providers,
		cfg:        cfg,
		broker:     broker,
		callsStore: callsStore,
		logger:     logger,
	}
}

// POST handles chat completion requests.
func (h *CompletionHandler) POST(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := r.Context().Value(ctxkeys.TenantIDKey).(string)
	agentID, _ := r.Context().Value(ctxkeys.UserIDKey).(string)
	workflowID := r.Header.Get("X-Workflow-ID")

	var req adapters.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Model == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model is required"})
		return
	}

	modelEntry, err := h.registry.GetByName(r.Context(), tenantID, req.Model)
	if err != nil {
		if errors.Is(err, store.ErrNoRows) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("model %q not found", req.Model)})
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error resolving model"})
		}
		return
	}

	provider, err := h.providers.GetByID(r.Context(), modelEntry.ProviderID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error resolving provider"})
		return
	}

	startTime := time.Now()
	adapter, err := buildProviderAdapter(provider, h.cfg)
	if err != nil {
		h.logCall(tenantID, agentID, workflowID, req.Model, provider.ID, 0, 0, time.Since(startTime).Milliseconds(), "error", err.Error())
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("provider not configured: %s", err.Error())})
		return
	}

	resp, err := adapter.Chat(r.Context(), req)
	if err != nil {
		h.logger.Printf("provider chat error: %v", err)
		resp, err = h.failover(r.Context(), tenantID, req.Model, provider, req)
		if err != nil {
			h.logCall(tenantID, agentID, workflowID, req.Model, provider.ID, 0, 0, time.Since(startTime).Milliseconds(), "failover", err.Error())
			h.broker.ModelFailoverPublished(r.Context(), tenantID, req.Model, provider.Name, "none", err.Error())
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("all providers failed: %s", err.Error())})
			return
		}
	}

	promptTokens := resp.Usage.PromptTokens
	completionTokens := resp.Usage.CompletionTokens
	costUSD := calcCost(modelEntry, promptTokens, completionTokens)

	h.logCall(tenantID, agentID, workflowID, req.Model, provider.ID, promptTokens, completionTokens, time.Since(startTime).Milliseconds(), "success", "")
	h.broker.ModelCallPublished(r.Context(), tenantID, agentID, workflowID, req.Model, provider.Name,
		promptTokens, completionTokens, int(time.Since(startTime).Milliseconds()), costUSD, "success")
	h.broker.ModelCostRecorded(r.Context(), tenantID, agentID, req.Model, costUSD, "llm-inference")

	writeJSON(w, http.StatusOK, resp)
}

func (h *CompletionHandler) failover(ctx context.Context, tenantID, modelName string, excludeProvider *store.ModelProvider, req adapters.ChatRequest) (*adapters.ChatResponse, error) {
	providers, err := h.providers.ActiveByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, p := range providers {
		if p.ID == excludeProvider.ID {
			continue
		}
		adapter, err := buildProviderAdapter(&p, h.cfg)
		if err != nil {
			continue
		}
		resp, err := adapter.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		h.logger.Printf("failover to %q failed: %v", p.Name, err)
	}
	return nil, fmt.Errorf("all failover providers exhausted")
}

func (h *CompletionHandler) logCall(tenantID, agentID, workflowID, modelName, providerID string,
	promptTokens, completionTokens int, latencyMs int64, status, errMsg string) {
	call := &store.ModelCall{
		TenantID:         tenantID,
		AgentID:          agentID,
		WorkflowID:       workflowID,
		ModelName:        modelName,
		ProviderID:       providerID,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		CostUSD:          0,
		Status:           status,
		ErrorMessage:     errMsg,
		LatencyMs:        int(latencyMs),
	}
	if h.callsStore != nil {
		if err := h.callsStore.Create(context.Background(), call); err != nil {
			h.logger.Printf("failed to record model call: %v", err)
		}
	}
}

// EmbeddingsHandler handles POST /v1/models/embeddings.
type EmbeddingsHandler struct {
	registry   ModelRegistryStore
	providers  ModelProvidersStore
	cfg        *config.Config
	callsStore ModelCallsStore
	logger     *log.Logger
}

// NewEmbeddingsHandler creates a new embeddings handler.
func NewEmbeddingsHandler(registry ModelRegistryStore, providers ModelProvidersStore,
	cfg *config.Config, callsStore ModelCallsStore, logger *log.Logger) *EmbeddingsHandler {
	return &EmbeddingsHandler{
		registry:   registry,
		providers:  providers,
		cfg:        cfg,
		callsStore: callsStore,
		logger:     logger,
	}
}

// POST handles embedding requests.
func (h *EmbeddingsHandler) POST(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := r.Context().Value(ctxkeys.TenantIDKey).(string)
	agentID, _ := r.Context().Value(ctxkeys.UserIDKey).(string)
	workflowID := r.Header.Get("X-Workflow-ID")

	var req adapters.EmbedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Model == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model is required"})
		return
	}

	modelEntry, err := h.registry.GetByName(r.Context(), tenantID, req.Model)
	if err != nil {
		if errors.Is(err, store.ErrNoRows) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("model %q not found", req.Model)})
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error resolving model"})
		}
		return
	}

	if !modelEntry.SupportsEmbed {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model does not support embeddings"})
		return
	}

	provider, err := h.providers.GetByID(r.Context(), modelEntry.ProviderID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error resolving provider"})
		return
	}

	adapter, err := buildProviderAdapter(provider, h.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("provider not configured: %s", err.Error())})
		return
	}

	startTime := time.Now()
	resp, err := adapter.Embeddings(r.Context(), req)
	latency := time.Since(startTime).Milliseconds()

	if err != nil {
		h.logger.Printf("embeddings error: %v", err)
		h.logEmbedCall(tenantID, agentID, workflowID, req.Model, provider.ID, 0, latency, "error", err.Error())
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "embedding request failed"})
		return
	}

	promptTokens := resp.Usage.PromptTokens
	costUSD := calcCost(modelEntry, promptTokens, 0)
	h.logEmbedCall(tenantID, agentID, workflowID, req.Model, provider.ID, promptTokens, latency, "success", "")
	_ = costUSD // cost tracked via model_calls for embeddings

	writeJSON(w, http.StatusOK, resp)
}

func (h *EmbeddingsHandler) logEmbedCall(tenantID, agentID, workflowID, modelName, providerID string,
	promptTokens int, latencyMs int64, status, errMsg string) {
	call := &store.ModelCall{
		TenantID:     tenantID,
		AgentID:      agentID,
		WorkflowID:   workflowID,
		ModelName:    modelName,
		ProviderID:   providerID,
		PromptTokens: promptTokens,
		TotalTokens:  promptTokens,
		Status:       status,
		ErrorMessage: errMsg,
		LatencyMs:    int(latencyMs),
	}
	if h.callsStore != nil {
		if err := h.callsStore.Create(context.Background(), call); err != nil {
			h.logger.Printf("failed to record embeddings call: %v", err)
		}
	}
}