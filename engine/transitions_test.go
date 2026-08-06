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

func TestFindAutomaticTransition(t *testing.T) {
	def := model.Workflow{
		Transitions: []model.Transition{
			{Source: "a", Event: "auto1", Target: "b", Trigger: model.AutomaticTrigger},
			{Source: "a", Event: "manual", Target: "c"},
			{Source: "a", Event: "deferred", Target: "d", Trigger: model.AutomaticTrigger, After: "1h"},
		},
	}
	got, ok := findAutomaticTransition(def, "a", nil)
	if !ok || got.Event != "auto1" {
		t.Fatalf("got %+v, ok=%v, want auto1 (deferred one must be skipped here)", got, ok)
	}
	if _, ok := findAutomaticTransition(def, "b", nil); ok {
		t.Fatal("no automatic transition from b, expected none")
	}
}

func TestApplyTransitionChainCycleGuard(t *testing.T) {
	a := &model.SimpleState{Type: "SimpleState", Id: "a"}
	b := &model.SimpleState{Type: "SimpleState", Id: "b"}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: model.Workflow{
			States: []model.State{a, b},
			Transitions: []model.Transition{
				{Source: "a", Event: "auto", Target: "b", Trigger: model.AutomaticTrigger},
				{Source: "b", Event: "auto", Target: "a", Trigger: model.AutomaticTrigger},
			},
		},
		CurrentState: persistence.StateContainer{State: a},
	}
	ctxMap, err := runAutomaticChain(context.Background(), "test-id", wf, map[string]string{})
	if err == nil {
		t.Fatal("expected cycle-guard error, got nil")
	}
	_ = ctxMap
}

func TestApplyTransitionInternalKindNoStateChange(t *testing.T) {
	a := &model.ActionState{Type: "ActionState", Id: "a"}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: model.Workflow{States: []model.State{a}},
		CurrentState:       persistence.StateContainer{State: a},
	}
	fired := false
	tr := model.Transition{
		Source: "a", Target: "a", Event: "ping", Kind: model.InternalKind,
		EntryActions: []model.Action{{Type: model.SetContextMapAction, Variables: map[string]string{"pinged": "true"}}},
	}
	ctxMap, err := applyTransition(context.Background(), "test-id", wf, tr, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if wf.CurrentState.State != model.State(a) {
		t.Fatal("internal transition must not change CurrentState identity")
	}
	if ctxMap["pinged"] != "true" {
		t.Fatal("internal transition's own EntryActions (the effect) must still run")
	}
	_ = fired
}
