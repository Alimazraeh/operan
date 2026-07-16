package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/model-abstraction/internal/config"
	"github.com/operan/model-abstraction/internal/ctxkeys"
	"github.com/operan/model-abstraction/internal/store"
)

// mockRegistryStore satisfies the ModelRegistryStore interface.
type mockRegistryStore struct{}

func (m *mockRegistryStore) Create(ctx context.Context, model *store.ModelRegistry) error {
	model.ID = "m1"
	return nil
}
func (m *mockRegistryStore) GetByID(ctx context.Context, id string) (*store.ModelRegistry, error) {
	if id == "not-found" {
		return nil, errors.New("no rows in result set")
	}
	return &store.ModelRegistry{ID: id, TenantID: "tenant-001", ModelName: "gpt-4"}, nil
}
func (m *mockRegistryStore) GetByName(ctx context.Context, tenantID, modelName string) (*store.ModelRegistry, error) {
	if modelName == "not-found" {
		return nil, store.ErrNoRows
	}
	return &store.ModelRegistry{
		ID:            "m1",
		TenantID:      tenantID,
		ModelName:     modelName,
		ProviderID:    "p1",
		SupportsChat:  true,
		SupportsEmbed: true,
		CostPerToken:  map[string]any{"prompt": 0.00001, "completion": 0.00002},
	}, nil
}
func (m *mockRegistryStore) ListByTenant(ctx context.Context, tenantID string, providerID *string, page, pageSize int) ([]store.ModelRegistry, int, error) {
	return nil, 0, nil
}
func (m *mockRegistryStore) Update(ctx context.Context, model *store.ModelRegistry) error { return nil }
func (m *mockRegistryStore) SetDefault(ctx context.Context, tenantID, modelID string) error {
	return nil
}

// mockProviderStore satisfies the ModelProvidersStore interface.
type mockProviderStore struct{}

func (m *mockProviderStore) Create(ctx context.Context, p *store.ModelProvider) error { return nil }
func (m *mockProviderStore) GetByID(ctx context.Context, id string) (*store.ModelProvider, error) {
	if id == "not-found" {
		return nil, errors.New("no rows in result set")
	}
	return &store.ModelProvider{ID: id, Name: "openai", Type: "openai", BaseURL: "https://api.openai.com"}, nil
}
func (m *mockProviderStore) ListByTenant(ctx context.Context, tenantID string, page, pageSize int) ([]store.ModelProvider, int, error) {
	return nil, 0, nil
}
func (m *mockProviderStore) SoftDelete(ctx context.Context, id string) error   { return nil }
func (m *mockProviderStore) Update(ctx context.Context, p *store.ModelProvider) error { return nil }
func (m *mockProviderStore) ActiveByTenant(ctx context.Context, tenantID string) ([]store.ModelProvider, error) {
	return nil, nil
}

// mockCallsStore satisfies the ModelCallsStore interface.
type mockCallsStore struct{}

func (m *mockCallsStore) Create(ctx context.Context, call *store.ModelCall) error { return nil }

func newTestConfig() *config.Config {
	return &config.Config{
		JWTSecret: "test-secret",
		ProviderAPIKeys: map[string]string{
			"openai":    "sk-test-openai",
			"anthropic": "sk-ant-test",
			"ollama":    "",
			"litellm":   "sk-test-litellm",
		},
	}
}

func withTenant(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxkeys.TenantIDKey, "tenant-001"))
}

func withCtx(r *http.Request) context.Context {
	return context.WithValue(r.Context(), ctxkeys.TenantIDKey, "tenant-001")
}

func jsonBody(s string) *strings.Reader {
	return strings.NewReader(s)
}

// === Completions Tests ===

