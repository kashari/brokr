package persistence

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/kashari/brokr/model"
)

// StateContainer wraps model.State (and, when positioned inside a
// CompositeState, the active Substate) so it can be stored in a jsonb
// column. model.State is an interface, so GORM's generic
// reflect.New(fieldType)+json.Unmarshal serializer can't populate it
// directly on read; StateContainer implements Scan/Value itself instead,
// using model.DecodeState to resolve the concrete implementation.
// Embedding State means most existing read call sites
// (wf.CurrentState.State.GetId(), etc.) keep working; new code that needs
// to be substate-aware should use EffectiveId/EffectiveState instead.
type StateContainer struct {
	State    model.State
	Substate model.State
}

// stateEnvelope is StateContainer's on-disk jsonb shape.
type stateEnvelope struct {
	State    json.RawMessage `json:"state"`
	Substate json.RawMessage `json:"substate,omitempty"`
}

// EffectiveId returns the id of whichever state is "current" from an
// event-matching or do-activity point of view: the Substate if
// positioned inside a composite, else the top-level State.
func (c StateContainer) EffectiveId() string {
	return c.EffectiveState().GetId()
}

// EffectiveState is the model.State counterpart of EffectiveId.
func (c StateContainer) EffectiveState() model.State {
	if c.Substate != nil {
		return c.Substate
	}
	return c.State
}

func (c StateContainer) Value() (driver.Value, error) {
	if c.State == nil {
		return nil, nil
	}
	stateJSON, err := json.Marshal(c.State)
	if err != nil {
		return nil, fmt.Errorf("marshal state: %w", err)
	}
	env := stateEnvelope{State: stateJSON}
	if c.Substate != nil {
		subJSON, err := json.Marshal(c.Substate)
		if err != nil {
			return nil, fmt.Errorf("marshal substate: %w", err)
		}
		env.Substate = subJSON
	}
	b, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("marshal state envelope: %w", err)
	}
	return string(b), nil
}

func (c *StateContainer) Scan(value any) error {
	if value == nil {
		c.State = nil
		c.Substate = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported Scan source type %T for StateContainer", value)
	}

	var env stateEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("unmarshal state envelope: %w", err)
	}
	state, err := model.DecodeState(env.State)
	if err != nil {
		return err
	}
	c.State = state
	c.Substate = nil
	if len(env.Substate) > 0 {
		sub, err := model.DecodeState(env.Substate)
		if err != nil {
			return err
		}
		c.Substate = sub
	}
	return nil
}
