package model

import "testing"

func TestCompositeStateDecodesPolymorphicSubstates(t *testing.T) {
	data := []byte(`{
		"type": "CompositeState",
		"id": "review",
		"initialSubstate": "collecting",
		"history": "shallow",
		"substates": [
			{"type": "SimpleState", "id": "collecting"},
			{"type": "ActionState", "id": "verifying", "entryActions": []}
		],
		"subTransitions": [
			{"source": "collecting", "target": "verifying", "event": "submit"}
		]
	}`)
	s, err := DecodeState(data)
	if err != nil {
		t.Fatal(err)
	}
	comp, ok := s.(*CompositeState)
	if !ok {
		t.Fatalf("got %T, want *CompositeState", s)
	}
	if len(comp.Substates) != 2 {
		t.Fatalf("got %d substates, want 2", len(comp.Substates))
	}
	if comp.Substates[1].GetType() != ActionStateType {
		t.Fatalf("substate[1] type = %q, want ActionState (polymorphic decode failed)", comp.Substates[1].GetType())
	}
	if comp.History != ShallowHistory {
		t.Fatalf("History = %q, want shallow", comp.History)
	}
	if comp.GetDoActions() != nil {
		t.Fatal("CompositeState itself carries no do-activities in this iteration")
	}
}
