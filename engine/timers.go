package engine

import (
	"context"
	"sync"
	"time"

	"github.com/kashari/brokr/model"
	"github.com/kashari/golog"
)

var (
	timerMu    sync.Mutex
	timerStops = make(map[string][]context.CancelFunc)
)

// dispatchAsyncFn is DispatchAsync, indirected so tests can substitute a
// fake instead of needing a live dispatcher/DB.
var dispatchAsyncFn = DispatchAsync

// stopTimers cancels every pending deferred transition for instanceId.
// Safe to call for an instance with none pending.
func stopTimers(instanceId string) {
	timerMu.Lock()
	cancels := timerStops[instanceId]
	delete(timerStops, instanceId)
	timerMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

// startTimers schedules one goroutine per transition, each firing
// DispatchAsync(instanceId, t.Event) after t.After elapses unless
// stopTimers(instanceId) cancels it first (because a real event moved the
// instance out of the state before the deadline). A transition whose
// After fails to parse as a Go duration is logged and skipped rather than
// blocking the others.
func startTimers(instanceId string, transitions []model.Transition) {
	if len(transitions) == 0 {
		return
	}
	timerMu.Lock()
	defer timerMu.Unlock()
	for _, t := range transitions {
		d, err := time.ParseDuration(t.After)
		if err != nil {
			golog.Error("timer for instance [{}] event [{}]: invalid after duration {}: {}", instanceId, t.Event, t.After, err.Error())
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		timerStops[instanceId] = append(timerStops[instanceId], cancel)
		event := t.Event
		go func() {
			select {
			case <-time.After(d):
				if err := dispatchAsyncFn(instanceId, event); err != nil {
					golog.Error("timer dispatch for instance [{}] event [{}] failed: {}", instanceId, event, err.Error())
				}
			case <-ctx.Done():
			}
		}()
	}
}

// StopAllTimers cancels every pending deferred transition across every
// instance. Called during graceful shutdown alongside Drain/DrainAsyncPool.
func StopAllTimers() {
	timerMu.Lock()
	stops := timerStops
	timerStops = make(map[string][]context.CancelFunc)
	timerMu.Unlock()
	for _, cancels := range stops {
		for _, cancel := range cancels {
			cancel()
		}
	}
}
