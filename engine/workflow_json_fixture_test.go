package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kashari/brokr/model"
)

func TestWorkflowJSONFixtureParsesAndIsInternallyConsistent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "workflow.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wf model.Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		t.Fatalf("workflow.json failed to parse: %v", err)
	}

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
