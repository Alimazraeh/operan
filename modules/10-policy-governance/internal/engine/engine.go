package engine

import (
	"context"
	"time"

	"github.com/operan/policy-governance/internal/ctxkeys"
	"github.com/operan/policy-governance/internal/events"
	"github.com/operan/policy-governance/internal/store"
)

// EventPublisher publishes events to the event broker (Kafka).
type EventPublisher interface {
	PublishEvaluation(ctx context.Context, event events.EvaluationEvent)
	PublishViolation(ctx context.Context, event events.ViolationEvent)
}

// policyStore defines the method needed by the evaluation engine.
type policyStore interface {
	ListActiveForTenant(ctx context.Context, tenantID string) ([]store.Policy, error)
}

// Engine is the core policy evaluation engine.
type Engine struct {
	policyStore policyStore
	ruleEngine  *RuleEngine
	cache       *Cache
	eventPub    EventPublisher
}

// NewEngine creates a new policy evaluation engine.
func NewEngine(ps policyStore, eventPub EventPublisher) *Engine {
	return &Engine{
		policyStore: ps,
		ruleEngine:  NewRuleEngine(),
		cache:       NewCache(5 * time.Minute),
		eventPub:    eventPub,
	}
}

// EvaluateRequest is the policy evaluation request.
type EvaluateRequest struct {
	TenantID      string
	AgentID       string
	AgentRole     string
	DepartmentID  string
	Resource      string
	ResourceType  string
	ActionType    string
	ResourceScope string
	DataClass     string
	Cost          float64
	Metadata      map[string]interface{}
}

// EvaluateResult is the policy evaluation result.
type EvaluateResult struct {
	Allowed    bool
	Action     string
	PolicyName string
	Reason     string
	Warnings   []string
	Rules      []RuleMatch
}

// RuleMatch represents a matched rule.
type RuleMatch struct {
	PolicyID    string
	PolicyName  string
	Effect      string
	Description string
}

// Evaluate checks policies against a request and returns the result.
func (e *Engine) Evaluate(ctx context.Context, req EvaluateRequest) (*EvaluateResult, error) {
	start := time.Now()

	cacheKey := buildCacheKey(req)
	if result, ok := e.cache.Get(cacheKey); ok {
		return result, nil
	}

	policies, err := e.policyStore.ListActiveForTenant(ctx, req.TenantID)
	if err != nil {
		return nil, err
	}

	reqCtx := buildRequestContext(req)

	result := &EvaluateResult{
		Allowed:  false,
		Action:   "deny",
		Reason:   "No matching policy — default deny",
		Warnings: []string{},
		Rules:    []RuleMatch{},
	}

	for _, policy := range policies {
		if !e.policyApplies(policy, req) {
			continue
		}

		condMatch := true
		if policy.ConditionExpression != nil {
			condMatch = e.ruleEngine.EvaluateConditions(policy.ConditionExpression, reqCtx)
		}
		if !condMatch {
			continue
		}

		desc := ""
		if policy.Description != nil {
			desc = *policy.Description
		}
		ruleMatch := RuleMatch{
			PolicyID:    policy.ID,
			PolicyName:  policy.Name,
			Effect:      policy.Effect,
			Description: desc,
		}

		switch policy.Effect {
		case "enforce":
			switch policy.Action {
			case "deny":
				result.Allowed = false
				result.Action = "deny"
				result.PolicyName = policy.Name
				result.Reason = "Policy denied: " + policy.Name
				result.Rules = append(result.Rules, ruleMatch)
				e.publishViolation(ctx, req, &policy, result)
				e.cache.Set(cacheKey, result)
				return result, nil
			case "allow":
				result.Allowed = true
				result.Action = "allow"
				result.PolicyName = policy.Name
				result.Reason = "Policy allowed: " + policy.Name
				result.Rules = append(result.Rules, ruleMatch)
				e.publishEvaluation(ctx, req, result, time.Since(start))
				e.cache.Set(cacheKey, result)
				return result, nil
			case "proxy":
				result.Allowed = false
				result.Action = "proxy"
				result.PolicyName = policy.Name
				result.Reason = "Policy requires proxy/approval: " + policy.Name
				result.Rules = append(result.Rules, ruleMatch)
			}
		case "warn":
			warnMsg := "Warning from policy '" + policy.Name + "'"
			result.Warnings = append(result.Warnings, warnMsg)
			result.Rules = append(result.Rules, ruleMatch)
		case "log":
			result.Rules = append(result.Rules, ruleMatch)
		}
	}

	e.cache.Set(cacheKey, result)
	e.publishEvaluation(ctx, req, result, time.Since(start))
	return result, nil
}

