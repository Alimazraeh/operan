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

// The model spending its whole budget on reasoning and emitting nothing is a
// budget failure, not a refusal. It killed the flagship IT SOP on its first
// node: 2000 tokens was just under what a real step costs. One retry at double
// the budget is what turns that into a completed step.
func TestEmptyContentFromTruncationIsRetriedAtDoubleTheBudget(t *testing.T) {
	var budgets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		json.NewDecoder(r.Body).Decode(&req)
		budgets = append(budgets, req.MaxTokens)
		w.Header().Set("Content-Type", "application/json")
		if len(budgets) == 1 {
			// All budget spent thinking, no content — exactly what the gateway
			// returns for a reasoning model on a tight budget.
			w.Write([]byte(`{"choices":[{"finish_reason":"length","message":{"content":"","reasoning_content":"thinking hard..."}}],"usage":{"total_tokens":2000}}`))
			return
		}
		w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"the assessment"}}],"usage":{"total_tokens":2234}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", "qwen")
	res, err := c.Complete(context.Background(), "sys", "user", 2000)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.Content != "the assessment" {
		t.Fatalf("content = %q", res.Content)
	}
	if res.Truncated {
		t.Fatal("a completed retry must not be marked truncated")
	}
	if len(budgets) != 2 || budgets[0] != 2000 || budgets[1] != 4000 {
		t.Fatalf("budgets = %v, want [2000 4000]", budgets)
	}
}

// Two failures in a row stop. Doubling again only burns time and tokens, and
// the caller needs the real reason, including how much the model spent
// thinking — that is the evidence for raising the configured budget.
func TestBudgetExhaustionGivesUpAfterOneRetryAndSaysWhy(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"finish_reason":"length","message":{"content":"","reasoning_content":"0123456789"}}],"usage":{"total_tokens":1}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", "qwen")
	_, err := c.Complete(context.Background(), "sys", "user", 1000)
	if err == nil {
		t.Fatal("want an error")
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (one attempt, one retry)", calls)
	}
	for _, want := range []string{"1000-token budget", "10 characters", "retried at 2000"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err.Error(), want)
		}
	}
}

// Content that arrives but is cut off mid-sentence is real work and must be
// returned — flagged, so nothing downstream presents it as finished.
func TestTruncatedContentIsReturnedAndFlagged(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"finish_reason":"length","message":{"content":"the assessment is incomp"}}],"usage":{"total_tokens":4000}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", "qwen")
	res, err := c.Complete(context.Background(), "sys", "user", 4000)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !res.Truncated {
		t.Fatal("a cut-off draft must be marked truncated")
	}
	if res.Content != "the assessment is incomp" {
		t.Fatalf("content = %q", res.Content)
	}
	if calls != 1 {
		t.Fatalf("calls = %d — partial content must not trigger a retry", calls)
	}
}

// An empty answer the model chose to give is not a budget problem, so retrying
// it would just cost twice as much to fail the same way.
func TestEmptyContentWithoutTruncationIsNotRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"finish_reason":"content_filter","message":{"content":""}}],"usage":{"total_tokens":10}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", "qwen")
	if _, err := c.Complete(context.Background(), "sys", "user", 4000); err == nil {
		t.Fatal("want an error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}
