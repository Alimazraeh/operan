package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── PipelinePostgres ─────────────────────────────────────────────────────────

type PipelinePostgres struct {
	db *sql.DB
}

func (r *PipelinePostgres) Create(p *store.Pipeline) error {
	ctx := defaultCtx()
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	if p.Status == "" {
		p.Status = store.PipelineStatusActive
	}

	stepsBytes, _ := json.Marshal(p.Steps)
	variablesBytes, _ := json.Marshal(p.Variables)
	tagsBytes, _ := json.Marshal(p.Tags)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO pipelines
			(id, tenant_id, name, description, steps, error_handling,
			 timeout_minutes, max_retries, trigger_type, variables,
			 status, execution_count, created_by, tags)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		p.ID, p.TenantID, p.Name, p.Description,
		stepsBytes, encodeJSONB(p.ErrorHandling),
		p.TimeoutMinutes, p.MaxRetries, p.TriggerType,
		variablesBytes, p.Status, p.ExecutionCount,
		p.CreatedBy, tagsBytes,
	)
	return err
}

func (r *PipelinePostgres) GetByID(id string) (*store.Pipeline, error) {
	ctx := defaultCtx()
	query := `SELECT id,tenant_id,name,description,steps,error_handling,
		timeout_minutes,max_retries,trigger_type,variables,
		status,execution_count,last_execution_at,success_rate,avg_duration_ms,
		created_by,created_at,updated_at,tags
		FROM pipelines WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanPipeline(row)
}

func (r *PipelinePostgres) GetByIDAndTenant(id, tenantID string) (*store.Pipeline, error) {
	ctx := defaultCtx()
	query := `SELECT id,tenant_id,name,description,steps,error_handling,
		timeout_minutes,max_retries,trigger_type,variables,
		status,execution_count,last_execution_at,success_rate,avg_duration_ms,
		created_by,created_at,updated_at,tags
		FROM pipelines WHERE id=$1 AND tenant_id=$2`
	row := r.db.QueryRowContext(ctx, query, id, tenantID)
	return scanPipeline(row)
}

func (r *PipelinePostgres) UpdateStatusAndTenant(id, tenantID string, status store.PipelineStatus) error {
	ctx := defaultCtx()
	result, err := r.db.ExecContext(ctx,
		"UPDATE pipelines SET status=$1 WHERE id=$2 AND tenant_id=$3", status, id, tenantID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("pipeline %s not found or tenant mismatch", id)
	}
	return nil
}

func (r *PipelinePostgres) Update(id string, name, description *string, steps []store.PipelineStep, errorHandling *store.PipelineErrorHandlingConfig, timeoutMinutes, maxRetries *int, status *store.PipelineStatus, variables *map[string]interface{}, tags *[]string) (*store.Pipeline, error) {
	ctx := defaultCtx()
	query := "UPDATE pipelines SET updated_at=now()"
	args := []interface{}{}
	argIdx := 1

	if name != nil {
		query += fmt.Sprintf(", name=$%d", argIdx)
		args = append(args, *name)
		argIdx++
	}
	if description != nil {
		query += fmt.Sprintf(", description=$%d", argIdx)
		args = append(args, *description)
		argIdx++
	}
	if steps != nil {
		bytes, _ := json.Marshal(steps)
		query += fmt.Sprintf(", steps=$%d", argIdx)
		args = append(args, bytes)
		argIdx++
	}
	if errorHandling != nil {
		query += fmt.Sprintf(", error_handling=$%d", argIdx)
		args = append(args, encodeJSONB(errorHandling))
		argIdx++
	}
	if timeoutMinutes != nil {
		query += fmt.Sprintf(", timeout_minutes=$%d", argIdx)
		args = append(args, *timeoutMinutes)
		argIdx++
	}
	if maxRetries != nil {
		query += fmt.Sprintf(", max_retries=$%d", argIdx)
		args = append(args, *maxRetries)
		argIdx++
	}
	if status != nil {
		query += fmt.Sprintf(", status=$%d", argIdx)
		args = append(args, *status)
		argIdx++
	}
	if variables != nil {
		bytes, _ := json.Marshal(variables)
		query += fmt.Sprintf(", variables=$%d", argIdx)
		args = append(args, bytes)
		argIdx++
	}
	if tags != nil {
		bytes, _ := json.Marshal(tags)
		query += fmt.Sprintf(", tags=$%d", argIdx)
		args = append(args, bytes)
		argIdx++
	}

	query += " WHERE id=$" + fmt.Sprint(argIdx)
	args = append(args, id)

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *PipelinePostgres) UpdateStatus(id string, status store.PipelineStatus) error {
	ctx := defaultCtx()
	_, err := r.db.ExecContext(ctx,
		"UPDATE pipelines SET status=$1, updated_at=now() WHERE id=$2", status, id)
	return err
}

func (r *PipelinePostgres) Delete(id string) error {
	ctx := defaultCtx()
	_, err := r.db.ExecContext(ctx, "DELETE FROM pipelines WHERE id=$1", id)
	return err
}

func (r *PipelinePostgres) List(tenantID string, page, pageSize int, status *string) ([]*store.Pipeline, int, error) {
	ctx := defaultCtx()
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	countQuery := "SELECT COUNT(*) FROM pipelines WHERE tenant_id=$1"
	params := []interface{}{tenantID}
	if status != nil && *status != "" {
		countQuery += " AND status=$2"
		params = append(params, *status)
	}
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, params...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `SELECT id,tenant_id,name,description,steps,error_handling,
		timeout_minutes,max_retries,trigger_type,variables,
		status,execution_count,last_execution_at,success_rate,avg_duration_ms,
		created_by,created_at,updated_at,tags
		FROM pipelines WHERE tenant_id=$1`
	if status != nil && *status != "" {
		query += " AND status=$2"
	}
	query += " ORDER BY created_at DESC LIMIT $3 OFFSET $4"
	params = append(params, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []*store.Pipeline
	for rows.Next() {
		p, err := scanPipeline(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, p)
	}
	return results, total, nil
}

func (r *PipelinePostgres) IncrementExecutionCount(pipelineID string, success bool) error {
	ctx := defaultCtx()
	_, err := r.db.ExecContext(ctx,
		`UPDATE pipelines
		 SET execution_count=execution_count+1,
		       success_rate=CASE WHEN execution_count=0 THEN 100.0
		              ELSE (success_rate * execution_count + CASE WHEN $2 THEN 1 ELSE 0 END) / (execution_count + 1)
		           END
		 WHERE id=$1`, pipelineID, success)
	return err
}

// ─── scanPipeline ─────────────────────────────────────────────────────────────

func scanPipeline(row scanner) (*store.Pipeline, error) {
	var p store.Pipeline
	var stepsBytes, errorHandlingBytes, variablesBytes, tagsBytes []byte
	var lastExecAt sql.NullTime

	err := row.Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description,
		&stepsBytes, &errorHandlingBytes,
		&p.TimeoutMinutes, &p.MaxRetries, &p.TriggerType,
		&variablesBytes, &p.Status, &p.ExecutionCount,
		&lastExecAt, &p.SuccessRate, &p.AvgDurationMS,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt, &tagsBytes,
	)
	if err != nil {
		return nil, err
	}

	if lastExecAt.Valid {
		p.LastExecutionAt = &lastExecAt.Time
	}
	if len(stepsBytes) > 0 {
		_ = json.Unmarshal(stepsBytes, &p.Steps)
	}
	if len(errorHandlingBytes) > 0 {
		_ = json.Unmarshal(errorHandlingBytes, &p.ErrorHandling)
	}
	if len(variablesBytes) > 0 {
		_ = json.Unmarshal(variablesBytes, &p.Variables)
	}
	if len(tagsBytes) > 0 {
		_ = json.Unmarshal(tagsBytes, &p.Tags)
	}
	return &p, nil
}

// ─── ExecutionPostgres ─────────────────────────────────────────────────────────

type ExecutionPostgres struct {
	db *sql.DB
}

func (r *ExecutionPostgres) Create(e *store.PipelineExecution) error {
	ctx := defaultCtx()
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	e.CreatedAt = time.Now().UTC()
	if e.Status == "" {
		e.Status = store.PipelineExecutionPending
	}

	inputsBytes, _ := json.Marshal(e.Inputs)
	outputsBytes, _ := json.Marshal(e.Outputs)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO executions
			(id, pipeline_id, tenant_id, status, inputs, outputs,
			 current_step_id, current_step_status, error_message,
			 retry_count, duration_ms, started_at, completed_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		e.ID, e.PipelineID, e.TenantID, e.Status,
		inputsBytes, outputsBytes,
		e.CurrentStepID, e.CurrentStepStatus, e.ErrorMessage,
		e.RetryCount, e.DurationMS,
		e.StartedAt, e.CompletedAt,
	)
	return err
}

func (r *ExecutionPostgres) GetByID(id string) (*store.PipelineExecution, error) {
	ctx := defaultCtx()
	query := `SELECT id,pipeline_id,tenant_id,status,inputs,outputs,
		current_step_id,current_step_status,error_message,
		retry_count,duration_ms,started_at,completed_at,created_at
		FROM executions WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanExecution(row)
}

