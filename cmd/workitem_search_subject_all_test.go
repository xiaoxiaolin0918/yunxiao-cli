package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildWorkitemSearchFiltersSubjectAllAND(t *testing.T) {
	filters := buildWorkitemSearchFilters(workitemSearchFilterInput{
		SubjectAll: "清空,筛选",
	})
	var subjects []string
	for _, item := range filters {
		m := item.(map[string]any)
		if m["fieldIdentifier"] != "subject" {
			continue
		}
		if m["operator"] != "CONTAINS" || m["className"] != "string" {
			t.Fatalf("subject shape=%v", m)
		}
		vals, ok := m["value"].([]string)
		if !ok || len(vals) != 1 {
			t.Fatalf("value=%v", m["value"])
		}
		subjects = append(subjects, vals[0])
	}
	if len(subjects) != 2 || subjects[0] != "清空" || subjects[1] != "筛选" {
		t.Fatalf("subjects=%v want [清空 筛选]", subjects)
	}
}

func TestBuildWorkitemSearchFiltersSubjectAndSubjectAllCombine(t *testing.T) {
	filters := buildWorkitemSearchFilters(workitemSearchFilterInput{
		Subject:    "打印",
		SubjectAll: "销售员,导出",
	})
	var subjects []string
	for _, item := range filters {
		m := item.(map[string]any)
		if m["fieldIdentifier"] == "subject" {
			subjects = append(subjects, m["value"].([]string)[0])
		}
	}
	if len(subjects) != 3 {
		t.Fatalf("subjects=%v want 3 AND terms", subjects)
	}
	want := map[string]bool{"打印": true, "销售员": true, "导出": true}
	for _, s := range subjects {
		if !want[s] {
			t.Fatalf("unexpected subject %q in %v", s, subjects)
		}
	}
}

func TestBuildWorkitemSearchFiltersSubjectAllChineseComma(t *testing.T) {
	filters := buildWorkitemSearchFilters(workitemSearchFilterInput{
		SubjectAll: "清空，筛选",
	})
	n := 0
	for _, item := range filters {
		m := item.(map[string]any)
		if m["fieldIdentifier"] == "subject" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("chinese comma should split into 2 filters, got %d: %#v", n, filters)
	}
}

func TestBuildWorkitemSearchFiltersSubjectAllEmptyNoop(t *testing.T) {
	filters := buildWorkitemSearchFilters(workitemSearchFilterInput{SubjectAll: " , , "})
	for _, item := range filters {
		m := item.(map[string]any)
		if m["fieldIdentifier"] == "subject" {
			t.Fatalf("empty tokens should not add subject filter: %#v", filters)
		}
	}
}

func TestRecentDaysUpdatedAfterShanghai(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 18, 14, 30, 0, 0, loc)
	got, err := recentDaysUpdatedAfter(30, now)
	if err != nil {
		t.Fatal(err)
	}
	if got != "2026-09-18 00:00:00" {
		t.Fatalf("got %q want 2026-09-18 00:00:00", got)
	}
}

func TestRecentDaysUpdatedAfterRejectsNonPositive(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	if _, err := recentDaysUpdatedAfter(0, now); err == nil {
		t.Fatal("want error for 0")
	}
	if _, err := recentDaysUpdatedAfter(-1, now); err == nil {
		t.Fatal("want error for negative")
	}
}

func TestResolveWorkitemSearchUpdatedAfterRecentDays(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 10, 18, 14, 30, 0, 0, loc)
	got, err := resolveWorkitemSearchUpdatedAfter("", 7, now)
	if err != nil {
		t.Fatal(err)
	}
	if got != "2026-10-11 00:00:00" {
		t.Fatalf("got %q", got)
	}
	got, err = resolveWorkitemSearchUpdatedAfter("2026-01-01 00:00:00", 0, now)
	if err != nil || got != "2026-01-01 00:00:00" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	_, err = resolveWorkitemSearchUpdatedAfter("2026-01-01 00:00:00", 7, now)
	if err == nil || !strings.Contains(err.Error(), "recent-days") {
		t.Fatalf("want conflict err, got %v", err)
	}
}

func TestBuildWorkitemSearchFiltersSubjectAllInConditionsJSON(t *testing.T) {
	filters := buildWorkitemSearchFilters(workitemSearchFilterInput{
		SubjectAll:   "清空,筛选",
		UpdatedAfter: "2026-09-18 00:00:00",
	})
	conds := map[string]any{"conditionGroups": []any{filters}}
	raw, err := json.Marshal(conds)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Count(s, `"fieldIdentifier":"subject"`) != 2 {
		t.Fatalf("want 2 subject filters: %s", s)
	}
	if !strings.Contains(s, "清空") || !strings.Contains(s, "筛选") {
		t.Fatalf("missing terms: %s", s)
	}
	if !strings.Contains(s, `"fieldIdentifier":"gmtModified"`) || !strings.Contains(s, "2026-09-18 00:00:00") {
		t.Fatalf("missing gmtModified window: %s", s)
	}
}
