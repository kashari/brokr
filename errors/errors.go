package errors

type NoTransitionError struct {
	CurrentState string
	Event        string
}

func (e *NoTransitionError) Error() string {
	return "no transition found for event '" + e.Event + "' in state '" + e.CurrentState + "'"
}
