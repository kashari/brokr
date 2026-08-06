package engine

import (
	"testing"
	"time"

	"github.com/kashari/brokr/model"
)

func TestFindDeferredTransitions(t *testing.T) {
	def := model.Workflow{
		Transitions: []model.Transition{
			{Source: "a", Event: "reminder", Target: "b", Trigger: model.AutomaticTrigger, After: "1h"},
			{Source: "a", Event: "hard_timeout", Target: "c", Trigger: model.AutomaticTrigger, After: "24h"},
			{Source: "a", Event: "immediate", Target: "d", Trigger: model.AutomaticTrigger},
			{Source: "a", Event: "manual", Target: "e"},
		},
	}
	got := findDeferredTransitions(def, "a", nil)
	if len(got) != 2 {
		t.Fatalf("got %d deferred transitions, want 2 (immediate/manual must be excluded)", len(got))
	}
}

func TestTimerFiresDispatchAsyncAfterDelay(t *testing.T) {
	restore := dispatchAsyncFn
	fired := make(chan string, 1)
	dispatchAsyncFn = func(id, event string) error {
		fired <- event
		return nil
	}
	defer func() { dispatchAsyncFn = restore }()

	startTimers("instance-z", []model.Transition{
		{Event: "fire_me", After: "10ms"},
	})
	select {
	case ev := <-fired:
		if ev != "fire_me" {
			t.Fatalf("got event %q, want fire_me", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timer did not fire in time")
	}
	stopTimers("instance-z")
}

func TestStopTimersCancelsBeforeFiring(t *testing.T) {
	restore := dispatchAsyncFn
	fired := make(chan string, 1)
	dispatchAsyncFn = func(id, event string) error {
		fired <- event
		return nil
	}
	defer func() { dispatchAsyncFn = restore }()

	startTimers("instance-w", []model.Transition{{Event: "should_not_fire", After: "50ms"}})
	stopTimers("instance-w")

	select {
	case <-fired:
		t.Fatal("timer fired after being stopped")
	case <-time.After(150 * time.Millisecond):
		// expected: nothing fired
	}
}
