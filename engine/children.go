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

// createChildWorkflowInstance is the shared core of CreateChildWorkflowInstance
// and CreateChildWorkflowInstancesBatchWithGeneration. forkGeneration is ""
// for ad hoc single-child creation (CreateChildWorkflowAction), non-empty
// for children spawned by a formal Fork transition.
func createChildWorkflowInstance(parentId string, childDefinition model.Workflow, forkGeneration string) (uuid.UUID, error) {
	db := config.Db

	var parent persistence.WorkflowInstance
	if result := db.First(&parent, "id = ?", parentId); result.Error != nil {
		return uuid.Nil, result.Error
	}

	parentUUID, err := uuid.Parse(parentId)
	if err != nil {
		return uuid.Nil, err
	}

	initState, initSub, err := resolveInitialPosition(childDefinition.States[0])
	if err != nil {
		return uuid.Nil, err
	}

	id := uuid.New()
	golog.Info("Creating child workflow instance [{}] under parent [{}]", id.String(), parentId)
	child := &persistence.WorkflowInstance{
		Id:                 id,
		ParentId:           &parentUUID,
		ForkGeneration:     forkGeneration,
		WorkflowDefinition: childDefinition,
		CurrentState:       persistence.StateContainer{State: initState, Substate: initSub},
	}
	child.Journey = append(child.Journey, initialJourneyEntry(initState, initSub))
	child.Complete = isEndState(*child)
	if result := db.Create(child); result.Error != nil {
		return uuid.Nil, result.Error
	}
	// Arm the initial state's do-activity/timers, once the insert has
	// committed — same rationale as NewWorkflowInstance.
	armStateActivities(id.String(), child, child.ContextMap)
	return id, nil
}

// CreateChildWorkflowInstance creates a new workflow instance linked to
// parentId as its parent. parentId must reference an existing (non-withdrawn)
// instance.
func CreateChildWorkflowInstance(parentId string, childDefinition model.Workflow) (uuid.UUID, error) {
	return createChildWorkflowInstance(parentId, childDefinition, "")
}

// CreateChildWorkflowInstancesBatch creates every child in defs concurrently
// (independent inserts, unstamped generation) and returns their ids in the
// same order as defs. It backs model.CreateChildWorkflowsFunc.
func CreateChildWorkflowInstancesBatch(parentId string, defs []model.Workflow) ([]string, error) {
	return CreateChildWorkflowInstancesBatchWithGeneration(parentId, defs, "")
}

// CreateChildWorkflowInstancesBatchWithGeneration is CreateChildWorkflowInstancesBatch
// with every created child stamped with forkGeneration, so a later Join
// transition can scope allChildrenComplete to exactly this cohort (Task 10).
func CreateChildWorkflowInstancesBatchWithGeneration(parentId string, defs []model.Workflow, forkGeneration string) ([]string, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	ids := make([]string, len(defs))
	g := new(errgroup.Group)
	for i := range defs {
		i := i
		g.Go(func() error {
			id, err := createChildWorkflowInstance(parentId, defs[i], forkGeneration)
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

// allChildrenComplete reports whether every (non-withdrawn) child of
// parentId in forkGeneration has reached one of its own workflow's end
// states. A parent with no such children is vacuously complete. When
// forkGeneration is "" (an instance whose children were all created via
// the ad hoc CreateChildWorkflowAction, never a formal Fork transition),
// it falls back to counting every non-withdrawn child — today's exact
// behavior, preserved for backward compatibility.
func allChildrenComplete(ctx context.Context, parentId string, forkGeneration string) (bool, error) {
	db := config.Db.WithContext(ctx)

	q := db.Model(&persistence.WorkflowInstance{}).
		Where("parent_id = ? AND complete = ?", parentId, false)
	if forkGeneration != "" {
		q = q.Where("fork_generation = ?", forkGeneration)
	}

	var incomplete int64
	if result := q.Count(&incomplete); result.Error != nil {
		return false, result.Error
	}
	return incomplete == 0, nil
}

func isEndState(wf persistence.WorkflowInstance) bool {
	if wf.CurrentState.State == nil {
		return false
	}
	for _, end := range wf.WorkflowDefinition.EndStates {
		if end == wf.CurrentState.State.GetId() {
			return true
		}
	}
	return false
}
