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

func TestStateContainerScansLegacyBareState(t *testing.T) {
	// Rows written before the envelope shape existed hold the bare state
	// object. Scan must still read them.
	legacy := []byte(`{"type":"SimpleState","id":"application_started","bulletName":"Started"}`)

	var out StateContainer
	if err := out.Scan(legacy); err != nil {
		t.Fatalf("Scan(legacy bare state): %v", err)
	}
	if out.State == nil {
		t.Fatal("State is nil")
	}
	if out.State.GetId() != "application_started" {
		t.Fatalf("State.GetId() = %q, want application_started", out.State.GetId())
	}
	if out.Substate != nil {
		t.Fatalf("Substate = %+v, want nil for a legacy bare-state row", out.Substate)
	}
	if out.EffectiveId() != "application_started" {
		t.Fatalf("EffectiveId() = %q, want application_started", out.EffectiveId())
	}
	if _, ok := out.State.(*model.SimpleState); !ok {
		t.Fatalf("State has concrete type %T, want *model.SimpleState", out.State)
	}
}

func TestStateContainerEffectiveIdWithoutSubstate(t *testing.T) {
	c := StateContainer{State: &model.SimpleState{Type: "SimpleState", Id: "top"}}
	if c.EffectiveId() != "top" {
		t.Fatalf("EffectiveId() = %q, want top", c.EffectiveId())
	}
}
