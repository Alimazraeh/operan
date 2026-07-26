package engine

import (
	"context"
	"testing"
	"time"

	"github.com/operan/policy-governance/internal/events"
	"github.com/operan/policy-governance/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nilPublisher struct{}

func (n *nilPublisher) PublishEvaluation(ctx context.Context, event events.EvaluationEvent) {}
func (n *nilPublisher) PublishViolation(ctx context.Context, event events.ViolationEvent)   {}

func newTestEngine(t *testing.T) (*Engine, *mockPolicyStore) {
	t.Helper()
	ms := &mockPolicyStore{}
	e := NewEngine(ms, &nilPublisher{})
	return e, ms
}

type mockPolicyStore struct {
	policies []store.Policy
}

func (m *mockPolicyStore) ListActiveForTenant(ctx context.Context, tenantID string) ([]store.Policy, error) {
	var result []store.Policy
	for _, p := range m.policies {
		if p.TenantID == tenantID && p.IsActive {
			result = append(result, p)
		}
	}
	// Sort by priority descending (higher priority first)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Priority > result[i].Priority {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result, nil
}

func TestEvaluate_NoMatchingPolicies(t *testing.T) {
	e, _ := newTestEngine(t)
	result, err := e.Evaluate(context.Background(), EvaluateRequest{
		TenantID:   "tenant-1",
		Resource:   "send_email",
		ActionType: "send",
	})
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, "deny", result.Action)
	assert.Contains(t, result.Reason, "default deny")
}

func TestEvaluate_SingleAllowPolicy(t *testing.T) {
	e, ms := newTestEngine(t)
	ms.policies = []store.Policy{
		{
			TenantID: "tenant-1", Name: "allow-email", Action: "allow",
			Scope: "global", Effect: "enforce", ResourceType: "all", IsActive: true,
		},
	}
	result, err := e.Evaluate(context.Background(), EvaluateRequest{
		TenantID: "tenant-1", Resource: "send_email", ActionType: "send",
	})
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, "allow", result.Action)
	assert.Equal(t, "allow-email", result.PolicyName)
}

func TestEvaluate_SingleDenyPolicy(t *testing.T) {
	e, ms := newTestEngine(t)
	ms.policies = []store.Policy{
		{
			TenantID: "tenant-1", Name: "deny-email", Action: "deny",
			Scope: "global", Effect: "enforce", ResourceType: "all", IsActive: true,
		},
	}
	result, err := e.Evaluate(context.Background(), EvaluateRequest{
		TenantID: "tenant-1", Resource: "send_email", ActionType: "send",
	})
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, "deny", result.Action)
}

func TestEvaluate_DenyOverridesAllow(t *testing.T) {
	e, ms := newTestEngine(t)
	ms.policies = []store.Policy{
		{
			TenantID: "tenant-1", Name: "allow-email", Action: "allow",
			Scope: "global", Effect: "enforce", ResourceType: "all", IsActive: true,
		},
		{
			TenantID: "tenant-1", Name: "deny-email", Action: "deny",
			Scope: "global", Effect: "enforce", Priority: 90, IsActive: true,
		},
	}
	result, err := e.Evaluate(context.Background(), EvaluateRequest{
		TenantID: "tenant-1", Resource: "send_email", ActionType: "send",
	})
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, "deny", result.Action)
	assert.Equal(t, "deny-email", result.PolicyName)
}

func TestEvaluate_ProxyTriggersWarning(t *testing.T) {
	e, ms := newTestEngine(t)
	ms.policies = []store.Policy{
		{
			TenantID: "tenant-1", Name: "proxy-email", Action: "proxy",
			Scope: "global", Effect: "enforce", ResourceType: "all", IsActive: true,
		},
	}
	result, err := e.Evaluate(context.Background(), EvaluateRequest{
		TenantID: "tenant-1", Resource: "send_email", ActionType: "send",
	})
	require.NoError(t, err)
	assert.Equal(t, "proxy", result.Action)
	assert.Equal(t, "proxy-email", result.PolicyName)
}

func TestEvaluate_MultiplePoliciesPriorityOrdering(t *testing.T) {
	e, ms := newTestEngine(t)
	ms.policies = []store.Policy{
		{
			TenantID: "tenant-1", Name: "low-priority-allow", Action: "allow",
			Scope: "global", Effect: "enforce", Priority: 10, IsActive: true,
		},
		{
			TenantID: "tenant-1", Name: "high-priority-deny", Action: "deny",
			Scope: "global", Effect: "enforce", Priority: 90, IsActive: true,
		},
	}
	result, err := e.Evaluate(context.Background(), EvaluateRequest{
		TenantID: "tenant-1", Resource: "send_email", ActionType: "send",
	})
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, "high-priority-deny", result.PolicyName)
}

