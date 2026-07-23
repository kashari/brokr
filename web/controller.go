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
