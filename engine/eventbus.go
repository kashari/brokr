package engine

import (
	"encoding/json"

	"github.com/kashari/brokr/dto"
	"github.com/kashari/brokr/persistence"
	"github.com/kashari/draupnir"
	"github.com/kashari/golog"
)

// EventBus fans out workflow instance transition events. Each instance's id
// is used as the topic, so a client can subscribe to just the instance it
// cares about (see web.StreamWorkflowInstanceEvents).
var EventBus = draupnir.NewBroker()

// publishTransition publishes wf's current state as a "transition" event on
// topic id, so any client subscribed to that workflow instance sees it live.
func publishTransition(id, event string, wf persistence.WorkflowInstance) {
	payload, err := json.Marshal(dto.WorkflowTransitionEvent{
		WorkflowInstanceId: id,
		Event:              event,
		LastTransition:     wf.LastTransition,
		CurrentState:       wf.CurrentState.State,
	})
	if err != nil {
		golog.Error("failed to marshal transition event for instance [{}]: {}", id, err.Error())
		return
	}
	EventBus.Publish(id, draupnir.Event{Event: "transition", Data: payload})
}
