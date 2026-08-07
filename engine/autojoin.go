package engine

import (
	"context"

	"github.com/kashari/brokr/config"
	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
	"github.com/kashari/golog"
)

// findPendingJoinTransition returns the first Kind == JoinKind (or legacy
// Join: true) transition out of wf's current position, if any. It scopes
// the search exactly the way findCandidateTransition does (see
// transitionScopes): a composite's SubTransitions are searched against the
// active substate before the workflow's top-level Transitions. Without
// that scoping a join authored inside a composite would fire when a client
// sent the event by hand but never automatically — "joins work in testing,
// hang in production".
func findPendingJoinTransition(wf *persistence.WorkflowInstance) (model.Transition, bool) {
	for _, sc := range transitionScopes(wf) {
		for _, t := range sc.transitions {
			if t.Source == sc.sourceId && t.IsJoin() {
				return t, true
			}
		}
	}
	return model.Transition{}, false
}

// attemptAutoJoin is called (in its own goroutine) whenever a child
// instance completes. It checks whether parentId's current state has a
// pending join transition that's now satisfiable, and if so fires it
// through the normal dispatcher path — the join's Guard, exit/entry
// actions, and persistence all run through processEvent exactly as if a
// client had sent the event, just triggered by a sibling completing
// instead of an HTTP request.
// The caller must inflight.Add(1) before spawning it; the matching Done is
// deferred here so the goroutine stays counted right through its own
// DispatchAsync (which does its own Add), closing the shutdown race where
// Drain() could return between the two.
func attemptAutoJoin(parentId string) {
	defer inflight.Done()

	ctx := context.Background()
	db := config.Db.WithContext(ctx)

	var parent persistence.WorkflowInstance
	if result := db.First(&parent, "id = ?", parentId); result.Error != nil {
		// Parent may have been deleted/withdrawn concurrently; nothing to do.
		return
	}

	t, ok := findPendingJoinTransition(&parent)
	if !ok {
		return
	}
	if !t.Guard.Evaluate(parent.ContextMap) {
		return
	}

	complete, err := allChildrenComplete(ctx, parentId, parent.PendingForkGeneration)
	if err != nil {
		golog.Error("auto-join: checking children of [{}] failed: {}", parentId, err.Error())
		return
	}
	if !complete {
		return
	}

	if err := DispatchAsync(parentId, t.Event); err != nil {
		golog.Error("auto-join: dispatching [{}] to [{}] failed: {}", t.Event, parentId, err.Error())
	}
}
