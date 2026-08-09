package dto

import (
	"time"

	"github.com/kashari/brokr/model"
)

// JourneyEntry is one completed step in a workflow instance's history — its
// initial entry into its first state (Event == "", added by instance
// creation, see Task 2), or a transition that fired afterward (added by
// applyTransition). Appended to persistence.WorkflowInstance.Journey so a
// client can render the instance's whole journey without having listened
// to its SSE stream from the very first event.
type JourneyEntry struct {
	Timestamp    time.Time            `json:"timestamp"`
	Event        string               `json:"event"`
	Trigger      model.TriggerType    `json:"trigger,omitempty"`
	Kind         model.TransitionKind `json:"kind,omitempty"`
	FromState    string               `json:"fromState,omitempty"`
	FromSubstate string               `json:"fromSubstate,omitempty"`
	ToState      string               `json:"toState"`
	ToSubstate   string               `json:"toSubstate,omitempty"`
}
