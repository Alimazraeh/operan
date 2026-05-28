package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── EscalationPostgres ───────────────────────────────────────────────────────

type EscalationPostgres struct {
	db *sql.DB
}

func (r *EscalationPostgres) Create(e *store.Escalation) error {
	ctx := defaultCtx()
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	e.CreatedAt = time.Now().UTC()
	if e.Status == "" {
		e.Status = store.EscalationPending
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO escalations
			(id, workflow_id, node_id, tenant_id, department_id,
			 status, severity, reason, escalated_to,
			 acknowledged_at, resolved_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		e.ID, e.WorkflowID, e.NodeID, e.TenantID, e.DepartmentID,
		e.Status, e.Severity, e.Reason, e.EscalatedTo,
		e.AcknowledgedAt, e.ResolvedAt,
	)
	return err
}

func (r *EscalationPostgres) GetByID(id string) (*store.Escalation, error) {
	ctx := defaultCtx()
	query := `SELECT id,workflow_id,node_id,tenant_id,department_id,
		status,severity,reason,escalated_to,created_at,
		acknowledged_at,resolved_at
		FROM escalations WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanEscalation(row)
}

func (r *EscalationPostgres) ListByWorkflow(workflowID string) ([]*store.Escalation, error) {
	ctx := defaultCtx()
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,workflow_id,node_id,tenant_id,department_id,
			status,severity,reason,escalated_to,created_at,
			acknowledged_at,resolved_at
			FROM escalations WHERE workflow_id=$1 ORDER BY created_at DESC`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*store.Escalation
	for rows.Next() {
		e, err := scanEscalation(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, nil
}

func (r *EscalationPostgres) Acknowledge(id string) error {
	ctx := defaultCtx()
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx,
		`UPDATE escalations SET status=$1, acknowledged_at=$2 WHERE id=$3 AND status=$4`,
		store.EscalationAcknowledged, now, id, store.EscalationPending)
	return err
}

func (r *EscalationPostgres) Resolve(id string) error {
	ctx := defaultCtx()
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx,
		`UPDATE escalations SET status=$1, resolved_at=$2 WHERE id=$3 AND status=$4`,
		store.EscalationResolved, now, id, store.EscalationAcknowledged)
	return err
}

// ─── scanEscalation ───────────────────────────────────────────────────────────

func scanEscalation(row scanner) (*store.Escalation, error) {
	var e store.Escalation
	var ackAt, resAt sql.NullTime

	err := row.Scan(
		&e.ID, &e.WorkflowID, &e.NodeID, &e.TenantID, &e.DepartmentID,
		&e.Status, &e.Severity, &e.Reason, &e.EscalatedTo,
		&e.CreatedAt, &ackAt, &resAt,
	)
	if err != nil {
		return nil, err
	}
	if ackAt.Valid {
		e.AcknowledgedAt = &ackAt.Time
	}
	if resAt.Valid {
		e.ResolvedAt = &resAt.Time
	}
	return &e, nil
}

// ─── RetryRecordPostgres ──────────────────────────────────────────────────────

type RetryRecordPostgres struct {
	db *sql.DB
}

func (r *RetryRecordPostgres) Create(rr *store.RetryRecord) error {
	ctx := defaultCtx()
	if rr.ID == "" {
		rr.ID = uuid.New().String()
	}
	rr.CreatedAt = time.Now().UTC()
	if rr.Status == "" {
		rr.Status = store.RetryPending
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO retry_records
			(id, workflow_id, node_id, tenant_id, attempt_number,
			 status, error_code, error_message, completed_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		rr.ID, rr.WorkflowID, rr.NodeID, rr.TenantID,
		rr.AttemptNumber, rr.Status, rr.ErrorCode, rr.ErrorMessage, rr.CompletedAt,
	)
	return err
}

func (r *RetryRecordPostgres) GetByID(id string) (*store.RetryRecord, error) {
	ctx := defaultCtx()
	query := `SELECT id,workflow_id,node_id,tenant_id,attempt_number,
		status,error_code,error_message,created_at,completed_at
		FROM retry_records WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanRetryRecord(row)
}

func (r *RetryRecordPostgres) ListByWorkflow(workflowID string) ([]*store.RetryRecord, error) {
	ctx := defaultCtx()
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,workflow_id,node_id,tenant_id,attempt_number,
			status,error_code,error_message,created_at,completed_at
			FROM retry_records WHERE workflow_id=$1 ORDER BY attempt_number ASC`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*store.RetryRecord
	for rows.Next() {
		rr, err := scanRetryRecord(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rr)
	}
	return results, nil
}

func (r *RetryRecordPostgres) UpdateStatus(id string, status store.RetryStatus, errorCode, errorMessage string) error {
	ctx := defaultCtx()
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx,
		`UPDATE retry_records SET status=$1, error_code=$2, error_message=$3, completed_at=$4
		 WHERE id=$5`,
		status, errorCode, errorMessage, &now, id)
	return err
}

// ─── scanRetryRecord ──────────────────────────────────────────────────────────

func scanRetryRecord(row scanner) (*store.RetryRecord, error) {
	var rr store.RetryRecord
	var completedAt sql.NullTime

	err := row.Scan(
		&rr.ID, &rr.WorkflowID, &rr.NodeID, &rr.TenantID,
		&rr.AttemptNumber, &rr.Status, &rr.ErrorCode, &rr.ErrorMessage,
		&rr.CreatedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		rr.CompletedAt = &completedAt.Time
	}
	return &rr, nil
}

// ─── StackHealthPostgres ──────────────────────────────────────────────────────

type StackHealthPostgres struct {
	db *sql.DB
}

func (r *StackHealthPostgres) Create(e *store.StackHealthEntry) error {
	ctx := defaultCtx()
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	e.At = time.Now().UTC()

	stacksBytes, _ := json.Marshal(e.Stacks)
	configBytes, _ := json.Marshal(e.Config)
	metadataBytes, _ := json.Marshal(e.Metadata)
	graphDefBytes, _ := json.Marshal(e.GraphDef)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO stack_health
			(id, tenant_id, stacks, at, stack_type, stack_name, status,
			 config, metadata, graph_def)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		e.ID, e.TenantID, stacksBytes, e.At,
		e.StackType, e.StackName, e.Status,
		configBytes, metadataBytes, graphDefBytes,
	)
	return err
}

