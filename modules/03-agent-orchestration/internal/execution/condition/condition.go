// Package condition evaluates the boolean expressions that department SOPs
// attach to conditional steps.
//
// The grammar is deliberately the one the catalogue actually uses, no more:
//
//	expr    := or
//	or      := and ( ("OR"|"or") and )*
//	and     := cmp ( ("AND"|"and") cmp )*
//	cmp     := operand [ ("=="|"!="|">"|"<"|">="|"<=") operand
//	                   | "in" list | "not" "in" list ]
//	operand := path | number | string | bareword | "true" | "false"
//	list    := "[" ( operand ("," operand)* )? "]"
//
// A bare operand on its own is a truthiness test, which covers expressions
// like `control_test_fails`.
//
// Evaluation never guesses. If the expression cannot be parsed, or references
// a variable the run does not carry, Evaluate reports ok=false with a reason
// rather than returning a value the caller might act on. Callers are expected
// to surface that, not to pick a branch anyway.
package condition

import (
	"fmt"
	"strconv"
	"strings"
)

// Result is the outcome of evaluating one expression.
type Result struct {
	Value  bool   // meaningful only when OK
	OK     bool   // false when the expression could not be decided
	Reason string // why it could not be decided
}

func undecided(format string, args ...interface{}) Result {
	return Result{OK: false, Reason: fmt.Sprintf(format, args...)}
}

// Evaluate resolves expr against vars. Variable lookup accepts dotted paths
// ("incident.severity") walking nested maps, and falls back to the flat key.
func Evaluate(expr string, vars map[string]interface{}) Result {
	toks, err := lex(expr)
	if err != nil {
		return undecided("%v", err)
	}
	if len(toks) == 0 {
		return undecided("empty condition")
	}
	p := &parser{toks: toks, vars: vars}
	res := p.parseOr()
	if !res.OK {
		return res
	}
	if p.pos != len(p.toks) {
		return undecided("unexpected %q at position %d", p.toks[p.pos].text, p.pos)
	}
	return res
}

// ── lexer ───────────────────────────────────────────────────

type tokKind int

const (
	tokWord tokKind = iota // identifiers, barewords, numbers, keywords
	tokString
	tokOp
	tokLBracket
	tokRBracket
	tokComma
)

type token struct {
	kind tokKind
	text string
}

func lex(s string) ([]token, error) {
	var out []token
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '\'' || c == '"':
			quote := c
			j := i + 1
			for j < len(s) && s[j] != quote {
				j++
			}
			if j >= len(s) {
				return nil, fmt.Errorf("unterminated string literal")
			}
			out = append(out, token{tokString, s[i+1 : j]})
			i = j + 1
		case c == '[':
			out = append(out, token{tokLBracket, "["})
			i++
		case c == ']':
			out = append(out, token{tokRBracket, "]"})
			i++
		case c == ',':
			out = append(out, token{tokComma, ","})
			i++
		case strings.ContainsRune("=!<>", rune(c)):
			if i+1 < len(s) && s[i+1] == '=' {
				out = append(out, token{tokOp, s[i : i+2]})
				i += 2
			} else if c == '<' || c == '>' {
				out = append(out, token{tokOp, string(c)})
				i++
			} else {
				return nil, fmt.Errorf("stray %q — did you mean %q?", string(c), string(c)+"=")
			}
		default:
			j := i
			// Barewords may contain dots and hyphens: role.requires_clearance,
			// settlement-opportunity-exists.
			for j < len(s) && !strings.ContainsRune(" \t\n\r[],='\"!<>", rune(s[j])) {
				j++
			}
			if j == i {
				return nil, fmt.Errorf("unexpected character %q", string(s[i]))
			}
			out = append(out, token{tokWord, s[i:j]})
			i = j
		}
	}
	return out, nil
}

// ── parser / evaluator ──────────────────────────────────────

type parser struct {
	toks []token
	pos  int
	vars map[string]interface{}
}

func (p *parser) peek() (token, bool) {
	if p.pos < len(p.toks) {
		return p.toks[p.pos], true
	}
	return token{}, false
}

func (p *parser) peekWord(words ...string) bool {
	t, ok := p.peek()
	if !ok || t.kind != tokWord {
		return false
	}
	for _, w := range words {
		if strings.EqualFold(t.text, w) {
			return true
		}
	}
	return false
}

func (p *parser) parseOr() Result {
	left := p.parseAnd()
	for p.peekWord("or") {
		p.pos++
		right := p.parseAnd()
		// Both sides must be decidable: an undecided operand could flip the
		// answer, so the whole expression is undecided.
		if !left.OK {
			return left
		}
		if !right.OK {
			return right
		}
		left = Result{Value: left.Value || right.Value, OK: true}
	}
	return left
}

