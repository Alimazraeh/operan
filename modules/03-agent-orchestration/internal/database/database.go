// Package database provides PostgreSQL connection management, migration,
// and common DB utilities for the repository layer.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Config holds PostgreSQL connection parameters.
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	MaxOpen  int
	MaxIdle  int
	MaxLifetime time.Duration
}

// DefaultConfig returns sensible defaults for local development.
func DefaultConfig() Config {
	return Config{
		Host:     "localhost",
		Port:     "5432",
		User:     "postgres",
		Password: "postgres",
		DBName:   "orchestration",
		MaxOpen:  25,
		MaxIdle:  5,
		MaxLifetime: 5 * time.Minute,
	}
}

// DSN builds a PostgreSQL DSN from config.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		c.Host, c.Port, c.User, c.Password, c.DBName,
	)
}

// OpenPool creates a connection pool and runs migrations.
func OpenPool(ctx context.Context, cfg Config) (*sql.DB, error) {
	dsn := cfg.DSN()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpen)
	db.SetMaxIdleConns(cfg.MaxIdle)
	db.SetConnMaxLifetime(cfg.MaxLifetime)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	if err := Migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	log.Printf("[db] connected to postgres://%s@%s:%s/%s", cfg.User, cfg.Host, cfg.Port, cfg.DBName)
	return db, nil
}

