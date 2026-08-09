package engine

import (
	"github.com/kashari/brokr/dto"
	"github.com/kashari/brokr/persistence"
)

// buildVisualizationData composes an already-loaded workflow instance,
// its precomputed possible events, and its children into one
// dto.VisualizationData. It touches no database itself — GetVisualizationData
// is the thin DB-touching wrapper around it — so the composition logic
// (instance summary shape, nil-safety around ParentId/Substate) is testable
// without a live DB.
func buildVisualizationData(wf persistence.WorkflowInstance, possibleEvents []string, children []dto.ChildInstance) dto.VisualizationData {
	var parentId *string
	if wf.ParentId != nil {
		s := wf.ParentId.String()
		parentId = &s
	}

	currentState := ""
	if wf.CurrentState.State != nil {
		currentState = wf.CurrentState.State.GetId()
	}
	currentSubstate := ""
	if wf.CurrentState.Substate != nil {
		currentSubstate = wf.CurrentState.Substate.GetId()
	}

	return dto.VisualizationData{
		Instance: dto.InstanceSummary{
			Id:              wf.Id.String(),
			ParentId:        parentId,
			WorkflowId:      wf.WorkflowDefinition.Id,
			WorkflowVersion: wf.WorkflowDefinition.Version,
			CurrentState:    currentState,
			CurrentSubstate: currentSubstate,
			Complete:        wf.Complete,
			LastTransition:  wf.LastTransition,
			ContextMap:      wf.ContextMap,
			CreatedAt:       wf.CreatedAt,
			UpdatedAt:       wf.UpdatedAt,
		},
		Graph:          dto.BuildGraph(wf.WorkflowDefinition),
		Journey:        wf.Journey,
		PossibleEvents: possibleEvents,
		Children:       children,
	}
}

// GetVisualizationData loads workflow instance id and composes its full
// visualizer payload: its own summary, its definition's analyzed graph,
// its recorded journey, the events it could accept right now, and any
// children it has spawned.
func GetVisualizationData(id string) (dto.VisualizationData, error) {
	wf, err := GetWorkflowInstanceRaw(id)
	if err != nil {
		return dto.VisualizationData{}, err
	}
	events := possibleEventsFor(&wf, wf.ContextMap)
	children, err := GetChildWorkflowInstances(id)
	if err != nil {
		return dto.VisualizationData{}, err
	}
	return buildVisualizationData(wf, events, children), nil
}
