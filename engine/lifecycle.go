package engine

// Start wires the engine's cross-package hooks. It is called by brokr.Boot,
// not from init, so that importing the engine has no side effects — a library
// that mutates another package's globals at import time is a library you
// cannot reason about.
//
// It is safe to call more than once: every step is an idempotent assignment.
//
// The engine's goroutines are not started here because they cannot be. The
// dispatcher spawns one actor per workflow instance on that instance's first
// event and reaps it when idle, and timers and do-activities are spawned by
// the transitions that arm them.
func Start() {
	registerChildHooks()
}
