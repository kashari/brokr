// Package brokr is an embeddable finite state machine with an event bus.
//
// The host application owns the database connection and the HTTP server; brokr
// contributes a draupnir router carrying the state machine's endpoints, a
// boot/shutdown lifecycle for its background work, and schema migration and
// definition seeding.
//
//	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
//	b, err := brokr.New(brokr.Config{DB: db, AutoMigrate: true})
//	if err := b.Boot(ctx); err != nil { ... }
//	defer b.Shutdown(context.Background())
//	http.ListenAndServe(":8080", b.Router())
//
// Engine state (the dispatcher, timers, event bus and worker pool) is
// process-global, so exactly one *Brokr may be booted at a time.
package brokr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/kashari/brokr/engine"
	"github.com/kashari/brokr/internal/store"
	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
	"github.com/kashari/brokr/web"
	"github.com/kashari/draupnir"
	"gorm.io/gorm"
)

// Config configures a Brokr. Only DB is required.
type Config struct {
	// DB is the connection brokr runs every query on. The host opens, tunes
	// and closes it; brokr reads no environment variables and never dials.
	//
	// Size the pool for concurrency: the dispatcher runs one actor goroutine
	// per active workflow instance, so database/sql's default of two idle
	// connections serializes them into a bottleneck.
	DB *gorm.DB

	// RoutePrefix is prepended to every registered path, e.g. "/api/v1".
	// Empty mounts at the root.
	RoutePrefix string

	// AutoMigrate runs Migrate during Boot.
	AutoMigrate bool
}

// Brokr is a configured state machine engine.
type Brokr struct {
	cfg Config
}

// Only one Brokr can be live at a time, because the engine keeps its
// dispatcher, timers, event bus and worker pool in package state. The guard
// turns that constraint into an error instead of one engine silently
// replacing another's database handle.
var (
	bootMu sync.Mutex
	live   *Brokr
)

// New validates cfg and returns an unbooted Brokr. It never panics and never
// reads the environment.
func New(cfg Config) (*Brokr, error) {
	if cfg.DB == nil {
		return nil, errors.New("brokr: Config.DB is required")
	}
	if cfg.RoutePrefix != "" {
		cfg.RoutePrefix = "/" + strings.Trim(cfg.RoutePrefix, "/")
	}
	return &Brokr{cfg: cfg}, nil
}

// Migrate creates or updates the workflow instance and definition tables.
func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("brokr: Migrate needs a database handle")
	}
	return db.AutoMigrate(&persistence.WorkflowInstance{}, &persistence.WorkflowDefinition{})
}

// Boot installs the database handle, optionally migrates, wires the engine's
// hooks and starts its worker pool. Call it before serving.
//
// It starts what can be started ahead of time. Dispatcher actors are spawned
// per workflow instance on first event and evicted when idle, and timers and
// do-activities are spawned per transition; those are inherently lazy and Boot
// does not pre-warm them.
func (b *Brokr) Boot(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	bootMu.Lock()
	defer bootMu.Unlock()
	if live == b {
		return errors.New("brokr: already booted")
	}
	if live != nil {
		return errors.New("brokr: another engine is already booted in this process; engine state is global, so Shutdown it first")
	}

	store.Set(b.cfg.DB)
	if b.cfg.AutoMigrate {
		if err := Migrate(b.cfg.DB); err != nil {
			store.Set(nil)
			return fmt.Errorf("brokr: auto-migrate: %w", err)
		}
	}

	engine.Start()
	model.StartAsyncPool()
	live = b
	return nil
}

// Shutdown stops accepting new background work and waits for what is in
// flight, up to ctx's deadline. Call it after the HTTP server has stopped
// accepting requests, so no new transitions arrive mid-drain.
//
// Timers and do-activities are cancelled before the drain rather than after
// it: a timer that fired mid-drain would enqueue a fresh transition and keep
// the drain going.
func (b *Brokr) Shutdown(ctx context.Context) error {
	bootMu.Lock()
	if live != b {
		bootMu.Unlock()
		return errors.New("brokr: not booted")
	}
	live = nil
	bootMu.Unlock()

	engine.StopAllTimers()
	engine.StopAllDoActivities()

	drained := make(chan struct{})
	go func() {
		defer close(drained)
		engine.Drain()
		model.DrainAsyncPool()
	}()

	select {
	case <-drained:
		store.Set(nil)
		return nil
	case <-ctx.Done():
		// Work is still running against the handle, so it stays installed.
		return fmt.Errorf("brokr: shutdown timed out with work still in flight: %w", ctx.Err())
	}
}

// Router returns a new draupnir router serving only brokr's endpoints. Use
// RegisterRoutes instead to mount them into a router the host already owns.
//
// The returned router has no file logging configured; add it with draupnir's
// WithFileLogging if you want it.
func (b *Brokr) Router() *draupnir.Router {
	r := draupnir.New()
	b.RegisterRoutes(r)
	return r
}

// RegisterRoutes registers brokr's endpoints on r under Config.RoutePrefix.
//
// The paths are concatenated rather than grouped because draupnir's
// RouterGroup has no SSE method, and the event stream must be registered on
// the router itself.
func (b *Brokr) RegisterRoutes(r *draupnir.Router) {
	p := b.cfg.RoutePrefix
	r.POST(p+"/workflows", web.CreateBlueprint)
	r.GET(p+"/workflows/:id", web.GetBlueprint)
	r.POST(p+"/workflows/:id/events", web.SendEventToInstance)
	r.GET(p+"/workflows/:id/possible-events", web.GetPossibleEvents)
	r.GET(p+"/workflows/:id/context", web.GetContext)
	r.GET(p+"/workflows/:id/visualization-data", web.GetVisualizationData)
	r.GET(p+"/workflows/:id/visualizer", web.GetVisualizerPage)
	r.SSE(p+"/workflows/:id/events/stream", web.StreamWorkflowInstanceEvents)
	r.POST(p+"/workflows/:id/children", web.CreateChildBlueprint)
	r.GET(p+"/workflows/:id/children", web.GetChildren)
	r.DELETE(p+"/workflows/:id/children/:childId", web.WithdrawChild)
}
