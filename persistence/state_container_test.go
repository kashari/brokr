package persistence

import (
	"testing"

	"github.com/kashari/brokr/model"
)

func TestStateContainerRoundTripsWithSubstate(t *testing.T) {
	c := StateContainer{
		State:    &model.CompositeState{Type: "CompositeState", Id: "review"},
		Substate: &model.SimpleState{Type: "SimpleState", Id: "collecting"},
	}
	v, err := c.Value()
	if err != nil {
		t.Fatal(err)
	}

	var out StateContainer
	if err := out.Scan(v); err != nil {
		t.Fatal(err)
	}
	if out.EffectiveId() != "collecting" {
		t.Fatalf("EffectiveId() = %q, want collecting", out.EffectiveId())
	}
	if out.State.GetId() != "review" {
		t.Fatalf("State.GetId() = %q, want review", out.State.GetId())
	}
}

func TestStateContainerEffectiveIdWithoutSubstate(t *testing.T) {
	c := StateContainer{State: &model.SimpleState{Type: "SimpleState", Id: "top"}}
	if c.EffectiveId() != "top" {
		t.Fatalf("EffectiveId() = %q, want top", c.EffectiveId())
	}
}
