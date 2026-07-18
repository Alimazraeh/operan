package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// AuditStore is a persistent audit event store backed by PostgreSQL.
type AuditStore struct {
	pool *pgxpool.Pool
}

// NewAuditStore creates a new persistent audit store.
func NewAuditStore(pool *pgxpool.Pool) *AuditStore {
	return &AuditStore{pool: pool}
}

// Create persists an audit event to the database.
func (s *AuditStore) Create(ctx context.Context, event *models.AuditEvent) error {
	return s.CreateWithTenant(ctx, event, "")
}

// CreateWithTenant persists an audit event, optionally enforcing tenant isolation.
func (s *AuditStore) CreateWithTenant(ctx context.Context, event *models.AuditEvent, tenantID string) error {
	if event.ActorID == "" {
		return fmt.Errorf("actor_id is required")
	}
	if event.Action == "" {
		return fmt.Errorf("action is required")
	}

	detailsJSON := "{}"
	if event.Details != nil {
		bytes, err := json.Marshal(event.Details)
		if err != nil {
			return fmt.Errorf("marshal details: %w", err)
		}
		detailsJSON = string(bytes)
	}

	metadataJSON := "{}"
	if event.Metadata != nil {
		bytes, err := json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		metadataJSON = string(bytes)
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_events (tenant_id, action, actor_id, actor_type, resource_type, resource_id, result, details_json, metadata, severity)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, event.TenantID, event.Action, event.ActorID, event.ActorType,
		event.ResourceType, event.ResourceID, event.Result,
		detailsJSON, metadataJSON, event.Severity)

	if err != nil {
		return fmt.Errorf("create audit event: %w", err)
	}
	return nil
}

// List returns paginated audit events with optional filters.
func (s *AuditStore) List(ctx context.Context, tenantID, actorID, action string, from, to *time.Time, limit, offset int) ([]models.AuditEvent, int, error) {
	query := `
		SELECT id, tenant_id, action, actor_id, actor_type, resource_type, resource_id, result, details_json, metadata, severity, timestamp
		FROM audit_events WHERE 1=1
	`
	var args []interface{}
	argIdx := 1

	if tenantID != "" {
		query += fmt.Sprintf(" AND tenant_id=$%d", argIdx)
		args = append(args, tenantID)
		argIdx++
	}
	if actorID != "" {
		query += fmt.Sprintf(" AND actor_id=$%d", argIdx)
		args = append(args, actorID)
		argIdx++
	}
	if action != "" {
		query += fmt.Sprintf(" AND action=$%d", argIdx)
		args = append(args, action)
		argIdx++
	}
	if from != nil {
		query += fmt.Sprintf(" AND timestamp >= $%d", argIdx)
		args = append(args, *from)
		argIdx++
	}
	if to != nil {
		query += fmt.Sprintf(" AND timestamp <= $%d", argIdx)
		args = append(args, *to)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit events: %w", err)
	}
	defer rows.Close()

	var events []models.AuditEvent
	for rows.Next() {
		var event models.AuditEvent
		var detailsJSON, metadataJSON string
		if err := rows.Scan(
			&event.ID, &event.TenantID, &event.Action, &event.ActorID, &event.ActorType,
			&event.ResourceType, &event.ResourceID, &event.Result, &detailsJSON, &metadataJSON,
			&event.Severity, &event.Timestamp,
		); err != nil {
			return nil, 0, fmt.Errorf("scan audit event: %w", err)
		}
		if err := json.Unmarshal([]byte(detailsJSON), &event.Details); err != nil {
			return nil, 0, fmt.Errorf("parse details JSON: %w", err)
		}
		if err := json.Unmarshal([]byte(metadataJSON), &event.Metadata); err != nil {
			return nil, 0, fmt.Errorf("parse metadata JSON: %w", err)
		}
		events = append(events, event)
	}

	// Get total count
	countQuery := `SELECT COUNT(*) FROM audit_events WHERE 1=1`
	var countArgs []interface{}
	countArgIdx := 1

	if tenantID != "" {
		countQuery += fmt.Sprintf(" AND tenant_id=$%d", countArgIdx)
		countArgs = append(countArgs, tenantID)
		countArgIdx++
	}
	if actorID != "" {
		countQuery += fmt.Sprintf(" AND actor_id=$%d", countArgIdx)
		countArgs = append(countArgs, actorID)
		countArgIdx++
	}
	if action != "" {
		countQuery += fmt.Sprintf(" AND action=$%d", countArgIdx)
		countArgs = append(countArgs, action)
		countArgIdx++
	}
	if from != nil {
		countQuery += fmt.Sprintf(" AND timestamp >= $%d", countArgIdx)
		countArgs = append(countArgs, *from)
		countArgIdx++
	}
	if to != nil {
		countQuery += fmt.Sprintf(" AND timestamp <= $%d", countArgIdx)
		countArgs = append(countArgs, *to)
		countArgIdx++
	}

	var total int
	err = s.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count audit events: %w", err)
	}

	return events, total, nil
}

// GetByID retrieves a single audit event by ID.
func (s *AuditStore) GetByID(ctx context.Context, id string) (*models.AuditEvent, error) {
	var event models.AuditEvent
	var detailsJSON, metadataJSON string

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, action, actor_id, actor_type, resource_type, resource_id, result, details_json, metadata, severity, timestamp
		FROM audit_events WHERE id = $1
	`, id).Scan(
		&event.ID, &event.TenantID, &event.Action, &event.ActorID, &event.ActorType,
		&event.ResourceType, &event.ResourceID, &event.Result, &detailsJSON, &metadataJSON,
		&event.Severity, &event.Timestamp,
	)
	if err != nil {
		return nil, fmt.Errorf("get audit event by ID: %w", err)
	}

	if err := json.Unmarshal([]byte(detailsJSON), &event.Details); err != nil {
		return nil, fmt.Errorf("parse details JSON: %w", err)
	}
	if err := json.Unmarshal([]byte(metadataJSON), &event.Metadata); err != nil {
		return nil, fmt.Errorf("parse metadata JSON: %w", err)
	}

	return &event, nil
}