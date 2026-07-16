package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// PGPerfStore implements PerfStore against PostgreSQL.
type PGPerfStore struct {
	pool PgxPool
}

// NewPGPerfStore creates a new PostgreSQL-backed performance store.
func NewPGPerfStore(pool PgxPool) *PGPerfStore {
	return &PGPerfStore{pool: pool}
}

func (s *PGPerfStore) RecordMetric(metric *RoutingPerformance) error {
	now := time.Now()
	// Apply schema defaults for zero values
	if metric.QualityScore == 0 {
		metric.QualityScore = 50
	}

	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO routing_performance
		 (tenant_id, model_id, task_type, avg_latency_ms, p99_latency_ms, error_rate,
		  calls_count, avg_cost_usd, quality_score, last_call_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 ON CONFLICT (tenant_id, model_id, task_type) DO UPDATE SET
			 avg_latency_ms = EXCLUDED.avg_latency_ms,
			 p99_latency_ms = EXCLUDED.p99_latency_ms,
			 error_rate = EXCLUDED.error_rate,
			 calls_count = EXCLUDED.calls_count,
			 avg_cost_usd = EXCLUDED.avg_cost_usd,
			 quality_score = EXCLUDED.quality_score,
			 last_call_at = EXCLUDED.last_call_at,
			 updated_at = EXCLUDED.updated_at`,
		metric.TenantID, metric.ModelID, metric.TaskType, metric.AvgLatencyMs,
		metric.P99LatencyMs, metric.ErrorRate, metric.CallsCount, metric.AvgCostUSD,
		metric.QualityScore, metric.LastCallAt, now,
	)
	if err != nil {
		return fmt.Errorf("record metric: %w", err)
	}
	return nil
}

func (s *PGPerfStore) GetByModel(tenantID, modelID string) ([]RoutingPerformance, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, tenant_id, model_id, task_type, avg_latency_ms, p99_latency_ms,
		 error_rate, calls_count, avg_cost_usd, quality_score, last_call_at, updated_at
		 FROM routing_performance WHERE tenant_id = $1 AND model_id = $2`,
		tenantID, modelID,
	)
	if err != nil {
		return nil, fmt.Errorf("get perf by model: %w", err)
	}
	defer rows.Close()

	var metrics []RoutingPerformance
	for rows.Next() {
		var m RoutingPerformance
		if err := rows.Scan(
			&m.ID, &m.TenantID, &m.ModelID, &m.TaskType, &m.AvgLatencyMs,
			&m.P99LatencyMs, &m.ErrorRate, &m.CallsCount, &m.AvgCostUSD,
			&m.QualityScore, &m.LastCallAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan perf: %w", err)
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func (s *PGPerfStore) GetByTaskType(tenantID, taskType string) ([]RoutingPerformance, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, tenant_id, model_id, task_type, avg_latency_ms, p99_latency_ms,
		 error_rate, calls_count, avg_cost_usd, quality_score, last_call_at, updated_at
		 FROM routing_performance WHERE tenant_id = $1 AND task_type = $2`,
		tenantID, taskType,
	)
	if err != nil {
		return nil, fmt.Errorf("get perf by task: %w", err)
	}
	defer rows.Close()

	var metrics []RoutingPerformance
	for rows.Next() {
		var m RoutingPerformance
		var lastCallAt time.Time
		if err := rows.Scan(
			&m.ID, &m.TenantID, &m.ModelID, &m.TaskType, &m.AvgLatencyMs,
			&m.P99LatencyMs, &m.ErrorRate, &m.CallsCount, &m.AvgCostUSD,
			&m.QualityScore, &lastCallAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan perf: %w", err)
		}
		m.LastCallAt = &lastCallAt
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func (s *PGPerfStore) GetByModelAndTask(tenantID, modelID, taskType string) (*RoutingPerformance, error) {
	var m RoutingPerformance
	var lastCallAt time.Time
	err := s.pool.QueryRow(context.Background(),
		`SELECT id, tenant_id, model_id, task_type, avg_latency_ms, p99_latency_ms,
		 error_rate, calls_count, avg_cost_usd, quality_score, last_call_at, updated_at
		 FROM routing_performance WHERE tenant_id = $1 AND model_id = $2 AND task_type = $3`,
		tenantID, modelID, taskType,
	).Scan(
		&m.ID, &m.TenantID, &m.ModelID, &m.TaskType, &m.AvgLatencyMs,
		&m.P99LatencyMs, &m.ErrorRate, &m.CallsCount, &m.AvgCostUSD,
		&m.QualityScore, &lastCallAt, &m.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get perf by model+task: %w", err)
	}
	m.LastCallAt = &lastCallAt
	return &m, nil
}