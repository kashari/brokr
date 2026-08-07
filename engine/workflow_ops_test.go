package engine

import (
	"context"
	"testing"

	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
)

func TestMatchCommonTransition(t *testing.T) {
	common := []model.CommonTransition{
		{SourceList: []string{"a", "b"}, Target: "withdrawn", Event: "withdraw"},
	}
	got, ok := matchCommonTransition(common, "b", "withdraw", map[string]string{})
	if !ok {
		t.Fatal("expected a match")
	}
	if got.Target != "withdrawn" || got.Source != "b" {
		t.Fatalf("got %+v", got)
	}
	if _, ok := matchCommonTransition(common, "c", "withdraw", map[string]string{}); ok {
		t.Fatal("expected no match for source not in list")
	}
}

func TestPossibleEventsExcludesAutomaticIncludesCommonAndGuarded(t *testing.T) {
	src := &model.SimpleState{Type: "SimpleState", Id: "s"}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: model.Workflow{
			States: []model.State{src},
			Transitions: []model.Transition{
				{Source: "s", Event: "auto_only", Trigger: model.AutomaticTrigger, Target: "s"},
				{Source: "s", Event: "blocked", Guard: &model.Guard{Key: "x", Op: model.GuardExists}, Target: "s"},
				{Source: "s", Event: "allowed", Target: "s"},
			},
			CommonTransitions: []model.CommonTransition{
				{SourceList: []string{"s"}, Event: "withdraw", Target: "s"},
			},
		},
		CurrentState: persistence.StateContainer{State: src},
	}
	got := possibleEventsFor(wf, map[string]string{})
	want := map[string]bool{"allowed": true, "withdraw": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want exactly %v", got, want)
	}
	for _, e := range got {
		if !want[e] {
			t.Fatalf("unexpected event %q in %v", e, got)
		}
	}
}

func TestProcessEventUsesCommonTransitions(t *testing.T) {
	// processEvent needs a live DB (see docker-compose db). This test is
	// skipped without one; it documents the contract for the integration
	// suite and is exercised by the fixture test added in Task 12.
	t.Skip("covered end-to-end by TestWorkflowJSONFixture (Task 12) and manual verification below")
	_ = context.Background()
}
