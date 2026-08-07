package engine

import (
	"testing"

	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
)

func TestFindPendingJoinTransition(t *testing.T) {
	waiting := &model.SimpleState{Type: "SimpleState", Id: "waiting"}
	elsewhere := &model.SimpleState{Type: "SimpleState", Id: "elsewhere"}
	def := model.Workflow{
		States: []model.State{waiting, elsewhere},
		Transitions: []model.Transition{
			{Source: "waiting", Event: "regions_done", Target: "next", Kind: model.JoinKind},
			{Source: "waiting", Event: "cancel", Target: "cancelled"},
		},
	}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: def,
		CurrentState:       persistence.StateContainer{State: waiting},
	}
	got, ok := findPendingJoinTransition(wf)
	if !ok || got.Event != "regions_done" {
		t.Fatalf("got %+v, ok=%v, want regions_done", got, ok)
	}

	wf.CurrentState = persistence.StateContainer{State: elsewhere}
	if _, ok := findPendingJoinTransition(wf); ok {
		t.Fatal("expected no join transition from a state with none")
	}
}

// A join authored inside a composite state must be found automatically,
// exactly as findCandidateTransition would find it for a hand-sent event.
func TestFindPendingJoinTransitionInsideComposite(t *testing.T) {
	waiting := &model.SimpleState{Type: "SimpleState", Id: "waiting"}
	comp := &model.CompositeState{
		Type: "CompositeState", Id: "regions", InitialSubstate: "waiting",
		Substates: []model.State{waiting},
		SubTransitions: []model.Transition{
			{Source: "waiting", Event: "regions_done", Target: "next", Kind: model.JoinKind},
		},
	}
	wf := &persistence.WorkflowInstance{
		WorkflowDefinition: model.Workflow{States: []model.State{comp}},
		CurrentState:       persistence.StateContainer{State: comp, Substate: waiting},
	}

	got, ok := findPendingJoinTransition(wf)
	if !ok || got.Event != "regions_done" {
		t.Fatalf("got %+v, ok=%v, want the composite's regions_done sub-transition", got, ok)
	}

	// And the same event must resolve identically through the manual path.
	manual, ok := findCandidateTransition(wf, "regions_done", nil)
	if !ok || manual.Target != got.Target {
		t.Fatalf("manual dispatch resolved %+v, ok=%v — must match the auto-join lookup %+v", manual, ok, got)
	}
}
