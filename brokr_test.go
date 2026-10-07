package brokr

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kashari/draupnir"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type unreachableDriver struct{}

func (unreachableDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("brokr tests run without a database")
}

func init() { sql.Register("brokr-unreachable", unreachableDriver{}) }

// stubDB is a real *gorm.DB whose every query fails to connect. A zero-value
// &gorm.DB{} would be simpler but panics on use, and handlers reach the
// database from background goroutines (the dispatcher's per-instance actor)
// where a test's recover cannot catch it. Failing queries keep the suite
// database-free, as it has always been, while letting each handler run its
// real error path.
var stubDB = sync.OnceValue(func() *gorm.DB {
	sqlDB, err := sql.Open("brokr-unreachable", "")
	if err != nil {
		panic(err)
	}
	db, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sqlDB}),
		&gorm.Config{DisableAutomaticPing: true, Logger: logger.Discard},
	)
	if err != nil {
		panic(err)
	}
	return db
})

func newBooted(t *testing.T) *Brokr {
	t.Helper()
	b, err := New(Config{DB: stubDB()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := b.Boot(context.Background()); err != nil {
		t.Fatalf("Boot: %v", err)
	}
	t.Cleanup(func() { _ = b.Shutdown(context.Background()) })
	return b
}

func TestNewRejectsNilDB(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("New with a nil DB: got nil error, want one — the host must own the connection")
	}
}

func TestNewTrimsTrailingSlashFromPrefix(t *testing.T) {
	b, err := New(Config{DB: stubDB(), RoutePrefix: "/api/v1/"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := b.cfg.RoutePrefix; got != "/api/v1" {
		t.Fatalf("RoutePrefix = %q, want %q — otherwise every route gets a double slash", got, "/api/v1")
	}
}

func TestSecondBootIsRejected(t *testing.T) {
	newBooted(t)

	other, err := New(Config{DB: stubDB()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := other.Boot(context.Background()); err == nil {
		t.Fatal("second Boot: got nil error, want one — engine state is process-global, so a second Boot would clobber the first engine's DB handle")
	}
}

func TestShutdownReleasesTheBootGuard(t *testing.T) {
	b, err := New(Config{DB: stubDB()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := b.Boot(context.Background()); err != nil {
		t.Fatalf("Boot: %v", err)
	}
	if err := b.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := b.Boot(context.Background()); err != nil {
		t.Fatalf("Boot after Shutdown: %v, want nil — Shutdown must release the guard", err)
	}
	_ = b.Shutdown(context.Background())
}

func TestShutdownWithoutBootErrors(t *testing.T) {
	b, err := New(Config{DB: stubDB()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := b.Shutdown(context.Background()); err == nil {
		t.Fatal("Shutdown without Boot: got nil error, want one")
	}
}

// routed reports whether the router matched a route for method+path and ran
// its handler.
//
// Status alone cannot answer that. Handlers fail against the unreachable stub
// database, and three of them (GetBlueprint, GetContext, GetVisualizationData)
// report a failed lookup as 404 — the same status draupnir uses for a path it
// has no route for. The two are told apart by how they are written: draupnir
// calls http.NotFound, which is plain text, while every handler answers
// through ctx.JSON.
func routed(h http.Handler, method, path string) bool {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")

	// The SSE handler blocks until the client goes away; a cancelled request
	// context makes stream.Done() fire immediately.
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		return true
	}
	return strings.Contains(rec.Header().Get("Content-Type"), "json")
}

// wantRoutes is the state machine's HTTP surface, relative to RoutePrefix.
var wantRoutes = []struct {
	method string
	path   string
}{
	{http.MethodPost, "/workflows"},
	{http.MethodGet, "/workflows/wf-1"},
	{http.MethodPost, "/workflows/wf-1/events"},
	{http.MethodGet, "/workflows/wf-1/possible-events"},
	{http.MethodGet, "/workflows/wf-1/context"},
	{http.MethodGet, "/workflows/wf-1/visualization-data"},
	{http.MethodGet, "/workflows/wf-1/visualizer"},
	{http.MethodGet, "/workflows/wf-1/events/stream"},
	{http.MethodPost, "/workflows/wf-1/children"},
	{http.MethodGet, "/workflows/wf-1/children"},
	{http.MethodDelete, "/workflows/wf-1/children/child-1"},
}

func TestRouterRegistersEveryRoute(t *testing.T) {
	b := newBooted(t)
	r := b.Router()

	for _, rt := range wantRoutes {
		if !routed(r, rt.method, rt.path) {
			t.Errorf("%s %s: not registered", rt.method, rt.path)
		}
	}
}

func TestRouterDoesNotRegisterUnknownPaths(t *testing.T) {
	b := newBooted(t)
	r := b.Router()

	if routed(r, http.MethodGet, "/not-a-brokr-route") {
		t.Fatal("GET /not-a-brokr-route: reported as registered — routed() cannot tell registered from unregistered, so every other routing test here is meaningless")
	}
}

func TestRoutePrefixIsAppliedToEveryRoute(t *testing.T) {
	b, err := New(Config{DB: stubDB(), RoutePrefix: "/api/v1"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := b.Boot(context.Background()); err != nil {
		t.Fatalf("Boot: %v", err)
	}
	defer b.Shutdown(context.Background())

	r := b.Router()
	for _, rt := range wantRoutes {
		if !routed(r, rt.method, "/api/v1"+rt.path) {
			t.Errorf("%s /api/v1%s: not registered — prefix not applied to this route", rt.method, rt.path)
		}
		if routed(r, rt.method, rt.path) {
			t.Errorf("%s %s: served — unprefixed paths must not be served when RoutePrefix is set", rt.method, rt.path)
		}
	}
}

func TestRegisterRoutesMountsIntoAHostRouter(t *testing.T) {
	b := newBooted(t)

	host := draupnir.New()
	host.GET("/healthz", func(ctx *draupnir.Context) {
		ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	b.RegisterRoutes(host)

	if !routed(host, http.MethodGet, "/healthz") {
		t.Error("GET /healthz: not served — mounting brokr must leave the host's own routes alone")
	}
	for _, rt := range wantRoutes {
		if !routed(host, rt.method, rt.path) {
			t.Errorf("%s %s: not registered — RegisterRoutes did not register into the host router", rt.method, rt.path)
		}
	}
}

func TestMigrateRejectsNilDB(t *testing.T) {
	if err := Migrate(nil); err == nil {
		t.Fatal("Migrate(nil): got nil error, want one")
	}
}
