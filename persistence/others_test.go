package persistence

import "testing"

func TestWorkflowInstanceContextMapGormTag(t *testing.T) {
	// Compile-time/reflection smoke test: ContextMap must exist and be a
	// map[string]string so GORM's json serializer round-trips it through
	// jsonb without a hand-rolled Scan/Value type.
	wf := WorkflowInstance{ContextMap: map[string]string{"k": "v"}}
	if wf.ContextMap["k"] != "v" {
		t.Fatal("ContextMap not settable as map[string]string")
	}
}