// Migrate applies all schema migrations.
func Migrate(ctx context.Context, db *sql.DB) error {
	migrations := []string{
		// ─── tenants ────────────────────────────────────────────────────────
		createTableTenants,
		// ─── workflows ──────────────────────────────────────────────────────
		createTableWorkflows,
		createTableWorkflowVariables,
		createTableWorkflowCheckpoints,
		createTableWorkflowEvents,
		// ─── schedules ─────────────────────────────────────────────────────
		createTableSchedules,
		// ─── agents ────────────────────────────────────────────────────────
		createTableAgents,
		createTableAgentAssignments,
		createTableAgentAvailability,
		// ─── pipelines ─────────────────────────────────────────────────────
		createTablePipelines,
		createTablePipelineSteps,
		// ─── executions ────────────────────────────────────────────────────
		createTableExecutions,
		createTableExecutionSteps,
		// ─── human tasks ───────────────────────────────────────────────────
		createTableHumanTasks,
		// ─── escalations ───────────────────────────────────────────────────
		createTableEscalations,
		// ─── retry records ─────────────────────────────────────────────────
		createTableRetryRecords,
		// ─── stack health ──────────────────────────────────────────────────
		createTableStackHealth,
	}

	for _, sql := range migrations {
		if _, err := db.ExecContext(ctx, sql); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}

// ─── Migration SQL ──────────────────────────────────────────────────────────

const createTableTenants = `
CREATE TABLE IF NOT EXISTS tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    domain      TEXT,
    status      TEXT NOT NULL DEFAULT 'active',
    settings    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tenants_domain ON tenants(domain);
`

const createTableWorkflows = `
CREATE TABLE IF NOT EXISTS workflows (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    department_id   TEXT,
    name            TEXT NOT NULL,
    version         TEXT NOT NULL DEFAULT '1',
    status          TEXT NOT NULL DEFAULT 'pending',
    current_nodes   JSONB NOT NULL DEFAULT '[]',
    graph           JSONB NOT NULL DEFAULT '{}',
    priority        INT NOT NULL DEFAULT 5,
    description     TEXT DEFAULT '',
    created_by      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_workflows_tenant ON workflows(tenant_id);
CREATE INDEX IF NOT EXISTS idx_workflows_status ON workflows(status);
CREATE INDEX IF NOT EXISTS idx_workflows_department ON workflows(department_id);
`

const createTableWorkflowVariables = `
CREATE TABLE IF NOT EXISTS workflow_variables (
    workflow_id   UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    key           TEXT NOT NULL,
    value         JSONB NOT NULL DEFAULT '{}',
    version       INT NOT NULL DEFAULT 1,
    PRIMARY KEY (workflow_id, key)
);
CREATE INDEX IF NOT EXISTS idx_wv_tenant ON workflow_variables(tenant_id);
`

const createTableWorkflowCheckpoints = `
CREATE TABLE IF NOT EXISTS workflow_checkpoints (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    node_id         TEXT NOT NULL,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT now(),
    state_snapshot  JSONB NOT NULL DEFAULT '{}',
    checksum        TEXT
);
CREATE INDEX IF NOT EXISTS idx_wc_workflow ON workflow_checkpoints(workflow_id);
`

const createTableWorkflowEvents = `
CREATE TABLE IF NOT EXISTS workflow_events (
    event_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id   UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    node_id       TEXT,
    event_type    TEXT NOT NULL,
    timestamp     TIMESTAMPTZ NOT NULL DEFAULT now(),
    details       JSONB NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_we_workflow ON workflow_events(workflow_id);
`

const createTableSchedules = `
CREATE TABLE IF NOT EXISTS schedules (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id),
    name                TEXT NOT NULL,
    cron                TEXT NOT NULL,
    workflow_template_id TEXT NOT NULL,
    variables           JSONB NOT NULL DEFAULT '{}',
    enabled             BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_schedules_tenant ON schedules(tenant_id);
CREATE INDEX IF NOT EXISTS idx_schedules_enabled ON schedules(enabled);
`

const createTableAgents = `
CREATE TABLE IF NOT EXISTS agents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    name            TEXT NOT NULL,
    type            TEXT NOT NULL DEFAULT 'general',
    capabilities    JSONB NOT NULL DEFAULT '[]',
    status          TEXT NOT NULL DEFAULT 'available',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_agents_tenant ON agents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
`

const createTableAgentAssignments = `
CREATE TABLE IF NOT EXISTS agent_assignments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    workflow_id     UUID NOT NULL REFERENCES workflows(id),
    node_id         TEXT NOT NULL,
    agent_id        UUID NOT NULL REFERENCES agents(id),
    parameters      JSONB NOT NULL DEFAULT '{}',
    assigned_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_aa_workflow ON agent_assignments(workflow_id);
CREATE INDEX IF NOT EXISTS idx_aa_agent ON agent_assignments(agent_id);
`

const createTableAgentAvailability = `
CREATE TABLE IF NOT EXISTS agent_availability (
    agent_id             UUID PRIMARY KEY REFERENCES agents(id),
    status               TEXT NOT NULL DEFAULT 'available',
    current_workflows    INT NOT NULL DEFAULT 0,
    max_concurrency      INT NOT NULL DEFAULT 1,
    last_seen_at         TIMESTAMPTZ
);
`

const createTablePipelines = `
CREATE TABLE IF NOT EXISTS pipelines (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    name            TEXT NOT NULL,
    description     TEXT DEFAULT '',
    steps           JSONB NOT NULL DEFAULT '[]',
    error_handling  JSONB NOT NULL DEFAULT '{}',
    timeout_minutes INT NOT NULL DEFAULT 30,
    max_retries     INT NOT NULL DEFAULT 0,
    trigger_type    TEXT NOT NULL DEFAULT 'manual',
    variables       JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'active',
    execution_count INT NOT NULL DEFAULT 0,
    last_execution_at  TIMESTAMPTZ,
    success_rate    FLOAT8 NOT NULL DEFAULT 0,
    avg_duration_ms  FLOAT8 NOT NULL DEFAULT 0,
    created_by      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    tags            JSONB NOT NULL DEFAULT '[]'
);
CREATE INDEX IF NOT EXISTS idx_pipelines_tenant ON pipelines(tenant_id);
CREATE INDEX IF NOT EXISTS idx_pipelines_status ON pipelines(status);
`

const createTablePipelineSteps = `
CREATE TABLE IF NOT EXISTS pipeline_steps (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id UUID NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    step_id     TEXT NOT NULL,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL DEFAULT 'api_call',
    inputs      JSONB NOT NULL DEFAULT '{}',
    config      JSONB NOT NULL DEFAULT '{}',
    condition   TEXT DEFAULT '',
    timeout_seconds INT NOT NULL DEFAULT 60,
    on_error    TEXT NOT NULL DEFAULT 'fail',
    next_step_id TEXT,
    parallel_branches INT NOT NULL DEFAULT 0,
    CONSTRAINT uk_pipeline_step UNIQUE (pipeline_id, step_id)
);
CREATE INDEX IF NOT EXISTS idx_ps_pipeline ON pipeline_steps(pipeline_id);
`

const createTableExecutions = `
CREATE TABLE IF NOT EXISTS executions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     UUID NOT NULL REFERENCES pipelines(id),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    status          TEXT NOT NULL DEFAULT 'pending',
    inputs          JSONB NOT NULL DEFAULT '{}',
    outputs         JSONB NOT NULL DEFAULT '{}',
    current_step_id TEXT,
    current_step_status TEXT,
    error_message   TEXT DEFAULT '',
    retry_count     INT NOT NULL DEFAULT 0,
    duration_ms     FLOAT8 NOT NULL DEFAULT 0,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_exec_pipeline ON executions(pipeline_id);
CREATE INDEX IF NOT EXISTS idx_exec_tenant ON executions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_exec_status ON executions(status);
`

const createTableExecutionSteps = `
CREATE TABLE IF NOT EXISTS execution_steps (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id    UUID NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    step_id         TEXT NOT NULL,
    step_name       TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    inputs          JSONB NOT NULL DEFAULT '{}',
    outputs         JSONB NOT NULL DEFAULT '{}',
    error_message   TEXT DEFAULT '',
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    duration_ms     FLOAT8 NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_es_execution ON execution_steps(execution_id);
`

const createTableHumanTasks = `
CREATE TABLE IF NOT EXISTS human_tasks (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL REFERENCES tenants(id),
    pipeline_execution_id   UUID NOT NULL REFERENCES executions(id),
    step_id                 TEXT NOT NULL,
    assignee_type           TEXT NOT NULL DEFAULT 'user',
    assignee_id             TEXT,
    task_type               TEXT NOT NULL DEFAULT 'approval',
    instructions            TEXT,
    context                 JSONB NOT NULL DEFAULT '{}',
    timeout_minutes         INT NOT NULL DEFAULT 60,
    label                   TEXT,
    priority                TEXT NOT NULL DEFAULT 'normal',
    status                  TEXT NOT NULL DEFAULT 'pending',
    response                JSONB NOT NULL DEFAULT '{}',
    responded_by            TEXT,
    responded_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ht_tenant ON human_tasks(tenant_id);
CREATE INDEX IF NOT EXISTS idx_ht_status ON human_tasks(status);
CREATE INDEX IF NOT EXISTS idx_ht_execution ON human_tasks(pipeline_execution_id);
`

const createTableEscalations = `
CREATE TABLE IF NOT EXISTS escalations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id         UUID NOT NULL REFERENCES workflows(id),
    node_id             TEXT,
    tenant_id           UUID NOT NULL REFERENCES tenants(id),
    department_id       TEXT,
    status              TEXT NOT NULL DEFAULT 'pending',
    severity            TEXT NOT NULL DEFAULT 'medium',
    reason              TEXT NOT NULL,
    escalated_to        TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    acknowledged_at     TIMESTAMPTZ,
    resolved_at         TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_esc_workflow ON escalations(workflow_id);
CREATE INDEX IF NOT EXISTS idx_esc_tenant ON escalations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_esc_status ON escalations(status);
`

const createTableRetryRecords = `
CREATE TABLE IF NOT EXISTS retry_records (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id     UUID NOT NULL REFERENCES workflows(id),
    node_id         TEXT,
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    attempt_number  INT NOT NULL DEFAULT 1,
    status          TEXT NOT NULL DEFAULT 'pending',
    error_code      TEXT DEFAULT '',
    error_message   TEXT DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_rr_workflow ON retry_records(workflow_id);
CREATE INDEX IF NOT EXISTS idx_rr_tenant ON retry_records(tenant_id);
CREATE INDEX IF NOT EXISTS idx_rr_status ON retry_records(status);
`

const createTableStackHealth = `
CREATE TABLE IF NOT EXISTS stack_health (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    stacks      JSONB NOT NULL DEFAULT '{}',
    at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    stack_type  TEXT,
    stack_name  TEXT,
    status      TEXT DEFAULT 'unknown',
    config      JSONB NOT NULL DEFAULT '{}',
    metadata    JSONB NOT NULL DEFAULT '{}',
    graph_def   JSONB NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_sh_tenant ON stack_health(tenant_id);
CREATE INDEX IF NOT EXISTS idx_sh_type ON stack_health(stack_type);
CREATE INDEX IF NOT EXISTS idx_sh_at ON stack_health(at DESC);
`
