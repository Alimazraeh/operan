package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompleteReturnsContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer k" {
			w.WriteHeader(401)
			return
		}
		var req chatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.MaxTokens != 1500 || req.Model != "qwen" {
			t.Errorf("req = %+v", req)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{
				"finish_reason": "stop",
				"message":       map[string]string{"content": "Drafted contract for " + req.Messages[len(req.Messages)-1].Content},
			}},
			"usage": map[string]int{"total_tokens": 42},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "k", "qwen")
	res, err := c.Complete(context.Background(), "you draft contracts", "Acme", 1500)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(res.Content, "Acme") || res.Tokens != 42 {
		t.Errorf("result = %+v", res)
	}
	if c.Model() != "qwen" {
		t.Errorf("model = %s", c.Model())
	}
}

func TestEmptyContentIsError(t *testing.T) {
	// Reasoning model burned its budget thinking → null content.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{
				"finish_reason": "length",
				"message":       map[string]interface{}{"content": nil},
			}},
		})
	}))
	defer srv.Close()
	if _, err := New(srv.URL, "", "m").Complete(context.Background(), "", "x", 50); err == nil {
		t.Error("expected error for empty content")
	}
}

func TestUpstreamErrorSurfaces(t *testing.T) {
	if _, err := New("http://127.0.0.1:1", "", "m").Complete(context.Background(), "", "x", 0); err == nil {
		t.Error("expected error for unreachable gateway")
	}
}
