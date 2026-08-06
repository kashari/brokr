package engine

import (
	"testing"

	"github.com/kashari/brokr/model"
)

func TestFindPendingJoinTransition(t *testing.T) {
	def := model.Workflow{
		Transitions: []model.Transition{
			{Source: "waiting", Event: "regions_done", Target: "next", Kind: model.JoinKind},
			{Source: "waiting", Event: "cancel", Target: "cancelled"},
		},
	}
	got, ok := findPendingJoinTransition(def, "waiting")
	if !ok || got.Event != "regions_done" {
		t.Fatalf("got %+v, ok=%v, want regions_done", got, ok)
	}
	if _, ok := findPendingJoinTransition(def, "elsewhere"); ok {
		t.Fatal("expected no join transition from a state with none")
	}
}
