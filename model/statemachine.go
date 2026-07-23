package model

import (
	"encoding/json"
	"fmt"

	"github.com/kashari/golog"
)

type Workflow struct {
	Id                string             `json:"id"`
	Version           string             `json:"version"`
	Deprecated        bool               `json:"deprecated"`
	CreationDate      string             `json:"creationDate"`
	UpdateDate        string             `json:"updateDate"`
	InitialState      string             `json:"initialState"`
	States            []State            `json:"states"`
	Transitions       []Transition       `json:"transitions"`
	CommonTransitions []CommonTransition `json:"commonTransitions"`
	EndStates         []string           `json:"endStates"`
}

// SimpleStateType and ActionStateType are the "type" discriminator values
// expected on each element of Workflow.States.
const (
	SimpleStateType = "SimpleState"
	ActionStateType = "ActionState"
)

// DecodeState unmarshals a single JSON state object into its concrete State
// implementation, dispatching on its "type" field. Exported so persistence
// can reuse it when scanning a State back out of the database.
func DecodeState(data []byte) (State, error) {
	var peek struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, fmt.Errorf("decode state: %w", err)
	}
	switch peek.Type {
	case ActionStateType:
		var s ActionState
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("decode ActionState: %w", err)
		}
		return &s, nil
	case SimpleStateType, "":
		var s SimpleState
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("decode SimpleState: %w", err)
		}
		return &s, nil
	default:
		return nil, fmt.Errorf("unknown state type %q", peek.Type)
	}
}

// UnmarshalJSON decodes a Workflow, resolving each element of States to its
// concrete SimpleState/ActionState implementation via DecodeState. Without
// this, encoding/json has no way to unmarshal into the polymorphic State
// interface.
func (w *Workflow) UnmarshalJSON(data []byte) error {
	type alias Workflow
	aux := &struct {
		States []json.RawMessage `json:"states"`
		*alias
	}{alias: (*alias)(w)}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	w.States = make([]State, 0, len(aux.States))
	for _, raw := range aux.States {
		s, err := DecodeState(raw)
		if err != nil {
			return err
		}
		w.States = append(w.States, s)
	}
	return nil
}

type State interface {
	IsResumable() bool
	GetType() string
	GetId() string
	GetFrontendBullet() bool
	GetBulletName() string
	GetResumeEvent() string
	GetProductStatus() string
	GetStatus() string
	ExecuteEntryActions(token string, ctxMap map[string]string) (map[string]string, error)
	ExecuteExitActions(token string, ctxMap map[string]string) (map[string]string, error)
}

type Transition struct {
	Type         string   `json:"type"`
	Source       string   `json:"source"`
	Target       string   `json:"target"`
	Event        string   `json:"event"`
	EntryActions []Action `json:"entryActions"`
	ExitActions  []Action `json:"exitActions"`
	// Join, if true, means this transition may only fire once every one of
	// the instance's (non-withdrawn) children has reached one of its own
	// workflow's EndStates.
	Join bool `json:"join"`
}

type CommonTransition struct {
	SourceList   []string `json:"sourceList"`
	Target       string   `json:"target"`
	Event        string   `json:"event"`
	EntryActions []Action `json:"entryActions"`
	ExitActions  []Action `json:"exitActions"`
}

type ActionType string

const (
	HttpRequestAction         ActionType = "HttpRequestAction"
	SetContextMapAction       ActionType = "SetContextMapAction"
	CreateChildWorkflowAction ActionType = "CreateChildWorkflowAction"
)

type Action struct {
	Type           ActionType        `json:"type"`
	Method         string            `json:"method"`
	Url            string            `json:"url"`
	ExpectResponse bool              `json:"expectResponse"`
	ForwardToken   bool              `json:"forwardToken"`
	Status         string            `json:"status"`
	Variables      map[string]string `json:"variables"`
	// ChildWorkflow is the inline definition to instantiate as a child
	// instance when Type is CreateChildWorkflowAction.
	ChildWorkflow *Workflow `json:"childWorkflow,omitempty"`
}

// CreateChildWorkflowFunc creates a child workflow instance under parentId
// and returns the new child's id. It is nil until engine wires it up (see
// engine's init), which breaks the import cycle model would otherwise have
// with engine/persistence.
var CreateChildWorkflowFunc func(parentId string, childDefinition Workflow) (string, error)

func (a *Action) Execute(auth string, ctxMap map[string]string) (map[string]string, error) {
	switch a.Type {
	case HttpRequestAction:
		if !a.ExpectResponse {
			// Nothing will be merged back into ctxMap for this action, so it's
			// safe to fire it off without blocking the transition on its result.
			snapshot := copyContext(ctxMap)
			go func() {
				if _, err := executeHttpRequestAction(a, snapshot, auth); err != nil {
					golog.Error("async HTTP action [{}] {} failed: {}", a.Method, a.Url, err.Error())
				}
			}()
			return ctxMap, nil
		}
		return executeHttpRequestAction(a, ctxMap, auth)
	case SetContextMapAction:
		return executeSetContextMapAction(a, ctxMap)
	case CreateChildWorkflowAction:
		return executeCreateChildAction(a, ctxMap, auth)
	default:
		return ctxMap, nil
	}
}
