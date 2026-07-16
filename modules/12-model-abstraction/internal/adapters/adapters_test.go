package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func makeMockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func TestOpenAIAdapter_Chat(t *testing.T) {
	server := makeMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ChatResponse{
			ID:    "chatcmpl-123",
			Model: "gpt-4",
			Choices: []ChatChoice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: "Hello!",
					},
					Finish: "stop",
				},
			},
			Usage: ChatUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		})
	})
	defer server.Close()

	adapter := NewOpenAIAdapter(ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
	})

	resp, err := adapter.Chat(context.Background(), ChatRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "chatcmpl-123" {
		t.Errorf("expected ID 'chatcmpl-123', got %q", resp.ID)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("expected 'Hello!', got %q", resp.Choices[0].Message.Content)
	}
}

func TestOpenAIAdapter_ChatError(t *testing.T) {
	server := makeMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "server error", "type": "server_error"},
		})
	})
	defer server.Close()

	adapter := NewOpenAIAdapter(ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
	})

	_, err := adapter.Chat(context.Background(), ChatRequest{Model: "gpt-4"})
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestOpenAIAdapter_Embeddings(t *testing.T) {
	server := makeMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(EmbedResponse{
			Model:  "text-embedding-3-small",
			Object: "list",
			Data: []EmbedDoc{
				{Index: 0, Embedding: []float64{0.1, 0.2, 0.3}, Object: "embedding"},
			},
			Usage: EmbedUsage{PromptTokens: 5, TotalTokens: 5},
		})
	})
	defer server.Close()

	adapter := NewOpenAIAdapter(ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
	})

	resp, err := adapter.Embeddings(context.Background(), EmbedRequest{Model: "text-embedding-3-small", Input: []string{"hello"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 embedding, got %d", len(resp.Data))
	}
}

func TestAnthropicAdapter_Chat(t *testing.T) {
	server := makeMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(anthropicChatResponse{
			ID:    "msg_123",
			Model: "claude-3-opus-20240229",
			Stop:  "end_turn",
			CContent: []struct {
				Type  string `json:"type"`
				Text  string `json:"text,omitempty"`
			}{
				{Type: "text", Text: "Anthropic response"},
			},
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{InputTokens: 10, OutputTokens: 20},
		})
	})
	defer server.Close()

	adapter := NewAnthropicAdapter(ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "sk-ant-test",
	})

	resp, err := adapter.Chat(context.Background(), ChatRequest{
		Model:    "claude-3-opus-20240229",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
		MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "msg_123" {
		t.Errorf("expected ID 'msg_123', got %q", resp.ID)
	}
	if resp.Choices[0].Message.Content != "Anthropic response" {
		t.Errorf("expected 'Anthropic response', got %q", resp.Choices[0].Message.Content)
	}
}

func TestAnthropicAdapter_EmbeddingsNotSupported(t *testing.T) {
	adapter := NewAnthropicAdapter(ProviderConfig{BaseURL: "http://example.com", APIKey: "test"})
	_, err := adapter.Embeddings(context.Background(), EmbedRequest{Model: "test"})
	if err == nil {
		t.Fatal("expected error for unsupported embeddings")
	}
}

func TestOllamaAdapter_Chat(t *testing.T) {
	server := makeMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":               "ollama-123",
			"model":            "llama2",
			"message":          map[string]any{"role": "assistant", "content": "Ollama response"},
			"done":             true,
			"prompt_eval_count": 10.0,
			"eval_count":       5.0,
		})
	})
	defer server.Close()

	adapter := NewOllamaAdapter(ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "",
	})

	resp, err := adapter.Chat(context.Background(), ChatRequest{
		Model:    "llama2",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Choices[0].Message.Content != "Ollama response" {
		t.Errorf("expected 'Ollama response', got %q", resp.Choices[0].Message.Content)
	}
}

func TestOllamaAdapter_Embeddings(t *testing.T) {
	server := makeMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"embeddings": [][]any{{0.1, 0.2, 0.3}},
		})
	})
	defer server.Close()

	adapter := NewOllamaAdapter(ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "",
	})

	resp, err := adapter.Embeddings(context.Background(), EmbedRequest{
		Model: "nomic-embed-text",
		Input: []string{"hello"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 embedding, got %d", len(resp.Data))
	}
}

func TestLiteLLMAdapter_Chat(t *testing.T) {
	server := makeMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ChatResponse{
			ID:      "litellm-123",
			Model:   "gpt-4",
			Choices: []ChatChoice{{Index: 0, Message: ChatMessage{Role: "assistant", Content: "LiteLLM response"}}},
			Usage:   ChatUsage{PromptTokens: 5, CompletionTokens: 10, TotalTokens: 15},
		})
	})
	defer server.Close()

	adapter := NewLiteLLMAdapter(ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
	})

	resp, err := adapter.Chat(context.Background(), ChatRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Choices[0].Message.Content != "LiteLLM response" {
		t.Errorf("expected 'LiteLLM response', got %q", resp.Choices[0].Message.Content)
	}
}

func TestDoJSONRequest_AuthHeader(t *testing.T) {
	server := makeMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("expected Bearer auth header, got %q", auth)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	defer server.Close()

	var resp map[string]string
	err := doJSONRequest(context.Background(), server.URL, "/test", "my-api-key", nil, 30000, &resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", resp["status"])
	}
}

func TestDoJSONRequest_Timeout(t *testing.T) {
	// Use a port that won't respond.
	adapter := NewOpenAIAdapter(ProviderConfig{
		BaseURL: "http://127.0.0.1:59999",
		APIKey:  "test",
	})
	_, err := adapter.Chat(context.Background(), ChatRequest{Model: "test"})
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}