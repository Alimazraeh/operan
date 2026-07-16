package extract

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTMLExtractor_Extract(t *testing.T) {
	htmlContent := `
		<!DOCTYPE html>
		<html>
		<body>
			<h1>Main Title</h1>
			<p>This is the main content paragraph.</p>
			<nav>Skip navigation links</nav>
			<footer>Footer content</footer>
		</body>
		</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(htmlContent))
	}))
	defer server.Close()

	e := NewHTMLExtractor()
	source := Source{URL: server.URL + "/page.html", Filename: "page.html"}
	ctx := context.Background()
	result, err := e.Extract(ctx, source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !strings.Contains(result.Text, "Main Title") {
		t.Errorf("expected 'Main Title' in text, got %q", result.Text)
	}
	// Nav and footer should be skipped.
	if strings.Contains(strings.ToLower(result.Text), "skip navigation") {
		t.Error("nav content should be excluded")
	}
	if strings.Contains(strings.ToLower(result.Text), "footer") {
		t.Error("footer content should be excluded")
	}
	if result.Meta["file_type"] != "html" {
		t.Errorf("expected file_type 'html', got %q", result.Meta["file_type"])
	}
}

func TestHTMLExtractor_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body></body></html>"))
	}))
	defer server.Close()

	e := NewHTMLExtractor()
	source := Source{URL: server.URL + "/empty.html"}
	ctx := context.Background()
	result, err := e.Extract(ctx, source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(strings.TrimSpace(result.Text)) != 0 {
		t.Errorf("expected empty text for empty body, got %q", result.Text)
	}
}