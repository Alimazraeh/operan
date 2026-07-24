package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// CostEventStore handles cost_events CRUD.
type CostEventStore struct {
	pool PgxPool
}

// NewCostEventStore creates a new CostEventStore.
func NewCostEventStore(pool PgxPool) *CostEventStore {
	return &CostEventStore{pool: pool}
}

// Create inserts a cost event and returns it with the generated ID.
func (s *CostEventStore) Create(ctx context.Context, e *CostEvent) error {
	e.ID = uuid.New().String()
	e.RecordedAt = time.Now()

	query := `
		INSERT INTO cost_events
			(tenant_id, agent_id, source_module, source_id, model_name,
			 cost_usd, prompt_tokens, completion_tokens, event_type,
			 billing_tag, event_timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, recorded_at`

	return s.pool.QueryRow(ctx, query,
		e.TenantID, e.AgentID, e.SourceModule, e.SourceID, e.ModelName,
		e.CostUSD, e.PromptTokens, e.CompletionTokens, e.EventType,
		e.BillingTag, e.EventTimestamp,
	).Scan(&e.ID, &e.RecordedAt)
}

// List returns paginated cost events for a tenant.
func (s *CostEventStore) List(ctx context.Context, tenantID, agentID, sourceModule string, page, pageSize int) ([]CostEvent, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var where string
	var args []interface{}
	argIdx := 1
	where += "WHERE tenant_id = $1"
	args = append(args, tenantID)
	argIdx++

	if agentID != "" {
		where += fmt.Sprintf(" AND agent_id = $%d", argIdx)
		args = append(args, agentID)
		argIdx++
	}
	if sourceModule != "" {
		where += fmt.Sprintf(" AND source_module = $%d", argIdx)
		args = append(args, sourceModule)
		argIdx++
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM cost_events " + where
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, agent_id, source_module, source_id, model_name,
		       cost_usd, prompt_tokens, completion_tokens, event_type,
		       billing_tag, event_timestamp, recorded_at
		FROM cost_events `+where+` ORDER BY event_timestamp DESC LIMIT $`+fmt.Sprint(argIdx)+` OFFSET $`+fmt.Sprint(argIdx+1)+``,
		append(args, pageSize, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []CostEvent
	for rows.Next() {
		var e CostEvent
		var agentID, sourceID, modelName, billingTag pgtype.Text
		if err := rows.Scan(
			&e.ID, &e.TenantID, &agentID, &e.SourceModule, &sourceID,
			&modelName, &e.CostUSD, &e.PromptTokens, &e.CompletionTokens,
			&e.EventType, &billingTag, &e.EventTimestamp, &e.RecordedAt,
		); err != nil {
			return nil, 0, err
		}
		if agentID.Valid {
			e.AgentID = &agentID.String
		}
		if sourceID.Valid {
			e.SourceID = &sourceID.String
		}
		if modelName.Valid {
			e.ModelName = &modelName.String
		}
		if billingTag.Valid {
			e.BillingTag = &billingTag.String
		}
		items = append(items, e)
	}

	return items, total, nil
}

// SumCostByTenant returns total cost for a tenant within a time range.
func (s *CostEventStore) SumCostByTenant(ctx context.Context, tenantID string, from, to time.Time) (float64, error) {
	var total float64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cost_usd), 0) FROM cost_events
		WHERE tenant_id = $1 AND event_timestamp >= $2 AND event_timestamp < $3`,
		tenantID, from, to,
	).Scan(&total)
	return total, err
}

// SumCostByTenantAndAgent returns total cost for a tenant+agent within a time range.
func (s *CostEventStore) SumCostByTenantAndAgent(ctx context.Context, tenantID, agentID string, from, to time.Time) (float64, error) {
	var total float64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cost_usd), 0) FROM cost_events
		WHERE tenant_id = $1 AND agent_id = $2 AND event_timestamp >= $3 AND event_timestamp < $4`,
		tenantID, agentID, from, to,
	).Scan(&total)
	return total, err
}

// GetTotalSpent returns total cost for a tenant (all time).
func (s *CostEventStore) GetTotalSpent(ctx context.Context, tenantID string) (float64, error) {
	var total float64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cost_usd), 0) FROM cost_events WHERE tenant_id = $1`, tenantID).Scan(&total)
	return total, err
}
