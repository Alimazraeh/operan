package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// AuditStore provides in-memory CRUD operations for audit events.
type AuditStore struct {
	mu       sync.RWMutex
	events   []*models.AuditEvent // ordered by timestamp
}

// NewAuditStore creates a new in-memory audit store.
func NewAuditStore() *AuditStore {
	return &AuditStore{
		events: make([]*models.AuditEvent, 0),
	}
}

// Create appends an audit event.
func (s *AuditStore) Create(event *models.AuditEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if event.Action == "" {
		return fmt.Errorf("action is required")
	}
	if event.ActorID == "" {
		return fmt.Errorf("actor_id is required")
	}
	if event.ActorType == "" {
		return fmt.Errorf("actor_type is required")
	}
	if event.ResourceType == "" {
		return fmt.Errorf("resource_type is required")
	}
	if event.ResourceID == "" {
		return fmt.Errorf("resource_id is required")
	}
	if event.Result == "" {
		return fmt.Errorf("result is required")
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Store details as JSON
	if event.Details != nil {
		data, err := json.Marshal(event.Details)
		if err != nil {
			return fmt.Errorf("marshal details: %w", err)
		}
		event.DetailsJSON = string(data)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, event)

	return nil
}

// List returns audit events for a tenant with filtering and pagination.
func (s *AuditStore) List(tenantID, actorID, action string, from, to *time.Time, limit, offset int) ([]models.AuditEvent, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Filter events
	var filtered []*models.AuditEvent
	for _, event := range s.events {
		if event.TenantID != tenantID {
			continue
		}
		if actorID != "" && event.ActorID != actorID {
			continue
		}
		if action != "" && event.Action != action {
			continue
		}
		if from != nil && event.Timestamp.Before(*from) {
			continue
		}
		if to != nil && event.Timestamp.After(*to) {
			continue
		}
		filtered = append(filtered, event)
	}

	total := len(filtered)

	// Sort by timestamp descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// Apply pagination
	if offset >= total {
		return []models.AuditEvent{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}

	result := make([]models.AuditEvent, 0, end-offset)
	for _, event := range filtered[offset:end] {
		e := *event
		e.Details = unmarshalMap(e.DetailsJSON)
		result = append(result, e)
	}

	return result, total, nil
}

// GetByID retrieves an audit event by ID.
func (s *AuditStore) GetByID(id string) (*models.AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, event := range s.events {
		if event.ID == id {
			e := *event
			e.Details = unmarshalMap(e.DetailsJSON)
			return &e, nil
		}
	}

	return nil, fmt.Errorf("audit event not found")
}

// ParseFilter parses the query parameters for GetAuditTrailsRequest.
func ParseFilter(raw string) map[string]string {
	result := make(map[string]string)
	if raw == "" {
		return result
	}

	for _, pair := range strings.Split(raw, "&") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}

	return result
}
