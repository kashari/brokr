package persistence

import (
	"testing"

	"github.com/kashari/brokr/model"
)

func TestWorkflowDefinitionFieldsAreSettable(t *testing.T) {
	def := WorkflowDefinition{
		Name:       "order-lifecycle",
		Definition: model.Workflow{Id: "order-lifecycle", Kind: model.UserWorkflowKind},
	}
	if def.Name != "order-lifecycle" {
		t.Fatal("Name not settable as string")
	}
	if def.Definition.Id != "order-lifecycle" {
		t.Fatal("Definition not settable as model.Workflow")
	}
}
