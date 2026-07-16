package migrations

import (
	"context"
	"embed"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed 001_create_schema.sql
var migrationSQL string

// RunMigrations applies all database migrations.
func RunMigrations(pool *pgxpool.Pool) error {
	// Create the connector_definitions table
	_, err := pool.Exec(context.Background(), migrationSQL)
	if err != nil {
		return err
	}
	return nil
}

// EmbeddedMigrations contains all migration files.
var EmbeddedMigrations = embed.FS{}

// GetMigration returns a migration file by name.
func GetMigration(name string) (string, error) {
	data, err := EmbeddedMigrations.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Logger logs migration progress.
type Logger struct{}

// NewLogger creates a new migration logger.
func NewLogger() *Logger {
	return &Logger{}
}

// Info logs an informational message.
func (l *Logger) Info(msg string) {
	log.Printf("[migration] %s", msg)
}

// Error logs an error message.
func (l *Logger) Error(msg string) {
	log.Printf("[migration] ERROR: %s", msg)
}