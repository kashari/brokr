package engine

import (
	"github.com/google/uuid"

	"github.com/kashari/brokr/config"
	"github.com/kashari/brokr/dto"
	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
	"github.com/kashari/golog"
)

func init() {
	model.CreateChildWorkflowFunc = func(parentId string, childDefinition model.Workflow) (string, error) {
		id, err := CreateChildWorkflowInstance(parentId, childDefinition)
		if err != nil {
			return "", err
		}
		return id.String(), nil
	}
}

// CreateChildWorkflowInstance creates a new workflow instance linked to
// parentId as its parent. parentId must reference an existing (non-withdrawn)
// instance.
func CreateChildWorkflowInstance(parentId string, childDefinition model.Workflow) (uuid.UUID, error) {
	db := config.Db

	var parent persistence.WorkflowInstance
	if result := db.First(&parent, "id = ?", parentId); result.Error != nil {
		return uuid.Nil, result.Error
	}

	parentUUID, err := uuid.Parse(parentId)
	if err != nil {
		return uuid.Nil, err
	}

	id := uuid.New()
	golog.Info("Creating child workflow instance [{}] under parent [{}]", id.String(), parentId)
	child := &persistence.WorkflowInstance{
		Id:                 id,
		ParentId:           &parentUUID,
		WorkflowDefinition: childDefinition,
		CurrentState:       persistence.StateContainer{State: childDefinition.States[0]},
	}
	if result := db.Create(child); result.Error != nil {
		return uuid.Nil, result.Error
	}
	return id, nil
}

// GetChildWorkflowInstances lists the (non-withdrawn) children of parentId.
func GetChildWorkflowInstances(parentId string) ([]dto.ChildInstance, error) {
	db := config.Db

	var children []persistence.WorkflowInstance
	if result := db.Find(&children, "parent_id = ?", parentId); result.Error != nil {
		return nil, result.Error
	}

	instances := make([]dto.ChildInstance, 0, len(children))
	for _, child := range children {
		instances = append(instances, dto.ChildInstance{
			Id:           child.Id.String(),
			CurrentState: child.CurrentState.State,
			Complete:     isEndState(child),
		})
	}
	return instances, nil
}

// WithdrawChildWorkflowInstance soft-deletes childId, provided it belongs to
// parentId, severing it from any future join check on its parent.
func WithdrawChildWorkflowInstance(parentId, childId string) error {
	db := config.Db

	var child persistence.WorkflowInstance
	if result := db.First(&child, "id = ? AND parent_id = ?", childId, parentId); result.Error != nil {
		return result.Error
	}

	if result := db.Delete(&child); result.Error != nil {
		return result.Error
	}
	golog.Info("Withdrew child workflow instance [{}] from parent [{}]", childId, parentId)
	return nil
}

// allChildrenComplete reports whether every (non-withdrawn) child of
// parentId has reached one of its own workflow's end states. A parent with
// no children is vacuously complete.
func allChildrenComplete(parentId string) (bool, error) {
	db := config.Db

	var children []persistence.WorkflowInstance
	if result := db.Find(&children, "parent_id = ?", parentId); result.Error != nil {
		return false, result.Error
	}

	for _, child := range children {
		if !isEndState(child) {
			return false, nil
		}
	}
	return true, nil
}

func isEndState(wf persistence.WorkflowInstance) bool {
	if wf.CurrentState.State == nil {
		return false
	}
	for _, end := range wf.WorkflowDefinition.EndStates {
		if end == wf.CurrentState.GetId() {
			return true
		}
	}
	return false
}
