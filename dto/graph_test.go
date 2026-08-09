package dto

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kashari/brokr/model"
)

// loadWorkflowFixture parses the repo's workflow.json — mirrors
// engine/workflow_json_fixture_test.go's helper of the same name.
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

func TestBuildGraphFromRealWorkflow(t *testing.T) {
	wf := loadWorkflowFixture(t)
	g := BuildGraph(wf)

	if g.WorkflowId != "bank-account-opening" || g.InitialState != "application_started" {
		t.Fatalf("got WorkflowId=%q InitialState=%q", g.WorkflowId, g.InitialState)
	}
	if len(g.Nodes) < len(wf.States) {
		t.Fatalf("got %d nodes, want at least %d (one per top-level state)", len(g.Nodes), len(wf.States))
	}
	if len(g.Edges) < len(wf.Transitions) {
		t.Fatalf("got %d edges, want at least %d (one per top-level transition)", len(g.Edges), len(wf.Transitions))
	}

	var initial, personalDetails *GraphNode
	for i := range g.Nodes {
		switch g.Nodes[i].Id {
		case "application_started":
			initial = &g.Nodes[i]
		case "personal_details_collection":
			personalDetails = &g.Nodes[i]
		}
	}
	if initial == nil || !initial.IsInitial {
		t.Fatalf("application_started node = %+v, want IsInitial=true", initial)
	}
	if personalDetails == nil {
		t.Fatal("personal_details_collection node not found")
	}
	if len(personalDetails.EntryActions) != 1 || personalDetails.EntryActions[0].Type != string(model.SetContextMapAction) {
		t.Fatalf("personal_details_collection.EntryActions = %+v, want one SetContextMapAction", personalDetails.EntryActions)
	}

	for _, end := range g.EndStates {
		found := false
		for _, n := range g.Nodes {
			if n.Id == end && n.IsEnd {
				found = true
			}
		}
		if !found {
			t.Errorf("end state %q not marked IsEnd on its node", end)
		}
	}

	// A CommonTransition must expand into one edge per entry in its
	// SourceList, each carrying scope "common".
	var commonEdges int
	for _, e := range g.Edges {
		if e.Scope == "common" {
			commonEdges++
		}
	}
	wantCommonEdges := 0
	for _, ct := range wf.CommonTransitions {
		wantCommonEdges += len(ct.SourceList)
	}
	if commonEdges != wantCommonEdges {
		t.Fatalf("got %d common-scoped edges, want %d", commonEdges, wantCommonEdges)
	}
}
