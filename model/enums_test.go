package model

import (
	"encoding/json"
	"testing"
)

func TestTransitionKindCaseInsensitiveUnmarshal(t *testing.T) {
	data := []byte(`{"kind":"internal","source":"a","target":"b","event":"e"}`)
	var tr Transition
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	if tr.Kind != InternalKind {
		t.Fatalf("Kind = %q, want %q", tr.Kind, InternalKind)
	}
}

func TestActionTypeCaseInsensitiveUnmarshal(t *testing.T) {
	data := []byte(`{"type":"httprequestaction","method":"GET","url":"http://x"}`)
	var a Action
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatal(err)
	}
	if a.Type != HttpRequestAction {
		t.Fatalf("Type = %q, want %q", a.Type, HttpRequestAction)
	}
}

func TestTransitionTriggerAlreadyUppercaseStillWorks(t *testing.T) {
	data := []byte(`{"type":"AUTOMATIC","source":"a","target":"b","event":"e"}`)
	var tr Transition
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	if tr.Trigger != AutomaticTrigger {
		t.Fatalf("Trigger = %q, want %q", tr.Trigger, AutomaticTrigger)
	}
}

func TestUnknownTransitionKindRejected(t *testing.T) {
	data := []byte(`{"kind":"bogus","source":"a","target":"b","event":"e"}`)
	var tr Transition
	if err := json.Unmarshal(data, &tr); err == nil {
		t.Fatal("expected an error for an unrecognized transition kind")
	}
}
