package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations applies all SQL migrations to the database.
func RunMigrations(pool *pgxpool.Pool) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS sandbox_profiles (
			id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id               VARCHAR(255) NOT NULL,
			name                    VARCHAR(200) NOT NULL,
			description             TEXT,
			cpu_cores               NUMERIC(5,2) NOT NULL DEFAULT 0.5,
			memory_mb               INT NOT NULL DEFAULT 256,
			timeout_seconds         INT NOT NULL DEFAULT 60,
			network_access          BOOLEAN NOT NULL DEFAULT false,
			allowed_tools           TEXT[] NOT NULL DEFAULT '{}',
			filesystem_access       BOOLEAN NOT NULL DEFAULT true,
			max_file_size_mb        INT NOT NULL DEFAULT 50,
			max_output_size_kb      INT NOT NULL DEFAULT 1024,
			is_active               BOOLEAN NOT NULL DEFAULT true,
			created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(tenant_id, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_profiles_tenant ON sandbox_profiles(tenant_id)`,
		`CREATE TABLE IF NOT EXISTS sandbox_instances (
			id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id               VARCHAR(255) NOT NULL,
			agent_id                VARCHAR(255),
			profile_id              UUID NOT NULL REFERENCES sandbox_profiles(id),
			tool_name               VARCHAR(200) NOT NULL,
			input_data              TEXT,
			exit_code               INT,
			stdout                  TEXT,
			stderr                  TEXT,
			status                  VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'timeout', 'failed', 'policy_denied')),
			cpu_time_ms             INT,
			memory_peak_mb          INT,
			error_message           TEXT,
			started_at              TIMESTAMPTZ,
			completed_at            TIMESTAMPTZ,
			created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_instances_tenant ON sandbox_instances(tenant_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_instances_agent  ON sandbox_instances(tenant_id, agent_id)`,
	}

	for _, m := range migrations {
		if _, err := pool.Exec(context.Background(), m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}