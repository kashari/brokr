package dto

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kashari/brokr/model"
)

// GraphActionSummary is a human-readable rendering of a model.Action for
// the visualizer — the frontend never needs to interpret Action.Type
// itself.
type GraphActionSummary struct {
	Type  string `json:"type"`
	Label string `json:"label"`
}

func summarizeAction(a model.Action) GraphActionSummary {
	switch a.Type {
	case model.HttpRequestAction:
		return GraphActionSummary{Type: string(a.Type), Label: fmt.Sprintf("%s %s", a.Method, a.Url)}
	case model.SetContextMapAction:
		keys := make([]string, 0, len(a.Variables))
		for k := range a.Variables {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return GraphActionSummary{Type: string(a.Type), Label: "set " + strings.Join(keys, ", ")}
	case model.CreateChildWorkflowAction:
		label := "spawn child workflow"
		if a.ChildWorkflow != nil && a.ChildWorkflow.Id != "" {
			label = "spawn child: " + a.ChildWorkflow.Id
		}
		return GraphActionSummary{Type: string(a.Type), Label: label}
	default:
		return GraphActionSummary{Type: string(a.Type), Label: string(a.Type)}
	}
}

func summarizeActions(actions []model.Action) []GraphActionSummary {
	if len(actions) == 0 {
		return nil
	}
	out := make([]GraphActionSummary, 0, len(actions))
	for _, a := range actions {
		out = append(out, summarizeAction(a))
	}
	return out
}

// GraphGuardSummary mirrors model.Guard for the wire — kept separate so
// the frontend never has to import/understand model.GuardOp.
type GraphGuardSummary struct {
	Key   string `json:"key"`
	Op    string `json:"op"`
	Value string `json:"value,omitempty"`
}

func summarizeGuard(g *model.Guard) *GraphGuardSummary {
	if g == nil {
		return nil
	}
	return &GraphGuardSummary{Key: g.Key, Op: string(g.Op), Value: g.Value}
}

// GraphNode is one state (or substate) in a workflow definition, flattened
// for rendering: a CompositeState appears once as a node with Substates
// set, and its substates appear as their own nodes with ParentId set to
// the composite's id.
type GraphNode struct {
	Id              string               `json:"id"`
	Type            string               `json:"type"`
	BulletName      string               `json:"bulletName"`
	Status          string               `json:"status"`
	ProductStatus   string               `json:"productStatus"`
	FrontendBullet  bool                 `json:"frontendBullet"`
	ResumeEvent     string               `json:"resumeEvent,omitempty"`
	IsInitial       bool                 `json:"isInitial"`
	IsEnd           bool                 `json:"isEnd"`
	IsHappyFlow     bool                 `json:"isHappyFlow"`
	ParentId        string               `json:"parentId,omitempty"`
	EntryActions    []GraphActionSummary `json:"entryActions,omitempty"`
	ExitActions     []GraphActionSummary `json:"exitActions,omitempty"`
	DoActions       []GraphActionSummary `json:"doActions,omitempty"`
	Substates       []string             `json:"substates,omitempty"`
	InitialSubstate string               `json:"initialSubstate,omitempty"`
	History         string               `json:"history,omitempty"`
}

// GraphEdge is one transition (top-level, a composite's sub-transition, or
// a CommonTransition expanded per matching source) in a workflow
// definition. Scope is "top", "sub:<compositeId>", or "common".
type GraphEdge struct {
	Id            string               `json:"id"`
	Source        string               `json:"source"`
	Target        string               `json:"target"`
	Event         string               `json:"event"`
	Trigger       string               `json:"trigger,omitempty"`
	Kind          string               `json:"kind,omitempty"`
	Guard         *GraphGuardSummary   `json:"guard,omitempty"`
	EntryActions  []GraphActionSummary `json:"entryActions,omitempty"`
	Join          bool                 `json:"join,omitempty"`
	After         string               `json:"after,omitempty"`
	EntersHistory bool                 `json:"entersHistory,omitempty"`
	ForkTargets   []string             `json:"forkTargets,omitempty"`
	Scope         string               `json:"scope"`
}

// Graph is the full analyzed shape of a model.Workflow definition — every
// state (including composite substates) and every transition, ready for
// the visualizer to render without re-deriving any of the model's
// polymorphism or scoping rules itself.
type Graph struct {
	WorkflowId   string      `json:"workflowId"`
	Version      string      `json:"version"`
	InitialState string      `json:"initialState"`
	EndStates    []string    `json:"endStates"`
	Nodes        []GraphNode `json:"nodes"`
	Edges        []GraphEdge `json:"edges"`
}

// stateActions returns a state's EntryActions/ExitActions, if it has any —
// only ActionState and CompositeState carry them; SimpleState has none, and
// none of these three expose them through the model.State interface itself.
func stateActions(s model.State) (entry, exit []model.Action) {
	switch v := s.(type) {
	case *model.ActionState:
		return v.EntryActions, v.ExitActions
	case *model.CompositeState:
		return v.EntryActions, v.ExitActions
	default:
		return nil, nil
	}
}

func nodeFor(s model.State, parentId string, endStates map[string]bool, initialId string) GraphNode {
	entry, exit := stateActions(s)
	n := GraphNode{
		Id:             s.GetId(),
		Type:           s.GetType(),
		BulletName:     s.GetBulletName(),
		Status:         s.GetStatus(),
		ProductStatus:  s.GetProductStatus(),
		FrontendBullet: s.GetFrontendBullet(),
		ResumeEvent:    s.GetResumeEvent(),
		IsInitial:      s.GetId() == initialId,
		IsEnd:          endStates[s.GetId()],
		IsHappyFlow:    s.GetIsHappyFlow(),
		ParentId:       parentId,
		EntryActions:   summarizeActions(entry),
		ExitActions:    summarizeActions(exit),
		DoActions:      summarizeActions(s.GetDoActions()),
	}
	if comp, ok := s.(*model.CompositeState); ok {
		n.InitialSubstate = comp.InitialSubstate
		n.History = string(comp.History)
		for _, sub := range comp.Substates {
			n.Substates = append(n.Substates, sub.GetId())
		}
	}
	return n
}

func edgeFor(idx int, scope string, t model.Transition) GraphEdge {
	forkTargets := make([]string, 0, len(t.ForkTargets))
	for _, ft := range t.ForkTargets {
		ref := ft.Ref
		if ref == "" && ft.ChildWorkflow != nil {
			ref = ft.ChildWorkflow.Id
		}
		forkTargets = append(forkTargets, ref)
	}
	return GraphEdge{
		Id:            fmt.Sprintf("%s-%s-%s-%s-%d", scope, t.Source, t.Target, t.Event, idx),
		Source:        t.Source,
		Target:        t.Target,
		Event:         t.Event,
		Trigger:       string(t.Trigger),
		Kind:          string(t.Kind),
		Guard:         summarizeGuard(t.Guard),
		EntryActions:  summarizeActions(t.EntryActions),
		Join:          t.IsJoin(),
		After:         t.After,
		EntersHistory: t.EntersHistory,
		ForkTargets:   forkTargets,
		Scope:         scope,
	}
}

// BuildGraph analyzes a workflow definition into the flattened node/edge
// shape the visualizer renders. It is a pure function of wf — no instance
// state, no I/O — so it works for any workflow definition, not just ones
// with a live instance.
func BuildGraph(wf model.Workflow) Graph {
	endStates := make(map[string]bool, len(wf.EndStates))
	for _, id := range wf.EndStates {
		endStates[id] = true
	}

	g := Graph{
		WorkflowId:   wf.Id,
		Version:      wf.Version,
		InitialState: wf.InitialState,
		EndStates:    wf.EndStates,
	}

	for _, s := range wf.States {
		g.Nodes = append(g.Nodes, nodeFor(s, "", endStates, wf.InitialState))
		if comp, ok := s.(*model.CompositeState); ok {
			for _, sub := range comp.Substates {
				g.Nodes = append(g.Nodes, nodeFor(sub, comp.Id, endStates, wf.InitialState))
			}
			for i, t := range comp.SubTransitions {
				g.Edges = append(g.Edges, edgeFor(i, "sub:"+comp.Id, t))
			}
		}
	}

	for i, t := range wf.Transitions {
		g.Edges = append(g.Edges, edgeFor(i, "top", t))
	}

	for i, ct := range wf.CommonTransitions {
		for _, source := range ct.SourceList {
			g.Edges = append(g.Edges, edgeFor(i, "common", model.Transition{
				Trigger:      ct.Trigger,
				Kind:         ct.Kind,
				Source:       source,
				Target:       ct.Target,
				Event:        ct.Event,
				Guard:        ct.Guard,
				EntryActions: ct.EntryActions,
			}))
		}
	}

	return g
}
