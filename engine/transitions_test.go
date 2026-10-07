package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

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

	ctxMap, _, err := applyTransition(context.Background(), "test-id", wf, tr, map[string]string{})
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
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: def,
		CurrentState:       persistence.StateContainer{State: &model.SimpleState{Type: "SimpleState", Id: "a"}},
	}
	got, ok := findAutomaticTransition(wf, nil)
	if !ok || got.Event != "auto1" {
		t.Fatalf("got %+v, ok=%v, want auto1 (deferred one must be skipped here)", got, ok)
	}
	wf.CurrentState = persistence.StateContainer{State: &model.SimpleState{Type: "SimpleState", Id: "b"}}
	if _, ok := findAutomaticTransition(wf, nil); ok {
		t.Fatal("no automatic transition from b, expected none")
	}
}

func TestApplyTransitionEntersAndLeavesComposite(t *testing.T) {
	collecting := &model.SimpleState{Type: "SimpleState", Id: "collecting"}
	verifying := &model.ActionState{Type: "ActionState", Id: "verifying"}
	comp := &model.CompositeState{
		Type: "CompositeState", Id: "review", InitialSubstate: "collecting",
		Substates:      []model.State{collecting, verifying},
		SubTransitions: []model.Transition{{Source: "collecting", Target: "verifying", Event: "submit"}},
		History:        model.ShallowHistory,
	}
	start := &model.SimpleState{Type: "SimpleState", Id: "start"}
	done := &model.SimpleState{Type: "SimpleState", Id: "done"}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: model.Workflow{
			States: []model.State{start, comp, done},
			Transitions: []model.Transition{
				{Source: "start", Target: "review", Event: "begin"},
				{Source: "review", Target: "done", Event: "abandon"},
			},
		},
		CurrentState: persistence.StateContainer{State: start},
	}

	// Enter the composite: lands on its InitialSubstate.
	t1, ok := findCandidateTransition(wf, "begin", nil)
	if !ok {
		t.Fatal("expected to find begin transition")
	}
	if _, _, err := applyTransition(context.Background(), "id", wf, t1, nil); err != nil {
		t.Fatal(err)
	}
	if wf.CurrentState.EffectiveId() != "collecting" {
		t.Fatalf("EffectiveId() = %q, want collecting (composite's InitialSubstate)", wf.CurrentState.EffectiveId())
	}

	// Move within the composite via a SubTransition (local).
	t2, ok := findCandidateTransition(wf, "submit", nil)
	if !ok {
		t.Fatal("expected to find submit sub-transition")
	}
	if _, _, err := applyTransition(context.Background(), "id", wf, t2, nil); err != nil {
		t.Fatal(err)
	}
	if wf.CurrentState.EffectiveId() != "verifying" || wf.CurrentState.State.GetId() != "review" {
		t.Fatalf("got state=%q substate=%q, want review/verifying", wf.CurrentState.State.GetId(), wf.CurrentState.EffectiveId())
	}

	// Leave the composite: history must record "verifying" as the last active substate.
	t3, ok := findCandidateTransition(wf, "abandon", nil)
	if !ok {
		t.Fatal("expected to find abandon transition (bubbled out of the composite)")
	}
	if _, _, err := applyTransition(context.Background(), "id", wf, t3, nil); err != nil {
		t.Fatal(err)
	}
	if wf.CurrentState.EffectiveId() != "done" {
		t.Fatalf("EffectiveId() = %q, want done", wf.CurrentState.EffectiveId())
	}
	if wf.CompositeHistory["review"] != "verifying" {
		t.Fatalf("CompositeHistory[review] = %q, want verifying", wf.CompositeHistory["review"])
	}
}