func (r *ExecutionPostgres) GetByIDAndTenant(id, tenantID string) (*store.PipelineExecution, error) {
	ctx := defaultCtx()
	query := `SELECT id,pipeline_id,tenant_id,status,inputs,outputs,
		current_step_id,current_step_status,error_message,
		retry_count,duration_ms,started_at,completed_at,created_at
		FROM executions WHERE id=$1 AND tenant_id=$2`
	row := r.db.QueryRowContext(ctx, query, id, tenantID)
	return scanExecution(row)
}

func (r *ExecutionPostgres) UpdateStatusAndTenant(id, tenantID string, status store.PipelineExecutionStatus) error {
	ctx := defaultCtx()
	result, err := r.db.ExecContext(ctx,
		`UPDATE executions SET status=$1,
		 started_at=COALESCE(started_at, now()),
		 completed_at=CASE WHEN $1 IN ('completed','failed','cancelled') THEN now() ELSE completed_at END
		 WHERE id=$2 AND tenant_id=$3`, status, id, tenantID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("execution %s not found or tenant mismatch", id)
	}
	return nil
}

func (r *ExecutionPostgres) UpdateStatus(id string, status store.PipelineExecutionStatus) error {
	ctx := defaultCtx()
	_, err := r.db.ExecContext(ctx,
		`UPDATE executions SET status=$1,
		 started_at=COALESCE(started_at, now()),
		 completed_at=CASE WHEN $1 IN ('completed','failed','cancelled') THEN now() ELSE completed_at END
		 WHERE id=$2`, status, id)
	return err
}

