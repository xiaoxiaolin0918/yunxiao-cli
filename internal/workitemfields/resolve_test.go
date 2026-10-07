package workitemfields

import (
	"strings"
	"testing"
)

func TestResolveOptionIDByDisplayAndID(t *testing.T) {
	fields := parse(t, config)
	var priority Field
	for _, f := range fields {
		if f.ID == "priority" {
			priority = f
			break
		}
	}
	high := "\u9ad8"
	id, err := ResolveOptionID(priority, high)
	if err != nil || id != "prio-high" {
		t.Fatalf("display: id=%q err=%v", id, err)
	}
	id, err = ResolveOptionID(priority, "prio-low")
	if err != nil || id != "prio-low" {
		t.Fatalf("id passthrough: id=%q err=%v", id, err)
	}
	_, err = ResolveOptionID(priority, "missing-value")
	if err == nil || !strings.Contains(err.Error(), "unknown value") || !strings.Contains(err.Error(), "prio-high") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveCustomFieldValues(t *testing.T) {
	fields := parse(t, config)
	cf := map[string]any{"priority": "\u9ad8", "mod-1": "MES", "note": "keep"}
	if err := ResolveCustomFieldValues(fields, cf); err != nil {
		t.Fatal(err)
	}
	if cf["priority"] != "prio-high" || cf["mod-1"] != "m-a" || cf["note"] != "keep" {
		t.Fatalf("cf=%v", cf)
	}
}

func TestResolveCustomFieldValuesMultiList(t *testing.T) {
	f := Field{ID: "tags", Name: "tags", Format: "multiList", Options: []Option{
		{ID: "t1", DisplayValue: "A", Value: "A"},
		{ID: "t2", DisplayValue: "B", Value: "B"},
	}}
	cf := map[string]any{"tags": []any{"A", "t2"}}
	if err := ResolveCustomFieldValues([]Field{f}, cf); err != nil {
		t.Fatal(err)
	}
	got := cf["tags"].([]any)
	if got[0] != "t1" || got[1] != "t2" {
		t.Fatalf("got=%v", got)
	}
}
