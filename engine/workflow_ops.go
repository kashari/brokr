package engine

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/kashari/brokr/config"
	"github.com/kashari/brokr/errors"
	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
	"github.com/kashari/golog"
)

// matchCommonTransition returns the first CommonTransition in common whose
// SourceList contains sourceId and whose Event matches, synthesized into a
// plain Transition so callers don't need two code paths.
func matchCommonTransition(common []model.CommonTransition, sourceId, event string) (model.Transition, bool) {
	for _, ct := range common {
		if ct.Event != event {
			continue
		}
		for _, s := range ct.SourceList {
			if s == sourceId {
				return model.Transition{
					Source:       sourceId,
					Target:       ct.Target,
					Event:        ct.Event,
					EntryActions: ct.EntryActions,
				}, true
			}
		}
	}
	return model.Transition{}, false
}

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
		CurrentState:       persistence.StateContainer{State: workflowDefinition.States[0]},
		LastTransition:     "",
	}
	wf.Complete = isEndState(*wf)
	if result := db.Create(wf); result.Error != nil {
		return uuid.Nil, result.Error
	}
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

// SendEventToWorkflowInstance processes an event for a specific workflow instance.
//
// It retrieves the workflow instance from the database, identifies the current state, and checks for a valid transition
// based on the provided event. If a valid transition is found, it executes the exit actions of the current state,
// updates the current state to the target state of the transition, executes the entry actions of the new state,
// and saves the updated workflow instance back to the database.
//
// Parameters:
//
// - id: The unique identifier of the workflow instance to which the event is sent.
//
// - event: The event to process for the workflow instance.
//
// Returns:
//
// - string: The ID of the new current state after processing the event.
//
// - error: An error object if an error occurred during processing or if no valid transition was found; otherwise, nil.
func SendEventToWorkflowInstance(id string, event string) (newState string, err error) {
	return Dispatch(context.Background(), id, event)
}

// processEvent performs one FSM step for instance id. It is only ever invoked
// from an instanceActor goroutine, so all events for a single instance are
// serialized here — eliminating the read-modify-write race the old inline
// handler had. The row read + write run in one transaction (atomic, and the
// seam for row locking if this ever needs to scale past one process), and the
// resulting state is published to the event bus after a successful commit.
func processEvent(ctx context.Context, id string, event string) (newState string, err error) {
	db := config.Db.WithContext(ctx)
	var wf persistence.WorkflowInstance

	defer func() {
		if err == nil {
			publishTransition(id, event, wf)
		}
	}()

	txErr := db.Transaction(func(tx *gorm.DB) error {
		if result := tx.First(&wf, "id = ?", id); result.Error != nil {
			return result.Error
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
			transition, found = matchCommonTransition(wf.WorkflowDefinition.CommonTransitions, currentState.GetId(), event)
		}

		if !found {
			return &errors.NoTransitionError{CurrentState: currentState.GetId(), Event: event}
		}

		if transition.Join {
			complete, joinErr := allChildrenComplete(ctx, id)
			if joinErr != nil {
				return joinErr
			}
			if !complete {
				return &errors.ChildrenNotCompleteError{CurrentState: currentState.GetId(), Event: event}
			}
		}

		// Execute exit actions of the current state
		ctxMap, aerr := currentState.ExecuteExitActions(ctx, id, nil)
		if aerr != nil {
			return aerr
		}

		// Update the current state to the target state of the transition
		for _, state := range wf.WorkflowDefinition.States {
			if state.GetId() == transition.Target {
				wf.CurrentState = persistence.StateContainer{State: state}
				break
			}
		}

		// Execute entry actions of the new current state
		ctxMap, aerr = wf.CurrentState.ExecuteEntryActions(ctx, id, ctxMap)
		if aerr != nil {
			return aerr
		}

		// Execute entry actions of the transition
		ctxMap, aerr = model.ExecuteActions(ctx, id, ctxMap, transition.EntryActions)
		if aerr != nil {
			return aerr
		}

		wf.LastTransition = fmt.Sprintf("Event: %s, From: %s, To: %s", event, currentState.GetId(), wf.CurrentState.GetId())
		wf.Complete = isEndState(wf)
		wf.Version++

		if result := tx.Save(&wf); result.Error != nil {
			return result.Error
		}
		return nil
	})
	if txErr != nil {
		return "", txErr
	}

	golog.Info("Workflow instance [{}] transitioned to state [{}]", id, wf.CurrentState.GetId())
	return wf.CurrentState.GetId(), nil
}

func GetPossibleEventsForWorkflowInstance(id string) ([]string, error) {
	db := config.Db
	var wf persistence.WorkflowInstance
	result := db.First(&wf, "id = ?", id)
	if result.Error != nil {
		return nil, result.Error
	}

	currentState := wf.CurrentState
	golog.Info("Getting possible events for workflow instance [{}] in state [{}]", id, currentState.GetId())

	var possibleEvents []string
	for _, t := range wf.WorkflowDefinition.Transitions {
		if t.Source == currentState.GetId() {
			possibleEvents = append(possibleEvents, t.Event)
		}
	}
	for _, ct := range wf.WorkflowDefinition.CommonTransitions {
		for _, s := range ct.SourceList {
			if s == currentState.GetId() {
				possibleEvents = append(possibleEvents, ct.Event)
				break
			}
		}
	}

	return possibleEvents, nil
}