func (r *ExecutionPostgres) Delete(id string) error {
	ctx := defaultCtx()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "DELETE FROM execution_steps WHERE execution_id=$1", id)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM executions WHERE id=$1", id)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *ExecutionPostgres) ListByPipeline(pipelineID string, page, pageSize int, status *string) ([]*store.PipelineExecution, int, error) {
	ctx := defaultCtx()
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	countQuery := "SELECT COUNT(*) FROM executions WHERE pipeline_id=$1"
	params := []interface{}{pipelineID}
	if status != nil && *status != "" {
		countQuery += " AND status=$2"
		params = append(params, *status)
	}
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, params...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `SELECT id,pipeline_id,tenant_id,status,inputs,outputs,
		current_step_id,current_step_status,error_message,
		retry_count,duration_ms,started_at,completed_at,created_at
		FROM executions WHERE pipeline_id=$1`
	if status != nil && *status != "" {
		query += " AND status=$2"
	}
	query += " ORDER BY created_at DESC LIMIT $3 OFFSET $4"
	params = append(params, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []*store.PipelineExecution
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, e)
	}
	return results, total, nil
}

func (r *ExecutionPostgres) ListByTenant(tenantID string, page, pageSize int) ([]*store.PipelineExecution, int, error) {
	ctx := defaultCtx()
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	countQuery := "SELECT COUNT(*) FROM executions WHERE tenant_id=$1"
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, tenantID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `SELECT id,pipeline_id,tenant_id,status,inputs,outputs,
		current_step_id,current_step_status,error_message,
		retry_count,duration_ms,started_at,completed_at,created_at
		FROM executions WHERE tenant_id=$1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, tenantID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []*store.PipelineExecution
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, e)
	}
	return results, total, nil
}

func (r *ExecutionPostgres) AddStep(step *store.PipelineExecutionStep) error {
	ctx := defaultCtx()
	if step.ID == "" {
		step.ID = uuid.New().String()
	}

	inputsBytes, _ := json.Marshal(step.Inputs)
	outputsBytes, _ := json.Marshal(step.Outputs)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO execution_steps
			(id, execution_id, step_id, step_name, status, inputs, outputs,
			 error_message, started_at, completed_at, duration_ms)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		step.ID, step.ExecutionID, step.StepID, step.StepName, step.Status,
		inputsBytes, outputsBytes, step.ErrorMessage,
		step.StartedAt, step.CompletedAt, step.DurationMS,
	)
	return err
}

func (r *ExecutionPostgres) GetSteps(executionID string) ([]*store.PipelineExecutionStep, error) {
	ctx := defaultCtx()
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,execution_id,step_id,step_name,status,inputs,outputs,
			error_message,started_at,completed_at,duration_ms
			FROM execution_steps WHERE execution_id=$1 ORDER BY created_at`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*store.PipelineExecutionStep
	for rows.Next() {
		step, err := scanExecutionStep(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, step)
	}
	return results, nil
}

