package dto

import "github.com/kashari/brokr/model"

// WorkflowTransitionEvent is the payload published to a workflow instance's
// event stream after each completed transition.
type WorkflowTransitionEvent struct {
	WorkflowInstanceId string      `json:"workflowInstanceId"`
	Event              string      `json:"event"`
	LastTransition     string      `json:"lastTransition"`
	CurrentState       model.State `json:"currentState"`
}

// ChildInstance describes one child workflow instance in a parent's children
// listing.
type ChildInstance struct {
	Id           string      `json:"id"`
	CurrentState model.State `json:"currentState"`
	Complete     bool        `json:"complete"`
}

type EventSentResponse struct {
	WorkflowInstanceId string `json:"workflowInstanceId"`
	CurrentState       string `json:"currentState"`
	LastTransition     string `json:"lastTransition"`
}

type GenericErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type NoTransitionErrorResponse struct {
	Code         int    `json:"code"`
	Message      string `json:"message"`
	CurrentState string `json:"currentState"`
	Event        string `json:"event"`
}
