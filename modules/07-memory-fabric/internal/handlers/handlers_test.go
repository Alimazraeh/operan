package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/operan/modules/07-memory-fabric/internal/ctxkeys"
	"github.com/operan/modules/07-memory-fabric/internal/events"
	"github.com/operan/modules/07-memory-fabric/internal/store"
)

const testTenant = "11111111-1111-1111-1111-111111111111"

func testHandlers() (*MemoryHandlers, *http.ServeMux) {
	h := NewMemoryHandlers(store.NewVectorStore(), store.NewPolicyStore(), store.NewOperationStore(), events.NewPublisher(), 100, 1000)
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)
	return h, mux
}

// do performs a request with tenant/user context injected (as middleware would).
func do(mux *http.ServeMux, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	ctx := ctxkeys.WithTenantID(context.Background(), testTenant)
	ctx = ctxkeys.WithUserID(ctx, "user-1")
	ctx = ctxkeys.WithRequestID(ctx, "req-1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func ingestOne(t *testing.T, mux *http.ServeMux, content, embeddingType string, extra map[string]interface{}) string {
	t.Helper()
	item := map[string]interface{}{
		"document_id":      "22222222-2222-2222-2222-222222222222",
		"embedding_type":   embeddingType,
		"semantic_content": content,
	}
	for k, v := range extra {
		item[k] = v
	}
	w := do(mux, "POST", "/vectors", map[string]interface{}{
		"tenant_id": testTenant,
		"items":     []interface{}{item},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("ingest: status %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Ingested int `json:"ingested"`
	}
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.Ingested != 1 {
		t.Fatalf("ingested = %d, want 1: %s", res.Ingested, w.Body.String())
	}
	// Fetch the ID via list (newest first).
	lw := do(mux, "GET", "/vectors?page_size=1", nil)
	var list struct {
		Items []store.MemoryVector `json:"items"`
	}
	json.Unmarshal(lw.Body.Bytes(), &list)
	if len(list.Items) == 0 {
		t.Fatal("list returned no items after ingest")
	}
	return list.Items[0].ID
}

func TestIngestAndGetVector(t *testing.T) {
	_, mux := testHandlers()
	id := ingestOne(t, mux, "hello memory", "department", nil)

	w := do(mux, "GET", "/vectors/"+id, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: status %d", w.Code)
	}
	var v store.MemoryVector
	json.Unmarshal(w.Body.Bytes(), &v)
	if v.SemanticContent != "hello memory" || v.TenantID != testTenant {
		t.Errorf("vector = %+v", v)
	}
}

func TestIngestValidation(t *testing.T) {
	_, mux := testHandlers()

	// Empty items
	w := do(mux, "POST", "/vectors", map[string]interface{}{"tenant_id": testTenant, "items": []interface{}{}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty items: status %d, want 400", w.Code)
	}

	// Tenant mismatch
	w = do(mux, "POST", "/vectors", map[string]interface{}{"tenant_id": "other-tenant", "items": []interface{}{map[string]interface{}{"document_id": "d", "embedding_type": "platform", "semantic_content": "x"}}})
	if w.Code != http.StatusForbidden {
		t.Errorf("tenant mismatch: status %d, want 403", w.Code)
	}

	// Partial failure counts
	w = do(mux, "POST", "/vectors", map[string]interface{}{
		"tenant_id": testTenant,
		"items": []interface{}{
			map[string]interface{}{"document_id": "22222222-2222-2222-2222-222222222222", "embedding_type": "platform", "semantic_content": "ok"},
			map[string]interface{}{"document_id": "", "embedding_type": "platform", "semantic_content": "missing doc"},
		},
	})
	var res struct {
		Ingested int      `json:"ingested"`
		Failed   int      `json:"failed"`
		Errors   []string `json:"errors"`
	}
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.Ingested != 1 || res.Failed != 1 || len(res.Errors) != 1 {
		t.Errorf("partial failure: %+v", res)
	}
}

func TestListVectorsFiltersAndErrors(t *testing.T) {
	_, mux := testHandlers()
	ingestOne(t, mux, "alpha", "department", nil)
	ingestOne(t, mux, "beta", "platform", nil)

	w := do(mux, "GET", "/vectors?embedding_type=department", nil)
	var list struct {
		Items []store.MemoryVector `json:"items"`
		Total int                  `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Errorf("filtered total = %d, want 1", list.Total)
	}

	if w := do(mux, "GET", "/vectors?embedding_type=bogus", nil); w.Code != http.StatusBadRequest {
		t.Errorf("bad embedding_type: status %d, want 400", w.Code)
	}
	if w := do(mux, "GET", "/vectors?segment_type=bogus", nil); w.Code != http.StatusBadRequest {
		t.Errorf("bad segment_type: status %d, want 400", w.Code)
	}
}

func TestUpdateVector(t *testing.T) {
	_, mux := testHandlers()
	id := ingestOne(t, mux, "original", "department", nil)

	w := do(mux, "PUT", "/vectors/"+id, map[string]interface{}{
		"semantic_content": "revised",
		"segment_type":     "fact",
		"ttl":              time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: status %d: %s", w.Code, w.Body.String())
	}
	var v store.MemoryVector
	json.Unmarshal(w.Body.Bytes(), &v)
	if v.SemanticContent != "revised" || v.SegmentType != store.SegmentFact || v.TTL == nil {
		t.Errorf("updated vector = %+v", v)
	}

	// Explicit null clears TTL.
	req := httptest.NewRequest("PUT", "/vectors/"+id, bytes.NewBufferString(`{"ttl": null}`))
	ctx := ctxkeys.WithTenantID(context.Background(), testTenant)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req.WithContext(ctx))
	if rec.Code != http.StatusOK {
		t.Fatalf("ttl null update: status %d", rec.Code)
	}
	json.Unmarshal(rec.Body.Bytes(), &v)
	if v.TTL != nil {
		t.Error("ttl: null should clear the TTL")
	}

	if w := do(mux, "PUT", "/vectors/nonexistent", map[string]interface{}{"metadata": map[string]string{}}); w.Code != http.StatusNotFound {
		t.Errorf("update missing: status %d, want 404", w.Code)
	}
	if w := do(mux, "PUT", "/vectors/"+id, map[string]interface{}{"segment_type": "bogus"}); w.Code != http.StatusBadRequest {
		t.Errorf("bad segment_type: status %d, want 400", w.Code)
	}
}

func TestDeleteVector(t *testing.T) {
	_, mux := testHandlers()
	id := ingestOne(t, mux, "to be removed", "department", nil)

	if w := do(mux, "DELETE", "/vectors/"+id, nil); w.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d", w.Code)
	}
	if w := do(mux, "GET", "/vectors/"+id, nil); w.Code != http.StatusNotFound {
		t.Errorf("get after delete: status %d, want 404", w.Code)
	}
	if w := do(mux, "DELETE", "/vectors/"+id, nil); w.Code != http.StatusNotFound {
		t.Errorf("double delete: status %d, want 404", w.Code)
	}
}

func TestSearchMemory(t *testing.T) {
	_, mux := testHandlers()
	ingestOne(t, mux, "The customer demo runs on Friday afternoon", "department", nil)
	ingestOne(t, mux, "Completely unrelated telemetry noise", "department", nil)

	w := do(mux, "POST", "/search", map[string]interface{}{
		"tenant_id":           testTenant,
		"query":               "customer demo Friday",
		"embedding_type":      "department",
		"relevance_threshold": 0.5,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("search: status %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Items []map[string]interface{} `json:"items"`
		Total int                      `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.Total != 1 {
		t.Fatalf("search total = %d, want 1", res.Total)
	}
	item := res.Items[0]
	for _, f := range []string{"id", "vector_id", "score", "content", "rank", "source_type"} {
		if _, ok := item[f]; !ok {
			t.Errorf("result missing field %q", f)
		}
	}
}

func TestSearchValidation(t *testing.T) {
	_, mux := testHandlers()
	cases := []struct {
		body map[string]interface{}
		want int
	}{
		{map[string]interface{}{"embedding_type": "department"}, http.StatusBadRequest},                                              // no query
		{map[string]interface{}{"query": "x", "embedding_type": "bogus"}, http.StatusBadRequest},                                     // bad type
		{map[string]interface{}{"query": "x", "embedding_type": "department", "relevance_threshold": 2.0}, http.StatusBadRequest},    // bad threshold
		{map[string]interface{}{"query": "x", "embedding_type": "department", "tenant_id": "wrong"}, http.StatusForbidden},           // tenant mismatch
	}
	for i, c := range cases {
		if w := do(mux, "POST", "/search", c.body); w.Code != c.want {
			t.Errorf("case %d: status %d, want %d", i, w.Code, c.want)
		}
	}
}

func TestAgentMemory(t *testing.T) {
	_, mux := testHandlers()
	agentID := "33333333-3333-3333-3333-333333333333"
	ingestOne(t, mux, "remember this", "agent_personal", map[string]interface{}{"metadata": map[string]interface{}{"agent_id": agentID}})

	w := do(mux, "GET", "/agents/"+agentID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("agent memory: status %d: %s", w.Code, w.Body.String())
	}
	var mem store.AgentMemory
	json.Unmarshal(w.Body.Bytes(), &mem)
	if mem.AgentID != agentID || len(mem.PersonalMemories) != 1 || mem.EphemeralWindow == nil || mem.Status != "active" {
		t.Errorf("agent memory = %+v", mem)
	}

	if w := do(mux, "GET", "/agents/unknown-agent", nil); w.Code != http.StatusNotFound {
		t.Errorf("unknown agent: status %d, want 404", w.Code)
	}
}

func TestGC(t *testing.T) {
	_, mux := testHandlers()
	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	ingestOne(t, mux, "expired memory", "agent_ephemeral", map[string]interface{}{"ttl": past})
	ingestOne(t, mux, "fresh memory", "department", nil)

	// Dry run reports but keeps the vector.
	w := do(mux, "POST", "/gc", map[string]interface{}{"dry_run": true})
	if w.Code != http.StatusAccepted {
		t.Fatalf("gc dry run: status %d: %s", w.Code, w.Body.String())
	}
	var op store.OperationStatus
	json.Unmarshal(w.Body.Bytes(), &op)
	if op.Status != "completed" || op.BatchSize != 1 {
		t.Errorf("dry run op = %+v", op)
	}

	// Real run deletes it.
	w = do(mux, "POST", "/gc", nil)
	json.Unmarshal(w.Body.Bytes(), &op)
	if op.BatchSize != 1 {
		t.Errorf("gc op = %+v", op)
	}
	lw := do(mux, "GET", "/vectors", nil)
	var list struct {
		Total int `json:"total"`
	}
	json.Unmarshal(lw.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Errorf("after gc total = %d, want 1", list.Total)
	}

	if w := do(mux, "POST", "/gc", map[string]interface{}{"memory_type": "bogus"}); w.Code != http.StatusBadRequest {
		t.Errorf("bad memory_type: status %d, want 400", w.Code)
	}
}

func TestRetentionPolicies(t *testing.T) {
	_, mux := testHandlers()

	w := do(mux, "POST", "/retention-policies", map[string]interface{}{
		"memory_type":     "ephemeral",
		"ttl_seconds":     3600,
		"auto_gc_enabled": true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create policy: status %d: %s", w.Code, w.Body.String())
	}
	var p store.RetentionPolicy
	json.Unmarshal(w.Body.Bytes(), &p)
	if p.ID == "" || p.TenantID != testTenant || p.MemoryType != store.MemoryEphemeral {
		t.Errorf("policy = %+v", p)
	}

	lw := do(mux, "GET", "/retention-policies", nil)
	var list struct {
		Items []store.RetentionPolicy `json:"items"`
		Total int                     `json:"total"`
	}
	json.Unmarshal(lw.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Errorf("list total = %d, want 1", list.Total)
	}

	if w := do(mux, "POST", "/retention-policies", map[string]interface{}{"memory_type": "bogus"}); w.Code != http.StatusBadRequest {
		t.Errorf("bad memory_type: status %d, want 400", w.Code)
	}
	if w := do(mux, "POST", "/retention-policies", map[string]interface{}{"memory_type": "personal", "tenant_id": "wrong"}); w.Code != http.StatusForbidden {
		t.Errorf("tenant mismatch: status %d, want 403", w.Code)
	}
	if w := do(mux, "POST", "/retention-policies", map[string]interface{}{"memory_type": "personal", "ttl_seconds": -1}); w.Code != http.StatusBadRequest {
		t.Errorf("negative ttl: status %d, want 400", w.Code)
	}
}

func TestErrorSchemaShape(t *testing.T) {
	_, mux := testHandlers()
	w := do(mux, "GET", "/vectors/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d", w.Code)
	}
	var e map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &e)
	for _, f := range []string{"code", "message", "request_id"} {
		if _, ok := e[f]; !ok {
			t.Errorf("error missing contract field %q: %s", f, w.Body.String())
		}
	}
	if fmt.Sprintf("%v", e["code"]) != "404" {
		t.Errorf("code = %v, want 404", e["code"])
	}
}

func TestIngestPublishesEvent(t *testing.T) {
	h, mux := testHandlers()
	captured := &captureBroker{}
	h.Publisher.SetBroker(captured)

	ingestOne(t, mux, "event check", "department", nil)
	if len(captured.topics) == 0 || captured.topics[0] != "operan.memory.vector.ingested" {
		t.Errorf("topics = %v", captured.topics)
	}
}

type captureBroker struct {
	mu     sync.Mutex
	topics []string
}

func (c *captureBroker) Publish(_ context.Context, topic string, _, _ []byte, _ map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.topics = append(c.topics, topic)
	return nil
}

func (c *captureBroker) Close() error { return nil }

// fakeEmbedder maps known texts to fixed vectors so cosine ranking is exact.
type fakeEmbedder struct{ vectors map[string][]float64 }

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i, t := range texts {
		v, ok := f.vectors[t]
		if !ok {
			v = []float64{0, 0, 1}
		}
		out[i] = v
	}
	return out, nil
}

func (f *fakeEmbedder) Model() string { return "fake-embedder" }

func TestSearchUsesEmbedderForRanking(t *testing.T) {
	h, mux := testHandlers()
	h.Embedder = &fakeEmbedder{vectors: map[string][]float64{
		"billing preferences for Acme": {1, 0, 0}, // query
		"Acme wants quarterly billing": {0.9, 0.1, 0},
		"The office plant needs water": {0, 1, 0},
	}}

	// Ingest two memories without vectors — the embedder vectorizes them.
	ingestOne(t, mux, "Acme wants quarterly billing", "department", nil)
	ingestOne(t, mux, "The office plant needs water", "department", nil)

	// Stored vectors carry the embedder's model name.
	lw := do(mux, "GET", "/vectors?page_size=10", nil)
	var list struct {
		Items []store.MemoryVector `json:"items"`
	}
	json.Unmarshal(lw.Body.Bytes(), &list)
	for _, v := range list.Items {
		if v.EmbeddingModel != "fake-embedder" || len(v.EmbeddingVector) != 3 {
			t.Errorf("vector not embedded: model=%q dims=%d", v.EmbeddingModel, len(v.EmbeddingVector))
		}
	}

	// Token overlap would score ~0 here ("billing" is the only shared token
	// with one memory) — cosine on embeddings must rank the billing memory
	// on top with a high score.
	w := do(mux, "POST", "/search", map[string]interface{}{
		"query":               "billing preferences for Acme",
		"embedding_type":      "department",
		"relevance_threshold": 0.8,
	})
	var res struct {
		Items []map[string]interface{} `json:"items"`
		Total int                      `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.Total != 1 {
		t.Fatalf("search total = %d, want 1 (cosine above 0.8)", res.Total)
	}
	if res.Items[0]["content"] != "Acme wants quarterly billing" {
		t.Errorf("top hit = %v", res.Items[0]["content"])
	}
}
