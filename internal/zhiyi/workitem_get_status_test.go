package zhiyi

import (
	"errors"
	"testing"
)

func TestWorkItemStatusDisplayable(t *testing.T) {
	cases := []struct {
		name string
		item map[string]any
		want bool
	}{
		{"nil item", nil, false},
		{"null status", map[string]any{"status": nil}, false},
		{"missing status", map[string]any{"id": "1"}, false},
		{"id only", map[string]any{"status": map[string]any{"id": "100005"}}, false},
		{"displayName", map[string]any{"status": map[string]any{"id": "100005", "displayName": "doing"}}, true},
		{"name fallback", map[string]any{"status": map[string]any{"id": "100005", "name": "doing"}}, true},
		{"string status id", map[string]any{"status": "100005"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WorkItemStatusDisplayable(tc.item); got != tc.want {
				t.Fatalf("WorkItemStatusDisplayable = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEnsureWorkItemGetStatus_MergesFromRefresh(t *testing.T) {
	item := map[string]any{
		"id":           "wi-6045",
		"serialNumber": "ZYPT-6045",
		"subject":      "x",
		"status":       nil,
	}
	got, refreshed, failed := EnsureWorkItemGetStatus(item, func() (map[string]any, error) {
		return map[string]any{
			"id":           "wi-6045",
			"serialNumber": "ZYPT-6045",
			"status": map[string]any{
				"id":          "100005",
				"name":        "doing",
				"displayName": "doing",
			},
		}, nil
	})
	if !refreshed || failed {
		t.Fatalf("refreshed=%v failed=%v", refreshed, failed)
	}
	if !WorkItemStatusDisplayable(got) {
		t.Fatalf("status not displayable: %#v", got["status"])
	}
	st := StatusBrief(got)
	if st["id"] != "100005" || st["displayName"] != "doing" {
		t.Fatalf("status brief=%#v", st)
	}
	if item["status"] == nil && got["status"] == nil {
		t.Fatal("expected status merged onto result")
	}
}

func TestEnsureWorkItemGetStatus_SkipsWhenAlreadyDisplayable(t *testing.T) {
	item := map[string]any{
		"status": map[string]any{"id": "1", "displayName": "Open"},
	}
	got, refreshed, failed := EnsureWorkItemGetStatus(item, func() (map[string]any, error) {
		t.Fatal("refresh must not run")
		return nil, nil
	})
	if refreshed || failed || got["status"] == nil {
		t.Fatalf("refreshed=%v failed=%v status=%#v", refreshed, failed, got["status"])
	}
}

func TestEnsureWorkItemGetStatus_MarksFailedOnRefreshError(t *testing.T) {
	item := map[string]any{"id": "wi-1", "status": nil}
	got, refreshed, failed := EnsureWorkItemGetStatus(item, func() (map[string]any, error) {
		return nil, errors.New("search boom")
	})
	if refreshed || !failed {
		t.Fatalf("refreshed=%v failed=%v", refreshed, failed)
	}
	if WorkItemStatusDisplayable(got) {
		t.Fatalf("should stay non-displayable: %#v", got["status"])
	}
}

func TestEnsureWorkItemGetStatus_MarksFailedWhenRefreshAlsoLacksStatus(t *testing.T) {
	item := map[string]any{"id": "wi-1", "status": nil}
	got, refreshed, failed := EnsureWorkItemGetStatus(item, func() (map[string]any, error) {
		return map[string]any{"id": "wi-1", "status": nil}, nil
	})
	if refreshed || !failed {
		t.Fatalf("refreshed=%v failed=%v got=%#v", refreshed, failed, got)
	}
}

func TestNormalizeWorkItemStatus_CoercesStringID(t *testing.T) {
	item := map[string]any{"status": "100005"}
	NormalizeWorkItemStatus(item)
	st, _ := item["status"].(map[string]any)
	if st == nil || st["id"] != "100005" {
		t.Fatalf("status=%#v", item["status"])
	}
}

func TestMergeWorkItemStatus_CopiesDisplayable(t *testing.T) {
	dst := map[string]any{"id": "a", "status": nil}
	src := map[string]any{"status": map[string]any{"id": "9", "displayName": "Done"}}
	if !MergeWorkItemStatus(dst, src) {
		t.Fatal("expected merge")
	}
	st, _ := dst["status"].(map[string]any)
	if st["displayName"] != "Done" {
		t.Fatalf("status=%#v", st)
	}
}