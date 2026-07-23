package model

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

func (s *ActionState) ExecuteEntryActions(token string, ctxMap map[string]string) (map[string]string, error) {
	if ctxMap == nil {
		ctxMap = make(map[string]string)
	}
	for _, action := range s.EntryActions {
		var err error
		ctxMap, err = action.Execute(token, ctxMap)
		if err != nil {
			return ctxMap, err
		}
	}
	return ctxMap, nil
}

func (s *ActionState) ExecuteExitActions(token string, ctxMap map[string]string) (map[string]string, error) {
	if ctxMap == nil {
		ctxMap = make(map[string]string)
	}
	for _, action := range s.ExitActions {
		var err error
		ctxMap, err = action.Execute(token, ctxMap)
		if err != nil {
			return ctxMap, err
		}
	}
	return ctxMap, nil
}
