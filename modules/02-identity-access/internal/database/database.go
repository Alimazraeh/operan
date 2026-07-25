package database

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migration represents a database migration file.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// LoadMigrations returns all migration files sorted by version number.
func LoadMigrations() ([]Migration, error) {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	var migrationList []Migration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "V") || !strings.HasSuffix(name, ".sql") {
			continue
		}

		// Extract version from filename like V001__init_schema.sql
		versionStr := strings.TrimPrefix(name, "V")
		versionStr = strings.Split(versionStr, "__")[0]
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			return nil, fmt.Errorf("parse migration version from %s: %w", name, err)
		}

		content, err := migrations.ReadFile(path.Join("migrations", name))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}

		migrationList = append(migrationList, Migration{
			Version: version,
			Name:    name,
			SQL:     string(content),
		})
	}

	// Sort by version ascending
	sort.Slice(migrationList, func(i, j int) bool {
		return migrationList[i].Version < migrationList[j].Version
	})

	return migrationList, nil
}

// RunMigrations applies all pending migrations to the database.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrationList, err := LoadMigrations()
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	// Create schema version table if not exists
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	// Get applied versions
	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("scan schema version: %w", err)
		}
		applied[version] = true
	}

	// Apply pending migrations
	for _, m := range migrationList {
		if applied[m.Version] {
			log.Printf("[MIGRATE] skip V%d %s (already applied)", m.Version, m.Name)
			continue
		}

		log.Printf("[MIGRATE] applying V%d %s...", m.Version, m.Name)
		if err := applyMigration(ctx, pool, m); err != nil {
			return fmt.Errorf("apply migration V%d %s: %w", m.Version, m.Name, err)
		}
	}

	log.Printf("[MIGRATE] all %d migrations applied successfully", len(migrationList))
	return nil
}

// applyMigration applies a single migration within a transaction.
func applyMigration(ctx context.Context, pool *pgxpool.Pool, m Migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Run migration SQL in chunks (split by semicolons)
	statements := splitStatements(m.SQL)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		_, err := tx.Exec(ctx, stmt)
		if err != nil {
			return fmt.Errorf("execute statement: %w", err)
		}
	}

	// Record migration
	_, err = tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", m.Version)
	if err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit(ctx)
}

// splitStatements splits SQL into individual statements by semicolon.
func splitStatements(sql string) []string {
	var statements []string
	var current strings.Builder

	runes := []rune(sql)
	flush := func() {
		if strings.TrimSpace(current.String()) != "" {
			statements = append(statements, current.String())
		}
		current.Reset()
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Line comment — copy to end of line; a ';' inside it is not a
		// statement boundary.
		if r == '-' && i+1 < len(runes) && runes[i+1] == '-' {
			for i < len(runes) && runes[i] != '\n' {
				current.WriteRune(runes[i])
				i++
			}
			if i < len(runes) {
				current.WriteRune(runes[i])
			}
			continue
		}

		// Block comment.
		if r == '/' && i+1 < len(runes) && runes[i+1] == '*' {
			for i < len(runes) {
				current.WriteRune(runes[i])
				if runes[i] == '/' && i > 0 && runes[i-1] == '*' && i > 1 {
					break
				}
				i++
			}
			continue
		}

		// Dollar-quoted body: $$...$$ or $tag$...$tag$. Everything inside is
		// literal, including semicolons — this is what a DO block uses, and
		// splitting through one produces fragments that are not valid SQL.
		if r == '$' {
			if tag, ok := dollarTag(runes, i); ok {
				current.WriteString(tag)
				i += len([]rune(tag))
				for i < len(runes) {
					if runes[i] == '$' {
						if t2, ok2 := dollarTag(runes, i); ok2 && t2 == tag {
							current.WriteString(tag)
							i += len([]rune(tag)) - 1
							break
						}
					}
					current.WriteRune(runes[i])
					i++
				}
				continue
			}
		}

		// Quoted string or identifier — '' and "" escape by doubling.
		if r == '\'' || r == '"' {
			quote := r
			current.WriteRune(r)
			i++
			for i < len(runes) {
				current.WriteRune(runes[i])
				if runes[i] == quote {
					if i+1 < len(runes) && runes[i+1] == quote {
						i++
						current.WriteRune(runes[i])
					} else {
						break
					}
				}
				i++
			}
			continue
		}

		if r == ';' {
			flush()
			continue
		}
		current.WriteRune(r)
	}
	flush()
	return statements
}

// dollarTag reports the dollar-quote delimiter starting at i ("$$" or "$tag$").
func dollarTag(runes []rune, i int) (string, bool) {
	if runes[i] != '$' {
		return "", false
	}
	for j := i + 1; j < len(runes); j++ {
		if runes[j] == '$' {
			return string(runes[i : j+1]), true
		}
		if !(runes[j] == '_' || (runes[j] >= 'a' && runes[j] <= 'z') ||
			(runes[j] >= 'A' && runes[j] <= 'Z') || (runes[j] >= '0' && runes[j] <= '9')) {
			return "", false
		}
	}
	return "", false
}

// HealthCheck verifies the database connection is working.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	return pool.Ping(ctx)
}

// NewPool creates a new PostgreSQL connection pool.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// ReadMigrationFile reads a migration file by version number.
func ReadMigrationFile(version int) (string, error) {
	migrations, err := LoadMigrations()
	if err != nil {
		return "", err
	}

	for _, m := range migrations {
		if m.Version == version {
			return m.SQL, nil
		}
	}

	return "", fmt.Errorf("migration V%d not found", version)
}

// GetAppliedVersions returns the list of applied migration versions.
func GetAppliedVersions(ctx context.Context, pool *pgxpool.Pool) ([]int, error) {
	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	var versions []int
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, version)
	}

	return versions, nil
}

// GetPendingMigrations returns migrations that haven't been applied yet.
func GetPendingMigrations(ctx context.Context, pool *pgxpool.Pool) ([]Migration, error) {
	applied, err := GetAppliedVersions(ctx, pool)
	if err != nil {
		return nil, err
	}

	appliedSet := make(map[int]bool)
	for _, v := range applied {
		appliedSet[v] = true
	}

	all, err := LoadMigrations()
	if err != nil {
		return nil, err
	}

	var pending []Migration
	for _, m := range all {
		if !appliedSet[m.Version] {
			pending = append(pending, m)
		}
	}

	return pending, nil
}

// ClosePool safely closes the database pool.
func ClosePool(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}

// Ensure io is used (for potential future logging/debugging).
var _ = io.Discard
