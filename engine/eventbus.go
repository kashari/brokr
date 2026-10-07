package engine

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/kashari/brokr/dto"
	"github.com/kashari/brokr/persistence"
	"github.com/kashari/draupnir"
	"github.com/kashari/golog"
)

// EventBus fans out workflow instance transition events. Each instance's id
// is used as the topic, so a client can subscribe to just the instance it
// cares about (see web.StreamWorkflowInstanceEvents).
//
// The per-subscriber buffer is enlarged well beyond draupnir's default of 16:
// with many instances transitioning concurrently, a burst of events for one
// instance would otherwise be silently dropped for a momentarily-slow SSE
// client (Broker.Publish drops rather than blocks).
var EventBus = draupnir.NewBroker(draupnir.WithSubscriberBuffer(eventBusBuffer()))

func eventBusBuffer() int {
	if v := os.Getenv("BROKR_SSE_BUFFER"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 256
}

// publishTransition publishes wf's current state as a "transition" event on
// topic id, so any client subscribed to that workflow instance sees it live.
func publishTransition(id, event string, wf persistence.WorkflowInstance) {
	payload, err := json.Marshal(dto.WorkflowTransitionEvent{
		WorkflowInstanceId: id,
		Event:              event,
		LastTransition:     wf.LastTransition,
		CurrentState:       wf.CurrentState.State,
		Substate:           wf.CurrentState.Substate,
	})
	if err != nil {
		golog.Error("failed to marshal transition event for instance [{}]: {}", id, err.Error())
		return
	}
	EventBus.Publish(id, draupnir.Event{Event: "transition", Data: payload})
}
