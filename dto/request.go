package dto

// CreateInstanceRequest is the POST /workflows request body: the name of
// a WorkflowDefinition already registered via cmd/seed-workflows. The
// full definition is no longer accepted inline — see README "Registering
// workflow definitions".
type CreateInstanceRequest struct {
	Name string `json:"name"`
}