func TestCompletionHandler_POST_InvalidBody(t *testing.T) {
	h := NewCompletionHandler(&mockRegistryStore{}, &mockProviderStore{}, newTestConfig(), nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/models/completions", jsonBody(`not json`))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.POST(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCompletionHandler_POST_EmptyModel(t *testing.T) {
	h := NewCompletionHandler(&mockRegistryStore{}, &mockProviderStore{}, newTestConfig(), nil, nil, nil)
	body, _ := json.Marshal(map[string]any{
		"model":    "",
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/models/completions", strings.NewReader(string(body)))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.POST(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCompletionHandler_POST_ModelNotFound(t *testing.T) {
	h := NewCompletionHandler(&mockRegistryStore{}, &mockProviderStore{}, newTestConfig(), nil, nil, nil)
	body, _ := json.Marshal(map[string]any{
		"model":    "not-found",
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/models/completions", strings.NewReader(string(body)))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.POST(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// === Embeddings Tests ===

func TestEmbeddingsHandler_POST_EmptyModel(t *testing.T) {
	h := NewEmbeddingsHandler(&mockRegistryStore{}, &mockProviderStore{}, newTestConfig(), nil, nil)
	body, _ := json.Marshal(map[string]any{"input": []string{"hello"}})
	req := httptest.NewRequest(http.MethodPost, "/v1/models/embeddings", strings.NewReader(string(body)))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.POST(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestEmbeddingsHandler_POST_InvalidBody(t *testing.T) {
	h := NewEmbeddingsHandler(&mockRegistryStore{}, &mockProviderStore{}, newTestConfig(), nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/models/embeddings", jsonBody(`not json`))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.POST(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestEmbeddingsHandler_POST_ModelNotFound(t *testing.T) {
	h := NewEmbeddingsHandler(&mockRegistryStore{}, &mockProviderStore{}, newTestConfig(), nil, nil)
	body, _ := json.Marshal(map[string]any{"model": "not-found", "input": []string{"hello"}})
	req := httptest.NewRequest(http.MethodPost, "/v1/models/embeddings", strings.NewReader(string(body)))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.POST(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// === Providers Tests ===

func TestProvidersHandler_HandlePOST(t *testing.T) {
	h := NewProvidersHandler(&mockProviderStore{}, newTestConfig())
	body := `{"name":"openai-test","type":"openai","base_url":"https://api.openai.com"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/model-providers", jsonBody(body))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.HandlePOST(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProvidersHandler_HandlePOST_InvalidType(t *testing.T) {
	h := NewProvidersHandler(&mockProviderStore{}, newTestConfig())
	body := `{"name":"test","type":"invalid","base_url":"https://api.test.com"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/model-providers", jsonBody(body))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.HandlePOST(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid type, got %d", w.Code)
	}
}

func TestProvidersHandler_HandlePOST_MissingFields(t *testing.T) {
	h := NewProvidersHandler(&mockProviderStore{}, newTestConfig())
	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/model-providers", jsonBody(body))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.HandlePOST(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", w.Code)
	}
}

func TestProvidersHandler_HandleGET(t *testing.T) {
	h := NewProvidersHandler(&mockProviderStore{}, newTestConfig())
	req := httptest.NewRequest(http.MethodGet, "/v1/model-providers?page=1&page_size=10", nil)
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.HandleGET(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProvidersHandler_HandleDELETE(t *testing.T) {
	h := NewProvidersHandler(&mockProviderStore{}, newTestConfig())
	req := httptest.NewRequest(http.MethodDelete, "/v1/model-providers/p1", nil)
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.HandleDELETE(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestProvidersHandler_HandlePATCH(t *testing.T) {
	h := NewProvidersHandler(&mockProviderStore{}, newTestConfig())
	body := `{"name":"openai-updated","priority":90}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/model-providers/p1", jsonBody(body))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.HandlePATCH(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProvidersHandler_HandlePATCH_NotFound(t *testing.T) {
	h := NewProvidersHandler(&mockProviderStore{}, newTestConfig())
	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/model-providers/not-found", jsonBody(body))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.HandlePATCH(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// === Models Tests ===

func TestModelsHandler_HandlePOST(t *testing.T) {
	h := NewModelsHandler(&mockRegistryStore{}, &mockProviderStore{})
	body := `{"model_name":"gpt-4","provider_id":"p1","supports_chat":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/model-registry", jsonBody(body))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.HandlePOST(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestModelsHandler_HandlePOST_InvalidProvider(t *testing.T) {
	// Provider "nonexistent" doesn't exist in our mock, so it returns 400.
	h := NewModelsHandler(&mockRegistryStore{}, &mockProviderStore{})
	body := `{"model_name":"gpt-4","provider_id":"not-found"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/model-registry", jsonBody(body))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.HandlePOST(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestModelsHandler_HandleGET(t *testing.T) {
	h := NewModelsHandler(&mockRegistryStore{}, &mockProviderStore{})
	req := httptest.NewRequest(http.MethodGet, "/v1/model-registry", nil)
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.HandleGET(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestModelsHandler_HandlePATCH(t *testing.T) {
	h := NewModelsHandler(&mockRegistryStore{}, &mockProviderStore{})
	body := `{"max_tokens":16384}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/model-registry/m1", jsonBody(body))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.HandlePATCH(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestModelsHandler_HandlePATCH_NotFound(t *testing.T) {
	h := NewModelsHandler(&mockRegistryStore{}, &mockProviderStore{})
	body := `{"max_tokens":16384}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/model-registry/not-found", jsonBody(body))
	req = withTenant(req)
	w := httptest.NewRecorder()
	h.HandlePATCH(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// === WriteJSON Tests ===

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusOK, map[string]string{"hello": "world"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestWriteJSON_NonOK(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusCreated, map[string]string{"id": "123"})

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestWriteJSON_Array(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusOK, []string{"a", "b", "c"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}