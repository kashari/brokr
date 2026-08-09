package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
)

// loadWorkflowFixture parses the repo's workflow.json — the real
// production definition — into a model.Workflow.
func loadWorkflowFixture(t *testing.T) model.Workflow {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "workflow.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wf model.Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		t.Fatalf("workflow.json failed to parse: %v", err)
	}
	return wf
}

func TestWorkflowJSONFixtureParsesAndIsInternallyConsistent(t *testing.T) {
	wf := loadWorkflowFixture(t)

	ids := make(map[string]bool, len(wf.States))
	for _, s := range wf.States {
		if ids[s.GetId()] {
			t.Errorf("duplicate state id %q", s.GetId())
		}
		ids[s.GetId()] = true
	}

	if !ids[wf.InitialState] {
		t.Errorf("initialState %q is not a defined state", wf.InitialState)
	}
	for _, end := range wf.EndStates {
		if !ids[end] {
			t.Errorf("endState %q is not a defined state", end)
		}
	}

	checkRef := func(kind, from, to string) {
		if !ids[to] {
			t.Errorf("%s %q -> %q: target %q is not a defined state", kind, from, to, to)
		}
	}
	for _, tr := range wf.Transitions {
		if !ids[tr.Source] {
			t.Errorf("transition source %q is not a defined state", tr.Source)
		}
		checkRef("transition", tr.Source, tr.Target)
	}
	for _, ct := range wf.CommonTransitions {
		checkRef("commonTransition", "(sourceList)", ct.Target)
		for _, s := range ct.SourceList {
			if !ids[s] {
				t.Errorf("commonTransition sourceList entry %q is not a defined state", s)
			}
		}
	}

	if len(wf.States) != 46 {
		t.Errorf("got %d states, want 46 (update this count if workflow.json is intentionally edited)", len(wf.States))
	}
	if len(wf.Transitions) != 59 {
		t.Errorf("got %d transitions, want 59", len(wf.Transitions))
	}
	if len(wf.CommonTransitions) != 3 {
		t.Errorf("got %d commonTransitions, want 3", len(wf.CommonTransitions))
	}
}

// TestWorkflowJSONFixtureDrivesRealTransitions exercises the exact
// functions processEvent calls — resolveInitialPosition, findCandidateTransition,
// applyTransition, runAutomaticChain — against the real workflow.json,
// without the GORM transaction wrapper (and so without a live DB). The
// sibling test above is purely structural; this one actually moves an
// instance through the state machine.
func TestWorkflowJSONFixtureDrivesRealTransitions(t *testing.T) {
	def := loadWorkflowFixture(t)

	// Build the instance the way NewWorkflowInstance does.
	state, sub, err := resolveInitialPosition(def.States[0])
	if err != nil {
		t.Fatal(err)
	}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: def,
		CurrentState:       persistence.StateContainer{State: state, Substate: sub},
	}
	if wf.CurrentState.EffectiveId() != def.InitialState {
		t.Fatalf("initial EffectiveId() = %q, want %q", wf.CurrentState.EffectiveId(), def.InitialState)
	}

	// application_started has a single AUTOMATIC transition out of it, so a
	// freshly-created instance must never come to rest there.
	ctxMap, moved, err := runAutomaticChain(context.Background(), "fixture-id", wf, map[string]string{})
	if err != nil {
		t.Fatalf("automatic chain out of %q failed: %v", def.InitialState, err)
	}
	if !moved {
		t.Fatal("expected the initial AUTOMATIC transition to fire")
	}
	if got := wf.CurrentState.EffectiveId(); got != "applicant_type_selection" {
		t.Fatalf("after the automatic chain, state = %q, want applicant_type_selection", got)
	}

	if len(wf.Journey) != 1 {
		t.Fatalf("after the automatic chain, Journey has %d entries, want 1: %+v", len(wf.Journey), wf.Journey)
	}
	if wf.Journey[0].FromState != "application_started" || wf.Journey[0].ToState != "applicant_type_selection" {
		t.Fatalf("journey entry = %+v, want application_started -> applicant_type_selection", wf.Journey[0])
	}
	if wf.Journey[0].Event != "start" || wf.Journey[0].Trigger != model.AutomaticTrigger {
		t.Fatalf("journey entry = %+v, want event=start trigger=AUTOMATIC", wf.Journey[0])
	}

	// A common transition ("withdraw", authored against a sourceList that
	// includes applicant_type_selection) must resolve and apply.
	tr, ok := findCandidateTransition(wf, "withdraw", ctxMap)
	if !ok {
		t.Fatalf("no candidate transition for withdraw from %q", wf.CurrentState.EffectiveId())
	}
	if tr.Target != "application_withdrawn" {
		t.Fatalf("withdraw resolved to target %q, want application_withdrawn", tr.Target)
	}
	ctxMap, moved, err = applyTransition(context.Background(), "fixture-id", wf, tr, ctxMap)
	if err != nil {
		t.Fatal(err)
	}
	if !moved {
		t.Fatal("withdraw is an external transition; applyTransition must report a move")
	}
	if wf.CurrentState.EffectiveId() != "application_withdrawn" {
		t.Fatalf("state = %q, want application_withdrawn", wf.CurrentState.EffectiveId())
	}
	if wf.LastTransition == "" {
		t.Fatal("LastTransition must be stamped by applyTransition")
	}

	if len(wf.Journey) != 2 {
		t.Fatalf("after withdraw, Journey has %d entries, want 2: %+v", len(wf.Journey), wf.Journey)
	}
	if wf.Journey[1].Event != "withdraw" || wf.Journey[1].ToState != "application_withdrawn" {
		t.Fatalf("journey entry = %+v, want event=withdraw -> application_withdrawn", wf.Journey[1])
	}

	// application_withdrawn is an end state, so the instance is complete and
	// offers no further events.
	if !isEndState(*wf) {
		t.Fatal("application_withdrawn must be recognised as an end state")
	}
	if events := possibleEventsFor(wf, ctxMap); len(events) != 0 {
		t.Fatalf("possibleEventsFor(end state) = %v, want none", events)
	}
}
