package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// === M12 Client Tests ===

func TestM12Client_EmbedChunk_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method != http.MethodPost {
			t.Error("expected POST")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"embedding":[0.1,0.2,0.3],"token_count":3}`))
	}))
	defer server.Close()

	client := NewM12Client(server.URL, 0)
	vectors, tokens, err := client.EmbedChunk(context.Background(), "tenant-1", "text-embedding-3-small", "Hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vectors) != 3 {
		t.Errorf("expected 3 vector dims, got %d", len(vectors))
	}
	if tokens != 3 {
		t.Errorf("expected 3 tokens, got %d", tokens)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestM12Client_EmbedChunk_EmptyText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"embedding":[],"token_count":0}`))
	}))
	defer server.Close()

	client := NewM12Client(server.URL, 0)
	vectors, _, err := client.EmbedChunk(context.Background(), "tenant-1", "model", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vectors) != 0 {
		t.Errorf("expected empty embedding, got %d dims", len(vectors))
	}
}

func TestM12Client_EmbedChunk_Timeout(t *testing.T) {
	// Use a non-routable IP to trigger a connection timeout.
	client := NewM12Client("http://192.0.2.1:12345", 100) // 100ms timeout
	_, _, err := client.EmbedChunk(context.Background(), "tenant-1", "model", "test")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestM12Client_EmbedChunk_500Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	client := NewM12Client(server.URL, 0)
	_, _, err := client.EmbedChunk(context.Background(), "tenant-1", "model", "test")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// === M07 Client Tests ===

func TestM07Client_StoreVectors_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"stored":3}`))
	}))
	defer server.Close()

	client := NewM07Client(server.URL, 0)
	err := client.StoreVectors(context.Background(), "tok", "tenant-1", []VectorItem{
		{DocumentID: "c1", EmbeddingType: "platform", SemanticContent: "alpha", Metadata: map[string]any{"chunk": 1}},
		{DocumentID: "c2", EmbeddingType: "platform", SemanticContent: "beta", Metadata: map[string]any{"chunk": 2}},
		{DocumentID: "c3", EmbeddingType: "platform", SemanticContent: "gamma", Metadata: map[string]any{"chunk": 3}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestM07Client_StoreVectors_Empty(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"stored":0}`))
	}))
	defer server.Close()

	client := NewM07Client(server.URL, 0)
	err := client.StoreVectors(context.Background(), "tok", "tenant-1", nil)
	if err != nil {
		t.Fatalf("unexpected error for empty vectors: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestM07Client_StoreVectors_Timeout(t *testing.T) {
	// Use a non-routable IP to trigger a connection timeout.
	client := NewM07Client("http://192.0.2.1:12345", 100)
	err := client.StoreVectors(context.Background(), "tok", "tenant-1", []VectorItem{{DocumentID: "c1", SemanticContent: "x"}})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// === M19 Client Tests ===

func TestM19Client_NormalizeText_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"normalized":"أهلا بالعالم"}`))
	}))
	defer server.Close()

	client := NewM19Client(server.URL, 0)
	result, err := client.NormalizeText(context.Background(), "مرحبا بالعالم")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "أهلا بالعالم" {
		t.Errorf("expected normalized text, got %q", result)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestM19Client_NormalizeText_Unavailable(t *testing.T) {
	// Connect to a port that nothing is listening on.
	client := NewM19Client("http://127.0.0.1:59999", 100) // 100ms timeout
	result, err := client.NormalizeText(context.Background(), "test")
	if err != nil {
		t.Fatalf("expected fallback (no error), got error: %v", err)
	}
	if result != "test" {
		t.Errorf("expected original text on fallback, got %q", result)
	}
}

func TestM19Client_NormalizeText_503(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":"service unavailable"}`))
	}))
	defer server.Close()

	client := NewM19Client(server.URL, 0)
	result, err := client.NormalizeText(context.Background(), "test")
	if err != nil {
		t.Fatalf("expected fallback (no error), got error: %v", err)
	}
	if result != "test" {
		t.Errorf("expected original text on 503, got %q", result)
	}
}