func TestCache_PolicyCacheHit(t *testing.T) {
	e, _ := newTestEngine(t)
	e.cache.Set("tenant-1:agent-1:send_email:send:", &EvaluateResult{
		Allowed: true, Action: "allow", PolicyName: "test-policy",
	})
	result, err := e.Evaluate(context.Background(), EvaluateRequest{
		TenantID: "tenant-1", AgentID: "agent-1", Resource: "send_email", ActionType: "send",
	})
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestCache_CacheMiss(t *testing.T) {
	e, _ := newTestEngine(t)
	assert.Equal(t, 0, e.cache.Size())
	result, err := e.Evaluate(context.Background(), EvaluateRequest{
		TenantID: "tenant-1", Resource: "send_email", ActionType: "send",
	})
	require.NoError(t, err)
	_ = result
	assert.Equal(t, 1, e.cache.Size())
}

func TestInvalidateCache(t *testing.T) {
	e, _ := newTestEngine(t)
	e.cache.Set("tenant-1:agent-1:test:action:", &EvaluateResult{Allowed: true})
	assert.Equal(t, 1, e.cache.Size())
	e.InvalidateCache(EvaluateRequest{
		TenantID: "tenant-1", AgentID: "agent-1", Resource: "test", ActionType: "action",
	})
	assert.Equal(t, 0, e.cache.Size())
}

// The cache key must include the actor: two agents in the same tenant asking
// about the same resource and action can be bound by different policies (an
// agent-scoped policy with a condition on agent_id, here) and each must get
// its own decision inside the cache's TTL — not whichever agent happened to
// ask first. Before the fix, buildCacheKey ignored the actor entirely, so
// the second agent's request was a cache *hit* against the first agent's
// answer.
func TestCache_DifferentActorsGetDifferentDecisions(t *testing.T) {
	e, ms := newTestEngine(t)
	ms.policies = []store.Policy{
		{
			TenantID: "tenant-1", Name: "allow-all", Action: "allow",
			Scope: "global", Effect: "enforce", ResourceType: "all", Priority: 10, IsActive: true,
		},
		{
			TenantID: "tenant-1", Name: "deny-agent-1", Action: "deny",
			Scope: "agent", Effect: "enforce", ResourceType: "all", Priority: 90, IsActive: true,
			ConditionExpression: map[string]interface{}{
				"field": "agent_id", "op": "eq", "value": "agent-1",
			},
		},
	}

	result1, err := e.Evaluate(context.Background(), EvaluateRequest{
		TenantID: "tenant-1", AgentID: "agent-1", Resource: "send_email", ActionType: "send",
	})
	require.NoError(t, err)
	result2, err := e.Evaluate(context.Background(), EvaluateRequest{
		TenantID: "tenant-1", AgentID: "agent-2", Resource: "send_email", ActionType: "send",
	})
	require.NoError(t, err)

	assert.False(t, result1.Allowed, "agent-1 is explicitly denied by its own policy")
	assert.True(t, result2.Allowed, "agent-2 falls through to the allow-all policy")

	// Both decisions are cached, under different keys — a re-ask inside the
	// TTL still gives agent-1 its own (denied) answer, not agent-2's.
	assert.Equal(t, 2, e.cache.Size())
	again1, err := e.Evaluate(context.Background(), EvaluateRequest{
		TenantID: "tenant-1", AgentID: "agent-1", Resource: "send_email", ActionType: "send",
	})
	require.NoError(t, err)
	assert.False(t, again1.Allowed, "agent-1's cached decision must still be its own")
}

func TestEvaluate_Timeout(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := e.Evaluate(ctx, EvaluateRequest{
		TenantID: "tenant-1", Resource: "send_email", ActionType: "send",
	})
	require.NoError(t, err)
}

func TestEvaluate_ActiveOnly(t *testing.T) {
	e, ms := newTestEngine(t)
	ms.policies = []store.Policy{
		{
			TenantID: "tenant-1", Name: "active-policy", Action: "allow",
			Scope: "global", Effect: "enforce", ResourceType: "all", IsActive: true,
		},
		{
			TenantID: "tenant-1", Name: "inactive-policy", Action: "deny",
			Scope: "global", Effect: "enforce", IsActive: false,
		},
	}
	result, err := e.Evaluate(context.Background(), EvaluateRequest{
		TenantID: "tenant-1", Resource: "send_email", ActionType: "send",
	})
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, "active-policy", result.PolicyName)
}

func TestEvaluate_InactivePoliciesIgnored(t *testing.T) {
	e, ms := newTestEngine(t)
	ms.policies = []store.Policy{
		{
			TenantID: "tenant-1", Name: "deny-policy", Action: "deny",
			Scope: "global", Effect: "enforce", IsActive: false,
		},
	}
	result, err := e.Evaluate(context.Background(), EvaluateRequest{
		TenantID: "tenant-1", Resource: "send_email", ActionType: "send",
	})
	require.NoError(t, err)
	// Default deny when no active policies match
	assert.False(t, result.Allowed)
}
