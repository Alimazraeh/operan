package store

import (
	"time"
)

// PolicyGroup represents a logical grouping of policies.
type PolicyGroup struct {
	ID          string                 `json:"id"`
	TenantID    string                 `json:"tenant_id"`
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	Priority    int                    `json:"priority"`
	IsActive    bool                   `json:"is_active"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Policy represents an individual policy rule.
type Policy struct {
	ID                   string                 `json:"id"`
	TenantID             string                 `json:"tenant_id"`
	GroupID              string                 `json:"group_id"`
	Name                 string                 `json:"name"`
	Description          *string                `json:"description,omitempty"`
	Action               string                 `json:"action"`
	Scope                string                 `json:"scope"`
	ResourceType         string                 `json:"resource_type"`
	ResourceTarget       *string                `json:"resource_target,omitempty"`
	ConditionExpression  map[string]interface{} `json:"condition_expression,omitempty"`
	Effect               string                 `json:"effect"`
	Priority             int                    `json:"priority"`
	IsActive             bool                   `json:"is_active"`
	CreatedBy            *string                `json:"created_by,omitempty"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
}

// PolicyAudit represents a policy evaluation audit record.
type PolicyAudit struct {
	ID                string                 `json:"id"`
	TenantID          string                 `json:"tenant_id"`
	PolicyID          *string                `json:"policy_id,omitempty"`
	GroupID           *string                `json:"group_id,omitempty"`
	RequestID         *string                `json:"request_id,omitempty"`
	AgentID           *string                `json:"agent_id,omitempty"`
	ResourceType      string                 `json:"resource_type"`
	ResourceTarget    *string                `json:"resource_target,omitempty"`
	RequestedAction   string                 `json:"requested_action"`
	Result            string                 `json:"result"`
	MatchedPolicyName *string                `json:"matched_policy_name,omitempty"`
	MatchedRuleIndex  *int                   `json:"matched_rule_index,omitempty"`
	EvaluationMS      int                    `json:"evaluation_ms"`
	RequestData       map[string]interface{} `json:"request_data,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
}

// PolicyFromRow converts raw SQL scan values into a Policy struct.
// Uses *any scans for nullable columns to work with pgxmock.
func PolicyFromRow(id, tenantID, groupID, name string, action, scope, resourceType, effect string,
	priority int, isActive bool, createdAt, updatedAt time.Time,
	desc, resTarget, createdBy *string, condExpr *[]byte,
) *Policy {
	p := &Policy{
		ID:            id,
		TenantID:      tenantID,
		GroupID:       groupID,
		Name:          name,
		Description:   desc,
		Action:        action,
		Scope:         scope,
		ResourceType:  resourceType,
		ResourceTarget: resTarget,
		Effect:        effect,
		Priority:      priority,
		IsActive:      isActive,
		CreatedBy:     createdBy,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
	if condExpr != nil && len(*condExpr) > 0 {
		p.ConditionExpression = jsonUnmarshal(*condExpr)
	}
	return p
}

// PolicyGroupFromRow converts raw SQL scan values into a PolicyGroup struct.
func PolicyGroupFromRow(id, tenantID, name string, priority int, isActive bool, createdAt, updatedAt time.Time,
	desc *string, meta map[string]interface{},
) *PolicyGroup {
	return &PolicyGroup{
		ID:          id,
		TenantID:    tenantID,
		Name:        name,
		Description: desc,
		Priority:    priority,
		IsActive:    isActive,
		Metadata:    meta,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}