func (r *StackHealthPostgres) GetByID(id string) (*store.StackHealthEntry, error) {
	ctx := defaultCtx()
	query := `SELECT id,tenant_id,stacks,at,stack_type,stack_name,status,
		config,metadata,graph_def
		FROM stack_health WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanStackHealth(row)
}

func (r *StackHealthPostgres) GetLatest() (*store.StackHealthEntry, error) {
	ctx := defaultCtx()
	query := `SELECT id,tenant_id,stacks,at,stack_type,stack_name,status,
		config,metadata,graph_def
		FROM stack_health ORDER BY at DESC LIMIT 1`
	row := r.db.QueryRowContext(ctx, query)
	return scanStackHealth(row)
}

func (r *StackHealthPostgres) Update(e *store.StackHealthEntry) error {
	ctx := defaultCtx()

	stacksBytes, _ := json.Marshal(e.Stacks)
	configBytes, _ := json.Marshal(e.Config)
	metadataBytes, _ := json.Marshal(e.Metadata)
	graphDefBytes, _ := json.Marshal(e.GraphDef)

	_, err := r.db.ExecContext(ctx,
		`UPDATE stack_health SET stacks=$1, at=$2, stack_type=$3, stack_name=$4,
		 status=$5, config=$6, metadata=$7, graph_def=$8
		 WHERE id=$9`,
		stacksBytes, e.At, e.StackType, e.StackName, e.Status,
		configBytes, metadataBytes, graphDefBytes, e.ID,
	)
	return err
}

func (r *StackHealthPostgres) Delete(id string) error {
	ctx := defaultCtx()
	_, err := r.db.ExecContext(ctx, "DELETE FROM stack_health WHERE id=$1", id)
	return err
}

func (r *StackHealthPostgres) ListByStack(tenantID, stackType string) ([]*store.StackHealthEntry, error) {
	ctx := defaultCtx()

	query := `SELECT id,tenant_id,stacks,at,stack_type,stack_name,status,
		config,metadata,graph_def
		FROM stack_health WHERE tenant_id=$1`
	args := []interface{}{tenantID}
	if stackType != "" {
		query += " AND stack_type=$2"
		args = append(args, stackType)
	}
	query += " ORDER BY at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*store.StackHealthEntry
	for rows.Next() {
		e, err := scanStackHealth(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, nil
}

// ─── scanStackHealth ──────────────────────────────────────────────────────────

func scanStackHealth(row scanner) (*store.StackHealthEntry, error) {
	var e store.StackHealthEntry
	var stacksBytes, configBytes, metadataBytes, graphDefBytes []byte

	err := row.Scan(
		&e.ID, &e.TenantID, &stacksBytes, &e.At,
		&e.StackType, &e.StackName, &e.Status,
		&configBytes, &metadataBytes, &graphDefBytes,
	)
	if err != nil {
		return nil, err
	}
	if len(stacksBytes) > 0 {
		_ = json.Unmarshal(stacksBytes, &e.Stacks)
	}
	if len(configBytes) > 0 {
		_ = json.Unmarshal(configBytes, &e.Config)
	}
	if len(metadataBytes) > 0 {
		_ = json.Unmarshal(metadataBytes, &e.Metadata)
	}
	if len(graphDefBytes) > 0 {
		_ = json.Unmarshal(graphDefBytes, &e.GraphDef)
	}
	return &e, nil
}
