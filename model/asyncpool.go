package model

import (
	"os"
	"runtime"
	"strconv"
	"sync"
)

// asyncPool is a bounded worker pool for fire-and-forget action work (today,
// non-ExpectResponse HTTP actions). It replaces the previous unbounded
// `go func()` per action: goroutines are capped, the queue applies
// backpressure, and DrainAsyncPool lets shutdown wait for in-flight jobs.
//
// It lives in the model package (rather than engine) so executions.go can use
// it without pulling in engine — the same import-cycle constraint that forces
// CreateChildWorkflowFunc to be a hook.
type asyncPool struct {
	jobs chan func()
	wg   sync.WaitGroup
}

var (
	pool     *asyncPool
	poolOnce sync.Once
)

func getPool() *asyncPool {
	poolOnce.Do(func() {
		workers := max(envInt("BROKR_ASYNC_WORKERS", runtime.GOMAXPROCS(0)*2), 1)
		queue := max(envInt("BROKR_ASYNC_QUEUE", 1024), 1)
		pool = &asyncPool{jobs: make(chan func(), queue)}
		for range workers {
			go pool.worker()
		}
	})
	return pool
}

func (p *asyncPool) worker() {
	for job := range p.jobs {
		job()
		p.wg.Done()
	}
}

// submit enqueues job, blocking when the queue is full (backpressure) so a
// burst of async actions can't spawn unbounded goroutines or exhaust memory.
func (p *asyncPool) submit(job func()) {
	p.wg.Add(1)
	p.jobs <- job
}

// submitAsync hands job to the shared bounded pool.
func submitAsync(job func()) {
	getPool().submit(job)
}

// DrainAsyncPool blocks until every submitted async job has finished. Intended
// for graceful shutdown, after the HTTP server has stopped accepting requests.
func DrainAsyncPool() {
	if pool != nil {
		pool.wg.Wait()
	}
}

func envInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultValue
}
