package brokr

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

func TestSeedDefinitionsRejectsNilDB(t *testing.T) {
	if _, err := SeedDefinitions(nil, t.TempDir()); err == nil {
		t.Fatal("SeedDefinitions(nil, dir): got nil error, want one")
	}
}

func TestSeedDefinitionsReportsAWriteFailure(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "wf.json", `{"id":"wf","states":[{"id":"start"}]}`)

	// stubDB's queries never reach a server, so this exercises the path where
	// the definitions parse but the upsert fails — the count must not be
	// reported as a success.
	n, err := SeedDefinitions(stubDB(), dir)
	if err == nil {
		t.Fatal("SeedDefinitions against an unreachable database: got nil error, want one")
	}
	if n != 0 {
		t.Fatalf("registered count = %d, want 0 when the upsert failed", n)
	}
}

func TestSeedDefinitionsRejectsABadFileBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.json", `{"id":"wf","states":[}`)

	if _, err := SeedDefinitions(stubDB(), dir); err == nil {
		t.Fatal("SeedDefinitions with unparseable JSON: got nil error, want one")
	}
}
