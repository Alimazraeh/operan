package embeddings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mockGateway(t *testing.T, dims int, wantAuth string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if wantAuth != "" && r.Header.Get("Authorization") != wantAuth {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		type item struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		}
		var data []item
		for i := range req.Input {
			vec := make([]float64, dims)
			vec[0] = float64(i + 1) // distinguishable per input
			data = append(data, item{Index: i, Embedding: vec})
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
	}))
}

func TestEmbedReturnsVectorsInOrder(t *testing.T) {
	srv := mockGateway(t, 4, "Bearer sk-test")
	defer srv.Close()

	c := New(srv.URL, "sk-test", "qwen3-embedding-4b")
	vecs, err := c.Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 2 || len(vecs[0]) != 4 {
		t.Fatalf("vecs = %d x %d", len(vecs), len(vecs[0]))
	}
	if vecs[0][0] != 1 || vecs[1][0] != 2 {
		t.Errorf("order not preserved: %v %v", vecs[0][0], vecs[1][0])
	}
	if c.Model() != "qwen3-embedding-4b" {
		t.Errorf("Model() = %s", c.Model())
	}
}

func TestEmbedErrors(t *testing.T) {
	srv := mockGateway(t, 4, "Bearer sk-test")
	defer srv.Close()

	// Wrong key → non-200 surfaces as error.
	bad := New(srv.URL, "wrong", "m")
	if _, err := bad.Embed(context.Background(), []string{"x"}); err == nil {
		t.Error("expected error on 401")
	}

	// Unreachable gateway.
	gone := New("http://127.0.0.1:1", "k", "m")
	if _, err := gone.Embed(context.Background(), []string{"x"}); err == nil {
		t.Error("expected error on unreachable gateway")
	}

	// Empty input is a no-op.
	ok := New(srv.URL, "sk-test", "m")
	if vecs, err := ok.Embed(context.Background(), nil); err != nil || vecs != nil {
		t.Errorf("empty input: %v %v", vecs, err)
	}
}
