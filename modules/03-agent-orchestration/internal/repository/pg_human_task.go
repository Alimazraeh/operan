package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── HumanTaskPostgres ────────────────────────────────────────────────────────

type HumanTaskPostgres struct {
	db *sql.DB
}

func (r *HumanTaskPostgres) Create(t *store.HumanTask) error {
	ctx := defaultCtx()
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	t.CreatedAt = time.Now().UTC()
	if t.Status == "" {
		t.Status = store.HumanTaskStatusPending
	}

	contextBytes, _ := json.Marshal(t.Context)
	responseBytes, _ := json.Marshal(t.Response)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO human_tasks
			(id, tenant_id, pipeline_execution_id, step_id,
			 assignee_type, assignee_id, task_type, instructions,
			 context, timeout_minutes, label, priority, status,
			 response, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		t.ID, t.TenantID, t.PipelineExecutionID, t.StepID,
		t.AssigneeType, t.AssigneeID, t.TaskType, t.Instructions,
		contextBytes, t.TimeoutMinutes, t.Label, t.Priority,
		t.Status, responseBytes, t.CreatedAt,
	)
	return err
}

func (r *HumanTaskPostgres) GetByID(id string) (*store.HumanTask, error) {
	ctx := defaultCtx()
	query := `SELECT id,tenant_id,pipeline_execution_id,step_id,
		assignee_type,assignee_id,task_type,instructions,
		context,timeout_minutes,label,priority,status,
		response,responded_by,responded_at,created_at
		FROM human_tasks WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanHumanTask(row)
}

func (r *HumanTaskPostgres) Respond(id string, action string, response map[string]interface{}, respondedBy, comments string) (*store.HumanTask, error) {
	ctx := defaultCtx()

	// Determine target status based on action
	targetStatus := store.HumanTaskStatusApproved
	if action == "reject" || action == "request_info" {
		targetStatus = store.HumanTaskStatusRejected
	}

	responseBytes, _ := json.Marshal(response)

	_, err := r.db.ExecContext(ctx,
		`UPDATE human_tasks
		 SET status=$1, response=$2, responded_by=$3, responded_at=now()
		 WHERE id=$4 AND status=$5`,
		targetStatus, responseBytes, respondedBy, id, store.HumanTaskStatusPending)
	if err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *HumanTaskPostgres) List(tenantID string, status *string) ([]*store.HumanTask, int, error) {
	ctx := defaultCtx()

	query := `SELECT COUNT(*) FROM human_tasks WHERE tenant_id=$1`
	params := []interface{}{tenantID}
	if status != nil && *status != "" {
		query += " AND status=$2"
		params = append(params, *status)
	}
	var total int
	err := r.db.QueryRowContext(ctx, query, params...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query = `SELECT id,tenant_id,pipeline_execution_id,step_id,
		assignee_type,assignee_id,task_type,instructions,
		context,timeout_minutes,label,priority,status,
		response,responded_by,responded_at,created_at
		FROM human_tasks WHERE tenant_id=$1`
	if status != nil && *status != "" {
		query += " AND status=$2"
	}
	query += " ORDER BY created_at DESC"
	params = append(params)

	rows, err := r.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []*store.HumanTask
	for rows.Next() {
		t, err := scanHumanTask(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, t)
	}
	return results, total, nil
}

// ─── scanHumanTask ────────────────────────────────────────────────────────────

func scanHumanTask(row scanner) (*store.HumanTask, error) {
	var t store.HumanTask
	var contextBytes, responseBytes []byte
	var respondedAt sql.NullTime

	err := row.Scan(
		&t.ID, &t.TenantID, &t.PipelineExecutionID, &t.StepID,
		&t.AssigneeType, &t.AssigneeID, &t.TaskType, &t.Instructions,
		&contextBytes, &t.TimeoutMinutes, &t.Label, &t.Priority,
		&t.Status, &responseBytes, &t.RespondedBy, &respondedAt, &t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if respondedAt.Valid {
		t.RespondedAt = &respondedAt.Time
	}
	if len(contextBytes) > 0 {
		_ = json.Unmarshal(contextBytes, &t.Context)
	}
	if len(responseBytes) > 0 {
		_ = json.Unmarshal(responseBytes, &t.Response)
	}
	return &t, nil
}
