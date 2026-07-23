package engine

import (
	"github.com/google/uuid"

	"github.com/kashari/brokr/config"
	"github.com/kashari/brokr/errors"
	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
	"github.com/kashari/golog"
)

// NewWorkflowInstance creates a new workflow instance in the database based on the provided workflow definition.
//
// It generates a new UUID for the instance, sets the initial state, and saves it to the database,
// then returns the UUID of the newly created workflow instance and any error encountered during the process.
//
// Parameters:
// - workflowDefinition: The definition of the workflow to instantiate, including its states and transitions.
//
// Returns:
//
// - uuid.UUID: The unique identifier of the newly created workflow instance.
//
// - error: An error object if an error occurred during the creation process; otherwise, nil.
func NewWorkflowInstance(workflowDefinition model.Workflow) (uuid.UUID, error) {
	db := config.Db
	id := uuid.New()
	golog.Info("Creating new workflow instance [{} -> {}] v. {}", id.String(), workflowDefinition.Id, workflowDefinition.Version)
	wf := &persistence.WorkflowInstance{
		Id:                 id,
		WorkflowDefinition: workflowDefinition,
		CurrentState:       workflowDefinition.States[0],
		LastTransition:     "",
	}
	db.Create(wf)
	return id, nil
}

// GetWorkflowInstance retrieves a workflow instance from the database by its unique identifier.
//
// It queries the database for a workflow instance with the specified ID and returns the corresponding workflow definition.
//
// Parameters:
// - id: The unique identifier of the workflow instance to retrieve.
//
// Returns:
//
// - model.Workflow: The workflow definition associated with the specified instance ID.
//
// - error: An error object if an error occurred during retrieval or if the instance was not found; otherwise, nil.
func GetWorkflowInstance(id string) (model.Workflow, error) {
	db := config.Db
	var wf persistence.WorkflowInstance
	result := db.First(&wf, "id = ?", id)
	if result.Error != nil {
		return model.Workflow{}, result.Error
	}
	return wf.WorkflowDefinition, nil
}

func SendEventToWorkflowInstance(id string, event string) (string, error) {
	db := config.Db
	var wf persistence.WorkflowInstance
	result := db.First(&wf, "id = ?", id)
	if result.Error != nil {
		return "", result.Error
	}

	currentState := wf.CurrentState
	golog.Info("Sending event [{}] to workflow instance [{}] in state [{}]", event, id, currentState.GetId())

	// Find the transition for the current state and event
	var transition model.Transition
	found := false
	for _, t := range wf.WorkflowDefinition.Transitions {
		if t.Source == currentState.GetId() && t.Event == event {
			transition = t
			found = true
			break
		}
	}

	if !found {
		return "", &errors.NoTransitionError{CurrentState: currentState.GetId(), Event: event}
	}

	// Execute exit actions of the current state
	ctxMap, err := currentState.ExecuteExitActions(id, nil)
	if err != nil {
		return "", err
	}

	// Update the current state to the target state of the transition
	for _, state := range wf.WorkflowDefinition.States {
		if state.GetId() == transition.Target {
			wf.CurrentState = state
			break
		}
	}

	// Execute entry actions of the new current state
	ctxMap, err = wf.CurrentState.ExecuteEntryActions(id, ctxMap)
	if err != nil {
		return "", err
	}

	// Execute entry actions of the transition
	for _, action := range transition.EntryActions {
		ctxMap, err = action.Execute(id, ctxMap)
		if err != nil {
			return "", err
		}
	}

	wf.LastTransition = transition.Event

	db.Save(&wf)

	golog.Info("Workflow instance [{}] transitioned to state [{}]", id, wf.CurrentState.GetId())
	return wf.CurrentState.GetId(), nil
}
