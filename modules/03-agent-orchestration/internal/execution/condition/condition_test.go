package condition

import "testing"

func TestEvaluateRealCatalogueExpressions(t *testing.T) {
	// Every expression below is taken verbatim from a department template.
	cases := []struct {
		expr string
		vars map[string]interface{}
		want bool
	}{
		// Membership over a nested path.
		{"incident.severity in ['critical', 'high']",
			map[string]interface{}{"incident": map[string]interface{}{"severity": "critical"}}, true},
		{"incident.severity in ['critical', 'high']",
			map[string]interface{}{"incident": map[string]interface{}{"severity": "low"}}, false},
		{"ticket.priority in ['high', 'critical']",
			map[string]interface{}{"ticket": map[string]interface{}{"priority": "high"}}, true},

		// Boolean equality.
		{"role.requires_security_clearance == true",
			map[string]interface{}{"role": map[string]interface{}{"requires_security_clearance": true}}, true},
		{"scan.has_critical_findings == true",
			map[string]interface{}{"scan": map[string]interface{}{"has_critical_findings": false}}, false},

		// Bareword literal on the right of a comparison.
		{"three_way_match == failed", map[string]interface{}{"three_way_match": "failed"}, true},
		{"three_way_match == failed", map[string]interface{}{"three_way_match": "passed"}, false},

		// Numeric comparison.
		{"invoice_amount > 25000", map[string]interface{}{"invoice_amount": 30000}, true},
		{"invoice_amount > 25000", map[string]interface{}{"invoice_amount": 100}, false},

		// OR across two decidable operands.
		{"three_way_match == failed OR invoice_amount > 25000",
			map[string]interface{}{"three_way_match": "passed", "invoice_amount": 30000}, true},
		{"three_way_match == failed OR invoice_amount > 25000",
			map[string]interface{}{"three_way_match": "passed", "invoice_amount": 100}, false},

		// Mixed-case AND/OR with precedence: OR binds looser than AND.
		{"vendor.rating < 3.0 OR vendor.rating < 3.5 AND trend_decreasing",
			map[string]interface{}{"vendor": map[string]interface{}{"rating": 3.4}, "trend_decreasing": true}, true},
		{"vendor.rating < 3.0 OR vendor.rating < 3.5 AND trend_decreasing",
			map[string]interface{}{"vendor": map[string]interface{}{"rating": 3.4}, "trend_decreasing": false}, false},

		// Bare truthiness.
		{"control_test_fails", map[string]interface{}{"control_test_fails": true}, true},
		{"control_test_fails", map[string]interface{}{"control_test_fails": false}, false},
		// Hyphenated barewords appear in the catalogue.
		{"settlement-opportunity-exists", map[string]interface{}{"settlement-opportunity-exists": true}, true},

		// The variable may arrive as a string across the wire.
		{"invoice_amount > 25000", map[string]interface{}{"invoice_amount": "30000"}, true},
	}
	for _, c := range cases {
		got := Evaluate(c.expr, c.vars)
		if !got.OK {
			t.Errorf("%q: undecided (%s), want %v", c.expr, got.Reason, c.want)
			continue
		}
		if got.Value != c.want {
			t.Errorf("%q = %v, want %v", c.expr, got.Value, c.want)
		}
	}
}

// The whole point: never return a usable answer we cannot justify.
func TestUndecidedRatherThanGuessing(t *testing.T) {
	cases := []struct {
		name string
		expr string
		vars map[string]interface{}
	}{
		{"missing variable", "incident.severity in ['critical']", map[string]interface{}{}},
		{"missing nested key", "vendor.rating < 3.0",
			map[string]interface{}{"vendor": map[string]interface{}{"name": "acme"}}},
		{"no variables at all", "control_test_fails", nil},
		// Real catalogue expression the grammar cannot express — units.
		{"unparseable units", "duration > 5 working days", map[string]interface{}{"duration": 7}},
		{"undecidable side of OR", "three_way_match == failed OR invoice_amount > 25000",
			map[string]interface{}{"three_way_match": "failed"}},
		{"empty", "", map[string]interface{}{}},
		{"unterminated string", "x == 'oops", map[string]interface{}{"x": "oops"}},
	}
	for _, c := range cases {
		got := Evaluate(c.expr, c.vars)
		if got.OK {
			t.Errorf("%s: %q decided %v — it must report undecided", c.name, c.expr, got.Value)
		}
		if got.Reason == "" {
			t.Errorf("%s: undecided without a reason", c.name)
		}
	}
}

// A false OR-operand must not mask a decidable true one, and vice versa —
// undecided is contagious because it could change the answer.
func TestUndecidedIsContagiousAcrossLogic(t *testing.T) {
	r := Evaluate("known == yes AND unknown_thing == 1", map[string]interface{}{"known": "yes"})
	if r.OK {
		t.Errorf("AND with an unresolvable operand must be undecided, got %v", r.Value)
	}
}
