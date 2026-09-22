package zhiyi

import (
	"encoding/json"
	"testing"
)

func TestBriefWorkItem_KeepsSerialNumberAndStatusDisplayName(t *testing.T) {
	item := map[string]any{
		"id":           "4b2469777047242f49ee90918c",
		"serialNumber": "ZYPT-9001",
		"subject":      "deploy task",
		"description":  "<p>huge html noise</p>",
		"status": map[string]any{
			"id":          "100005",
			"name":        "待处理",
			"displayName": "待处理",
			"nameEn":      "Pending",
		},
		"customFieldValues": []any{map[string]any{"fieldId": "x", "value": "y"}},
	}
	brief := BriefWorkItem(item)
	if brief["serialNumber"] != "ZYPT-9001" {
		t.Fatalf("serialNumber=%v", brief["serialNumber"])
	}
	st, _ := brief["status"].(map[string]any)
	if st == nil || st["displayName"] != "待处理" {
		t.Fatalf("status=%v", brief["status"])
	}
	if _, ok := brief["description"]; ok {
		t.Fatalf("description should be trimmed from brief: %#v", brief)
	}
	if _, ok := brief["customFieldValues"]; ok {
		t.Fatalf("customFieldValues should be trimmed: %#v", brief)
	}
	if brief["id"] != "4b2469777047242f49ee90918c" {
		t.Fatalf("id=%v", brief["id"])
	}
	if brief["subject"] != "deploy task" {
		t.Fatalf("subject=%v", brief["subject"])
	}
}

func TestEnsureWorkItemCreateFields_RefreshesNullSerialAndStatus(t *testing.T) {
	created := map[string]any{
		"id":           "4b2469777047242f49ee90918c",
		"serialNumber": nil,
		"status":       nil,
		"subject":      "deploy task",
		"description":  "noise",
	}
	fetched := false
	got := EnsureWorkItemCreateFields(created, func(id string) (map[string]any, error) {
		fetched = true
		if id != "4b2469777047242f49ee90918c" {
			t.Fatalf("id=%s", id)
		}
		return map[string]any{
			"id":           id,
			"serialNumber": "ZYPT-9001",
			"subject":      "deploy task",
			"description":  "noise",
			"status": map[string]any{
				"id":          "100005",
				"displayName": "待处理",
			},
		}, nil
	})
	if !fetched {
		t.Fatal("expected refresh GET")
	}
	brief := BriefWorkItem(got)
	if brief["serialNumber"] != "ZYPT-9001" {
		t.Fatalf("after refresh serial=%v", brief["serialNumber"])
	}
	st, _ := brief["status"].(map[string]any)
	if st["displayName"] != "待处理" {
		t.Fatalf("status=%v", st)
	}
	// Round-trip JSON like CLI envelope data
	raw, _ := json.Marshal(brief)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	if m["serialNumber"] == nil {
		t.Fatalf("json serialNumber null: %s", raw)
	}
	if m["status"] == nil {
		t.Fatalf("json status null: %s", raw)
	}
}

func TestEnsureWorkItemCreateFields_SkipsFetchWhenComplete(t *testing.T) {
	created := map[string]any{
		"id":           "abc",
		"serialNumber": "ZYPT-1",
		"status":       map[string]any{"id": "1", "displayName": "Open"},
	}
	got := EnsureWorkItemCreateFields(created, func(id string) (map[string]any, error) {
		t.Fatal("must not fetch")
		return nil, nil
	})
	if SerialNumber(got) != "ZYPT-1" {
		t.Fatal(got)
	}
}