func (e *Engine) policyApplies(policy store.Policy, req EvaluateRequest) bool {
	scopeLevel := getScopeLevel(policy.Scope)
	// Global policies match all requests
	if scopeLevel == 4 {
		return true
	}
	reqLevel := getRequestScopeLevel(req)
	if reqLevel < scopeLevel {
		return false
	}
	if policy.ResourceType != "all" && policy.ResourceType != req.ResourceType {
		return false
	}
	if policy.ResourceTarget != nil && *policy.ResourceTarget != "" && *policy.ResourceTarget != req.Resource {
		return false
	}
	return true
}

func getScopeLevel(scope string) int {
	switch scope {
	case "agent":
		return 1
	case "department":
		return 2
	case "tenant":
		return 3
	case "global":
		return 4
	default:
		return 0
	}
}

func getRequestScopeLevel(req EvaluateRequest) int {
	level := 0
	if req.AgentID != "" {
		level = 1
	}
	if req.DepartmentID != "" {
		level = 2
	}
	if req.TenantID != "" {
		level = 3
	}
	return level
}

func buildRequestContext(req EvaluateRequest) map[string]interface{} {
	ctx := map[string]interface{}{
		"agent_id":   req.AgentID,
		"agent_role": req.AgentRole,
		"resource":   req.Resource,
		"action":     req.ActionType,
		"data_class": req.DataClass,
		"cost":       req.Cost,
	}
	if req.Metadata != nil {
		for k, v := range req.Metadata {
			ctx[k] = v
		}
	}
	return ctx
}

func (e *Engine) publishEvaluation(ctx context.Context, req EvaluateRequest, result *EvaluateResult, elapsed time.Duration) {
	if e.eventPub == nil {
		return
	}
	requestID := ctxkeys.GetRequestID(ctx)
	event := events.EvaluationEvent{
		TenantID:     req.TenantID,
		RequestID:    requestID,
		AgentID:      req.AgentID,
		Resource:     req.Resource,
		ActionType:   req.ActionType,
		Result:       result.Action,
		PolicyName:   result.PolicyName,
		EvaluationMS: elapsed.Milliseconds(),
		DataClass:    req.DataClass,
		Cost:         req.Cost,
	}
	e.eventPub.PublishEvaluation(ctx, event)
}

func (e *Engine) publishViolation(ctx context.Context, req EvaluateRequest, policy *store.Policy, result *EvaluateResult) {
	if e.eventPub == nil {
		return
	}
	requestID := ctxkeys.GetRequestID(ctx)
	event := events.ViolationEvent{
		TenantID:   req.TenantID,
		RequestID:  requestID,
		AgentID:    req.AgentID,
		Resource:   req.Resource,
		ActionType: req.ActionType,
		PolicyName: policy.Name,
		Result:     "denied",
	}
	e.eventPub.PublishViolation(ctx, event)
}

// InvalidateCache invalidates a specific cache entry.
func (e *Engine) InvalidateCache(req EvaluateRequest) {
	key := buildCacheKey(req)
	e.cache.Invalidate(key)
}

// buildCacheKey creates a unique cache key for a request.
func buildCacheKey(req EvaluateRequest) string {
	return req.TenantID + ":" + req.Resource + ":" + req.ActionType + ":" + req.DataClass
}