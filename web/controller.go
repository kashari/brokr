package web

import (
	"net/http"

	"github.com/kashari/brokr/engine"
	"github.com/kashari/brokr/model"
	"github.com/kashari/draupnir"
)

func errResp(msg string) map[string]string {
	return map[string]string{"error": msg}
}

// HTTP

func CreateBlueprint(ctx *draupnir.Context) {
	var bp model.Workflow
	if err := ctx.BindJSON(&bp); err != nil {
		ctx.JSON(http.StatusBadRequest, errResp("invalid JSON: "+err.Error()))
		return
	}
	id, err := engine.NewWorkflowInstance(bp)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	ctx.JSON(http.StatusCreated, map[string]string{"id": id.String()})
}

func GetBlueprint(ctx *draupnir.Context) {
	id := ctx.Param("id")
	wf, err := engine.GetWorkflowInstance(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, errResp(err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, wf)
}

func SendEventToInstance(ctx *draupnir.Context) {
	id := ctx.Param("id")
	event := ctx.Query("event")
	newState, err := engine.SendEventToWorkflowInstance(id, event)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, newState)
}

func GetPossibleEvents(ctx *draupnir.Context) {
	id := ctx.Param("id")
	events, err := engine.GetPossibleEventsForWorkflowInstance(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, events)
}

// StreamWorkflowInstanceEvents streams every transition event for one workflow
// instance as Server-Sent Events until the client disconnects. This mirrors
// draupnir's Router.EVENTSTREAM loop, but the topic is resolved per-request
// from :id instead of being fixed at route-registration time.
func StreamWorkflowInstanceEvents(ctx *draupnir.Context, stream *draupnir.SSEStream) {
	id := ctx.Param("id")
	sub := engine.EventBus.Subscribe(id)
	defer engine.EventBus.Unsubscribe(sub)

	done := stream.Done()
	for {
		select {
		case <-done:
			return
		case ev, ok := <-sub.C:
			if !ok {
				return
			}
			if err := stream.Send(ev); err != nil {
				return
			}
		}
	}
}
