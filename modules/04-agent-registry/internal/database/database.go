// Package database gives the agent registry durable storage.
//
// Until this existed the registry was entirely in memory. It had restarted 24
// times and lost every agent registered against it, so the portal read
// "0 Agents", departments reported agents_count: 0, and the agent_id on every
// deployed org-chart position pointed at nothing. A registry that forgets what
// is registered is not a registry.
//
// The shape follows Module 02: memory stays the read path, every write also
// goes to PostgreSQL, and the process rehydrates at boot. Without that last
// part persistence is write-only and the restart still loses everything.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pool against dsn and verifies it answers. An empty dsn is
// not an error: the service runs in memory, and says so at boot.
func Connect(ctx context.Context, dsn string, maxConns int) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = int32(maxConns)
	}
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// schema is applied at every boot. Each statement is guarded, so running it
// against an already-migrated database is a no-op — the modules that used bare
// CREATE INDEX here crash-looped on their second start.
const schema = `
CREATE TABLE IF NOT EXISTS registry_agents (
	id                  TEXT PRIMARY KEY,
	tenant_id           TEXT NOT NULL,
	name                TEXT NOT NULL,
	role                TEXT NOT NULL,
	description         TEXT NOT NULL DEFAULT '',
	department_id       TEXT,
	status              TEXT NOT NULL DEFAULT 'active',
	current_version_id  TEXT,
	-- The nested value objects (objectives, memory access, runtime
	-- constraints, cost profile, budget, access control) are stored as one
	-- JSON document rather than a column each. They are read and written whole
	-- by the API and nothing queries inside them, so columns would buy
	-- nothing and drift from the Go structs on every field added.
	detail              JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_registry_agents_tenant ON registry_agents (tenant_id);
CREATE INDEX IF NOT EXISTS idx_registry_agents_dept ON registry_agents (department_id);

CREATE TABLE IF NOT EXISTS registry_agent_versions (
	id                   TEXT PRIMARY KEY,
	agent_id             TEXT NOT NULL,
	tenant_id            TEXT NOT NULL,
	version              TEXT NOT NULL,
	status               TEXT NOT NULL DEFAULT 'active',
	description          TEXT NOT NULL DEFAULT '',
	change_summary       TEXT NOT NULL DEFAULT '',
	diff_from_previous   TEXT,
	prompt_template_ref  TEXT,
	created_by           TEXT NOT NULL DEFAULT '',
	model_config         JSONB NOT NULL DEFAULT '{}'::jsonb,
	promoted_to          JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_registry_versions_agent ON registry_agent_versions (agent_id);
CREATE INDEX IF NOT EXISTS idx_registry_versions_tenant ON registry_agent_versions (tenant_id);
`

// Migrate creates the registry tables if they are absent.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
