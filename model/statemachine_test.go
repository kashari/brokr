package model

import (
	"encoding/json"
	"testing"
)

func TestTransitionTriggerJSONBackwardCompatible(t *testing.T) {
	// workflow.json already ships "type":"AUTOMATIC" and "join":true on real
	// transitions. Both must decode into the new fields without any change
	// to the JSON.
	data := []byte(`{"type":"AUTOMATIC","source":"a","target":"b","event":"e","join":true}`)
	var tr Transition
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	if tr.Trigger != AutomaticTrigger {
		t.Fatalf("Trigger = %q, want AUTOMATIC", tr.Trigger)
	}
	if !tr.IsJoin() {
		t.Fatal("legacy join:true must still count as a join")
	}
}

func TestTransitionKindExplicit(t *testing.T) {
	data := []byte(`{"kind":"Join","source":"a","target":"b","event":"e"}`)
	var tr Transition
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	if !tr.IsJoin() {
		t.Fatal("explicit kind:Join must count as a join even without the legacy bool")
	}
}
