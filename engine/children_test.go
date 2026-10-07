package engine

import "testing"

func TestCreateChildWorkflowInstancesBatchWithGenerationStampsGeneration(t *testing.T) {
	// Needs a live DB; there is no automated coverage of the actual insert.
	// The fork side that decides *what* generation to stamp is covered by
	// TestApplyTransitionForkStampsGenerationAndSetsPending (with a fake
	// createChildBatchFn); the persistence of that stamp is manual only.
	t.Skip("no automated coverage: requires a live DB, verified manually")
}

func TestAllChildrenCompleteFiltersByGeneration(t *testing.T) {
	// Needs a live DB. The SQL WHERE-clause shape has only been reviewed by
	// reading allChildrenComplete and verified manually against Postgres.
	t.Skip("no automated coverage: requires a live DB, verified manually")
}
