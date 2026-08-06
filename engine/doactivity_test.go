package engine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kashari/brokr/model"
)

func TestStartDoActivityRunsUntilStopped(t *testing.T) {
	var ran int32
	restore := doActivityRunFn
	doActivityRunFn = func(ctx context.Context, id string, actions []model.Action) {
		<-ctx.Done()
		atomic.AddInt32(&ran, 1)
	}
	defer func() { doActivityRunFn = restore }()

	startDoActivity("instance-x", []model.Action{{Type: model.SetContextMapAction}})
	if atomic.LoadInt32(&ran) != 0 {
		t.Fatal("activity should still be running (ctx not cancelled yet)")
	}
	stopDoActivity("instance-x")

	deadline := time.After(time.Second)
	for atomic.LoadInt32(&ran) == 0 {
		select {
		case <-deadline:
			t.Fatal("activity did not observe cancellation in time")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestStartDoActivityNoopOnEmptyActions(t *testing.T) {
	startDoActivity("instance-y", nil)
	stopDoActivity("instance-y") // must not panic on a never-started instance
}