func (r *ExecutionPostgres) IncrementRetryCount(id string) (int, error) {
	ctx := defaultCtx()
	var count int
	err := r.db.QueryRowContext(ctx,
		`UPDATE executions SET retry_count=retry_count+1 WHERE id=$1
		 RETURNING retry_count`, id).Scan(&count)
	return count, err
}

// ─── scanExecution ────────────────────────────────────────────────────────────

func scanExecution(row scanner) (*store.PipelineExecution, error) {
	var e store.PipelineExecution
	var inputsBytes, outputsBytes []byte
	var startedAt, completedAt sql.NullTime

	err := row.Scan(
		&e.ID, &e.PipelineID, &e.TenantID, &e.Status,
		&inputsBytes, &outputsBytes,
		&e.CurrentStepID, &e.CurrentStepStatus, &e.ErrorMessage,
		&e.RetryCount, &e.DurationMS,
		&startedAt, &completedAt, &e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if startedAt.Valid {
		e.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		e.CompletedAt = &completedAt.Time
	}
	if len(inputsBytes) > 0 {
		_ = json.Unmarshal(inputsBytes, &e.Inputs)
	}
	if len(outputsBytes) > 0 {
		_ = json.Unmarshal(outputsBytes, &e.Outputs)
	}
	return &e, nil
}

// ─── scanExecutionStep ─────────────────────────────────────────────────────────

func scanExecutionStep(row scanner) (*store.PipelineExecutionStep, error) {
	var step store.PipelineExecutionStep
	var inputsBytes, outputsBytes []byte
	var startedAt, completedAt sql.NullTime

	err := row.Scan(
		&step.ID, &step.ExecutionID, &step.StepID, &step.StepName,
		&step.Status, &inputsBytes, &outputsBytes,
		&step.ErrorMessage, &startedAt, &completedAt, &step.DurationMS,
	)
	if err != nil {
		return nil, err
	}

	if startedAt.Valid {
		step.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		step.CompletedAt = &completedAt.Time
	}
	if len(inputsBytes) > 0 {
		_ = json.Unmarshal(inputsBytes, &step.Inputs)
	}
	if len(outputsBytes) > 0 {
		_ = json.Unmarshal(outputsBytes, &step.Outputs)
	}
	return &step, nil
}
