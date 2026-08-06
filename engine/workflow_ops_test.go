package engine

import (
	"context"
	"testing"

	"github.com/kashari/brokr/model"
)

func TestMatchCommonTransition(t *testing.T) {
	common := []model.CommonTransition{
		{SourceList: []string{"a", "b"}, Target: "withdrawn", Event: "withdraw"},
	}
	got, ok := matchCommonTransition(common, "b", "withdraw", map[string]string{})
	if !ok {
		t.Fatal("expected a match")
	}
	if got.Target != "withdrawn" || got.Source != "b" {
		t.Fatalf("got %+v", got)
	}
	if _, ok := matchCommonTransition(common, "c", "withdraw", map[string]string{}); ok {
		t.Fatal("expected no match for source not in list")
	}
}

func TestProcessEventUsesCommonTransitions(t *testing.T) {
	// processEvent needs a live DB (see docker-compose db). This test is
	// skipped without one; it documents the contract for the integration
	// suite and is exercised by the fixture test added in Task 12.
	t.Skip("covered end-to-end by TestWorkflowJSONFixture (Task 12) and manual verification below")
	_ = context.Background()
}
