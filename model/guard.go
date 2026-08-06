package model

import "strconv"

// GuardOp is a comparison operator evaluated against a Transition's or
// CommonTransition's Guard.Key looked up in the workflow's ContextMap.
// Deliberately a closed, structured set (not a free-form expression
// language) to match the rest of the model's declarative-JSON style —
// see Action.Variables for the same design choice.
type GuardOp string

const (
	GuardEq        GuardOp = "eq"
	GuardNeq       GuardOp = "neq"
	GuardGt        GuardOp = "gt"
	GuardGte       GuardOp = "gte"
	GuardLt        GuardOp = "lt"
	GuardLte       GuardOp = "lte"
	GuardExists    GuardOp = "exists"
	GuardNotExists GuardOp = "not_exists"
)

// Guard is a condition on a Transition or CommonTransition, evaluated
// against the workflow instance's ContextMap. A nil Guard always passes
// (today's unconditional-match behavior).
type Guard struct {
	Key   string  `json:"key"`
	Op    GuardOp `json:"op"`
	Value string  `json:"value,omitempty"`
}

// Evaluate reports whether g passes against ctxMap. Numeric comparisons
// (gt/gte/lt/lte) that fail to parse as float64 on either side evaluate
// to false rather than panicking or falling back to string comparison —
// a malformed guard should block the transition, not silently misfire.
func (g *Guard) Evaluate(ctxMap map[string]string) bool {
	if g == nil {
		return true
	}
	actual, present := ctxMap[g.Key]
	switch g.Op {
	case GuardExists:
		return present
	case GuardNotExists:
		return !present
	case GuardEq:
		return present && actual == g.Value
	case GuardNeq:
		return !present || actual != g.Value
	case GuardGt, GuardGte, GuardLt, GuardLte:
		if !present {
			return false
		}
		a, aerr := strconv.ParseFloat(actual, 64)
		b, berr := strconv.ParseFloat(g.Value, 64)
		if aerr != nil || berr != nil {
			return false
		}
		switch g.Op {
		case GuardGt:
			return a > b
		case GuardGte:
			return a >= b
		case GuardLt:
			return a < b
		default:
			return a <= b
		}
	default:
		return false
	}
}
