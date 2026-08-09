package dto

import "time"

// InstanceSummary is the wire shape of a workflow instance's own fields —
// everything about "where it is right now," separate from its Graph
// (the definition it's running) and Journey (how it got here).
type InstanceSummary struct {
	Id              string            `json:"id"`
	ParentId        *string           `json:"parentId,omitempty"`
	WorkflowId      string            `json:"workflowId"`
	WorkflowVersion string            `json:"workflowVersion"`
	CurrentState    string            `json:"currentState"`
	CurrentSubstate string            `json:"currentSubstate,omitempty"`
	Complete        bool              `json:"complete"`
	LastTransition  string            `json:"lastTransition"`
	ContextMap      map[string]string `json:"contextMap"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

// VisualizationData is the full payload the visualizer frontend fetches
// for one workflow instance: its own summary, the analyzed topology of
// its definition, its recorded journey so far, the events it could
// accept right now, and any child instances it has spawned.
type VisualizationData struct {
	Instance       InstanceSummary `json:"instance"`
	Graph          Graph           `json:"graph"`
	Journey        []JourneyEntry  `json:"journey"`
	PossibleEvents []string        `json:"possibleEvents"`
	Children       []ChildInstance `json:"children,omitempty"`
}
