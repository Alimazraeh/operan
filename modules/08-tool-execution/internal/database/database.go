// Package database gives the capability layer durable storage.
//
// Bindings are customer configuration and invocations are the audit trail —
// neither may vanish on a pod restart. Same shape as Module 04: memory is the
// read path, writes go through, the process rehydrates at boot, and a service
// configured to persist but unable to reach the database refuses to start
// rather than answering as though it can.
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pool and verifies it answers.
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

// schema runs at every boot; each statement is guarded so re-running is a
// no-op (bare CREATE INDEX crash-looped four other modules).
const schema = `
CREATE TABLE IF NOT EXISTS m08_providers (
	id          TEXT PRIMARY KEY,
	tenant_id   TEXT NOT NULL,
	doc         JSONB NOT NULL,
	updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_m08_providers_tenant ON m08_providers (tenant_id);

CREATE TABLE IF NOT EXISTS m08_bindings (
	id          TEXT PRIMARY KEY,
	tenant_id   TEXT NOT NULL,
	doc         JSONB NOT NULL,
	updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_m08_bindings_tenant ON m08_bindings (tenant_id);

CREATE TABLE IF NOT EXISTS m08_invocations (
	id          TEXT PRIMARY KEY,
	tenant_id   TEXT NOT NULL,
	doc         JSONB NOT NULL,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_m08_invocations_tenant ON m08_invocations (tenant_id, created_at);
`

// Migrate creates the tables if absent.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Store reads and writes the three durable collections. Rows are whole JSON
// documents: the API reads and writes them whole, nothing queries inside them
// beyond tenant, and column-per-field would drift from the Go structs on
// every addition.
type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) upsertDoc(ctx context.Context, table, id, tenantID string, v interface{}) error {
	doc, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode %s %s: %w", table, id, err)
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO `+table+` (id, tenant_id, doc, updated_at) VALUES ($1,$2,$3,NOW())
		 ON CONFLICT (id) DO UPDATE SET doc = EXCLUDED.doc, updated_at = NOW()`,
		id, tenantID, doc)
	if err != nil {
		return fmt.Errorf("upsert %s %s: %w", table, id, err)
	}
	return nil
}

func (s *Store) UpsertProvider(ctx context.Context, id, tenantID string, v interface{}) error {
	return s.upsertDoc(ctx, "m08_providers", id, tenantID, v)
}

func (s *Store) UpsertBinding(ctx context.Context, id, tenantID string, v interface{}) error {
	return s.upsertDoc(ctx, "m08_bindings", id, tenantID, v)
}

// InsertInvocation is insert-only: the audit trail is immutable, and an id
// collision is a fault worth surfacing, not absorbing.
func (s *Store) InsertInvocation(ctx context.Context, id, tenantID string, v interface{}) error {
	doc, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode invocation %s: %w", id, err)
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO m08_invocations (id, tenant_id, doc) VALUES ($1,$2,$3)`, id, tenantID, doc)
	if err != nil {
		return fmt.Errorf("insert invocation %s: %w", id, err)
	}
	return nil
}

// LoadAll streams every row of a table's doc column, for boot rehydration.
func (s *Store) LoadAll(ctx context.Context, table string, each func(doc []byte) error) error {
	rows, err := s.pool.Query(ctx, `SELECT doc FROM `+table+` ORDER BY updated_at`)
	if table == "m08_invocations" {
		rows, err = s.pool.Query(ctx, `SELECT doc FROM m08_invocations ORDER BY created_at`)
	}
	if err != nil {
		return fmt.Errorf("load %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var doc []byte
		if err := rows.Scan(&doc); err != nil {
			return fmt.Errorf("scan %s: %w", table, err)
		}
		if err := each(doc); err != nil {
			return err
		}
	}
	return rows.Err()
}
