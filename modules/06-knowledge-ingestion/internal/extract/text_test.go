package extract

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTXTExtractor_Extract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, this is a test document.\nLine two of the document."))
	}))
	defer server.Close()

	e := NewTXTExtractor()
	source := Source{URL: server.URL + "/test.txt", Filename: "test.txt"}
	ctx := context.Background()
	result, err := e.Extract(ctx, source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !strings.Contains(result.Text, "Hello") {
		t.Errorf("expected 'Hello' in text, got %q", result.Text)
	}
	if result.Meta["file_type"] != "txt" {
		t.Errorf("expected file_type 'txt', got %q", result.Meta["file_type"])
	}
}

func TestTXTExtractor_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
	}))
	defer server.Close()

	e := NewTXTExtractor()
	source := Source{URL: server.URL + "/empty.txt"}
	ctx := context.Background()
	result, err := e.Extract(ctx, source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Text) != 0 {
		t.Errorf("expected empty text, got %q", result.Text)
	}
}