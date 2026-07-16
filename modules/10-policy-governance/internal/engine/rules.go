package engine

import (
	"fmt"
	"strings"
)

// RuleEngine evaluates condition expressions against request context.
type RuleEngine struct{}

// NewRuleEngine creates a new rule engine.
func NewRuleEngine() *RuleEngine {
	return &RuleEngine{}
}

// Condition represents a single condition in a policy.
type Condition struct {
	Field  string      `json:"field"`
	Op     string      `json:"op"`
	Value  interface{} `json:"value"`
}

// ConditionGroup groups conditions with AND/OR logic.
type ConditionGroup struct {
	Op         string        `json:"op"`
	Conditions []interface{} `json:"conditions"`
}

// EvaluateConditions evaluates a condition expression against request context.
func (re *RuleEngine) EvaluateConditions(cond map[string]interface{}, ctx map[string]interface{}) bool {
	op, ok := cond["op"].(string)
	if !ok {
		return false
	}

	switch strings.ToLower(op) {
	case "and":
		return re.evaluateAnd(cond, ctx)
	case "or":
		return re.evaluateOr(cond, ctx)
	default:
		return re.evaluateOperator(cond, ctx)
	}
}

// evaluateAnd checks if all sub-conditions are true.
func (re *RuleEngine) evaluateAnd(cond map[string]interface{}, ctx map[string]interface{}) bool {
	conditions, ok := cond["conditions"].([]interface{})
	if !ok {
		return false
	}
	for _, c := range conditions {
		if !re.evaluateCondition(c, ctx) {
			return false
		}
	}
	return true
}

// evaluateOr checks if any sub-conditions are true.
func (re *RuleEngine) evaluateOr(cond map[string]interface{}, ctx map[string]interface{}) bool {
	conditions, ok := cond["conditions"].([]interface{})
	if !ok {
		return false
	}
	for _, c := range conditions {
		if re.evaluateCondition(c, ctx) {
			return true
		}
	}
	return false
}

// evaluateCondition handles a single condition (map or ConditionGroup).
func (re *RuleEngine) evaluateCondition(c interface{}, ctx map[string]interface{}) bool {
	switch cond := c.(type) {
	case map[string]interface{}:
		return re.EvaluateConditions(cond, ctx)
	default:
		return false
	}
}

// evaluateOperator evaluates a simple operator condition.
func (re *RuleEngine) evaluateOperator(cond map[string]interface{}, ctx map[string]interface{}) bool {
	field, ok := cond["field"].(string)
	if !ok {
		return false
	}

	op, ok := cond["op"].(string)
	if !ok {
		return false
	}

	value := cond["value"]

	// Get field value from context
	fieldValue := re.getFieldValue(field, ctx)

	switch strings.ToLower(op) {
	case "eq":
		return re.eq(fieldValue, value)
	case "neq":
		return re.neq(fieldValue, value)
	case "in":
		return re.in(fieldValue, value)
	case "not_in":
		return re.notIn(fieldValue, value)
	case "gt":
		return re.gt(fieldValue, value)
	case "lt":
		return re.lt(fieldValue, value)
	case "gte":
		return re.gte(fieldValue, value)
	case "lte":
		return re.lte(fieldValue, value)
	case "exists":
		return re.exists(fieldValue)
	case "and":
		return re.evaluateAnd(cond, ctx)
	case "or":
		return re.evaluateOr(cond, ctx)
	default:
		return false
	}
}

// getFieldValue extracts a value from context, supporting dot-notation paths.
func (re *RuleEngine) getFieldValue(field string, ctx map[string]interface{}) interface{} {
	// Handle dot-notation (e.g., "metadata.key")
	if strings.Contains(field, ".") {
		parts := strings.Split(field, ".")
		val := ctx[parts[0]]
		for _, p := range parts[1:] {
			if m, ok := val.(map[string]interface{}); ok {
				val = m[p]
			} else {
				return nil
			}
		}
		return val
	}
	return ctx[field]
}

// eq checks equality with type-aware comparison.
func (re *RuleEngine) eq(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Same Go type → direct compare (avoids stringification)
	if reflectType(a) == reflectType(b) {
		switch av := a.(type) {
		case string:
			return av == b.(string)
		case bool:
			return av == b.(bool)
		default:
			return a == b
		}
	}

	// Both numeric → compare as float64
	aNum := re.toFloat(a)
	bNum := re.toFloat(b)
	return aNum == bNum
}

// reflectType returns a string representation of the Go type.
func reflectType(v interface{}) string {
	switch v.(type) {
	case string:
		return "string"
	case int, int64, int32:
		return "int"
	case float64, float32:
		return "float"
	case bool:
		return "bool"
	default:
		return fmt.Sprintf("%T", v)
	}
}

// neq checks inequality.
func (re *RuleEngine) neq(a, b interface{}) bool {
	return !re.eq(a, b)
}

// in checks if a value is in an array.
func (re *RuleEngine) in(val interface{}, arr interface{}) bool {
	switch arr := arr.(type) {
	case []interface{}:
		for _, item := range arr {
			if re.eq(val, item) {
				return true
			}
		}
	case []string:
		for _, item := range arr {
			if re.eq(val, item) {
				return true
			}
		}
	}
	return false
}

// notIn checks if a value is NOT in an array.
func (re *RuleEngine) notIn(val interface{}, arr interface{}) bool {
	return !re.in(val, arr)
}

// gt checks greater than.
func (re *RuleEngine) gt(a, b interface{}) bool {
	return re.compare(a, b) > 0
}

// lt checks less than.
func (re *RuleEngine) lt(a, b interface{}) bool {
	return re.compare(a, b) < 0
}

// gte checks greater than or equal.
func (re *RuleEngine) gte(a, b interface{}) bool {
	return re.compare(a, b) >= 0
}

// lte checks less than or equal.
func (re *RuleEngine) lte(a, b interface{}) bool {
	return re.compare(a, b) <= 0
}

// compare compares two numeric values.
func (re *RuleEngine) compare(a, b interface{}) int {
	aNum := re.toFloat(a)
	bNum := re.toFloat(b)
	if aNum > bNum {
		return 1
	}
	if aNum < bNum {
		return -1
	}
	return 0
}

// toFloat converts an interface{} to float64.
func (re *RuleEngine) toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case float64:
		return val
	case float32:
		return float64(val)
	case string:
		// Parse string to float
		// Simplified: just return 0 for non-numeric strings
		return 0
	default:
		return 0
	}
}

// exists checks if a value is present and non-empty.
func (re *RuleEngine) exists(val interface{}) bool {
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case string:
		return v != ""
	case []interface{}:
		return len(v) > 0
	case map[string]interface{}:
		return len(v) > 0
	default:
		return true
	}
}