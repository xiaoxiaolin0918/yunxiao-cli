package profile

import (
	"strings"
	"testing"
)

func TestResolvePriorityID_RequiresMapForAlias(t *testing.T) {
	p := &Profile{}
	if _, err := p.ResolvePriorityID("urgent"); err == nil {
		t.Fatal("expected error for unmapped urgent")
	}
	p.BugCreateFields.Priority = map[string]string{"urgent": "opt-urgent"}
	got, err := p.ResolvePriorityID("urgent")
	if err != nil || got != "opt-urgent" {
		t.Fatalf("got %q err %v", got, err)
	}
	got, err = p.ResolvePriorityID("49d15833fdb4a68e930be485d1")
	if err != nil || got != "49d15833fdb4a68e930be485d1" {
		t.Fatalf("passthrough: %q %v", got, err)
	}
}

func TestResolveSeriousLevelID_RequiresMapForAlias(t *testing.T) {
	p := &Profile{}
	_, err := p.ResolveSeriousLevelID("fatal")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "serious_level") {
		t.Fatalf("hint missing: %s", err.Error())
	}
	p.BugCreateFields.SeriousLevel = map[string]string{"slight": "opt-slight", "serious": "opt-serious"}
	got, err := p.ResolveSeriousLevelID("slight")
	if err != nil || got != "opt-slight" {
		t.Fatalf("got %q err %v", got, err)
	}
}
