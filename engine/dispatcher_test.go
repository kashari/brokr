package engine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestDispatchSerializesPerInstance is the key concurrency guarantee: many
// events fired concurrently at ONE instance must be processed one at a time
// (no overlap), which is what makes processEvent's read-modify-write safe.
// Run under `go test -race` to also catch data races.
func TestDispatchSerializesPerInstance(t *testing.T) {
	restore := processFn
	defer func() { processFn = restore }()

	var concurrent int32 // how many step invocations overlap right now
	var maxConcurrent int32
	var processed int32

	processFn = func(ctx context.Context, id string, event string) (string, error) {
		n := atomic.AddInt32(&concurrent, 1)
		for {
			m := atomic.LoadInt32(&maxConcurrent)
			if n <= m || atomic.CompareAndSwapInt32(&maxConcurrent, m, n) {
				break
			}
		}
		time.Sleep(time.Millisecond) // widen the overlap window if any
		atomic.AddInt32(&processed, 1)
		atomic.AddInt32(&concurrent, -1)
		return event, nil
	}

	const events = 50
	var wg sync.WaitGroup
	for i := 0; i < events; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := Dispatch(context.Background(), "same-instance", fmt.Sprintf("e%d", i)); err != nil {
				t.Errorf("dispatch: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&processed); got != events {
		t.Fatalf("processed %d events, want %d", got, events)
	}
	if got := atomic.LoadInt32(&maxConcurrent); got != 1 {
		t.Fatalf("max concurrent steps for one instance = %d, want 1 (events must serialize)", got)
	}
}

// TestDispatchParallelAcrossInstances confirms different instances are NOT
// serialized against each other — they run on independent actor goroutines.
func TestDispatchParallelAcrossInstances(t *testing.T) {
	restore := processFn
	defer func() { processFn = restore }()

	const instances = 16
	var running int32
	var maxRunning int32
	gate := make(chan struct{})

	processFn = func(ctx context.Context, id string, event string) (string, error) {
		n := atomic.AddInt32(&running, 1)
		for {
			m := atomic.LoadInt32(&maxRunning)
			if n <= m || atomic.CompareAndSwapInt32(&maxRunning, m, n) {
				break
			}
		}
		<-gate // hold until every instance's actor is in-flight
		atomic.AddInt32(&running, -1)
		return event, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < instances; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = Dispatch(context.Background(), fmt.Sprintf("instance-%d", i), "e")
		}(i)
	}

	// Wait until all instances are simultaneously in their step, then release.
	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(&running) < instances {
		select {
		case <-deadline:
			close(gate)
			wg.Wait()
			t.Fatalf("only %d/%d instances ran concurrently", atomic.LoadInt32(&running), instances)
		default:
			time.Sleep(time.Millisecond)
		}
	}
	close(gate)
	wg.Wait()

	if got := atomic.LoadInt32(&maxRunning); got != instances {
		t.Fatalf("max concurrent instances = %d, want %d", got, instances)
	}
}

// TestDispatchContextCancel ensures a cancelled caller context unblocks a
// synchronous Dispatch instead of hanging.
func TestDispatchContextCancel(t *testing.T) {
	restore := processFn
	defer func() { processFn = restore }()

	release := make(chan struct{})
	processFn = func(ctx context.Context, id string, event string) (string, error) {
		<-release
		return event, nil
	}
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Dispatch(ctx, "cancel-instance", "e"); err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}
