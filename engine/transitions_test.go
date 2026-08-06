package engine

import (
	"context"
	"testing"

	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
)

func TestFindCandidateTransitionGuardFiltering(t *testing.T) {
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: model.Workflow{
			States: []model.State{&model.SimpleState{Type: "SimpleState", Id: "review"}},
			Transitions: []model.Transition{
				{Source: "review", Event: "decide", Target: "rejected", Guard: &model.Guard{Key: "risk", Op: model.GuardGte, Value: "70"}},
				{Source: "review", Event: "decide", Target: "approved"},
			},
		},
		CurrentState: persistence.StateContainer{State: &model.SimpleState{Type: "SimpleState", Id: "review"}},
	}

	got, ok := findCandidateTransition(wf, "decide", map[string]string{"risk": "80"})
	if !ok || got.Target != "rejected" {
		t.Fatalf("high risk: got %+v, ok=%v, want target=rejected", got, ok)
	}

	got, ok = findCandidateTransition(wf, "decide", map[string]string{"risk": "10"})
	if !ok || got.Target != "approved" {
		t.Fatalf("low risk: got %+v, ok=%v, want target=approved (guard should skip the first candidate)", got, ok)
	}
}

func TestApplyTransitionRunsActionsInOrder(t *testing.T) {
	src := &model.SimpleState{Type: "SimpleState", Id: "a"}
	dst := &model.SimpleState{Type: "SimpleState", Id: "b"}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: model.Workflow{States: []model.State{src, dst}},
		CurrentState:       persistence.StateContainer{State: src},
	}
	tr := model.Transition{Source: "a", Target: "b", Event: "go"}

	ctxMap, err := applyTransition(context.Background(), "test-id", wf, tr, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if wf.CurrentState.State.GetId() != "b" {
		t.Fatalf("CurrentState = %q, want b", wf.CurrentState.State.GetId())
	}
	_ = ctxMap
}
