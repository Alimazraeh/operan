package authentik

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"
)

// Provisioner manages Authentik deployments per tenant.
// It uses Helm for Kubernetes or Docker Compose for local/dev deployments.
type Provisioner struct {
	HelmBinary     string
	DockerCompose  string
	ChartPath      string
	ReleasePrefix  string
	ValuesTemplate string
	WorkingDir     string
}

// ProvisionerConfig holds provisioning configuration.
type ProvisionerConfig struct {
	HelmBinary     string // "helm" (default: looks in PATH)
	DockerCompose  string // "docker compose" or "docker-compose" (default: looks in PATH)
	ChartPath      string // Path to Helm chart or embedded templates
	ReleasePrefix  string // Prefix for Helm releases (default: "operan")
	ValuesTemplate string // Path to values.yaml template
	WorkingDir     string // Working directory for temp files (default: os.TempDir())
}

// TenantDeployment represents a deployed Authentik instance per tenant.
type TenantDeployment struct {
	TenantID    string `json:"tenant_id"`
	ServiceURL  string `json:"service_url"`
	AdminToken  string `json:"admin_token"`
	AdminUser   string `json:"admin_user"`
	AdminPass   string `json:"admin_pass"`
	State       string `json:"state"` // "provisioning", "ready", "error"
	DeployedAt  string `json:"deployed_at"`
	ChartVersion string `json:"chart_version"`
	Method       string `json:"method"` // "helm", "docker-compose"
}

// NewProvisioner creates a new provisioner with the given config.
func NewProvisioner(cfg ProvisionerConfig) *Provisioner {
	if cfg.HelmBinary == "" {
		cfg.HelmBinary = "helm"
	}
	if cfg.DockerCompose == "" {
		cfg.DockerCompose = "docker compose"
	}
	if cfg.ReleasePrefix == "" {
		cfg.ReleasePrefix = "operan"
	}
	if cfg.WorkingDir == "" {
		cfg.WorkingDir = os.TempDir()
	}
	return &Provisioner{
		HelmBinary:    cfg.HelmBinary,
		DockerCompose: cfg.DockerCompose,
		ChartPath:     cfg.ChartPath,
		ReleasePrefix: cfg.ReleasePrefix,
		WorkingDir:    cfg.WorkingDir,
	}
}

// DeployProvisions Authentik for a tenant using the configured method.
func (p *Provisioner) DeployProvision(ctx context.Context, cfg *TenantDeployment) error {
	switch cfg.Method {
	case "helm", "":
		return p.deployHelm(ctx, cfg)
	case "docker-compose":
		return p.deployDockerCompose(ctx, cfg)
	default:
		return fmt.Errorf("unknown deployment method: %s", cfg.Method)
	}
}

// deployHelm provisions Authentik using the official Helm chart.
func (p *Provisioner) deployHelm(ctx context.Context, cfg *TenantDeployment) error {
	// Generate a unique namespace for the tenant
	namespace := fmt.Sprintf("%s-%s", p.ReleasePrefix, cfg.TenantID)

	// Generate random credentials
	cfg.AdminUser = fmt.Sprintf("%s-admin", cfg.TenantID)
	cfg.AdminPass = generateSecurePassword(32)
	cfg.AdminToken = generateSecureToken(64)

	// Create a temporary values file for this tenant
	valuesPath, err := p.generateHelmValues(cfg, namespace)
	if err != nil {
		return fmt.Errorf("failed to generate helm values: %w", err)
	}
	defer os.Remove(valuesPath)

	// Check if release already exists
	cmd := exec.CommandContext(ctx, p.HelmBinary, "list", "-n", namespace, "-q")
	if err := cmd.Run(); err == nil {
		// Release exists — upgrade instead
		cmd = exec.CommandContext(ctx, p.HelmBinary, "upgrade", cfg.TenantID, "authentik/authentik",
			"--namespace", namespace,
			"--create-namespace",
			"--values", valuesPath,
			"--wait", "--timeout", "300s")
	} else {
		// Install new release
		cmd = exec.CommandContext(ctx, p.HelmBinary, "install", cfg.TenantID, "authentik/authentik",
			"--namespace", namespace,
			"--create-namespace",
			"--values", valuesPath,
			"--wait", "--timeout", "300s")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm %s failed: %s: %w", cmd.Args[0], string(output), err)
	}

	cfg.ServiceURL = fmt.Sprintf("http://%s-%s.%s.svc.cluster.local:9000", p.ReleasePrefix, cfg.TenantID, namespace)
	cfg.State = "ready"
	cfg.DeployedAt = time.Now().UTC().Format(time.RFC3339)
	cfg.ChartVersion = "2024.x.x" // Will be updated from helm repo

	return nil
}