func TestApplyTransitionEntersHistoryOnReentry(t *testing.T) {
	collecting := &model.SimpleState{Type: "SimpleState", Id: "collecting"}
	verifying := &model.ActionState{Type: "ActionState", Id: "verifying"}
	comp := &model.CompositeState{
		Type: "CompositeState", Id: "review", InitialSubstate: "collecting",
		Substates: []model.State{collecting, verifying},
		History:   model.ShallowHistory,
	}
	elsewhere := &model.SimpleState{Type: "SimpleState", Id: "elsewhere"}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: model.Workflow{States: []model.State{comp, elsewhere}},
		CurrentState:       persistence.StateContainer{State: comp, Substate: verifying},
		CompositeHistory:   map[string]string{"review": "verifying"},
	}
	tr := model.Transition{Source: "review", Target: "elsewhere", Event: "leave"}
	if _, _, err := applyTransition(context.Background(), "id", wf, tr, nil); err != nil {
		t.Fatal(err)
	}

	reenter := model.Transition{Source: "elsewhere", Target: "review", Event: "back", EntersHistory: true}
	wf.WorkflowDefinition.Transitions = []model.Transition{reenter}
	if _, _, err := applyTransition(context.Background(), "id", wf, reenter, nil); err != nil {
		t.Fatal(err)
	}
	if wf.CurrentState.EffectiveId() != "verifying" {
		t.Fatalf("EffectiveId() = %q, want verifying (resumed from history, not InitialSubstate)", wf.CurrentState.EffectiveId())
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
	ctxMap, _, err := runAutomaticChain(context.Background(), "test-id", wf, map[string]string{})
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
	ctxMap, _, err := applyTransition(context.Background(), "test-id", wf, tr, map[string]string{})
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

func TestApplyTransitionForkStampsGenerationAndSetsPending(t *testing.T) {
	created := []model.Workflow{}
	restore := createChildBatchFn
	createChildBatchFn = func(parentId string, defs []model.Workflow, forkGeneration string) ([]string, error) {
		created = defs
		if forkGeneration == "" {
			t.Fatal("expected a non-empty fork generation")
		}
		return []string{"child-1", "child-2"}, nil
	}
	defer func() { createChildBatchFn = restore }()

	src := &model.SimpleState{Type: "SimpleState", Id: "review"}
	wait := &model.SimpleState{Type: "SimpleState", Id: "waiting"}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: model.Workflow{States: []model.State{src, wait}},
		CurrentState:       persistence.StateContainer{State: src},
	}
	tr := model.Transition{
		Source: "review", Target: "waiting", Event: "split", Kind: model.ForkKind,
		ForkTargets: []model.ForkTarget{
			{Ref: "region-a", ChildWorkflow: &model.Workflow{States: []model.State{&model.SimpleState{Type: "SimpleState", Id: "s"}}}},
			{Ref: "region-b", ChildWorkflow: &model.Workflow{States: []model.State{&model.SimpleState{Type: "SimpleState", Id: "s"}}}},
		},
	}

	if _, _, err := applyTransition(context.Background(), "parent-id", wf, tr, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 children created, got %d", len(created))
	}
	if wf.PendingForkGeneration == "" {
		t.Fatal("expected PendingForkGeneration to be set")
	}
	if wf.CurrentState.State.GetId() != "waiting" {
		t.Fatalf("CurrentState = %q, want waiting", wf.CurrentState.State.GetId())
	}
}

func TestApplyTransitionForkWithEmptyForkTargetsErrors(t *testing.T) {
	called := false
	restore := createChildBatchFn
	createChildBatchFn = func(parentId string, defs []model.Workflow, forkGeneration string) ([]string, error) {
		called = true
		return []string{"child-1"}, nil
	}
	defer func() { createChildBatchFn = restore }()

	src := &model.SimpleState{Type: "SimpleState", Id: "review"}
	wait := &model.SimpleState{Type: "SimpleState", Id: "waiting"}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: model.Workflow{States: []model.State{src, wait}},
		CurrentState:       persistence.StateContainer{State: src},
	}
	tr := model.Transition{
		Source: "review", Target: "waiting", Event: "split", Kind: model.ForkKind,
		ForkTargets: nil,
	}

	if _, _, err := applyTransition(context.Background(), "parent-id", wf, tr, map[string]string{}); err == nil {
		t.Fatal("expected an error for a fork transition with empty ForkTargets")
	}
	if called {
		t.Fatal("createChildBatchFn must not be called when ForkTargets is empty")
	}
	if wf.PendingForkGeneration != "" {
		t.Fatalf("PendingForkGeneration = %q, want empty (early return must happen before it's set)", wf.PendingForkGeneration)
	}
}

func TestResolveInitialPositionEntersCompositeInitialSubstate(t *testing.T) {
	collecting := &model.SimpleState{Type: "SimpleState", Id: "collecting"}
	verifying := &model.ActionState{Type: "ActionState", Id: "verifying"}
	comp := &model.CompositeState{
		Type: "CompositeState", Id: "review", InitialSubstate: "collecting",
		Substates:      []model.State{collecting, verifying},
		SubTransitions: []model.Transition{{Source: "collecting", Target: "verifying", Event: "submit"}},
	}
	def := model.Workflow{States: []model.State{comp}}

	// This is exactly what NewWorkflowInstance/CreateChildWorkflowInstance do
	// to build a new instance's StateContainer.
	state, sub, err := resolveInitialPosition(def.States[0])
	if err != nil {
		t.Fatal(err)
	}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: def,
		CurrentState:       persistence.StateContainer{State: state, Substate: sub},
	}
	if wf.CurrentState.State.GetId() != "review" {
		t.Fatalf("State.GetId() = %q, want review", wf.CurrentState.State.GetId())
	}
	if wf.CurrentState.EffectiveId() != "collecting" {
		t.Fatalf("EffectiveId() = %q, want collecting (the composite's InitialSubstate)", wf.CurrentState.EffectiveId())
	}
	// With Substate resolved, the composite's SubTransitions are in scope.
	if _, ok := findCandidateTransition(wf, "submit", nil); !ok {
		t.Fatal("composite SubTransitions must be reachable from a freshly-created instance")
	}
}

