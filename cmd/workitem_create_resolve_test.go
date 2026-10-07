package cmd

import (
	"strings"
	"testing"
)

func TestWorkitemCreateResolvesPriorityDisplayValue(t *testing.T) {
	s := newWiCreateServer(t, wiFieldsFixture)
	r := runWiCreate(t, true, false, "--priority", "\u9ad8", "--custom-fields", `{"mod-1":"MES"}`)
	if r.code != 0 {
		t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
	}
	req := wiDryRunRequest(t, r.stdout)
	body, _ := req["body"].(map[string]any)
	cf, _ := body["customFieldValues"].(map[string]any)
	if cf["priority"] != "prio-high" || cf["mod-1"] != "m-a" {
		t.Fatalf("cf=%v body=%v", cf, body)
	}
	if s.fieldsGETs != 1 {
		t.Fatalf("fieldsGETs=%d want 1", s.fieldsGETs)
	}
}

func TestWorkitemCreateRejectsUnknownPriorityDisplay(t *testing.T) {
	s := newWiCreateServer(t, wiFieldsFixture)
	r := runWiCreate(t, true, false, "--priority", "no-such", "--custom-fields", `{"mod-1":"m-a"}`)
	if r.code != 1 {
		t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
	}
	eb := wiErrorBody(t, r.stderr)
	if eb.Subtype != "invalid_option_value" || !strings.Contains(eb.Message, "unknown value") || !strings.Contains(eb.Message, "prio-high") {
		t.Fatalf("error=%+v", eb)
	}
	if len(s.posts) != 0 {
		t.Fatalf("posts=%d", len(s.posts))
	}
}

func TestWorkitemCreatePriorityConflictsWithCustomFields(t *testing.T) {
	_ = newWiCreateServer(t, wiFieldsFixture)
	r := runWiCreate(t, true, false, "--priority", "prio-high", "--custom-fields", `{"priority":"prio-low","mod-1":"m-a"}`)
	if r.code != 1 {
		t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
	}
	eb := wiErrorBody(t, r.stderr)
	if !strings.Contains(eb.Message, "conflicts") {
		t.Fatalf("error=%+v", eb)
	}
}

func TestWorkitemCreateHelpDocumentsPriorityResolve(t *testing.T) {
	if f := workitemCreateCmd.Flags().Lookup("priority"); f == nil {
		t.Fatal("--priority flag missing")
	}
	for _, want := range []string{"display value", "--priority", "#126"} {
		if !strings.Contains(workitemCreateCmd.Long, want) {
			t.Fatalf("help missing %q", want)
		}
	}
}
