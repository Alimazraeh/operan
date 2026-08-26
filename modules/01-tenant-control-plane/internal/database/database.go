// Package database gives the tenant control plane durable storage.
//
// Until this existed the control plane was entirely in memory: ten stores —
// tenants, subscriptions, secrets, deployments, environments, namespaces,
// resources, invoices, payment methods, policies — all wiped on every pod
// restart. A control plane that forgets which tenants exist is not a control
// plane.
//
// The shape follows Module 04: memory stays the read path, every write also
// goes to PostgreSQL, and the process rehydrates at boot.
package database

import (
	"context"
	"encoding/json"
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

// schema is applied at every boot. Every statement is guarded, so running it
// against an already-migrated database is a no-op.
//
// The nested value objects (quotas, metadata, manifests, specs, rules, line
// items) are stored as one JSON document per row rather than one column per
// field. They are read and written whole by the API and nothing queries
// inside them, so columns would buy nothing and drift from the Go structs on
// every field added.
const schema = `
CREATE TABLE IF NOT EXISTS tctl_tenants (
	id               TEXT PRIMARY KEY,
	tenant_id        TEXT NOT NULL,
	name             TEXT NOT NULL,
	display_name     TEXT NOT NULL DEFAULT '',
	plan             TEXT NOT NULL,
	region           TEXT NOT NULL,
	isolation_level  TEXT NOT NULL,
	status           TEXT NOT NULL,
	quota            JSONB NOT NULL DEFAULT '{}'::jsonb,
	contact_email    TEXT NOT NULL DEFAULT '',
	custom_metadata  JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tctl_tenants_tenant ON tctl_tenants (tenant_id);

CREATE TABLE IF NOT EXISTS tctl_subscriptions (
	id                    TEXT PRIMARY KEY,
	tenant_id             TEXT NOT NULL,
	plan                  TEXT NOT NULL,
	plan_name             TEXT NOT NULL DEFAULT '',
	status                TEXT NOT NULL,
	billing_cycle         TEXT NOT NULL,
	seat_count            INTEGER NOT NULL DEFAULT 1,
	unit_price            DOUBLE PRECISION NOT NULL DEFAULT 0,
	total_amount          DOUBLE PRECISION NOT NULL DEFAULT 0,
	currency              TEXT NOT NULL DEFAULT 'USD',
	current_period_start  TIMESTAMPTZ NOT NULL,
	current_period_end    TIMESTAMPTZ NOT NULL,
	next_billing_date     TIMESTAMPTZ NOT NULL,
	cancel_at_period_end  BOOLEAN NOT NULL DEFAULT FALSE,
	cancelled_at          TIMESTAMPTZ,
	custom_quotas         JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tctl_subscriptions_tenant ON tctl_subscriptions (tenant_id);

CREATE TABLE IF NOT EXISTS tctl_secrets (
	id               TEXT PRIMARY KEY,
	tenant_id        TEXT NOT NULL,
	key              TEXT NOT NULL,
	encrypted_value  TEXT NOT NULL,
	description      TEXT NOT NULL DEFAULT '',
	tags             JSONB NOT NULL DEFAULT '[]'::jsonb,
	version          INTEGER NOT NULL DEFAULT 1,
	created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tctl_secrets_tenant ON tctl_secrets (tenant_id);

CREATE TABLE IF NOT EXISTS tctl_deployments (
	id             TEXT PRIMARY KEY,
	tenant_id      TEXT NOT NULL,
	name           TEXT NOT NULL,
	version        TEXT NOT NULL,
	status         TEXT NOT NULL,
	strategy       TEXT NOT NULL,
	manifest       JSONB NOT NULL DEFAULT '{}'::jsonb,
	desired_state  JSONB NOT NULL DEFAULT '{}'::jsonb,
	current_state  JSONB NOT NULL DEFAULT '{}'::jsonb,
	error          TEXT NOT NULL DEFAULT '',
	resource_refs  JSONB NOT NULL DEFAULT '[]'::jsonb,
	namespace_id   TEXT NOT NULL DEFAULT '',
	previous_id    TEXT,
	created_by     TEXT NOT NULL DEFAULT '',
	notes          TEXT NOT NULL DEFAULT '',
	deployed_at    TIMESTAMPTZ,
	deprecated_at  TIMESTAMPTZ,
	created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tctl_deployments_tenant ON tctl_deployments (tenant_id);

CREATE TABLE IF NOT EXISTS tctl_environments (
	id                 TEXT PRIMARY KEY,
	tenant_id          TEXT NOT NULL,
	name               TEXT NOT NULL,
	type               TEXT NOT NULL,
	state              TEXT NOT NULL,
	isolation_level    TEXT NOT NULL,
	isolation_config   JSONB NOT NULL DEFAULT '{}'::jsonb,
	resources          JSONB NOT NULL DEFAULT '[]'::jsonb,
	network_config     JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_by         TEXT NOT NULL DEFAULT '',
	notes              TEXT NOT NULL DEFAULT '',
	activated_at       TIMESTAMPTZ,
	deactivated_at     TIMESTAMPTZ,
	created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tctl_environments_tenant ON tctl_environments (tenant_id);

CREATE TABLE IF NOT EXISTS tctl_namespaces (
	id              TEXT PRIMARY KEY,
	tenant_id       TEXT NOT NULL,
	name            TEXT NOT NULL,
	description     TEXT NOT NULL DEFAULT '',
	status          TEXT NOT NULL,
	config          JSONB NOT NULL DEFAULT '{}'::jsonb,
	resource_quota  JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tctl_namespaces_tenant ON tctl_namespaces (tenant_id);

CREATE TABLE IF NOT EXISTS tctl_resources (
	id          TEXT PRIMARY KEY,
	tenant_id   TEXT NOT NULL,
	name        TEXT NOT NULL,
	type        TEXT NOT NULL,
	region      TEXT NOT NULL,
	spec        JSONB NOT NULL DEFAULT '{}'::jsonb,
	status      TEXT NOT NULL,
	endpoint    TEXT NOT NULL DEFAULT '',
	created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tctl_resources_tenant ON tctl_resources (tenant_id);

CREATE TABLE IF NOT EXISTS tctl_agents (
	id                TEXT PRIMARY KEY,
	tenant_id         TEXT NOT NULL,
	name              TEXT NOT NULL,
	model             TEXT NOT NULL DEFAULT '',
	role              TEXT NOT NULL DEFAULT '',
	system_prompt     TEXT NOT NULL DEFAULT '',
	status            TEXT NOT NULL,
	current_workflow  TEXT,
	current_task      TEXT,
	tool_access       JSONB NOT NULL DEFAULT '{}'::jsonb,
	last_run_at       TIMESTAMPTZ,
	success_count     INTEGER NOT NULL DEFAULT 0,
	failure_count     INTEGER NOT NULL DEFAULT 0,
	created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tctl_agents_tenant ON tctl_agents (tenant_id);

CREATE TABLE IF NOT EXISTS tctl_invoices (
	id               TEXT PRIMARY KEY,
	tenant_id        TEXT NOT NULL,
	subscription_id  TEXT NOT NULL DEFAULT '',
	issue_date       TIMESTAMPTZ NOT NULL,
	due_date         TIMESTAMPTZ NOT NULL,
	due_date_raw     TEXT NOT NULL DEFAULT '',
	amount           DOUBLE PRECISION NOT NULL DEFAULT 0,
	currency         TEXT NOT NULL DEFAULT 'USD',
	status           TEXT NOT NULL,
	line_items       JSONB NOT NULL DEFAULT '[]'::jsonb,
	paid_at          TIMESTAMPTZ,
	created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tctl_invoices_tenant ON tctl_invoices (tenant_id);

CREATE TABLE IF NOT EXISTS tctl_payment_methods (
	id                TEXT PRIMARY KEY,
	tenant_id         TEXT NOT NULL,
	type              TEXT NOT NULL,
	last_four         TEXT NOT NULL DEFAULT '',
	expiry_month      INTEGER NOT NULL DEFAULT 0,
	expiry_year       INTEGER NOT NULL DEFAULT 0,
	billing_address   TEXT NOT NULL DEFAULT '',
	is_default        BOOLEAN NOT NULL DEFAULT FALSE,
	created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tctl_payment_methods_tenant ON tctl_payment_methods (tenant_id);

CREATE TABLE IF NOT EXISTS tctl_policies (
	id            TEXT PRIMARY KEY,
	tenant_id     TEXT NOT NULL,
	name          TEXT NOT NULL,
	description   TEXT NOT NULL DEFAULT '',
	scope         TEXT NOT NULL,
	action        TEXT NOT NULL,
	rules         JSONB NOT NULL DEFAULT '{}'::jsonb,
	priority      TEXT NOT NULL,
	enabled       BOOLEAN NOT NULL DEFAULT TRUE,
	effect        TEXT NOT NULL DEFAULT '',
	last_eval_at  TIMESTAMPTZ,
	created_by    TEXT NOT NULL DEFAULT '',
	created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tctl_policies_tenant ON tctl_policies (tenant_id);
`

// Migrate creates the control-plane tables if they are absent.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// ControlPlaneStore reads and writes the durable rows for every store the
// control plane keeps in memory. One store type serves all ten tables so the
// boot path in main stays a single line per entity.
type ControlPlaneStore struct{ pool *pgxpool.Pool }

// NewControlPlaneStore returns a store over pool.
func NewControlPlaneStore(pool *pgxpool.Pool) *ControlPlaneStore {
	return &ControlPlaneStore{pool: pool}
}

// MarshalJSONB is a small helper so callers do not have to handle the nil case
// at every call site — a nil value must become '{}', not SQL NULL.
func MarshalJSONB(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 || string(b) == "null" {
		return []byte("{}"), nil
	}
	return b, nil
}

// MarshalJSONBArray is the array variant: a nil slice must become '[]'.
func MarshalJSONBArray(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 || string(b) == "null" {
		return []byte("[]"), nil
	}
	return b, nil
}

