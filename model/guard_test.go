package model

import (
	"encoding/json"
	"testing"
)

func TestGuardEvaluate(t *testing.T) {
	ctx := map[string]string{"riskScore": "72", "flag": "yes"}
	cases := []struct {
		name string
		g    Guard
		want bool
	}{
		{"eq match", Guard{Key: "flag", Op: GuardEq, Value: "yes"}, true},
		{"eq mismatch", Guard{Key: "flag", Op: GuardEq, Value: "no"}, false},
		{"neq", Guard{Key: "flag", Op: GuardNeq, Value: "no"}, true},
		{"gte numeric pass", Guard{Key: "riskScore", Op: GuardGte, Value: "70"}, true},
		{"gte numeric fail", Guard{Key: "riskScore", Op: GuardGte, Value: "80"}, false},
		{"lt numeric", Guard{Key: "riskScore", Op: GuardLt, Value: "100"}, true},
		{"exists", Guard{Key: "flag", Op: GuardExists}, true},
		{"not_exists missing key", Guard{Key: "missing", Op: GuardNotExists}, true},
		{"not_exists present key", Guard{Key: "flag", Op: GuardNotExists}, false},
		{"unparseable numeric is false, not a panic", Guard{Key: "flag", Op: GuardGt, Value: "10"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.g.Evaluate(ctx); got != c.want {
				t.Errorf("Evaluate() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestNilGuardAlwaysPasses(t *testing.T) {
	var g *Guard
	if !g.Evaluate(nil) {
		t.Fatal("nil guard must always pass (unguarded transition)")
	}
}

func TestGuardOpCaseInsensitiveUnmarshal(t *testing.T) {
	data := []byte(`{"key":"flag","op":"exists"}`)
	var g Guard
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if g.Op != GuardExists {
		t.Fatalf("Op = %q, want %q", g.Op, GuardExists)
	}
}
