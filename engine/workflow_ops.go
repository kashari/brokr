package engine

import (
	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
)

func NewWorkflowInstance(workflowDefinition model.Workflow) *persistence.WorkflowInstance {
	return &persistence.WorkflowInstance{
		WorkflowDefinition: workflowDefinition,
		CurrentState:       workflowDefinition.States[0],
		LastTransition:     "",
	}
}
