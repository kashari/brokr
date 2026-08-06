package engine

import (
	"context"
	"sync"

	"github.com/kashari/brokr/model"
	"github.com/kashari/golog"
)

var (
	doActivityMu    sync.Mutex
	doActivityStops = make(map[string]context.CancelFunc)
)

// doActivityRunFn executes a do-activity's actions. Indirected so tests
// can substitute a fake without waiting on real HTTP/DB actions.
var doActivityRunFn = func(ctx context.Context, id string, actions []model.Action) {
	if _, err := model.ExecuteActions(ctx, id, map[string]string{}, actions); err != nil && ctx.Err() == nil {
		golog.Error("do-activity for instance [{}] failed: {}", id, err.Error())
	}
}

// stopDoActivity cancels instanceId's currently-running do-activity, if
// any. Safe to call for an instance with none running (a no-op).
func stopDoActivity(instanceId string) {
	doActivityMu.Lock()
	cancel := doActivityStops[instanceId]
	delete(doActivityStops, instanceId)
	doActivityMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// startDoActivity launches actions as instanceId's do-activity, running
// until stopDoActivity(instanceId) is called (on leaving the state, actor
// eviction, or shutdown) or the process exits. A no-op when actions is
// empty. Callers must call stopDoActivity for the *previous* state before
// calling startDoActivity for the new one (applyTransition does this).
func startDoActivity(instanceId string, actions []model.Action) {
	if len(actions) == 0 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	doActivityMu.Lock()
	doActivityStops[instanceId] = cancel
	doActivityMu.Unlock()
	go doActivityRunFn(ctx, instanceId, actions)
}

// StopAllDoActivities cancels every currently-running do-activity. Called
// during graceful shutdown alongside Drain/DrainAsyncPool.
func StopAllDoActivities() {
	doActivityMu.Lock()
	stops := doActivityStops
	doActivityStops = make(map[string]context.CancelFunc)
	doActivityMu.Unlock()
	for _, cancel := range stops {
		cancel()
	}
}
