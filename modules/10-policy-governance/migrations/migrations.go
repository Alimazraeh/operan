// Package migrations applies the policy-governance schema at boot.
package migrations

import (
	"context"

	_ "embed"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed 001_create_schema.sql
var migrationSQL string

// RunMigrations applies all database migrations. The SQL is idempotent
// (CREATE TABLE IF NOT EXISTS), so running at every boot is safe.
func RunMigrations(pool *pgxpool.Pool) error {
	_, err := pool.Exec(context.Background(), migrationSQL)
	return err
}
