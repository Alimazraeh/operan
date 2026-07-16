package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuleEngine_EvaluateSimpleEq(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "action",
		"op":    "eq",
		"value": "send_email",
	}
	ctx := map[string]interface{}{"action": "send_email"}
	assert.True(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateNeq(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "data_class",
		"op":    "neq",
		"value": "restricted",
	}
	ctx := map[string]interface{}{"data_class": "internal"}
	assert.True(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateIn(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "action",
		"op":    "in",
		"value": []interface{}{"send_email", "send_invoice"},
	}
	ctx := map[string]interface{}{"action": "send_invoice"}
	assert.True(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateNotIn(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "data_class",
		"op":    "not_in",
		"value": []interface{}{"restricted"},
	}
	ctx := map[string]interface{}{"data_class": "confidential"}
	assert.True(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateLt(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "cost",
		"op":    "lt",
		"value": 1000,
	}
	ctx := map[string]interface{}{"cost": 500.0}
	assert.True(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateGt(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "cost",
		"op":    "gt",
		"value": 1000,
	}
	ctx := map[string]interface{}{"cost": 1500.0}
	assert.True(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateAnd(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"op": "and",
		"conditions": []interface{}{
			map[string]interface{}{"field": "action", "op": "in", "value": []interface{}{"send_email"}},
			map[string]interface{}{"field": "cost", "op": "lt", "value": 1000},
		},
	}
	ctx := map[string]interface{}{
		"action": "send_email",
		"cost":   500.0,
	}
	assert.True(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateOr(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"op": "or",
		"conditions": []interface{}{
			map[string]interface{}{"field": "action", "op": "eq", "value": "send_email"},
			map[string]interface{}{"field": "action", "op": "eq", "value": "create_opportunity"},
		},
	}
	ctx := map[string]interface{}{"action": "create_opportunity"}
	assert.True(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateNestedConditions(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"op": "and",
		"conditions": []interface{}{
			map[string]interface{}{
				"op": "or",
				"conditions": []interface{}{
					map[string]interface{}{"field": "action", "op": "eq", "value": "send_email"},
					map[string]interface{}{"field": "action", "op": "eq", "value": "send_invoice"},
				},
			},
			map[string]interface{}{"field": "cost", "op": "lt", "value": 1000},
		},
	}
	ctx := map[string]interface{}{
		"action": "send_invoice",
		"cost":   500.0,
	}
	assert.True(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateMissingField(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "nonexistent",
		"op":    "eq",
		"value": "send_email",
	}
	ctx := map[string]interface{}{}
	assert.False(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateUnknownField(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "unknown_field",
		"op":    "eq",
		"value": "send_email",
	}
	ctx := map[string]interface{}{"action": "send_email"}
	assert.False(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateExists(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "metadata",
		"op":    "exists",
	}
	ctx := map[string]interface{}{"metadata": map[string]interface{}{"key": "value"}}
	assert.True(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateExistsMissing(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "metadata",
		"op":    "exists",
	}
	ctx := map[string]interface{}{}
	assert.False(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateEmptyConditions(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{}
	ctx := map[string]interface{}{"action": "send_email"}
	assert.False(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateEqFalse(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "action",
		"op":    "eq",
		"value": "send_email",
	}
	ctx := map[string]interface{}{"action": "create_opportunity"}
	assert.False(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateInFalse(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "action",
		"op":    "in",
		"value": []interface{}{"send_email", "send_invoice"},
	}
	ctx := map[string]interface{}{"action": "create_opportunity"}
	assert.False(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateGte(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "cost",
		"op":    "gte",
		"value": 1000,
	}
	ctx := map[string]interface{}{"cost": 1000.0}
	assert.True(t, re.EvaluateConditions(cond, ctx))
}

func TestRuleEngine_EvaluateLte(t *testing.T) {
	re := NewRuleEngine()
	cond := map[string]interface{}{
		"field": "cost",
		"op":    "lte",
		"value": 1000,
	}
	ctx := map[string]interface{}{"cost": 1000.0}
	assert.True(t, re.EvaluateConditions(cond, ctx))
}