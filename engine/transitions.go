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

// transitionScope pairs a slice of transitions with the source id that
// candidates within it must be authored against.
type transitionScope struct {
	transitions []model.Transition
	sourceId    string
}

// transitionScopes returns the transition slices in scope for wf's current
// position, innermost first: when positioned inside a composite state, its
// SubTransitions (matched against the active substate's id) come before the
// workflow's top-level Transitions (matched against the composite's own id).
// That ordering is UML's rule — the innermost active state gets first
// refusal at an event — and every lookup keyed off "where the instance is
// right now" must use it, not just event matching. See findCandidateTransition,
// findAutomaticTransition, findDeferredTransitions and
// findPendingJoinTransition (engine/autojoin.go), which all share it.
func transitionScopes(wf *persistence.WorkflowInstance) []transitionScope {
	cs := wf.CurrentState
	var scopes []transitionScope
	if cs.Substate != nil {
		if comp, ok := cs.State.(*model.CompositeState); ok {
			scopes = append(scopes, transitionScope{transitions: comp.SubTransitions, sourceId: cs.Substate.GetId()})
		}
	}
	return append(scopes, transitionScope{transitions: wf.WorkflowDefinition.Transitions, sourceId: cs.State.GetId()})
}

// findCandidateTransition resolves the transition that should fire for
// event given wf's current position, searching innermost scope first (see
// transitionScopes). Only if no scoped transition matches does the search
// fall back to the workflow's CommonTransitions, authored against the
// top-level state's id — i.e. leaving the composite entirely.
func findCandidateTransition(wf *persistence.WorkflowInstance, event string, ctxMap map[string]string) (model.Transition, bool) {
	for _, sc := range transitionScopes(wf) {
		if t, ok := matchIn(sc.transitions, sc.sourceId, event, ctxMap); ok {
			return t, true
		}
	}
	return matchCommonTransition(wf.WorkflowDefinition.CommonTransitions, wf.CurrentState.State.GetId(), event, ctxMap)
}

// resolveTarget resolves t.Target into the (state, substate) pair the
// instance should occupy after t fires. A non-composite target yields a
// nil substate. A CompositeState target resolves to its InitialSubstate,
// unless t.EntersHistory is set and the composite's History is
// shallow/deep and history has a remembered substate for it.
func resolveTarget(wfDef model.Workflow, t model.Transition, history map[string]string) (model.State, model.State, error) {
	target := findStateById(wfDef.States, t.Target)
	if target == nil {
		return nil, nil, fmt.Errorf("transition target %q not found in workflow %q", t.Target, wfDef.Id)
	}
	comp, isComposite := target.(*model.CompositeState)
	if !isComposite {
		return target, nil, nil
	}
	substateId := comp.InitialSubstate
	if t.EntersHistory && comp.History != model.NoHistory {
		if last, ok := history[comp.Id]; ok && last != "" {
			substateId = last
		}
	}
	sub := comp.FindSubstate(substateId)
	if sub == nil {
		return nil, nil, fmt.Errorf("composite %q: substate %q not found", comp.Id, substateId)
	}
	return target, sub, nil
}

// resolveInitialPosition resolves the state a freshly-created instance
// starts in into the (state, substate) pair it should actually occupy —
// the creation-time counterpart of resolveTarget. A composite initial
// state resolves to its InitialSubstate; anything else yields a nil
// substate. Without this an instance whose first state is a composite
// would sit inside it with no active substate, so its SubTransitions
// would never be in scope and EffectiveState() would return the composite
// (whose GetDoActions() is always nil).
func resolveInitialPosition(state model.State) (model.State, model.State, error) {
	comp, isComposite := state.(*model.CompositeState)
	if !isComposite {
		return state, nil, nil
	}
	sub := comp.FindSubstate(comp.InitialSubstate)
	if sub == nil {
		return nil, nil, fmt.Errorf("composite %q: initial substate %q not found", comp.Id, comp.InitialSubstate)
	}
	return comp, sub, nil
}

// armStateActivities starts the do-activity and the deferred-transition
// timers for wf's *settled* current state.
//
// It is deliberately not called from applyTransition: applyTransition runs
// inside processEvent's database transaction, so arming there would leave
// a do-activity and timers running against a state the database may never
// actually record (a later chain hop or the Save itself can still fail and
// roll the transaction back). Callers must invoke this only once the state
// is durably persisted — see processEvent's post-commit defer and
// NewWorkflowInstance/CreateChildWorkflowInstance.
func armStateActivities(id string, wf *persistence.WorkflowInstance, ctxMap map[string]string) {
	startDoActivity(id, wf.CurrentState.EffectiveState().GetDoActions())
	startTimers(id, findDeferredTransitions(wf, ctxMap))
}

