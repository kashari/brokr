package model

import "context"

type SimpleState struct {
	Type           string `json:"type"`
	Id             string `json:"id"`
	FrontendBullet bool   `json:"frontendBullet"`
	BulletName     string `json:"bulletName"`
	ResumeEvent    string `json:"resumeEvent"`
	ProductStatus  string `json:"productStatus"`
	Status         string `json:"status"`
	IsHappyFlow    *bool  `json:"isHappyFlow,omitempty"`
}

func (s *SimpleState) IsResumable() bool {
	return s.ResumeEvent != ""
}

func (s *SimpleState) GetType() string {
	return s.Type
}

func (s *SimpleState) GetId() string {
	return s.Id
}

func (s *SimpleState) GetFrontendBullet() bool {
	return s.FrontendBullet
}

func (s *SimpleState) GetBulletName() string {
	return s.BulletName
}

func (s *SimpleState) GetResumeEvent() string {
	return s.ResumeEvent
}

func (s *SimpleState) GetProductStatus() string {
	return s.ProductStatus
}

func (s *SimpleState) GetStatus() string {
	return s.Status
}

func (s *SimpleState) GetIsHappyFlow() bool {
	return s.IsHappyFlow == nil || *s.IsHappyFlow
}

func (s *SimpleState) ExecuteEntryActions(ctx context.Context, token string, ctxMap map[string]string) (map[string]string, error) {
	return ctxMap, nil
}

func (s *SimpleState) ExecuteExitActions(ctx context.Context, token string, ctxMap map[string]string) (map[string]string, error) {
	return ctxMap, nil
}

func (s *SimpleState) GetDoActions() []Action {
	return nil
}
