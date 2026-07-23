package errors

type NoTransitionError struct {
	CurrentState string
	Event        string
}

func (e *NoTransitionError) Error() string {
	return "no transition found for event '" + e.Event + "' in state '" + e.CurrentState + "'"
}

type ChildrenNotCompleteError struct {
	CurrentState string
	Event        string
}

func (e *ChildrenNotCompleteError) Error() string {
	return "cannot fire event '" + e.Event + "' from state '" + e.CurrentState + "': one or more child workflow instances have not completed"
}
