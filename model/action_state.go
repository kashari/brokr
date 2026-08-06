package model

import "context"

type ActionState struct {
	Type           string   `json:"type"`
	Id             string   `json:"id"`
	FrontendBullet bool     `json:"frontendBullet"`
	BulletName     string   `json:"bulletName"`
	ResumeEvent    string   `json:"resumeEvent"`
	ProductStatus  string   `json:"productStatus"`
	Status         string   `json:"status"`
	ExpectResponse bool     `json:"expectResponse"`
	EntryActions   []Action `json:"entryActions"`
	ExitActions    []Action `json:"exitActions"`
	// DoActions run for as long as the instance stays in this state,
	// cancelled the moment any transition leaves it (see engine's
	// do-activity registry). Unlike EntryActions/ExitActions, a do-activity
	// does not block the transition that starts it — it's UML's "activity
	// concurrent with being in the state," not a state-change effect.
	DoActions []Action `json:"doActions,omitempty"`
}

func (s *ActionState) IsResumable() bool {
	return s.ResumeEvent != ""
}

func (s *ActionState) GetType() string {
	return s.Type
}

func (s *ActionState) GetId() string {
	return s.Id
}

func (s *ActionState) GetFrontendBullet() bool {
	return s.FrontendBullet
}

func (s *ActionState) GetBulletName() string {
	return s.BulletName
}

func (s *ActionState) GetResumeEvent() string {
	return s.ResumeEvent
}

func (s *ActionState) GetProductStatus() string {
	return s.ProductStatus
}

func (s *ActionState) GetStatus() string {
	return s.Status
}

func (s *ActionState) ExecuteEntryActions(ctx context.Context, token string, ctxMap map[string]string) (map[string]string, error) {
	return ExecuteActions(ctx, token, ctxMap, s.EntryActions)
}

func (s *ActionState) ExecuteExitActions(ctx context.Context, token string, ctxMap map[string]string) (map[string]string, error) {
	return ExecuteActions(ctx, token, ctxMap, s.ExitActions)
}

func (s *ActionState) GetDoActions() []Action {
	return s.DoActions
}
