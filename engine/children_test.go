package engine

import "testing"

func TestCreateChildWorkflowInstancesBatchWithGenerationStampsGeneration(t *testing.T) {
	t.Skip("requires a live DB; exercised by the fixture/integration test in Task 12 and manual verification below")
}

func TestAllChildrenCompleteFiltersByGeneration(t *testing.T) {
	t.Skip("requires a live DB; SQL WHERE-clause shape is verified by reading the query below and via manual verification")
}
