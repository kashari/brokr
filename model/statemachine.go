package model

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
	HttpRequestAction   ActionType = "HttpRequestAction"
	SetContextMapAction ActionType = "SetContextMapAction"
)

type Action struct {
	Type           ActionType        `json:"type"`
	Method         string            `json:"method"`
	Url            string            `json:"url"`
	ExpectResponse bool              `json:"expectResponse"`
	ForwardToken   bool              `json:"forwardToken"`
	Status         string            `json:"status"`
	Variables      map[string]string `json:"variables"`
}

func (a *Action) Execute(auth string, ctxMap map[string]string) (map[string]string, error) {
	switch a.Type {
	case HttpRequestAction:
		return executeHttpRequestAction(a, ctxMap, auth)
	case SetContextMapAction:
		return executeSetContextMapAction(a, ctxMap)
	default:
		return ctxMap, nil
	}
}