func TestResolveInitialPositionLeavesSimpleStateAlone(t *testing.T) {
	s := &model.SimpleState{Type: "SimpleState", Id: "start"}
	state, sub, err := resolveInitialPosition(s)
	if err != nil {
		t.Fatal(err)
	}
	if state != model.State(s) || sub != nil {
		t.Fatalf("got (%v, %v), want (start, nil)", state, sub)
	}
}

func TestResolveInitialPositionRejectsMissingInitialSubstate(t *testing.T) {
	comp := &model.CompositeState{Type: "CompositeState", Id: "review", InitialSubstate: "nope"}
	if _, _, err := resolveInitialPosition(comp); err == nil {
		t.Fatal("expected an error when the composite's InitialSubstate does not exist")
	}
}

// applyTransition must not arm the new state's do-activity itself — that
// happens inside processEvent's DB transaction, which may still roll back.
// Arming is the caller's job, via armStateActivities, after commit.
func TestApplyTransitionDefersDoActivityArmingToCaller(t *testing.T) {
	started := make(chan []model.Action, 4)
	restore := doActivityRunFn
	doActivityRunFn = func(ctx context.Context, id string, actions []model.Action) { started <- actions }
	defer func() { doActivityRunFn = restore }()
	defer stopDoActivity("arm-test")

	src := &model.SimpleState{Type: "SimpleState", Id: "a"}
	dst := &model.ActionState{
		Type: "ActionState", Id: "b",
		DoActions: []model.Action{{Type: model.SetContextMapAction, Variables: map[string]string{"k": "v"}}},
	}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: model.Workflow{States: []model.State{src, dst}},
		CurrentState:       persistence.StateContainer{State: src},
	}

	if _, _, err := applyTransition(context.Background(), "arm-test", wf, model.Transition{Source: "a", Target: "b", Event: "go"}, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	select {
	case a := <-started:
		t.Fatalf("applyTransition started a do-activity (%v); it must wait for the caller's armStateActivities", a)
	default:
	}

	armStateActivities("arm-test", wf, map[string]string{})
	select {
	case a := <-started:
		if len(a) != 1 {
			t.Fatalf("armStateActivities started %d actions, want 1", len(a))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("armStateActivities did not start the settled state's do-activity")
	}
}

func TestApplyTransitionInternalKindErrorDoesNotRecordJourney(t *testing.T) {
	a := &model.ActionState{Type: "ActionState", Id: "a"}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: model.Workflow{States: []model.State{a}},
		CurrentState:       persistence.StateContainer{State: a},
	}
	// Mock executeActionsFn to return an error
	restore := executeActionsFn
	executeActionsFn = func(ctx context.Context, id string, ctxMap map[string]string, actions []model.Action) (map[string]string, error) {
		return ctxMap, fmt.Errorf("action execution failed")
	}
	defer func() { executeActionsFn = restore }()

	tr := model.Transition{
		Source: "a", Target: "a", Event: "fail", Kind: model.InternalKind,
		EntryActions: []model.Action{{Type: model.SetContextMapAction, Variables: map[string]string{"key": "val"}}},
	}
	initialJourneyLen := len(wf.Journey)
	ctxMap, moved, err := applyTransition(context.Background(), "test-id", wf, tr, map[string]string{})
	if err == nil {
		t.Fatal("expected applyTransition to return an error, got nil")
	}
	if moved {
		t.Fatal("internal transition must return moved=false")
	}
	if len(wf.Journey) != initialJourneyLen {
		t.Fatalf("Journey length = %d, want %d (unchanged after failed internal transition)", len(wf.Journey), initialJourneyLen)
	}
	_ = ctxMap
}

