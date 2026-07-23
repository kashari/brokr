package persistence

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/kashari/brokr/model"
)

// StateContainer wraps model.State so it can be stored in a jsonb column.
// model.State is an interface, so GORM's generic reflect.New(fieldType)+
// json.Unmarshal serializer can't populate it directly on read; StateContainer
// implements Scan/Value itself instead, using model.DecodeState to resolve
// the concrete SimpleState/ActionState. Embedding State means every existing
// read call site (wf.CurrentState.GetId(), .ExecuteEntryActions(...), etc.)
// keeps working unchanged via method promotion.
type StateContainer struct {
	model.State
}

func (c StateContainer) Value() (driver.Value, error) {
	if c.State == nil {
		return nil, nil
	}
	b, err := json.Marshal(c.State)
	if err != nil {
		return nil, fmt.Errorf("marshal state: %w", err)
	}
	return string(b), nil
}

func (c *StateContainer) Scan(value any) error {
	if value == nil {
		c.State = nil
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
	state, err := model.DecodeState(data)
	if err != nil {
		return err
	}
	c.State = state
	return nil
}
