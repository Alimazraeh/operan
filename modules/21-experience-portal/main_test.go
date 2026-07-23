package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMarketingSiteAtRoot(t *testing.T) {
	mux := buildMux()

	// The marketing site is served at the root and its pages by filename.
	for _, path := range []string{"/", "/platform.html", "/departments.html"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, w.Code)
		}
		if !strings.Contains(w.Body.String(), `class="nav-logo">OPERAN`) {
			t.Errorf("%s did not serve the marketing site", path)
		}
	}
}

func TestSPAShellAndFallbackUnderApp(t *testing.T) {
	mux := buildMux()

	// The product platform SPA lives under /app/; extensionless deep links
	// fall back to the shell so the client router still resolves.
	for _, path := range []string{"/app", "/app/", "/app/departments", "/app/agents/abc-123"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, w.Code)
		}
		if !strings.Contains(w.Body.String(), "<title>Operan") {
			t.Errorf("%s did not serve the SPA shell", path)
		}
	}

	// Real SPA assets serve as themselves, not the shell.
	req := httptest.NewRequest("GET", "/app/js/app.js", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "<title>") {
		t.Errorf("/app/js/app.js: status %d, served HTML instead of JS", w.Code)
	}
}

func TestHealthz(t *testing.T) {
	mux := buildMux()
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz: %d", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["module"] != "experience-portal" {
		t.Errorf("healthz body: %s", w.Body.String())
	}
}

func TestProxyStripsPrefixAndForwards(t *testing.T) {
	// Stand in for a platform service.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"path": r.URL.Path, "auth": r.Header.Get("Authorization"), "tenant": r.Header.Get("X-Tenant-ID"),
		})
	}))
	defer upstream.Close()
	t.Setenv("MODULE21_SVC_MEMORY", upstream.URL)

	mux := buildMux()
	req := httptest.NewRequest("GET", "/svc/memory/vectors?page=1", nil)
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("X-Tenant-ID", "t1")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("proxy: status %d: %s", w.Code, w.Body.String())
	}
	var got map[string]string
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["path"] != "/vectors" {
		t.Errorf("upstream path = %q, want /vectors", got["path"])
	}
	if got["auth"] != "Bearer tok" || got["tenant"] != "t1" {
		t.Errorf("auth headers not forwarded: %+v", got)
	}
}

func TestProxyUpstreamDownReturns502JSON(t *testing.T) {
	t.Setenv("MODULE21_SVC_TOOLS", "http://127.0.0.1:1")
	mux := buildMux()
	req := httptest.NewRequest("GET", "/svc/tools/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status %d, want 502", w.Code)
	}
	if !strings.Contains(w.Body.String(), "UPSTREAM_UNAVAILABLE") {
		t.Errorf("body: %s", w.Body.String())
	}
}
