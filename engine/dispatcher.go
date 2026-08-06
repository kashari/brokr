package engine

import (
	"context"
	"hash/fnv"
	"sync"
	"time"
)

// The dispatcher is an actor-per-instance scheduler. Events for different
// instances run on independent goroutines (unbounded cross-instance
// parallelism), while all events for a single instance funnel through that
// instance's mailbox channel and are processed strictly one at a time. That
// per-instance serialization is what makes processEvent's read-modify-write
// safe under concurrent load — the guarantee the old inline handler lacked.
//
// Actors are created lazily on first event and evicted after an idle period,
// so idle instances cost nothing.

const (
	numShards        = 256
	mailboxBuffer    = 64
	actorIdleTimeout = 30 * time.Second
)

type result struct {
	newState string
	err      error
}

type command struct {
	ctx   context.Context
	event string
	reply chan result // nil for fire-and-forget (async) dispatch
}

type instanceActor struct {
	id      string
	mailbox chan command
	// pending counts commands enqueued but not yet processed. It is guarded by
	// the owning shard's mutex and lets an idle actor evict itself only when no
	// sender is mid-flight, closing the enqueue/evict race.
	pending int
}

type shard struct {
	mu     sync.Mutex
	actors map[string]*instanceActor
}

type dispatcher struct {
	shards [numShards]*shard
}

// processFn is the per-instance step the actor runs. It's a variable (rather
// than a direct call to processEvent) so tests can substitute a fake and assert
// the dispatcher's serialization/parallelism guarantees without a live DB.
//
// Assigned in init() rather than at the declaration site: processEvent now
// (transitively, via attemptAutoJoin -> DispatchAsync -> enqueue -> run)
// reads processFn itself, so `var processFn = processEvent` would make the
// compiler's static initializer-dependency analysis see processFn depending
// on itself (a real "initialization cycle" build error, not a false
// positive — Go's analysis follows function bodies, not just call timing).
// Moving the assignment into init() breaks that edge: init() runs after all
// package-level variables are initialized, so it's just a normal statement.
var processFn func(ctx context.Context, id string, event string) (string, error)

func init() {
	processFn = processEvent
}

// inflight tracks commands accepted but not yet fully processed, so graceful
// shutdown can wait for in-flight transitions to drain.
var inflight sync.WaitGroup

var defaultDispatcher = func() *dispatcher {
	d := &dispatcher{}
	for i := range d.shards {
		d.shards[i] = &shard{actors: make(map[string]*instanceActor)}
	}
	return d
}()

func (d *dispatcher) shardFor(id string) *shard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return d.shards[h.Sum32()%numShards]
}

// enqueue appends a command to id's mailbox, spawning the actor if needed. The
// blocking send happens outside the shard lock (so a full mailbox applies
// backpressure without stalling the whole shard) but is guarded against actor
// eviction by the pending counter incremented under the lock.
func (d *dispatcher) enqueue(id string, cmd command) error {
	sh := d.shardFor(id)
	sh.mu.Lock()
	a := sh.actors[id]
	if a == nil {
		a = &instanceActor{id: id, mailbox: make(chan command, mailboxBuffer)}
		sh.actors[id] = a
		go a.run(sh)
	}
	a.pending++
	inflight.Add(1)
	sh.mu.Unlock()

	select {
	case a.mailbox <- cmd:
		return nil
	case <-cmd.ctx.Done():
		sh.mu.Lock()
		a.pending--
		sh.mu.Unlock()
		inflight.Done()
		return cmd.ctx.Err()
	}
}

func (a *instanceActor) run(sh *shard) {
	timer := time.NewTimer(actorIdleTimeout)
	defer timer.Stop()
	for {
		select {
		case cmd := <-a.mailbox:
			if !timer.Stop() {
				<-timer.C
			}
			newState, err := processFn(cmd.ctx, a.id, cmd.event)
			if cmd.reply != nil {
				cmd.reply <- result{newState: newState, err: err}
			}
			sh.mu.Lock()
			a.pending--
			sh.mu.Unlock()
			inflight.Done()
			timer.Reset(actorIdleTimeout)

		case <-timer.C:
			sh.mu.Lock()
			if a.pending == 0 {
				delete(sh.actors, a.id)
				sh.mu.Unlock()
				return
			}
			// A sender is mid-flight; keep serving.
			sh.mu.Unlock()
			timer.Reset(actorIdleTimeout)
		}
	}
}

// Dispatch enqueues event for instance id and blocks until its transition
// completes, returning the resulting state (or ctx.Err() if the caller's
// context is cancelled first). This is the synchronous path used by the
// default send-event API.
func Dispatch(ctx context.Context, id string, event string) (string, error) {
	reply := make(chan result, 1) // buffered so the actor never blocks on a bailed caller
	if err := defaultDispatcher.enqueue(id, command{ctx: ctx, event: event, reply: reply}); err != nil {
		return "", err
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-reply:
		return r.newState, r.err
	}
}

// DispatchAsync enqueues event for instance id and returns immediately; the
// resulting transition is observable over the instance's SSE stream. The
// command is detached from any request context so it isn't cancelled when the
// HTTP handler returns.
func DispatchAsync(id string, event string) error {
	return defaultDispatcher.enqueue(id, command{ctx: context.Background(), event: event, reply: nil})
}

// Drain blocks until every accepted command has finished processing. Call it
// during graceful shutdown after the HTTP server has stopped accepting new
// requests.
func Drain() {
	inflight.Wait()
}
