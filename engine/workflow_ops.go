package engine

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

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
		CurrentState:       persistence.StateContainer{State: workflowDefinition.States[0]},
		LastTransition:     "",
	}
	// Chain any AUTOMATIC transitions out of the initial state before the
	// instance is ever persisted — nothing is saved yet, so running the
	// chain in-memory and creating once is simpler than wrapping this in
	// a transaction the way processEvent has to.
	ctxMap, err := runAutomaticChain(context.Background(), id.String(), wf, make(map[string]string))
	if err != nil {
		return uuid.Nil, err
	}
	wf.ContextMap = ctxMap
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
	var justCompleted bool

	defer func() {
		if err == nil {
			publishTransition(id, event, wf)
			// Notify the parent's actor in its own goroutine — DispatchAsync
			// can block if the parent's mailbox is momentarily full, and this
			// runs after the transaction above has already committed (no lock
			// held), so a slow/full parent mailbox can never stall this
			// instance's own actor loop.
			if justCompleted && wf.ParentId != nil {
				go attemptAutoJoin(wf.ParentId.String())
			}
		}
	}()

	txErr := db.Transaction(func(tx *gorm.DB) error {
		if result := tx.First(&wf, "id = ?", id); result.Error != nil {
			return result.Error
		}
		wasComplete := wf.Complete

		golog.Info("Sending event [{}] to workflow instance [{}] in state [{}]", event, id, wf.CurrentState.State.GetId())

		ctxMap := wf.ContextMap
		if ctxMap == nil {
			ctxMap = make(map[string]string)
		}

		transition, found := findCandidateTransition(&wf, event, ctxMap)
		if !found {
			return &errors.NoTransitionError{CurrentState: wf.CurrentState.State.GetId(), Event: event}
		}

		if transition.IsJoin() {
			complete, joinErr := allChildrenComplete(ctx, id, wf.PendingForkGeneration)
			if joinErr != nil {
				return joinErr
			}
			if !complete {
				return &errors.ChildrenNotCompleteError{CurrentState: wf.CurrentState.State.GetId(), Event: event}
			}
		}

		var aerr error
		ctxMap, aerr = applyTransition(ctx, id, &wf, transition, ctxMap)
		if aerr != nil {
			return aerr
		}

		ctxMap, aerr = runAutomaticChain(ctx, id, &wf, ctxMap)
		if aerr != nil {
			return aerr
		}

		wf.ContextMap = ctxMap
		wf.Complete = isEndState(wf)
		wf.Version++
		justCompleted = !wasComplete && wf.Complete

		if result := tx.Save(&wf); result.Error != nil {
			return result.Error
		}
		return nil
	})
	if txErr != nil {
		return "", txErr
	}

	golog.Info("Workflow instance [{}] transitioned to state [{}]", id, wf.CurrentState.State.GetId())
	return wf.CurrentState.State.GetId(), nil
}

// possibleEventsFor lists every event that would actually succeed if sent
// right now: USER/SYSTEM-triggered (not AUTOMATIC — those fire
// themselves), guard-passing, from wf's current effective position
// (substate-local events first, matching findCandidateTransition's own
// scoping), plus any matching CommonTransitions.
func possibleEventsFor(wf *persistence.WorkflowInstance, ctxMap map[string]string) []string {
	cs := wf.CurrentState
	var events []string
	collect := func(transitions []model.Transition, sourceId string) {
		for _, t := range transitions {
			if t.Source != sourceId || t.Trigger == model.AutomaticTrigger {
				continue
			}
			if !t.Guard.Evaluate(ctxMap) {
				continue
			}
			events = append(events, t.Event)
		}
	}
	if cs.Substate != nil {
		if comp, ok := cs.State.(*model.CompositeState); ok {
			collect(comp.SubTransitions, cs.Substate.GetId())
		}
	}
	collect(wf.WorkflowDefinition.Transitions, cs.State.GetId())
	for _, ct := range wf.WorkflowDefinition.CommonTransitions {
		if ct.Trigger == model.AutomaticTrigger || !ct.Guard.Evaluate(ctxMap) {
			continue
		}
		for _, s := range ct.SourceList {
			if s == cs.State.GetId() {
				events = append(events, ct.Event)
				break
			}
		}
	}
	return events
}

func GetPossibleEventsForWorkflowInstance(id string) ([]string, error) {
	db := config.Db
	var wf persistence.WorkflowInstance
	if result := db.First(&wf, "id = ?", id); result.Error != nil {
		return nil, result.Error
	}
	golog.Info("Getting possible events for workflow instance [{}] in state [{}]", id, wf.CurrentState.EffectiveId())
	return possibleEventsFor(&wf, wf.ContextMap), nil
}
