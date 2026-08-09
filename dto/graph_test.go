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

func TestBuildGraphWithCompositeState(t *testing.T) {
	// Build a synthetic workflow with a CompositeState to cover the
	// composite-state code path in BuildGraph.
	substateCollecting := &model.SimpleState{
		Type: "SimpleState",
		Id:   "collecting",
	}
	substateVerifying := &model.ActionState{
		Type:         "ActionState",
		Id:           "verifying",
		EntryActions: []model.Action{},
		ExitActions:  []model.Action{},
		DoActions:    []model.Action{},
	}
	compositeReview := &model.CompositeState{
		Type:            "CompositeState",
		Id:              "review",
		InitialSubstate: "collecting",
		Substates:       []model.State{substateCollecting, substateVerifying},
		SubTransitions: []model.Transition{
			{
				Source: "collecting",
				Target: "verifying",
				Event:  "submit",
			},
		},
		History: model.ShallowHistory,
	}
	stateStart := &model.SimpleState{
		Type: "SimpleState",
		Id:   "start",
	}
	stateEnd := &model.SimpleState{
		Type: "SimpleState",
		Id:   "end",
	}

	wf := model.Workflow{
		Id:           "test-composite-workflow",
		Version:      "1.0",
		InitialState: "start",
		EndStates:    []string{"end"},
		States: []model.State{
			stateStart,
			compositeReview,
			stateEnd,
		},
		Transitions: []model.Transition{
			{
				Source: "start",
				Target: "review",
				Event:  "begin_review",
			},
			{
				Source: "review",
				Target: "end",
				Event:  "complete",
			},
		},
		CommonTransitions: []model.CommonTransition{},
	}

	g := BuildGraph(wf)

	// Verify basic graph properties
	if g.WorkflowId != "test-composite-workflow" {
		t.Fatalf("got WorkflowId=%q, want test-composite-workflow", g.WorkflowId)
	}
	if g.InitialState != "start" {
		t.Fatalf("got InitialState=%q, want start", g.InitialState)
	}

	// Exact node count: 3 top-level states + 2 substates = 5 nodes
	wantNodes := 5
	if len(g.Nodes) != wantNodes {
		t.Fatalf("got %d nodes, want exactly %d (3 top-level + 2 substates)", len(g.Nodes), wantNodes)
	}

	// Exact edge count: 2 top-level transitions + 1 sub-transition = 3 edges
	wantEdges := 3
	if len(g.Edges) != wantEdges {
		t.Fatalf("got %d edges, want exactly %d (2 top-level + 1 sub-transition)", len(g.Edges), wantEdges)
	}

	// Verify the composite node exists and has correct properties
	var compositeNode *GraphNode
	for i := range g.Nodes {
		if g.Nodes[i].Id == "review" {
			compositeNode = &g.Nodes[i]
			break
		}
	}
	if compositeNode == nil {
		t.Fatal("composite node 'review' not found in graph")
	}
	if len(compositeNode.Substates) != 2 {
		t.Fatalf("composite node Substates = %v, want [collecting verifying]", compositeNode.Substates)
	}
	if compositeNode.Substates[0] != "collecting" || compositeNode.Substates[1] != "verifying" {
		t.Fatalf("composite node Substates = %v, want [collecting verifying]", compositeNode.Substates)
	}
	if compositeNode.InitialSubstate != "collecting" {
		t.Fatalf("composite node InitialSubstate = %q, want collecting", compositeNode.InitialSubstate)
	}
	if compositeNode.History != string(model.ShallowHistory) {
		t.Fatalf("composite node History = %q, want shallow", compositeNode.History)
	}

	// Verify substate nodes have correct ParentId
	for i := range g.Nodes {
		if g.Nodes[i].Id == "collecting" || g.Nodes[i].Id == "verifying" {
			if g.Nodes[i].ParentId != "review" {
				t.Fatalf("substate %q ParentId = %q, want review", g.Nodes[i].Id, g.Nodes[i].ParentId)
			}
		}
	}

	// Verify at least one sub-transition edge with correct scope
	var foundSubEdge bool
	for _, e := range g.Edges {
		if e.Scope == "sub:review" && e.Source == "collecting" && e.Target == "verifying" {
			foundSubEdge = true
			if e.Event != "submit" {
				t.Fatalf("sub-transition event = %q, want submit", e.Event)
			}
			break
		}
	}
	if !foundSubEdge {
		t.Fatal("no sub-transition edge found with Scope='sub:review', Source='collecting', Target='verifying'")
	}

	// Verify top-level transitions are scoped correctly
	var foundTopEdges int
	for _, e := range g.Edges {
		if e.Scope == "top" && (e.Source == "start" || e.Source == "review") {
			foundTopEdges++
		}
	}
	if foundTopEdges != 2 {
		t.Fatalf("got %d top-level edges, want exactly 2", foundTopEdges)
	}
}

func TestBuildGraphIsHappyFlowDefaultsTrueAndRespectsOverride(t *testing.T) {
	falseVal := false

	happy := &model.SimpleState{Type: "SimpleState", Id: "happy"} // IsHappyFlow left nil
	unhappy := &model.ActionState{Type: "ActionState", Id: "unhappy", IsHappyFlow: &falseVal}
	comp := &model.CompositeState{
		Type: "CompositeState", Id: "comp", InitialSubstate: "happy",
		Substates: []model.State{happy},
	}

	wf := model.Workflow{
		States: []model.State{comp, unhappy},
	}
	g := BuildGraph(wf)

	var compNode, subNode, unhappyNode *GraphNode
	for i := range g.Nodes {
		switch g.Nodes[i].Id {
		case "comp":
			compNode = &g.Nodes[i]
		case "happy":
			subNode = &g.Nodes[i]
		case "unhappy":
			unhappyNode = &g.Nodes[i]
		}
	}
	if compNode == nil || subNode == nil || unhappyNode == nil {
		t.Fatalf("missing expected nodes: comp=%v happy=%v unhappy=%v", compNode, subNode, unhappyNode)
	}
	if !compNode.IsHappyFlow {
		t.Error("composite state with nil IsHappyFlow should default to true")
	}
	if !subNode.IsHappyFlow {
		t.Error("substate with nil IsHappyFlow should default to true")
	}
	if unhappyNode.IsHappyFlow {
		t.Error("state with IsHappyFlow explicitly false should report false")
	}
}
