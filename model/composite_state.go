package model

import (
	"context"
	"encoding/json"
	"fmt"
)

// HistoryKind is a composite state's history pseudostate, if any. Deep and
// shallow behave identically at this model's single level of nesting
// (there's nothing deeper to remember); the distinction is kept so a
// future recursive-depth extension has a field to act on without another
// schema change.
type HistoryKind string

const (
	NoHistory      HistoryKind = ""
	ShallowHistory HistoryKind = "shallow"
	DeepHistory    HistoryKind = "deep"
)

// CompositeState is a UML composite state containing exactly one level of
// substates. While an instance is "inside" a CompositeState, events are
// offered to SubTransitions first (matched against the active substate);
// only if none match does the event bubble out to the workflow's
// top-level Transitions/CommonTransitions, exiting the composite entirely
// (see engine's findCandidateTransition, Task 16).
type CompositeState struct {
	Type            string       `json:"type"`
	Id              string       `json:"id"`
	FrontendBullet  bool         `json:"frontendBullet"`
	BulletName      string       `json:"bulletName"`
	ResumeEvent     string       `json:"resumeEvent"`
	ProductStatus   string       `json:"productStatus"`
	Status          string       `json:"status"`
	EntryActions    []Action     `json:"entryActions"`
	ExitActions     []Action     `json:"exitActions"`
	InitialSubstate string       `json:"initialSubstate"`
	Substates       []State      `json:"substates"`
	SubTransitions  []Transition `json:"subTransitions"`
	History         HistoryKind  `json:"history,omitempty"`
}

func (s *CompositeState) IsResumable() bool        { return s.ResumeEvent != "" }
func (s *CompositeState) GetType() string          { return s.Type }
func (s *CompositeState) GetId() string            { return s.Id }
func (s *CompositeState) GetFrontendBullet() bool  { return s.FrontendBullet }
func (s *CompositeState) GetBulletName() string    { return s.BulletName }
func (s *CompositeState) GetResumeEvent() string   { return s.ResumeEvent }
func (s *CompositeState) GetProductStatus() string { return s.ProductStatus }
func (s *CompositeState) GetStatus() string        { return s.Status }
func (s *CompositeState) GetDoActions() []Action   { return nil }

func (s *CompositeState) ExecuteEntryActions(ctx context.Context, token string, ctxMap map[string]string) (map[string]string, error) {
	return ExecuteActions(ctx, token, ctxMap, s.EntryActions)
}

func (s *CompositeState) ExecuteExitActions(ctx context.Context, token string, ctxMap map[string]string) (map[string]string, error) {
	return ExecuteActions(ctx, token, ctxMap, s.ExitActions)
}

// UnmarshalJSON resolves Substates to their concrete State implementations
// via DecodeState, mirroring Workflow.UnmarshalJSON — the same
// polymorphic-interface problem, one level down.
func (s *CompositeState) UnmarshalJSON(data []byte) error {
	type alias CompositeState
	aux := &struct {
		Substates []json.RawMessage `json:"substates"`
		*alias
	}{alias: (*alias)(s)}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	s.Substates = make([]State, 0, len(aux.Substates))
	for _, raw := range aux.Substates {
		sub, err := DecodeState(raw)
		if err != nil {
			return fmt.Errorf("composite %q: %w", s.Id, err)
		}
		s.Substates = append(s.Substates, sub)
	}
	return nil
}

// FindSubstate returns the substate with the given id, or nil.
func (s *CompositeState) FindSubstate(id string) State {
	for _, sub := range s.Substates {
		if sub.GetId() == id {
			return sub
		}
	}
	return nil
}