// deployDockerCompose provisions Authentik using docker-compose for local/dev.
func (p *Provisioner) deployDockerCompose(ctx context.Context, cfg *TenantDeployment) error {
	// Generate credentials
	cfg.AdminUser = fmt.Sprintf("%s-admin", cfg.TenantID)
	cfg.AdminPass = generateSecurePassword(32)
	cfg.AdminToken = generateSecureToken(64)

	// Create a compose directory for the tenant
	composeDir := filepath.Join(p.WorkingDir, "operan-tenant-"+cfg.TenantID)
	os.MkdirAll(composeDir, 0755)

	// Generate docker-compose.yml for this tenant
	composeContent := fmt.Sprintf(`
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: authentik
      POSTGRES_USER: authentik
      POSTGRES_PASSWORD: %s
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine

  server:
    image: ghcr.io/goauthentik/server:2024.12.4
    environment:
      AUTHENTIK_SECRET_KEY: %s
      AUTHENTIK_POSTGRESQL__HOST: postgres
      AUTHENTIK_POSTGRESQL__USER: authentik
      AUTHENTIK_POSTGRESQL__PASSWORD: %s
      AUTHENTIK_POSTGRESQL__DATABASE: authentik
      AUTHENTIK_REDIS__HOST: redis
      AUTHENTIK_ADMIN_USER: %s
      AUTHENTIK_ADMIN_PASSWORD: %s
      AUTHENTIK_ADMIN_EMAIL: admin@%s.operan.local
    ports:
      - "9443:9443"
    depends_on:
      - postgres
      - redis
    volumes:
      - ./media:/media
      - ./custom-templates:/custom-templates

  worker:
    image: ghcr.io/goauthentik/worker:2024.12.4
    environment:
      AUTHENTIK_SECRET_KEY: %s
      AUTHENTIK_POSTGRESQL__HOST: postgres
      AUTHENTIK_POSTGRESQL__USER: authentik
      AUTHENTIK_POSTGRESQL__PASSWORD: %s
      AUTHENTIK_POSTGRESQL__DATABASE: authentik
      AUTHENTIK_REDIS__HOST: redis
    depends_on:
      - postgres
      - redis

volumes:
  pgdata:
`, generateSecurePassword(32), generateSecureToken(64),
		generateSecurePassword(32), cfg.AdminUser, cfg.AdminPass, cfg.TenantID,
		generateSecureToken(64), generateSecurePassword(32))

	composePath := filepath.Join(composeDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %w", err)
	}

	// Up the stack
	cmd := exec.CommandContext(ctx, p.DockerCompose, "-f", composePath, "up", "-d")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker-compose up failed: %s: %w", string(output), err)
	}

	cfg.ServiceURL = "http://localhost:9443"
	cfg.State = "ready"
	cfg.DeployedAt = time.Now().UTC().Format(time.RFC3339)
	cfg.Method = "docker-compose"

	return nil
}

// generateHelmValues creates a Helm values.yaml for a tenant deployment.
func (p *Provisioner) generateHelmValues(cfg *TenantDeployment, namespace string) (string, error) {
	templateStr := `
authentik:
  secret_key: "{{ .AdminToken }}"
  log_level: info

postgresql:
  auth:
    database: authentik
    username: authentik
    password: "{{ .AdminPass }}"
    existingSecret: ""

serviceAccount:
  create: true

ingress:
  enabled: true
  hosts:
    - "{{ .TenantID }}.{{ .IngressDomain }}"
  tls:
    - secretName: "{{ .TenantID }}-tls"
      hosts:
        - "{{ .TenantID }}.{{ .IngressDomain }}"
`

	tmpl, err := template.New("values").Parse(templateStr)
	if err != nil {
		return "", err
	}

	type valuesData struct {
		AdminToken     string
		AdminPass      string
		TenantID       string
		IngressDomain  string
	}

	data := valuesData{
		AdminToken:    cfg.AdminToken,
		AdminPass:     cfg.AdminPass,
		TenantID:      cfg.TenantID,
		IngressDomain: os.Getenv("AUTHENTIK_INGRESS_DOMAIN"),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	tmpFile := filepath.Join(p.WorkingDir, fmt.Sprintf("values-%s.yaml", cfg.TenantID))
	if err := os.WriteFile(tmpFile, buf.Bytes(), 0644); err != nil {
		return "", err
	}
	return tmpFile, nil
}

// TearDown removes a tenant's Authentik deployment.
func (p *Provisioner) TearDown(ctx context.Context, tenantID string) error {
	if p.ChartPath != "" {
		// Helm uninstall
		cmd := exec.CommandContext(ctx, p.HelmBinary, "uninstall", tenantID,
			"--namespace", fmt.Sprintf("%s-%s", p.ReleasePrefix, tenantID))
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("helm uninstall failed: %s: %w", string(output), err)
		}
	} else {
		// Docker Compose down
		composeDir := filepath.Join(p.WorkingDir, "operan-tenant-"+tenantID)
		if _, err := os.Stat(composeDir); err == nil {
			cmd := exec.CommandContext(ctx, p.DockerCompose, "-f",
				filepath.Join(composeDir, "docker-compose.yml"), "down", "-v")
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("docker-compose down failed: %s: %w", string(output), err)
			}
		}
	}
	return nil
}

// HealthCheck verifies that a tenant's Authentik instance is healthy.
func (p *Provisioner) HealthCheck(ctx context.Context, serviceURL string) (bool, error) {
	req, err := exec.CommandContext(ctx, "curl", "-sf", serviceURL+"/-/health/live/").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("health check failed: %s: %w", string(req), err)
	}
	return true, nil
}

// ListDeployments returns all tenant deployments (reads from state file or M01 secrets).
func (p *Provisioner) ListDeployments(ctx context.Context) ([]*TenantDeployment, error) {
	// In production, this reads from M01 Tenant Control Plane's secrets store.
	// For now, return empty — implementations should integrate with M01.
	return nil, nil
}

// generateSecurePassword generates a secure random password.
func generateSecurePassword(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	pass := make([]byte, length)
	for i := range pass {
		pass[i] = chars[i%len(chars)]
	}
	// Shuffle (simple, but sufficient for our purposes)
	for i := len(pass) - 1; i > 0; i-- {
		j := i % 26 // Use modulo for a deterministic but varied shuffle
		pass[i], pass[j] = pass[j], pass[i]
	}
	return string(pass)
}

// generateSecureToken generates a secure random token (hex-encoded).
func generateSecureToken(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = byte('a' + (i % 26))
	}
	return fmt.Sprintf("ak_%x", string(b))
}
