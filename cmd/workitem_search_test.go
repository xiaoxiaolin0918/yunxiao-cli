package cmd

import (
	"encoding/json"
	"testing"
)

func TestAppendDateRangeFilterBothSides(t *testing.T) {
	filters := appendDateRangeFilter(nil, "gmtCreate", "2026-09-01 00:00:00", "2026-09-07 23:59:59")
	if len(filters) != 1 {
		t.Fatalf("len=%d", len(filters))
	}
	m, ok := filters[0].(map[string]any)
	if !ok {
		t.Fatalf("type %T", filters[0])
	}
	if m["className"] != "dateTime" || m["format"] != "input" || m["operator"] != "BETWEEN" {
		t.Fatalf("shape=%v", m)
	}
	if m["fieldIdentifier"] != "gmtCreate" {
		t.Fatalf("field=%v", m["fieldIdentifier"])
	}
	vals, ok := m["value"].([]string)
	if !ok || len(vals) != 1 || vals[0] != "2026-09-01 00:00:00" {
		t.Fatalf("value=%v", m["value"])
	}
	if m["toValue"] != "2026-09-07 23:59:59" {
		t.Fatalf("toValue=%v", m["toValue"])
	}
}

func TestAppendDateRangeFilterAfterOnly(t *testing.T) {
	filters := appendDateRangeFilter(nil, "gmtModified", "2026-09-01 00:00:00", "")
	m := filters[0].(map[string]any)
	if m["toValue"] != dateRangeOpenEnd {
		t.Fatalf("toValue=%v want open end", m["toValue"])
	}
	vals := m["value"].([]string)
	if vals[0] != "2026-09-01 00:00:00" {
		t.Fatalf("value=%v", vals)
	}
}

func TestAppendDateRangeFilterBeforeOnly(t *testing.T) {
	filters := appendDateRangeFilter(nil, "finishTime", "", "2026-09-07 23:59:59")
	m := filters[0].(map[string]any)
	vals := m["value"].([]string)
	if vals[0] != dateRangeOpenStart {
		t.Fatalf("value=%v want open start", vals)
	}
	if m["toValue"] != "2026-09-07 23:59:59" {
		t.Fatalf("toValue=%v", m["toValue"])
	}
	if m["fieldIdentifier"] != "finishTime" {
		t.Fatalf("field=%v", m["fieldIdentifier"])
	}
}

func TestAppendDateRangeFilterEmptyNoop(t *testing.T) {
	base := []any{map[string]any{"x": 1}}
	out := appendDateRangeFilter(base, "gmtCreate", "", "")
	if len(out) != 1 {
		t.Fatalf("len=%d", len(out))
	}
}

func TestWorkitemSearchConditionsJSONDateWindows(t *testing.T) {
	var filters []any
	filters = appendUserFilter(filters, "assignedTo", "uid-1")
	filters = appendDateRangeFilter(filters, "gmtCreate", "2026-09-01 00:00:00", "2026-09-07 23:59:59")
	filters = appendDateRangeFilter(filters, "gmtModified", "2026-08-01 00:00:00", "")
	filters = appendDateRangeFilter(filters, "finishTime", "", "2026-09-30 23:59:59")
	conds := map[string]any{"conditionGroups": []any{filters}}
	raw, err := json.Marshal(conds)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	groups, ok := parsed["conditionGroups"].([]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("groups=%v", parsed["conditionGroups"])
	}
	group, ok := groups[0].([]any)
	if !ok || len(group) != 4 {
		t.Fatalf("group len=%v raw=%s", group, raw)
	}
	// Spot-check finishTime BETWEEN open-start shape in JSON.
	foundFinish := false
	for _, item := range group {
		m, _ := item.(map[string]any)
		if m["fieldIdentifier"] == "finishTime" {
			foundFinish = true
			if m["operator"] != "BETWEEN" || m["className"] != "dateTime" {
				t.Fatalf("finish filter=%v", m)
			}
			vals, _ := m["value"].([]any)
			if len(vals) != 1 || vals[0] != dateRangeOpenStart {
				t.Fatalf("finish value=%v", m["value"])
			}
			if m["toValue"] != "2026-09-30 23:59:59" {
				t.Fatalf("finish toValue=%v", m["toValue"])
			}
		}
	}
	if !foundFinish {
		t.Fatal("finishTime filter missing")
	}
}

