package persistence

import (
	"time"

	"github.com/google/uuid"
	"github.com/kashari/brokr/model"
	"gorm.io/gorm"
)

type WorkflowInstance struct {
	Id                 uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	WorkflowDefinition model.Workflow `json:"workflowDefinition" gorm:"type:jsonb"`
	CurrentState       model.State    `json:"currentState" gorm:"type:jsonb"`
	LastTransition     string         `json:"lastTransition" gorm:"type:text"`
	CreatedAt          time.Time      `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt          time.Time      `json:"updated_at" gorm:"autoUpdateTime;index"`
	DeletedAt          gorm.DeletedAt `json:"deleted_at" gorm:"index;default:null"`
}
