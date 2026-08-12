package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kashari/brokr/model"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDefinitionsAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "minimal.json", `{
		"id": "minimal",
		"states": [
			{"type": "SimpleState", "id": "start"},
			{"type": "SimpleState", "id": "end"}
		],
		"transitions": [
			{"source": "start", "target": "end", "event": "go"}
		]
	}`)

	defs, err := loadDefinitions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 {
		t.Fatalf("got %d definitions, want 1", len(defs))
	}
	wf := defs[0]
	if wf.Kind != model.UserWorkflowKind {
		t.Fatalf("Kind = %q, want %q", wf.Kind, model.UserWorkflowKind)
	}
	if wf.Version == "" {
		t.Fatal("Version was not defaulted")
	}
	if wf.Transitions[0].Trigger != model.UserTrigger {
		t.Fatalf("Trigger = %q, want %q", wf.Transitions[0].Trigger, model.UserTrigger)
	}
}

func TestLoadDefinitionsRejectsMissingId(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.json", `{"states": [{"type": "SimpleState", "id": "start"}]}`)

	if _, err := loadDefinitions(dir); err == nil {
		t.Fatal("expected an error for a definition missing \"id\"")
	}
}

func TestLoadDefinitionsRejectsEmptyStates(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.json", `{"id": "empty", "states": []}`)

	if _, err := loadDefinitions(dir); err == nil {
		t.Fatal("expected an error for a definition with no states")
	}
}

func TestLoadDefinitionsIgnoresNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "minimal.json", `{
		"id": "minimal",
		"states": [{"type": "SimpleState", "id": "start"}]
	}`)
	writeFile(t, dir, "README.md", "not a workflow")

	defs, err := loadDefinitions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 {
		t.Fatalf("got %d definitions, want 1", len(defs))
	}
}

func TestUpsertRequiresLiveDB(t *testing.T) {
	// upsert (and therefore Run) hits config.Db and has no automated
	// coverage, for the same reason engine.NewWorkflowInstanceByName
	// doesn't — see engine/workflow_ops_test.go. Manually verified via
	// `go run ./cmd/seed-workflows` against the local docker-compose db.
	t.Skip("no DB-backed coverage: see comment above")
}
