package brokr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kashari/brokr/model"
	"github.com/kashari/brokr/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// loadDefinitions reads every *.json file directly inside dir, parses
// each into a model.Workflow, and applies model.ApplyDefaults to it.
// Returns an error naming the offending file on the first parse or
// validation failure.
func loadDefinitions(dir string) ([]model.Workflow, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var defs []model.Workflow
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var wf model.Workflow
		if err := json.Unmarshal(data, &wf); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if wf.Id == "" {
			return nil, fmt.Errorf("%s: workflow definition is missing \"id\"", path)
		}
		if len(wf.States) == 0 {
			return nil, fmt.Errorf("%s: workflow definition has no \"states\"", path)
		}
		model.ApplyDefaults(&wf)
		defs = append(defs, wf)
	}
	return defs, nil
}

// upsert registers wf under its Id, replacing any existing definition of
// the same name.
func upsert(db *gorm.DB, wf model.Workflow) error {
	rec := persistence.WorkflowDefinition{Name: wf.Id, Definition: wf}
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"definition", "updated_at"}),
	}).Create(&rec)
	return result.Error
}

// SeedDefinitions registers every *.json workflow definition directly inside
// dir, replacing any already registered under the same name, and returns how
// many were registered. Nothing is written if any file fails to parse or
// validate, so a bad file cannot leave a half-seeded database.
func SeedDefinitions(db *gorm.DB, dir string) (int, error) {
	if db == nil {
		return 0, errors.New("brokr: SeedDefinitions needs a database handle")
	}
	defs, err := loadDefinitions(dir)
	if err != nil {
		return 0, err
	}
	for _, wf := range defs {
		if err := upsert(db, wf); err != nil {
			return 0, fmt.Errorf("upsert %q: %w", wf.Id, err)
		}
	}
	return len(defs), nil
}
