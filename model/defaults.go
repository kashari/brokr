package model

import "time"

// ApplyDefaults normalizes a Workflow definition loaded from
// author-provided JSON before it is registered as a named definition (see
// cmd/seed-workflows): it fills in zero-value fields with sensible
// defaults, and force-sets Kind to UserWorkflowKind — every definition
// registered through that pipeline is user-authored, so Kind is not an
// author-settable override, unlike every other field this function
// defaults. It recurses into every nested Workflow (a
// CreateChildWorkflowAction's ChildWorkflow, a ForkTarget's ChildWorkflow)
// so the same rules apply at every level of a definition.
func ApplyDefaults(wf *Workflow) {
	if wf == nil {
		return
	}
	wf.Kind = UserWorkflowKind
	if wf.Version == "" {
		wf.Version = "1.0.0"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if wf.CreationDate == "" {
		wf.CreationDate = now
	}
	if wf.UpdateDate == "" {
		wf.UpdateDate = now
	}
	if wf.InitialState == "" && len(wf.States) > 0 {
		wf.InitialState = wf.States[0].GetId()
	}
	for _, s := range wf.States {
		applyStateDefaults(s)
	}
	for i := range wf.Transitions {
		applyTransitionDefaults(&wf.Transitions[i])
	}
	for i := range wf.CommonTransitions {
		applyCommonTransitionDefaults(&wf.CommonTransitions[i])
	}
}

func applyStateDefaults(s State) {
	switch st := s.(type) {
	case *SimpleState:
		if st.Type == "" {
			st.Type = SimpleStateType
		}
		if st.Status == "" {
			st.Status = "NEW"
		}
		if st.ProductStatus == "" {
			st.ProductStatus = "IN_PROGRESS"
		}
		if st.BulletName == "" {
			st.BulletName = st.Id
		}
	case *ActionState:
		if st.Type == "" {
			st.Type = ActionStateType
		}
		if st.Status == "" {
			st.Status = "NEW"
		}
		if st.ProductStatus == "" {
			st.ProductStatus = "IN_PROGRESS"
		}
		if st.BulletName == "" {
			st.BulletName = st.Id
		}
		for i := range st.EntryActions {
			applyActionDefaults(&st.EntryActions[i])
		}
		for i := range st.ExitActions {
			applyActionDefaults(&st.ExitActions[i])
		}
		for i := range st.DoActions {
			applyActionDefaults(&st.DoActions[i])
		}
	case *CompositeState:
		if st.Type == "" {
			st.Type = CompositeStateType
		}
		if st.Status == "" {
			st.Status = "NEW"
		}
		if st.ProductStatus == "" {
			st.ProductStatus = "IN_PROGRESS"
		}
		if st.BulletName == "" {
			st.BulletName = st.Id
		}
		if st.InitialSubstate == "" && len(st.Substates) > 0 {
			st.InitialSubstate = st.Substates[0].GetId()
		}
		for i := range st.EntryActions {
			applyActionDefaults(&st.EntryActions[i])
		}
		for i := range st.ExitActions {
			applyActionDefaults(&st.ExitActions[i])
		}
		for _, sub := range st.Substates {
			applyStateDefaults(sub)
		}
		for i := range st.SubTransitions {
			applyTransitionDefaults(&st.SubTransitions[i])
		}
	}
}

func applyTransitionDefaults(t *Transition) {
	if t.Trigger == "" {
		t.Trigger = UserTrigger
	}
	if t.Kind == "" {
		t.Kind = ExternalKind
	}
	for i := range t.EntryActions {
		applyActionDefaults(&t.EntryActions[i])
	}
	for i := range t.ForkTargets {
		if t.ForkTargets[i].ChildWorkflow != nil {
			ApplyDefaults(t.ForkTargets[i].ChildWorkflow)
		}
	}
}

func applyCommonTransitionDefaults(ct *CommonTransition) {
	if ct.Trigger == "" {
		ct.Trigger = UserTrigger
	}
	if ct.Kind == "" {
		ct.Kind = ExternalKind
	}
	for i := range ct.EntryActions {
		applyActionDefaults(&ct.EntryActions[i])
	}
}

func applyActionDefaults(a *Action) {
	if a.Type == HttpRequestAction && a.Method == "" {
		a.Method = "GET"
	}
	if a.ChildWorkflow != nil {
		ApplyDefaults(a.ChildWorkflow)
	}
}
