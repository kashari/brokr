package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kashari/brokr/config"
	"github.com/kashari/brokr/engine"
	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
	"github.com/kashari/brokr/web"
	"github.com/kashari/draupnir"
	"github.com/kashari/golog"
)

func main() {
	router := draupnir.New().WithFileLogging("brokr.log")

	config.InitDB()
	if err := config.Db.AutoMigrate(&persistence.WorkflowInstance{}); err != nil {
		golog.Error("Failed to auto-migrate schema: {}", err.Error())
		panic("failed to auto-migrate schema: " + err.Error())
	}

	router.POST("/workflows", web.CreateBlueprint)
	router.GET("/workflows/:id", web.GetBlueprint)
	router.POST("/workflows/:id/events", web.SendEventToInstance)
	router.GET("/workflows/:id/possible-events", web.GetPossibleEvents)
	router.SSE("/workflows/:id/events/stream", web.StreamWorkflowInstanceEvents)
	router.POST("/workflows/:id/children", web.CreateChildBlueprint)
	router.GET("/workflows/:id/children", web.GetChildren)
	router.DELETE("/workflows/:id/children/:childId", web.WithdrawChild)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Not using router.Start(): it hardcodes a 10s WriteTimeout on the
	// underlying http.Server, which would kill any SSE connection open
	// longer than that.
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	// Run the server in the background so main can block on a shutdown signal.
	serverErr := make(chan error, 1)
	go func() {
		golog.Info("Starting server on port {}", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		golog.Error("server error: {}", err.Error())
	case sig := <-stop:
		golog.Info("Received signal {}, shutting down gracefully", sig.String())
	}

	// Graceful shutdown: stop accepting new requests, then drain in-flight
	// transitions (dispatcher) and fire-and-forget action jobs (async pool)
	// before exiting, so max in-flight work isn't lost.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		golog.Error("graceful shutdown error: {}", err.Error())
	}
	engine.Drain()
	model.DrainAsyncPool()
	engine.StopAllDoActivities()
	engine.StopAllTimers()
	golog.Info("Shutdown complete")
}
