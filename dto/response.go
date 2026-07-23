package dto

type EventSentResponse struct {
	WorkflowInstanceId string `json:"workflowInstanceId"`
	CurrentState       string `json:"currentState"`
	LastTransition     string `json:"lastTransition"`
}

type GenericErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
