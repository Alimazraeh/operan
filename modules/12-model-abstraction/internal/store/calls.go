package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CallsStore handles model_calls recording.
type CallsStore struct {
	pool *pgxpool.Pool
}

// NewCallsStore creates a new CallsStore.
func NewCallsStore(pool *pgxpool.Pool) *CallsStore {
	return &CallsStore{pool: pool}
}

// Create records a model call invocation.
func (s *CallsStore) Create(ctx context.Context, c *ModelCall) error {
	query := `
		INSERT INTO model_calls
			(tenant_id, agent_id, workflow_id, model_name, provider_id,
			 prompt_tokens, completion_tokens, total_tokens, cost_usd,
			 status, error_message, latency_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at`

	return s.pool.QueryRow(ctx, query,
		c.TenantID, c.AgentID, c.WorkflowID, c.ModelName, c.ProviderID,
		c.PromptTokens, c.CompletionTokens, c.TotalTokens, c.CostUSD,
		c.Status, c.ErrorMessage, c.LatencyMs,
	).Scan(&c.ID, &c.CreatedAt)
}

// ListByTenant returns paginated model calls.
func (s *CallsStore) ListByTenant(ctx context.Context, tenantID string, page, pageSize int) ([]ModelCall, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var items []ModelCall
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, agent_id, workflow_id, model_name, provider_id,
		       prompt_tokens, completion_tokens, total_tokens, cost_usd,
		       status, error_message, latency_ms, created_at
		FROM model_calls
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, tenantID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var c ModelCall
		if err := rows.Scan(
			&c.ID, &c.TenantID, &c.AgentID, &c.WorkflowID, &c.ModelName, &c.ProviderID,
			&c.PromptTokens, &c.CompletionTokens, &c.TotalTokens, &c.CostUSD,
			&c.Status, &c.ErrorMessage, &c.LatencyMs, &c.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, c)
	}

	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM model_calls WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetByID returns a model call by ID.
func (s *CallsStore) GetByID(ctx context.Context, id string) (*ModelCall, error) {
	var c ModelCall
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, agent_id, workflow_id, model_name, provider_id,
		       prompt_tokens, completion_tokens, total_tokens, cost_usd,
		       status, error_message, latency_ms, created_at
		FROM model_calls
		WHERE id = $1`, id).Scan(
		&c.ID, &c.TenantID, &c.AgentID, &c.WorkflowID, &c.ModelName, &c.ProviderID,
		&c.PromptTokens, &c.CompletionTokens, &c.TotalTokens, &c.CostUSD,
		&c.Status, &c.ErrorMessage, &c.LatencyMs, &c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// SumCostByTenant returns the total cost for a tenant over all recorded calls.
func (s *CallsStore) SumCostByTenant(ctx context.Context, tenantID string) (float64, error) {
	var total float64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cost_usd), 0) FROM model_calls WHERE tenant_id = $1`, tenantID).Scan(&total)
	return total, err
}

// SumCostByModel returns the total cost and token count for a specific model.
func (s *CallsStore) SumCostByModel(ctx context.Context, tenantID, modelName string) (cost float64, totalTokens int, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cost_usd), 0), COALESCE(SUM(total_tokens), 0)
		FROM model_calls
		WHERE tenant_id = $1 AND model_name = $2`, tenantID, modelName).Scan(&cost, &totalTokens)
	return
}