// Package migrations applies the knowledge-ingestion schema at boot.
package migrations

import (
	"context"
	"embed"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var migrationFS embed.FS

// RunMigrations applies all *.sql files in name order. Every statement is
// idempotent (IF NOT EXISTS), so running at every boot is safe.
func RunMigrations(pool *pgxpool.Pool) error {
	entries, err := migrationFS.ReadDir(".")
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		sql, err := migrationFS.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(context.Background(), string(sql)); err != nil {
			return err
		}
	}
	return nil
}