// dto.JourneyEntry's convention (shared with dto.GraphNode and the
// visualizer frontend) is that *State fields always name a TOP-LEVEL state
// and *Substate fields always name a substate. Inside a composite,
// EffectiveId() returns the substate, so recording it as FromState would
// duplicate FromSubstate and never name the composite at all.
func TestApplyTransitionJourneyRecordsTopLevelFromState(t *testing.T) {
	newFixture := func() (*persistence.WorkflowInstance, *model.CompositeState) {
		collecting := &model.SimpleState{Type: "SimpleState", Id: "collecting"}
		verifying := &model.ActionState{Type: "ActionState", Id: "verifying"}
		comp := &model.CompositeState{
			Type: "CompositeState", Id: "review", InitialSubstate: "collecting",
			Substates:      []model.State{collecting, verifying},
			SubTransitions: []model.Transition{{Source: "collecting", Target: "verifying", Event: "submit"}},
			History:        model.ShallowHistory,
		}
		done := &model.SimpleState{Type: "SimpleState", Id: "done"}
		wf := &persistence.WorkflowInstance{
			WorkflowDefinition: model.Workflow{
				States:      []model.State{comp, done},
				Transitions: []model.Transition{{Source: "review", Target: "done", Event: "abandon"}},
			},
			// The instance starts INSIDE the composite, on its initial substate.
			CurrentState: persistence.StateContainer{State: comp, Substate: collecting},
		}
		return wf, comp
	}

	assertLast := func(t *testing.T, wf *persistence.WorkflowInstance, wantFrom, wantFromSub, wantTo, wantToSub string) {
		t.Helper()
		if len(wf.Journey) == 0 {
			t.Fatal("expected a journey entry to be appended")
		}
		got := wf.Journey[len(wf.Journey)-1]
		if got.FromState != wantFrom || got.FromSubstate != wantFromSub || got.ToState != wantTo || got.ToSubstate != wantToSub {
			t.Fatalf("journey entry = {From:%q FromSub:%q To:%q ToSub:%q}, want {From:%q FromSub:%q To:%q ToSub:%q}",
				got.FromState, got.FromSubstate, got.ToState, got.ToSubstate,
				wantFrom, wantFromSub, wantTo, wantToSub)
		}
	}

	t.Run("sub-transition inside the composite", func(t *testing.T) {
		wf, _ := newFixture()
		tr, ok := findCandidateTransition(wf, "submit", nil)
		if !ok {
			t.Fatal("expected to find the submit sub-transition")
		}
		if _, _, err := applyTransition(context.Background(), "id", wf, tr, nil); err != nil {
			t.Fatal(err)
		}
		if wf.CurrentState.State.GetId() != "review" || wf.CurrentState.EffectiveId() != "verifying" {
			t.Fatalf("position = %q/%q, want review/verifying", wf.CurrentState.State.GetId(), wf.CurrentState.EffectiveId())
		}
		// FromState names the composite, NOT the substate it duplicates.
		assertLast(t, wf, "review", "collecting", "review", "verifying")
	})

	t.Run("transition leaving the composite", func(t *testing.T) {
		wf, _ := newFixture()
		tr, ok := findCandidateTransition(wf, "abandon", nil)
		if !ok {
			t.Fatal("expected to find the abandon transition (bubbled out of the composite)")
		}
		if _, _, err := applyTransition(context.Background(), "id", wf, tr, nil); err != nil {
			t.Fatal(err)
		}
		assertLast(t, wf, "review", "collecting", "done", "")
	})

	t.Run("internal transition inside the composite", func(t *testing.T) {
		wf, comp := newFixture()
		comp.SubTransitions = append(comp.SubTransitions, model.Transition{
			Source: "collecting", Target: "collecting", Event: "ping", Kind: model.InternalKind,
		})
		tr, ok := findCandidateTransition(wf, "ping", nil)
		if !ok {
			t.Fatal("expected to find the ping internal sub-transition")
		}
		if _, moved, err := applyTransition(context.Background(), "id", wf, tr, nil); err != nil {
			t.Fatal(err)
		} else if moved {
			t.Fatal("an internal transition must report moved=false")
		}
		assertLast(t, wf, "review", "collecting", "review", "collecting")
	})
}

func TestInitialJourneyEntry(t *testing.T) {
	state := &model.SimpleState{Type: "SimpleState", Id: "s1"}

	entry := initialJourneyEntry(state, nil)
	if entry.ToState != "s1" || entry.ToSubstate != "" || entry.Event != "" {
		t.Fatalf("got %+v, want ToState=s1 ToSubstate=\"\" Event=\"\"", entry)
	}
	if entry.Timestamp.IsZero() {
		t.Fatal("expected a non-zero timestamp")
	}

	sub := &model.SimpleState{Type: "SimpleState", Id: "sub1"}
	entry2 := initialJourneyEntry(state, sub)
	if entry2.ToState != "s1" || entry2.ToSubstate != "sub1" {
		t.Fatalf("got %+v, want ToState=s1 ToSubstate=sub1", entry2)
	}
}
