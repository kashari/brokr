package engine

import (
	"testing"

	"github.com/google/uuid"
	"github.com/kashari/brokr/persistence"
)

func TestBuildVisualizationDataFromRealWorkflow(t *testing.T) {
	def := loadWorkflowFixture(t)

	state, sub, err := resolveInitialPosition(def.States[0])
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	wf := persistence.WorkflowInstance{
		Id:                 id,
		WorkflowDefinition: def,
		CurrentState:       persistence.StateContainer{State: state, Substate: sub},
		ContextMap:         map[string]string{"documentId": "doc-123"},
	}
	wf.Journey = append(wf.Journey, initialJourneyEntry(state, sub))

	data := buildVisualizationData(wf, []string{"start"}, nil)

	if data.Instance.Id != id.String() {
		t.Fatalf("Instance.Id = %q, want %q", data.Instance.Id, id.String())
	}
	if data.Instance.WorkflowId != "bank-account-opening" {
		t.Fatalf("Instance.WorkflowId = %q", data.Instance.WorkflowId)
	}
	if data.Instance.CurrentState != "application_started" {
		t.Fatalf("Instance.CurrentState = %q, want application_started", data.Instance.CurrentState)
	}
	if data.Instance.ParentId != nil {
		t.Fatalf("Instance.ParentId = %v, want nil for a root instance", data.Instance.ParentId)
	}
	if len(data.Journey) != 1 || data.Journey[0].ToState != "application_started" {
		t.Fatalf("Journey = %+v", data.Journey)
	}
	if data.Graph.WorkflowId != "bank-account-opening" || len(data.Graph.Nodes) == 0 {
		t.Fatalf("Graph not populated: %+v", data.Graph)
	}
	if len(data.PossibleEvents) != 1 || data.PossibleEvents[0] != "start" {
		t.Fatalf("PossibleEvents = %v", data.PossibleEvents)
	}
	if data.Children != nil {
		t.Fatalf("Children = %v, want nil when none were passed in", data.Children)
	}

	parent := uuid.New()
	wf.ParentId = &parent
	data2 := buildVisualizationData(wf, nil, nil)
	if data2.Instance.ParentId == nil || *data2.Instance.ParentId != parent.String() {
		t.Fatalf("Instance.ParentId = %v, want %q", data2.Instance.ParentId, parent.String())
	}
}
