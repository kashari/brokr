package persistence

import (
	"time"

	"github.com/kashari/brokr/model"
)

// WorkflowDefinition is a named, registered Workflow definition — the
// source of truth cmd/seed-workflows writes to and
// engine.NewWorkflowInstanceByName reads from, so a client can create an
// instance by supplying only a name instead of the full definition body
// (see README "Registering workflow definitions"). Name is the
// definition's own model.Workflow.Id, used as the lookup key.
type WorkflowDefinition struct {
	Name       string         `json:"name" gorm:"primaryKey"`
	Definition model.Workflow `json:"definition" gorm:"type:jsonb;serializer:json"`
	CreatedAt  time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
}
