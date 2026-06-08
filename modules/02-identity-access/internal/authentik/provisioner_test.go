package authentik

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestNewProvisioner_Defaults(t *testing.T) {
	p := NewProvisioner(ProvisionerConfig{})
	if p.HelmBinary != "helm" {
		t.Errorf("HelmBinary default = %q", p.HelmBinary)
	}
	if p.DockerCompose != "docker compose" {
		t.Errorf("DockerCompose default = %q", p.DockerCompose)
	}
	if p.ReleasePrefix != "operan" {
		t.Errorf("ReleasePrefix default = %q", p.ReleasePrefix)
	}
	if p.WorkingDir == "" {
		t.Error("WorkingDir should default to temp dir")
	}

	custom := NewProvisioner(ProvisionerConfig{
		HelmBinary: "helm3", DockerCompose: "docker-compose",
		ReleasePrefix: "acme", WorkingDir: "/tmp/x", ChartPath: "/charts",
	})
	if custom.HelmBinary != "helm3" || custom.ReleasePrefix != "acme" || custom.ChartPath != "/charts" {
		t.Errorf("custom config not applied: %+v", custom)
	}
}

func TestGenerateSecurePassword(t *testing.T) {
	p := generateSecurePassword(16)
	if len(p) != 16 {
		t.Errorf("password length = %d, want 16", len(p))
	}
	if generateSecurePassword(0) != "" {
		t.Error("zero length should produce empty password")
	}
}

func TestGenerateSecureToken(t *testing.T) {
	tok := generateSecureToken(8)
	if !strings.HasPrefix(tok, "ak_") {
		t.Errorf("token should have ak_ prefix, got %q", tok)
	}
}

func TestGenerateHelmValues(t *testing.T) {
	dir := t.TempDir()
	p := NewProvisioner(ProvisionerConfig{WorkingDir: dir})
	dep := &TenantDeployment{TenantID: "t1", AdminToken: "tok", AdminPass: "pw"}

	path, err := p.generateHelmValues(dep, "ns-t1")
	if err != nil {
		t.Fatalf("generateHelmValues() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("values file not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `secret_key: "tok"`) || !strings.Contains(content, `password: "pw"`) {
		t.Errorf("rendered values missing substitutions:\n%s", content)
	}
	if !strings.Contains(content, "t1.") {
		t.Errorf("rendered values missing tenant host:\n%s", content)
	}
}

func TestProvisioner_ListDeployments(t *testing.T) {
	p := NewProvisioner(ProvisionerConfig{})
	deps, err := p.ListDeployments(context.Background())
	if err != nil {
		t.Fatalf("ListDeployments() error = %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected empty deployments, got %d", len(deps))
	}
}

func TestProvisioner_TearDown_NoOp(t *testing.T) {
	// No ChartPath and no compose dir on disk -> TearDown is a no-op success.
	p := NewProvisioner(ProvisionerConfig{WorkingDir: t.TempDir()})
	if err := p.TearDown(context.Background(), "nonexistent-tenant"); err != nil {
		t.Errorf("TearDown() no-op error = %v", err)
	}
}

func TestProvisioner_HealthCheck(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not available")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/-/health/live/") {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := NewProvisioner(ProvisionerConfig{})
	ok, err := p.HealthCheck(context.Background(), srv.URL)
	if err != nil || !ok {
		t.Errorf("HealthCheck() = %v, err = %v; want true, nil", ok, err)
	}

	// Unreachable URL -> error.
	if ok, err := p.HealthCheck(context.Background(), "http://127.0.0.1:1"); err == nil || ok {
		t.Error("HealthCheck() to dead endpoint should fail")
	}
}