func TestAppendStatusFilterAndStatusStage(t *testing.T) {
	var filters []any
	filters = appendStatusFilter(filters, "100005,100010")
	filters = appendStatusStageFilter(filters, "1,2")
	if len(filters) != 2 {
		t.Fatalf("len=%d", len(filters))
	}
	st := filters[0].(map[string]any)
	if st["className"] != "status" || st["fieldIdentifier"] != "status" || st["format"] != "list" || st["operator"] != "CONTAINS" {
		t.Fatalf("status shape=%v", st)
	}
	vals, ok := st["value"].([]string)
	if !ok || len(vals) != 2 || vals[0] != "100005" || vals[1] != "100010" {
		t.Fatalf("status value=%v", st["value"])
	}
	ss := filters[1].(map[string]any)
	if ss["className"] != "statusStage" || ss["fieldIdentifier"] != "statusStage" {
		t.Fatalf("statusStage shape=%v", ss)
	}
	svals := ss["value"].([]string)
	if len(svals) != 2 || svals[0] != "1" || svals[1] != "2" {
		t.Fatalf("statusStage value=%v", ss["value"])
	}
}

func TestBuildWorkitemSearchFiltersStatusLandsInConditionsJSON(t *testing.T) {
	filters := buildWorkitemSearchFilters(workitemSearchFilterInput{
		Status:        "28,30",
		StatusStage:   "1",
		CreatedAfter:  "2026-09-01 00:00:00",
		CreatedBefore: "2026-09-07 23:59:59",
	})
	conds := map[string]any{"conditionGroups": []any{filters}}
	raw, err := json.Marshal(conds)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	group := parsed["conditionGroups"].([]any)[0].([]any)
	foundStatus, foundStage, foundCreate := false, false, false
	for _, item := range group {
		m := item.(map[string]any)
		switch m["fieldIdentifier"] {
		case "status":
			foundStatus = true
			if m["className"] != "status" || m["operator"] != "CONTAINS" {
				t.Fatalf("status=%v", m)
			}
		case "statusStage":
			foundStage = true
			if m["className"] != "statusStage" {
				t.Fatalf("stage=%v", m)
			}
		case "gmtCreate":
			foundCreate = true
		}
	}
	if !foundStatus || !foundStage || !foundCreate {
		t.Fatalf("missing filters status=%v stage=%v create=%v raw=%s", foundStatus, foundStage, foundCreate, raw)
	}
}

func TestAppendStatusFilterEmptyNoop(t *testing.T) {
	base := []any{map[string]any{"x": 1}}
	if len(appendStatusFilter(base, "")) != 1 {
		t.Fatal("status noop")
	}
	if len(appendStatusStageFilter(base, "")) != 1 {
		t.Fatal("stage noop")
	}
}


func TestAppendLabelsFilter(t *testing.T) {
	filters := appendLabelsFilter(nil, "lab-a,lab-b")
	if len(filters) != 1 {
		t.Fatalf("len=%d", len(filters))
	}
	m := filters[0].(map[string]any)
	if m["className"] != "tag" || m["fieldIdentifier"] != "tag" || m["format"] != "multiList" || m["operator"] != "CONTAINS" {
		t.Fatalf("shape=%v", m)
	}
	vals := m["value"].([]string)
	if len(vals) != 2 || vals[0] != "lab-a" || vals[1] != "lab-b" {
		t.Fatalf("value=%v", vals)
	}
	base := []any{map[string]any{"x": 1}}
	if len(appendLabelsFilter(base, "")) != 1 {
		t.Fatal("empty noop")
	}
	built := buildWorkitemSearchFilters(workitemSearchFilterInput{Labels: "lab-1"})
	found := false
	for _, item := range built {
		mm := item.(map[string]any)
		if mm["fieldIdentifier"] == "tag" {
			found = true
		}
	}
	if !found {
		t.Fatalf("labels missing in build filters: %#v", built)
	}
}
