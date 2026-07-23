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
	CreatedAt          time.Time      `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt          time.Time      `json:"updated_at" gorm:"autoUpdateTime;index"`
	DeletedAt          gorm.DeletedAt `json:"deleted_at" gorm:"index;default:null"`
}
