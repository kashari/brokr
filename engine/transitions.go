package engine

import (
	"context"
	"fmt"

	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
)

// findStateById returns the state in states with the given id, or nil.
func findStateById(states []model.State, id string) model.State {
	for _, s := range states {
		if s.GetId() == id {
			return s
		}
	}
	return nil
}

// matchIn returns the first transition in transitions whose Source and
// Event match and whose Guard (if any) passes against ctxMap — first
// authored match wins among passing candidates, matching the engine's
// existing "first match" semantics with guards layered on top.
func matchIn(transitions []model.Transition, sourceId, event string, ctxMap map[string]string) (model.Transition, bool) {
	for _, t := range transitions {
		if t.Source != sourceId || t.Event != event {
			continue
		}
		if !t.Guard.Evaluate(ctxMap) {
			continue
		}
		return t, true
	}
	return model.Transition{}, false
}

// matchCommonTransition returns the first CommonTransition whose
// SourceList contains sourceId and whose Event/Guard match, synthesized
// into a plain Transition.
func matchCommonTransition(common []model.CommonTransition, sourceId, event string, ctxMap map[string]string) (model.Transition, bool) {
	for _, ct := range common {
		if ct.Event != event {
			continue
		}
		inList := false
		for _, s := range ct.SourceList {
			if s == sourceId {
				inList = true
				break
			}
		}
		if !inList || !ct.Guard.Evaluate(ctxMap) {
			continue
		}
		return model.Transition{
			Source:       sourceId,
			Target:       ct.Target,
			Event:        ct.Event,
			Trigger:      ct.Trigger,
			Kind:         ct.Kind,
			Guard:        ct.Guard,
			EntryActions: ct.EntryActions,
		}, true
	}
	return model.Transition{}, false
}

// findCandidateTransition resolves the transition that should fire for
// event given wf's current position, searching top-level Transitions
// then CommonTransitions. Guard-failing candidates are skipped in favor
// of the next match, not treated as "no transition."
func findCandidateTransition(wf *persistence.WorkflowInstance, event string, ctxMap map[string]string) (model.Transition, bool) {
	sourceId := wf.CurrentState.State.GetId()
	if t, ok := matchIn(wf.WorkflowDefinition.Transitions, sourceId, event, ctxMap); ok {
		return t, true
	}
	return matchCommonTransition(wf.WorkflowDefinition.CommonTransitions, sourceId, event, ctxMap)
}

// applyTransition executes t against wf: exit actions of the current
// state, the state change itself, entry actions of the new state, then
// t's own EntryActions. It mutates wf.CurrentState and wf.LastTransition
// in place and returns the updated ctxMap. It does not touch the
// database — the caller (processEvent) persists wf once, after however
// many transitions fire in one call (see Task 6).
func applyTransition(ctx context.Context, id string, wf *persistence.WorkflowInstance, t model.Transition, ctxMap map[string]string) (map[string]string, error) {
	fromId := wf.CurrentState.State.GetId()

	ctxMap, err := wf.CurrentState.State.ExecuteExitActions(ctx, id, ctxMap)
	if err != nil {
		return ctxMap, err
	}

	target := findStateById(wf.WorkflowDefinition.States, t.Target)
	if target == nil {
		return ctxMap, fmt.Errorf("transition target %q not found in workflow %q", t.Target, wf.WorkflowDefinition.Id)
	}
	wf.CurrentState = persistence.StateContainer{State: target}

	ctxMap, err = target.ExecuteEntryActions(ctx, id, ctxMap)
	if err != nil {
		return ctxMap, err
	}

	ctxMap, err = model.ExecuteActions(ctx, id, ctxMap, t.EntryActions)
	if err != nil {
		return ctxMap, err
	}

	wf.LastTransition = fmt.Sprintf("Event: %s, From: %s, To: %s", t.Event, fromId, wf.CurrentState.State.GetId())
	return ctxMap, nil
}