func (p *parser) parseAnd() Result {
	left := p.parseCmp()
	for p.peekWord("and") {
		p.pos++
		right := p.parseCmp()
		if !left.OK {
			return left
		}
		if !right.OK {
			return right
		}
		left = Result{Value: left.Value && right.Value, OK: true}
	}
	return left
}

func (p *parser) parseCmp() Result {
	lhsTok, ok := p.peek()
	if !ok {
		return undecided("expression ended early")
	}
	lhs, lhsOK := p.parseOperand()

	// `in [...]` / `not in [...]`
	negate := false
	if p.peekWord("not") {
		p.pos++
		negate = true
	}
	if p.peekWord("in") {
		p.pos++
		list, err := p.parseList()
		if err != nil {
			return undecided("%v", err)
		}
		if !lhsOK {
			return undecided("cannot resolve %q", lhsTok.text)
		}
		found := false
		for _, item := range list {
			if looseEqual(lhs, item) {
				found = true
				break
			}
		}
		return Result{Value: found != negate, OK: true}
	}
	if negate {
		return undecided("`not` must be followed by `in`")
	}

	t, ok := p.peek()
	if !ok || t.kind != tokOp {
		// Bare truthiness: `control_test_fails`.
		if !lhsOK {
			return undecided("cannot resolve %q", lhsTok.text)
		}
		return Result{Value: truthy(lhs), OK: true}
	}
	p.pos++
	// The right-hand side of a comparison is a literal in this grammar — SOP
	// authors write `three_way_match == failed`, not a second variable — so an
	// unresolved bareword there is its own text, not an undecidable reference.
	rhs, _ := p.parseOperand()
	if !lhsOK {
		return undecided("cannot resolve %q", lhsTok.text)
	}

	switch t.text {
	case "==":
		return Result{Value: looseEqual(lhs, rhs), OK: true}
	case "!=":
		return Result{Value: !looseEqual(lhs, rhs), OK: true}
	case ">", "<", ">=", "<=":
		ln, lok := toFloat(lhs)
		rn, rok := toFloat(rhs)
		if !lok || !rok {
			return undecided("cannot compare %v %s %v numerically", lhs, t.text, rhs)
		}
		switch t.text {
		case ">":
			return Result{Value: ln > rn, OK: true}
		case "<":
			return Result{Value: ln < rn, OK: true}
		case ">=":
			return Result{Value: ln >= rn, OK: true}
		default:
			return Result{Value: ln <= rn, OK: true}
		}
	}
	return undecided("unsupported operator %q", t.text)
}

func (p *parser) parseList() ([]interface{}, error) {
	t, ok := p.peek()
	if !ok || t.kind != tokLBracket {
		return nil, fmt.Errorf("expected `[` after `in`")
	}
	p.pos++
	var out []interface{}
	for {
		t, ok := p.peek()
		if !ok {
			return nil, fmt.Errorf("unterminated list")
		}
		if t.kind == tokRBracket {
			p.pos++
			return out, nil
		}
		if t.kind == tokComma {
			p.pos++
			continue
		}
		// List members are literals; an unresolved bareword is its own text.
		v, _ := p.parseOperand()
		out = append(out, v)
	}
}

// parseOperand returns the operand's value and whether it resolved. A literal
// always resolves; a variable reference resolves only if the run carries it.
func (p *parser) parseOperand() (interface{}, bool) {
	t, ok := p.peek()
	if !ok {
		return nil, false
	}
	p.pos++
	if t.kind == tokString {
		return t.text, true
	}
	if t.kind != tokWord {
		return nil, false
	}
	switch strings.ToLower(t.text) {
	case "true":
		return true, true
	case "false":
		return false, true
	case "null", "nil":
		return nil, true
	}
	if n, err := strconv.ParseFloat(t.text, 64); err == nil {
		return n, true
	}
	if v, found := lookup(p.vars, t.text); found {
		return v, true
	}
	// A bareword that names nothing: usable as a literal on the right of a
	// comparison (`three_way_match == failed`), but not resolvable on its own.
	return t.text, false
}

// lookup walks a dotted path through nested maps, then tries the flat key.
func lookup(vars map[string]interface{}, path string) (interface{}, bool) {
	if vars == nil {
		return nil, false
	}
	if v, ok := vars[path]; ok {
		return v, true
	}
	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		return nil, false
	}
	var cur interface{} = vars
	for _, part := range parts {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	}
	return 0, false
}

// looseEqual compares across the string/number boundary, because SOP authors
// write `priority == 3` and the variable may arrive as "3".
func looseEqual(a, b interface{}) bool {
	if a == b {
		return true
	}
	if an, aok := toFloat(a); aok {
		if bn, bok := toFloat(b); bok {
			return an == bn
		}
	}
	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		return strings.EqualFold(as, bs)
	}
	return false
}

func truthy(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != "" && !strings.EqualFold(t, "false") && t != "0"
	case float64:
		return t != 0
	case int:
		return t != 0
	}
	return true
}