// applyTransition executes t against wf: exit actions of the current
// state, the state change itself, entry actions of the new state, then
// t's own EntryActions. It mutates wf.CurrentState and wf.LastTransition
// in place and returns the updated ctxMap plus whether the instance
// actually moved (false for an InternalKind transition, which by
// definition neither exits nor re-enters its state). It does not touch the
// database — the caller (processEvent) persists wf once, after however
// many transitions fire in one call (see Task 6).
//
// Side-effect ordering: leaving a state stops its do-activity and timers
// immediately (cancelling a goroutine early is harmless even if the
// enclosing transaction later rolls back — worst case it is re-armed on the
// next event), but *starting* the new state's do-activity and timers is
// left to the caller via armStateActivities, after the transaction commits.
//
// Known limitation — fork children escape the transaction: a ForkKind
// transition spawns its children through createChildBatchFn, which writes
// with config.Db rather than processEvent's enclosing *gorm.DB transaction.
// Those child rows are therefore committed independently and immediately.
// If anything later in the same processEvent call fails (a subsequent
// automatic-chain hop, or tx.Save), the parent's transition — including the
// PendingForkGeneration stamp — is rolled back while the children remain
// committed, leaving them orphaned against a generation the parent never
// recorded. Fixing this properly means threading the transaction handle
// down through CreateChildWorkflowInstancesBatchWithGeneration; until then
// the gap is documented rather than silent.
func applyTransition(ctx context.Context, id string, wf *persistence.WorkflowInstance, t model.Transition, ctxMap map[string]string) (map[string]string, bool, error) {
	if t.Kind == model.InternalKind {
		ctxMap, err := model.ExecuteActions(ctx, id, ctxMap, t.EntryActions)
		return ctxMap, false, err
	}

	stopDoActivity(id)
	stopTimers(id)

	fromId := wf.CurrentState.EffectiveId()

	// local: staying inside the same composite (t.Target names one of its
	// own substates), inferred rather than passed in — see Task 16's note.
	local := false
	if comp, ok := wf.CurrentState.State.(*model.CompositeState); ok && wf.CurrentState.Substate != nil {
		local = comp.FindSubstate(t.Target) != nil
	}

	var err error
	if wf.CurrentState.Substate != nil {
		ctxMap, err = wf.CurrentState.Substate.ExecuteExitActions(ctx, id, ctxMap)
		if err != nil {
			return ctxMap, true, err
		}
	}
	if !local {
		if wf.CurrentState.Substate != nil {
			if wf.CompositeHistory == nil {
				wf.CompositeHistory = make(map[string]string)
			}
			wf.CompositeHistory[wf.CurrentState.State.GetId()] = wf.CurrentState.Substate.GetId()
		}
		ctxMap, err = wf.CurrentState.State.ExecuteExitActions(ctx, id, ctxMap)
		if err != nil {
			return ctxMap, true, err
		}
	}

	if local {
		comp := wf.CurrentState.State.(*model.CompositeState)
		sub := comp.FindSubstate(t.Target)
		wf.CurrentState = persistence.StateContainer{State: comp, Substate: sub}
	} else {
		newState, newSub, rerr := resolveTarget(wf.WorkflowDefinition, t, wf.CompositeHistory)
		if rerr != nil {
			return ctxMap, true, rerr
		}
		wf.CurrentState = persistence.StateContainer{State: newState, Substate: newSub}

		if t.Kind == model.ForkKind {
			// Fork fires atomically: stamp one generation id for all regions so
			// a later join (Task 10/11) can tell which batch of children it's
			// waiting on, then spawn them. t.Target is the "fork-and-wait"
			// placeholder state the parent sits in until then.
			if len(t.ForkTargets) == 0 {
				return ctxMap, true, fmt.Errorf("fork transition %s->%s: forkTargets is empty (a fork must spawn at least one region)", t.Source, t.Target)
			}
			forkGen := uuid.New().String()
			defs := make([]model.Workflow, len(t.ForkTargets))
			for i, ft := range t.ForkTargets {
				if ft.ChildWorkflow == nil {
					return ctxMap, true, fmt.Errorf("fork transition %s->%s: forkTargets[%d] missing childWorkflow", t.Source, t.Target, i)
				}
				defs[i] = *ft.ChildWorkflow
			}
			if _, ferr := createChildBatchFn(id, defs, forkGen); ferr != nil {
				return ctxMap, true, fmt.Errorf("fork transition: %w", ferr)
			}
			wf.PendingForkGeneration = forkGen
		} else if t.IsJoin() {
			// Firing a join clears the generation the parent was waiting on —
			// it's no longer pending once the join transition itself applies.
			wf.PendingForkGeneration = ""
		}
	}

	if !local {
		ctxMap, err = wf.CurrentState.State.ExecuteEntryActions(ctx, id, ctxMap)
		if err != nil {
			return ctxMap, true, err
		}
	}
	if wf.CurrentState.Substate != nil {
		ctxMap, err = wf.CurrentState.Substate.ExecuteEntryActions(ctx, id, ctxMap)
		if err != nil {
			return ctxMap, true, err
		}
	}

	ctxMap, err = model.ExecuteActions(ctx, id, ctxMap, t.EntryActions)
	if err != nil {
		return ctxMap, true, err
	}

	wf.LastTransition = fmt.Sprintf("Event: %s, From: %s, To: %s", t.Event, fromId, wf.CurrentState.EffectiveId())

	// The new state's do-activity and timers are armed by the caller via
	// armStateActivities, once this transition is actually committed.
	return ctxMap, true, nil
}

