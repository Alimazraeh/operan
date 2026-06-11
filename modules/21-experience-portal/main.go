// Module 21 — Experience Portal
//
// The Web UI of the PRD's Experience Layer: a single binary that serves the
// embedded Operan portal SPA and reverse-proxies /svc/<name>/ to every
// platform service, so the browser stays same-origin (no CORS) and the JWT
// is minted client-side from the tenant's signing secret.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
)

//go:embed web
var webFS embed.FS

// serviceTargets maps the /svc/<name>/ prefix to the backing service.
// Defaults are the in-cluster DNS names; each is overridable via
// MODULE21_SVC_<NAME> for compose or local runs.
var serviceTargets = map[string]string{
	"tenant":        "http://tenant-control-plane.operan.svc.cluster.local:8080",
	"orchestration": "http://agent-orchestration.operan.svc.cluster.local:8080",
	"registry":      "http://agent-registry.operan.svc.cluster.local:8083",
	"templates":     "http://department-templates.operan.svc.cluster.local:8005",
	"memory":        "http://memory-fabric.operan.svc.cluster.local:8007",
	"tools":         "http://tool-execution.operan.svc.cluster.local:8008",
	"supervision":   "http://human-supervision.operan.svc.cluster.local:8009",
	"observability": "http://observability.operan.svc.cluster.local:8011",
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// newProxy strips /svc/<name> and forwards the rest to the target.
func newProxy(name, target string) http.Handler {
	u, err := url.Parse(target)
	if err != nil {
		log.Fatalf("invalid target for %s: %v", name, err)
	}
	prefix := "/svc/" + name
	p := httputil.NewSingleHostReverseProxy(u)
	director := p.Director
	p.Director = func(r *http.Request) {
		director(r)
		r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		r.Host = u.Host
	}
	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[PROXY] %s %s: %v", name, r.URL.Path, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `{"error":{"code":"UPSTREAM_UNAVAILABLE","message":"service %s unreachable"}}`, name)
	}
	return p
}

func buildMux() *http.ServeMux {
	mux := http.NewServeMux()

	for name, def := range serviceTargets {
		target := env("MODULE21_SVC_"+strings.ToUpper(name), def)
		mux.Handle("/svc/"+name+"/", newProxy(name, target))
	}

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","module":"experience-portal","version":"1.0.0"}`))
	})

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("embedded web assets: %v", err)
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// SPA: unknown paths (no extension) fall back to index.html so the
		// hash-less router still resolves after a refresh.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" && !strings.Contains(path, ".") {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
	return mux
}

func main() {
	port, err := strconv.Atoi(env("MODULE21_PORT", "8021"))
	if err != nil {
		log.Fatalf("invalid MODULE21_PORT: %v", err)
	}
	log.Printf("Module 21 — Experience Portal starting on :%d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), buildMux()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
