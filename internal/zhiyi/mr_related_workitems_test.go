package zhiyi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStripNullRelatedWorkItems(t *testing.T) {
	mr := map[string]any{
		"relatedWorkItems": nil,
		"workItemIds":      nil,
		"title":            "x",
	}
	if !StripNullRelatedWorkItems(mr) {
		t.Fatal("expected strip")
	}
	if _, ok := mr["relatedWorkItems"]; ok {
		t.Fatal("relatedWorkItems should be removed")
	}
	if _, ok := mr["workItemIds"]; ok {
		t.Fatal("workItemIds should be removed")
	}
	if mr["title"] != "x" {
		t.Fatal("other fields must stay")
	}
	// empty slice kept
	mr2 := map[string]any{"relatedWorkItems": []any{}}
	if StripNullRelatedWorkItems(mr2) {
		t.Fatal("empty slice is not null")
	}
	if _, ok := mr2["relatedWorkItems"]; !ok {
		t.Fatal("empty slice must remain")
	}
}

func TestStabilizeMergeRequest_StripsNullRelatedWorkItems(t *testing.T) {
	got := StabilizeMergeRequest(map[string]any{
		"localId":          float64(1),
		"status":           "UNDER_REVIEW",
		"relatedWorkItems": nil,
	})
	if _, ok := got["relatedWorkItems"]; ok {
		t.Fatalf("null relatedWorkItems must be stripped: %#v", got)
	}
}

func TestRelatedWorkItemsSummary(t *testing.T) {
	mr := map[string]any{
		"relatedWorkItems": []any{
			map[string]any{"id": "i1", "serialNumber": "ZYPT-1", "subject": "a"},
			map[string]any{"workItemId": "i1"}, // dedupe by id
			"bare-id",
		},
		"workItemIds": "i2, i3",
	}
	got := RelatedWorkItemsSummary(mr)
	b, _ := json.Marshal(got)
	s := string(b)
	for _, want := range []string{`"id":"i1"`, `"serialNumber":"ZYPT-1"`, `"id":"bare-id"`, `"id":"i2"`, `"id":"i3"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
	if RelatedWorkItemsSummary(map[string]any{"relatedWorkItems": nil}) != nil {
		t.Fatal("nil list -> nil summary")
	}
	if RelatedWorkItemsPresent(map[string]any{"relatedWorkItems": nil}) {
		t.Fatal("null is not present")
	}
	if !RelatedWorkItemsPresent(mr) {
		t.Fatal("expected present")
	}
}

func TestSummaryMergeRequest_IncludesRelatedWorkItems(t *testing.T) {
	mr := mrSummaryFixture()
	mr["relatedWorkItems"] = []any{map[string]any{"id": "wi", "serialNumber": "ZYPT-9"}}
	got := SummaryMergeRequest(mr)
	b, _ := json.Marshal(got)
	s := string(b)
	if !strings.Contains(s, `"relatedWorkItems":[{"id":"wi","serialNumber":"ZYPT-9"}]`) {
		t.Fatalf("summary missing relatedWorkItems: %s", s)
	}
	mr2 := mrSummaryFixture()
	mr2["relatedWorkItems"] = nil
	got2 := SummaryMergeRequest(mr2)
	b2, _ := json.Marshal(got2)
	if strings.Contains(string(b2), "relatedWorkItems") {
		t.Fatalf("null must not appear in summary: %s", b2)
	}
}

func TestAnnotateRelatedWorkItemsMeta(t *testing.T) {
	meta := AnnotateRelatedWorkItemsMeta(nil, map[string]any{"relatedWorkItems": nil})
	raw, ok := meta["related_work_items"].(map[string]any)
	if !ok {
		t.Fatalf("meta: %#v", meta)
	}
	if raw["reliable"] != false {
		t.Fatalf("reliable: %#v", raw["reliable"])
	}
	if raw["from_get"] != false {
		t.Fatalf("from_get: %#v", raw["from_get"])
	}
	verify, _ := raw["verify"].(string)
	if !strings.Contains(verify, "mrs link") {
		t.Fatalf("verify: %q", verify)
	}
	meta2 := AnnotateRelatedWorkItemsMeta(map[string]any{}, map[string]any{
		"relatedWorkItems": []any{map[string]any{"id": "x"}},
	})
	raw2 := meta2["related_work_items"].(map[string]any)
	if raw2["from_get"] != true || raw2["reliable"] != false {
		t.Fatalf("%#v", raw2)
	}
}
