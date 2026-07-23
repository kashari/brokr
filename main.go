package main

import (
	"net/http"
	"os"
	"time"

	"github.com/kashari/brokr/config"
	"github.com/kashari/brokr/web"
	"github.com/kashari/draupnir"
	"github.com/kashari/golog"
)

func main() {
	router := draupnir.New().WithFileLogging("brokr.log")

	config.InitDB()

	router.POST("/workflows", web.CreateBlueprint)
	router.GET("/workflows/:id", web.GetBlueprint)
	router.POST("/workflows/:id/events", web.SendEventToInstance)
	router.GET("/workflows/:id/possible-events", web.GetPossibleEvents)
	router.SSE("/workflows/:id/events/stream", web.StreamWorkflowInstanceEvents)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Not using router.Start(): it hardcodes a 10s WriteTimeout on the
	// underlying http.Server, which would kill any SSE connection open
	// longer than that.
	server := &http.Server{
		Addr:        ":" + port,
		Handler:     router,
		ReadTimeout: 10 * time.Second,
		IdleTimeout: 90 * time.Second,
	}

	golog.Info("Starting server on port {}", port)
	if err := server.ListenAndServe(); err != nil {
		golog.Error("server error: {}", err.Error())
	}
}
