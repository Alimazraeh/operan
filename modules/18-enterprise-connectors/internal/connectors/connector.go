package connectors

import (
	"context"
	"fmt"
	"sync"
)

// Connector is the interface that all enterprise connectors must implement.
type Connector interface {
	// Name returns the connector display name.
	Name() string

	// Type returns the connector type string (e.g., "smtp", "salesforce").
	Type() string

	// ValidateConfig validates the connector configuration.
	ValidateConfig(config map[string]interface{}) error

	// ValidateCredentials validates the credentials and returns a health check result.
	ValidateCredentials(ctx context.Context, credentials map[string]interface{}) (*HealthCheckResult, error)

	// Sync performs a data synchronization and returns sync results.
	Sync(ctx context.Context, credentials map[string]interface{}, config map[string]interface{}) (*SyncResult, error)

	// GetTools returns the tool definitions that this connector provides.
	GetTools() []ToolDefinition
}

// HealthCheckResult represents the result of a health check.
type HealthCheckResult struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message"`
}

// SyncResult represents the result of a sync operation.
type SyncResult struct {
	ObjectsFetched int      `json:"objects_fetched"`
	ObjectsUpdated int      `json:"objects_updated"`
	ObjectsFailed  int      `json:"objects_failed"`
	Errors         []string `json:"errors,omitempty"`
}

// ToolDefinition represents a tool that a connector provides to agents.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Returns     map[string]interface{} `json:"returns"`
}

// Registry manages connector implementations.
type Registry struct {
	connectors map[string]Connector
	mu         sync.RWMutex
}

// NewRegistry creates a new connector registry.
func NewRegistry() *Registry {
	return &Registry{
		connectors: make(map[string]Connector),
	}
}

// Register adds a connector to the registry.
func (r *Registry) Register(c Connector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connectors[c.Type()] = c
}

// Get retrieves a connector by type.
func (r *Registry) Get(connectorType string) (Connector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.connectors[connectorType]
	if !ok {
		return nil, fmt.Errorf("connector type %q not found in registry", connectorType)
	}
	return c, nil
}

// List returns all registered connector types.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]string, 0, len(r.connectors))
	for t := range r.connectors {
		types = append(types, t)
	}
	return types
}

// ListTools returns all tool definitions across all connectors.
func (r *Registry) ListTools() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var allTools []ToolDefinition
	for _, c := range r.connectors {
		allTools = append(allTools, c.GetTools()...)
	}
	return allTools
}