package model

import "testing"

func TestApplyDefaultsForcesKindToUserRegardlessOfInput(t *testing.T) {
	wf := &Workflow{Kind: "SYSTEM", States: []State{&SimpleState{Id: "s"}}}
	ApplyDefaults(wf)
	if wf.Kind != UserWorkflowKind {
		t.Fatalf("Kind = %q, want %q", wf.Kind, UserWorkflowKind)
	}
}

func TestApplyDefaultsFillsMissingTopLevelFields(t *testing.T) {
	wf := &Workflow{States: []State{&SimpleState{Id: "start"}}}
	ApplyDefaults(wf)
	if wf.Version == "" {
		t.Fatal("Version was not defaulted")
	}
	if wf.CreationDate == "" || wf.UpdateDate == "" {
		t.Fatal("CreationDate/UpdateDate were not defaulted")
	}
	if wf.InitialState != "start" {
		t.Fatalf("InitialState = %q, want %q", wf.InitialState, "start")
	}
}

func TestApplyDefaultsFillsMissingStateFields(t *testing.T) {
	s := &SimpleState{Id: "start"}
	wf := &Workflow{States: []State{s}}
	ApplyDefaults(wf)
	if s.Type != SimpleStateType {
		t.Fatalf("Type = %q, want %q", s.Type, SimpleStateType)
	}
	if s.Status != "NEW" {
		t.Fatalf("Status = %q, want NEW", s.Status)
	}
	if s.ProductStatus != "IN_PROGRESS" {
		t.Fatalf("ProductStatus = %q, want IN_PROGRESS", s.ProductStatus)
	}
	if s.BulletName != "start" {
		t.Fatalf("BulletName = %q, want %q", s.BulletName, "start")
	}
}

func TestApplyDefaultsDoesNotOverrideProvidedFields(t *testing.T) {
	s := &SimpleState{Id: "start", Status: "CUSTOM", BulletName: "Kickoff"}
	wf := &Workflow{Version: "9.9.9", States: []State{s}}
	ApplyDefaults(wf)
	if wf.Version != "9.9.9" {
		t.Fatalf("Version was overridden: got %q", wf.Version)
	}
	if s.Status != "CUSTOM" {
		t.Fatalf("Status was overridden: got %q", s.Status)
	}
	if s.BulletName != "Kickoff" {
		t.Fatalf("BulletName was overridden: got %q", s.BulletName)
	}
}

func TestApplyDefaultsSetsTransitionAndActionDefaults(t *testing.T) {
	wf := &Workflow{
		States: []State{&SimpleState{Id: "a"}, &SimpleState{Id: "b"}},
		Transitions: []Transition{
			{Source: "a", Target: "b", Event: "go", EntryActions: []Action{{Type: HttpRequestAction, Url: "http://x"}}},
		},
	}
	ApplyDefaults(wf)
	tr := wf.Transitions[0]
	if tr.Trigger != UserTrigger {
		t.Fatalf("Trigger = %q, want %q", tr.Trigger, UserTrigger)
	}
	if tr.Kind != ExternalKind {
		t.Fatalf("Kind = %q, want %q", tr.Kind, ExternalKind)
	}
	if tr.EntryActions[0].Method != "GET" {
		t.Fatalf("Method = %q, want GET", tr.EntryActions[0].Method)
	}
}

func TestApplyDefaultsRecursesIntoChildWorkflow(t *testing.T) {
	child := &Workflow{Kind: "SYSTEM", States: []State{&SimpleState{Id: "c"}}}
	wf := &Workflow{
		States: []State{&SimpleState{Id: "a"}},
		Transitions: []Transition{
			{
				Source: "a", Target: "a", Event: "spawn",
				EntryActions: []Action{{Type: CreateChildWorkflowAction, ChildWorkflow: child}},
			},
		},
	}
	ApplyDefaults(wf)
	if child.Kind != UserWorkflowKind {
		t.Fatalf("nested ChildWorkflow.Kind = %q, want %q", child.Kind, UserWorkflowKind)
	}
	if child.Version == "" {
		t.Fatal("nested ChildWorkflow.Version was not defaulted")
	}
}

func TestApplyDefaultsRecursesIntoCompositeSubstates(t *testing.T) {
	sub := &SimpleState{Id: "inner"}
	comp := &CompositeState{Id: "outer", Substates: []State{sub}}
	wf := &Workflow{States: []State{comp}}
	ApplyDefaults(wf)
	if comp.InitialSubstate != "inner" {
		t.Fatalf("InitialSubstate = %q, want %q", comp.InitialSubstate, "inner")
	}
	if sub.Status != "NEW" {
		t.Fatal("nested substate defaults were not applied")
	}
}

func TestApplyDefaultsRecursesIntoForkTargetsAndCompositeActionsAndSubTransitions(t *testing.T) {
	forkChild := &Workflow{Kind: "SYSTEM", States: []State{&SimpleState{Id: "fc"}}}
	wf := &Workflow{
		States: []State{
			&SimpleState{Id: "a"},
			&CompositeState{
				Id:           "outer",
				Substates:    []State{&SimpleState{Id: "inner"}},
				EntryActions: []Action{{Type: HttpRequestAction, Url: "http://x"}},
				ExitActions:  []Action{{Type: HttpRequestAction, Url: "http://y"}},
				SubTransitions: []Transition{
					{Source: "inner", Target: "inner", Event: "loop"},
				},
			},
		},
		Transitions: []Transition{
			{
				Source: "a", Target: "outer", Event: "fork",
				ForkTargets: []ForkTarget{{ChildWorkflow: forkChild}},
			},
		},
	}
	ApplyDefaults(wf)

	if forkChild.Kind != UserWorkflowKind {
		t.Fatalf("ForkTarget.ChildWorkflow.Kind = %q, want %q", forkChild.Kind, UserWorkflowKind)
	}
	if forkChild.Version == "" {
		t.Fatal("ForkTarget.ChildWorkflow.Version was not defaulted")
	}

	comp := wf.States[1].(*CompositeState)
	if comp.EntryActions[0].Method != "GET" {
		t.Fatalf("CompositeState.EntryActions[0].Method = %q, want GET", comp.EntryActions[0].Method)
	}
	if comp.ExitActions[0].Method != "GET" {
		t.Fatalf("CompositeState.ExitActions[0].Method = %q, want GET", comp.ExitActions[0].Method)
	}
	sub := comp.SubTransitions[0]
	if sub.Trigger != UserTrigger {
		t.Fatalf("CompositeState.SubTransitions[0].Trigger = %q, want %q", sub.Trigger, UserTrigger)
	}
	if sub.Kind != ExternalKind {
		t.Fatalf("CompositeState.SubTransitions[0].Kind = %q, want %q", sub.Kind, ExternalKind)
	}
}
