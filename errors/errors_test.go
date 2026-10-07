package errors

import "testing"

func TestWorkflowDefinitionNotFoundErrorMessage(t *testing.T) {
	err := &WorkflowDefinitionNotFoundError{Name: "order-lifecycle"}
	want := "no workflow definition registered under name 'order-lifecycle'"
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}
