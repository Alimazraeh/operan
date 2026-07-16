package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// AuditStore handles policy audit log operations.
type AuditStore struct {
	Pool PgxPool
}

// NewAuditStore creates a new AuditStore.
func NewAuditStore(pool PgxPool) *AuditStore {
	return &AuditStore{Pool: pool}
}

// Create inserts a new audit record.
func (s *AuditStore) Create(ctx context.Context, a *PolicyAudit) error {
	query := `
		INSERT INTO policy_audits (tenant_id, policy_id, group_id, request_id, agent_id,
			resource_type, resource_target, requested_action, result,
			matched_policy_name, matched_rule_index, evaluation_ms, request_data)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at`

	var reqDataJSON []byte
	if a.RequestData != nil {
		reqDataJSON, _ = json.Marshal(a.RequestData)
	}

	var created time.Time
	err := s.Pool.QueryRow(ctx, query,
		a.TenantID, a.PolicyID, a.GroupID, a.RequestID, a.AgentID,
		a.ResourceType, a.ResourceTarget, a.RequestedAction, a.Result,
		a.MatchedPolicyName, a.MatchedRuleIndex, a.EvaluationMS, reqDataJSON,
	).Scan(&a.ID, &created)
	if err != nil {
		return err
	}
	a.CreatedAt = created
	return nil
}

// List returns paginated audit records with optional filters.
func (s *AuditStore) List(ctx context.Context, tenantID string,
	agentID, result *string, from, to *time.Time,
	page, pageSize int,
) ([]PolicyAudit, int, error) {
	offset := (page - 1) * pageSize

	query := `
		SELECT id, tenant_id, policy_id, group_id, request_id, agent_id,
			resource_type, resource_target, requested_action, result,
			matched_policy_name, matched_rule_index, evaluation_ms, request_data, created_at
		FROM policy_audits WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	argIdx := 2

	if agentID != nil {
		query += fmt.Sprintf(" AND agent_id = $%d", argIdx)
		args = append(args, *agentID)
		argIdx++
	}
	if result != nil {
		query += fmt.Sprintf(" AND result = $%d", argIdx)
		args = append(args, *result)
		argIdx++
	}
	if from != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *from)
		argIdx++
	}
	if to != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *to)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var audits []PolicyAudit
	for rows.Next() {
		a, err := scanAuditRow(rows)
		if err != nil {
			return nil, 0, err
		}
		audits = append(audits, *a)
	}

	// Count query
	countQuery := `SELECT COUNT(*) FROM policy_audits WHERE tenant_id = $1`
	countArgs := []interface{}{tenantID}
	ci := 2
	if agentID != nil {
		countQuery += fmt.Sprintf(" AND agent_id = $%d", ci)
		countArgs = append(countArgs, *agentID)
		ci++
	}
	if result != nil {
		countQuery += fmt.Sprintf(" AND result = $%d", ci)
		countArgs = append(countArgs, *result)
		ci++
	}
	if from != nil {
		countQuery += fmt.Sprintf(" AND created_at >= $%d", ci)
		countArgs = append(countArgs, *from)
		ci++
	}
	if to != nil {
		countQuery += fmt.Sprintf(" AND created_at <= $%d", ci)
		countArgs = append(countArgs, *to)
	}

	var total int
	err = s.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return audits, total, nil
}

// scanAuditRow scans a single audit row.
func scanAuditRow(rows interface{ Scan(...interface{}) error }) (*PolicyAudit, error) {
	a := &PolicyAudit{}
	var policyID, groupID, reqID, agentID, resTarget, matchedPolicy *string
	var reqDataJSON *string
	var matchedRuleIdx *int

	err := rows.Scan(
		&a.ID, &a.TenantID, &policyID, &groupID, &reqID, &agentID,
		&a.ResourceType, &resTarget, &a.RequestedAction, &a.Result,
		&matchedPolicy, &matchedRuleIdx, &a.EvaluationMS, &reqDataJSON, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	a.PolicyID = policyID
	a.GroupID = groupID
	a.RequestID = reqID
	a.AgentID = agentID
	a.ResourceTarget = resTarget
	a.MatchedPolicyName = matchedPolicy
	a.MatchedRuleIndex = matchedRuleIdx
	if reqDataJSON != nil && len(*reqDataJSON) > 0 {
		a.RequestData = jsonUnmarshal([]byte(*reqDataJSON))
	}
	return a, nil
}