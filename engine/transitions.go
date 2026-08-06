package engine

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
)

// createChildBatchFn is engine.CreateChildWorkflowInstancesBatchWithGeneration,
// indirected so tests can substitute a fake instead of needing a live DB.
var createChildBatchFn = CreateChildWorkflowInstancesBatchWithGeneration

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
	if t.Kind == model.InternalKind {
		// UML internal transition: runs its own effect only. No exit, no
		// state change, no entry — the state's own entry/exit actions
		// never fire for a self-transition of this kind.
		return model.ExecuteActions(ctx, id, ctxMap, t.EntryActions)
	}

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

	if t.Kind == model.ForkKind {
		// Fork fires atomically: stamp one generation id for all regions so
		// a later join (Task 10/11) can tell which batch of children it's
		// waiting on, then spawn them. t.Target is the "fork-and-wait"
		// placeholder state the parent sits in until then.
		forkGen := uuid.New().String()
		defs := make([]model.Workflow, len(t.ForkTargets))
		for i, ft := range t.ForkTargets {
			if ft.ChildWorkflow == nil {
				return ctxMap, fmt.Errorf("fork transition %s->%s: forkTargets[%d] missing childWorkflow", t.Source, t.Target, i)
			}
			defs[i] = *ft.ChildWorkflow
		}
		if _, err := createChildBatchFn(id, defs, forkGen); err != nil {
			return ctxMap, fmt.Errorf("fork transition: %w", err)
		}
		wf.PendingForkGeneration = forkGen
	} else if t.IsJoin() {
		// Firing a join clears the generation the parent was waiting on —
		// it's no longer pending once the join transition itself applies.
		wf.PendingForkGeneration = ""
	}

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

// maxAutomaticHops bounds how many AUTOMATIC transitions can chain in a
// single processEvent call. A workflow author who accidentally creates a
// cycle of completion transitions (A -auto-> B -auto-> A) would otherwise
// hang the instance's actor forever; this turns that authoring bug into
// an error instead.
const maxAutomaticHops = 100

// findAutomaticTransition returns the first zero-delay AUTOMATIC
// transition out of sourceId whose Guard passes. After > 0 transitions
// are deferred timers (Task 13), not chained synchronously here.
func findAutomaticTransition(wfDef model.Workflow, sourceId string, ctxMap map[string]string) (model.Transition, bool) {
	for _, t := range wfDef.Transitions {
		if t.Source != sourceId || t.Trigger != model.AutomaticTrigger || t.After != "" {
			continue
		}
		if !t.Guard.Evaluate(ctxMap) {
			continue
		}
		return t, true
	}
	return model.Transition{}, false
}

// findDeferredTransitions is Task 13's counterpart: all AUTOMATIC
// transitions out of sourceId with After > 0, whose Guard passes.

// runAutomaticChain repeatedly applies findAutomaticTransition's match
// against wf's new position until none matches, capped by
// maxAutomaticHops. It mutates wf and returns the final ctxMap — the
// return value must be threaded back to the caller explicitly because
// applyTransition may hand back a freshly allocated map when ctxMap
// starts nil, so relying on Go's reference-type map semantics alone
// would be fragile.
func runAutomaticChain(ctx context.Context, id string, wf *persistence.WorkflowInstance, ctxMap map[string]string) (map[string]string, error) {
	for hop := 0; ; hop++ {
		if hop >= maxAutomaticHops {
			return ctxMap, fmt.Errorf("automatic transition chain from state %q exceeded %d hops (likely a cycle in the workflow definition)", wf.CurrentState.State.GetId(), maxAutomaticHops)
		}
		t, ok := findAutomaticTransition(wf.WorkflowDefinition, wf.CurrentState.State.GetId(), ctxMap)
		if !ok {
			return ctxMap, nil
		}
		var err error
		ctxMap, err = applyTransition(ctx, id, wf, t, ctxMap)
		if err != nil {
			return ctxMap, err
		}
	}
}