// maxAutomaticHops bounds how many AUTOMATIC transitions can chain in a
// single processEvent call. A workflow author who accidentally creates a
// cycle of completion transitions (A -auto-> B -auto-> A) would otherwise
// hang the instance's actor forever; this turns that authoring bug into
// an error instead.
const maxAutomaticHops = 100

// matchAutomaticIn returns the first zero-delay AUTOMATIC transition out
// of sourceId whose Guard passes.
func matchAutomaticIn(transitions []model.Transition, sourceId string, ctxMap map[string]string) (model.Transition, bool) {
	for _, t := range transitions {
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

// findAutomaticTransition returns the first zero-delay AUTOMATIC
// transition out of wf's current position whose Guard passes, searching
// substate scope first (see findCandidateTransition). After > 0
// transitions are deferred timers (Task 13), not chained synchronously
// here.
func findAutomaticTransition(wf *persistence.WorkflowInstance, ctxMap map[string]string) (model.Transition, bool) {
	for _, sc := range transitionScopes(wf) {
		if t, ok := matchAutomaticIn(sc.transitions, sc.sourceId, ctxMap); ok {
			return t, true
		}
	}
	return model.Transition{}, false
}

// matchDeferredIn returns every guard-passing AUTOMATIC transition out of
// sourceId with a non-empty After.
func matchDeferredIn(transitions []model.Transition, sourceId string, ctxMap map[string]string) []model.Transition {
	var out []model.Transition
	for _, t := range transitions {
		if t.Source != sourceId || t.Trigger != model.AutomaticTrigger || t.After == "" {
			continue
		}
		if !t.Guard.Evaluate(ctxMap) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// findDeferredTransitions returns every guard-passing AUTOMATIC
// transition out of wf's current position with a non-empty After — the
// counterpart to findAutomaticTransition's zero-delay matches. A state
// may have more than one (e.g. a reminder and a hard timeout).
func findDeferredTransitions(wf *persistence.WorkflowInstance, ctxMap map[string]string) []model.Transition {
	var out []model.Transition
	for _, sc := range transitionScopes(wf) {
		out = append(out, matchDeferredIn(sc.transitions, sc.sourceId, ctxMap)...)
	}
	return out
}

// runAutomaticChain repeatedly applies findAutomaticTransition's match
// against wf's new position until none matches, capped by
// maxAutomaticHops. It mutates wf and returns the final ctxMap plus
// whether any hop actually moved the instance — the ctxMap return value
// must be threaded back to the caller explicitly because applyTransition
// may hand back a freshly allocated map when ctxMap starts nil, so relying
// on Go's reference-type map semantics alone would be fragile.
func runAutomaticChain(ctx context.Context, id string, wf *persistence.WorkflowInstance, ctxMap map[string]string) (map[string]string, bool, error) {
	moved := false
	for hop := 0; ; hop++ {
		if hop >= maxAutomaticHops {
			return ctxMap, moved, fmt.Errorf("automatic transition chain from state %q exceeded %d hops (likely a cycle in the workflow definition)", wf.CurrentState.State.GetId(), maxAutomaticHops)
		}
		t, ok := findAutomaticTransition(wf, ctxMap)
		if !ok {
			return ctxMap, moved, nil
		}
		var hopMoved bool
		var err error
		ctxMap, hopMoved, err = applyTransition(ctx, id, wf, t, ctxMap)
		moved = moved || hopMoved
		if err != nil {
			return ctxMap, moved, err
		}
	}
}
