package engine

import (
	"context"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

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
	model.CreateChildWorkflowsFunc = CreateChildWorkflowInstancesBatch
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
	child.Complete = isEndState(*child)
	if result := db.Create(child); result.Error != nil {
		return uuid.Nil, result.Error
	}
	return id, nil
}

// CreateChildWorkflowInstancesBatch creates every child in defs concurrently
// (independent inserts) and returns their ids in the same order as defs. It
// backs model.CreateChildWorkflowsFunc, so a transition that spawns several
// children pays roughly one insert's worth of latency instead of N serial ones.
func CreateChildWorkflowInstancesBatch(parentId string, defs []model.Workflow) ([]string, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	ids := make([]string, len(defs))
	g := new(errgroup.Group)
	for i := range defs {
		i := i
		g.Go(func() error {
			id, err := CreateChildWorkflowInstance(parentId, defs[i])
			if err != nil {
				return err
			}
			ids[i] = id.String()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return ids, nil
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
			Complete:     child.Complete,
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

// allChildrenComplete reports whether every (non-withdrawn) child of parentId
// has reached one of its own workflow's end states. A parent with no children
// is vacuously complete. This is a single SQL count over the persisted
// Complete flag (set on each transition), so it no longer loads every child's
// definition jsonb just to scan EndStates in Go. Soft-deleted (withdrawn)
// children are excluded by GORM's default scope.
func allChildrenComplete(ctx context.Context, parentId string) (bool, error) {
	db := config.Db.WithContext(ctx)

	var incomplete int64
	if result := db.Model(&persistence.WorkflowInstance{}).
		Where("parent_id = ? AND complete = ?", parentId, false).
		Count(&incomplete); result.Error != nil {
		return false, result.Error
	}
	return incomplete == 0, nil
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
