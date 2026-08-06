package persistence

import (
	"time"

	"github.com/google/uuid"
	"github.com/kashari/brokr/model"
	"gorm.io/gorm"
)

type WorkflowInstance struct {
	Id                 uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ParentId           *uuid.UUID     `json:"parentId,omitempty" gorm:"type:uuid;index"`
	WorkflowDefinition model.Workflow `json:"workflowDefinition" gorm:"type:jsonb;serializer:json"`
	CurrentState       StateContainer `json:"currentState" gorm:"type:jsonb"`
	LastTransition     string         `json:"lastTransition" gorm:"type:text"`
	// ContextMap persists the workflow's ${var} interpolation variables
	// across transitions. Every transition loads it, threads it through
	// exit/entry/transition actions (which may add or overwrite keys via
	// SetContextMapAction or an ExpectResponse HTTP action), and saves the
	// result back in the same write that persists the new CurrentState.
	ContextMap map[string]string `json:"contextMap" gorm:"type:jsonb;serializer:json"`
	// Version is bumped on every persisted transition. It's an optimistic
	// concurrency marker: the actor-per-instance dispatcher already serializes
	// writes to one instance, so a stale Version would signal a serialization
	// bug rather than normal contention.
	Version int `json:"version" gorm:"default:0"`
	// Complete is true once CurrentState is one of the instance's own
	// WorkflowDefinition.EndStates. Persisting it lets a parent's join gate be
	// evaluated with a single SQL count instead of loading every child's jsonb.
	Complete bool `json:"complete" gorm:"index;default:false"`
	CreatedAt          time.Time      `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt          time.Time      `json:"updated_at" gorm:"autoUpdateTime;index"`
	DeletedAt          gorm.DeletedAt `json:"deleted_at" gorm:"index;default:null"`
}
