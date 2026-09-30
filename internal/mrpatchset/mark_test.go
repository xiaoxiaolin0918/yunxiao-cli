package mrpatchset

import "testing"

func TestMarkLatestFlagsExactlyOneSourceItem(t *testing.T) {
	out := decode(t, unordered)
	ps, ok := MarkLatest(out)
	if !ok || ps.BizID != "ps-v3" {
		t.Fatalf("got %+v ok=%v", ps, ok)
	}
	want := map[string]bool{"ps-target": false, "ps-v2": false, "ps-v3": true, "ps-v1": false}
	for i, it := range out.([]any) {
		m := it.(map[string]any)
		id := m["patchSetBizId"].(string)
		if m["latest"] != want[id] {
			t.Fatalf("item %d (%s) latest=%v", i, id, m["latest"])
		}
		// existing fields untouched
		if _, ok := m["createTime"]; !ok {
			t.Fatalf("item %d lost createTime", i)
		}
	}
	// order unchanged
	if out.([]any)[0].(map[string]any)["patchSetBizId"] != "ps-target" {
		t.Fatal("MarkLatest must not reorder items")
	}
}

func TestMarkLatestWrapped(t *testing.T) {
	out := decode(t, `{"result":[{"patchSetBizId":"a","versionNo":1,"relatedMergeItemType":"MERGE_SOURCE"},{"patchSetBizId":"b","versionNo":2,"relatedMergeItemType":"MERGE_SOURCE"}]}`)
	ps, ok := MarkLatest(out)
	if !ok || ps.BizID != "b" {
		t.Fatalf("got %+v ok=%v", ps, ok)
	}
	items := out.(map[string]any)["result"].([]any)
	if items[0].(map[string]any)["latest"] != false || items[1].(map[string]any)["latest"] != true {
		t.Fatalf("items=%v", items)
	}
}

func TestMarkLatestNoCandidate(t *testing.T) {
	out := decode(t, `[{"patchSetBizId":"t","versionNo":1,"relatedMergeItemType":"MERGE_TARGET"}]`)
	if _, ok := MarkLatest(out); ok {
		t.Fatal("target-only must not report a latest")
	}
	if out.([]any)[0].(map[string]any)["latest"] != false {
		t.Fatal("items should still carry latest=false")
	}
	if _, ok := MarkLatest(decode(t, `[]`)); ok {
		t.Fatal("empty must not report a latest")
	}
	if _, ok := MarkLatest(decode(t, `{"foo":1}`)); ok {
		t.Fatal("non-list must not report a latest")
	}
}

// Same candidate rule as comments create (m2 on #101): untyped items are not a
// fallback once the payload carries MERGE_TARGET entries.
func TestMarkLatestUntypedWithTargetHasNoLatest(t *testing.T) {
	out := decode(t, `[{"patchSetBizId":"u","versionNo":2},{"patchSetBizId":"t","versionNo":1,"relatedMergeItemType":"MERGE_TARGET"}]`)
	if ps, ok := MarkLatest(out); ok {
		t.Fatalf("got %+v, want no latest", ps)
	}
	for _, it := range out.([]any) {
		if it.(map[string]any)["latest"] != false {
			t.Fatalf("item %v should be latest=false", it)
		}
	}
}

func TestMarkLatestUntypedOnlyPayload(t *testing.T) {
	out := decode(t, `[{"patchSetBizId":"u1","versionNo":1},{"patchSetBizId":"u2","versionNo":2}]`)
	if ps, ok := MarkLatest(out); !ok || ps.BizID != "u2" {
		t.Fatalf("got %+v ok=%v", ps, ok)
	}
}

func TestMarkLatestOverwritesExistingLatestField(t *testing.T) {
	out := decode(t, `[{"patchSetBizId":"a","versionNo":1,"relatedMergeItemType":"MERGE_SOURCE","latest":true},{"patchSetBizId":"b","versionNo":2,"relatedMergeItemType":"MERGE_SOURCE","latest":"x"}]`)
	if _, ok := MarkLatest(out); !ok {
		t.Fatal("expected a latest patch set")
	}
	items := out.([]any)
	if items[0].(map[string]any)["latest"] != false || items[1].(map[string]any)["latest"] != true {
		t.Fatalf("items=%v", items)
	}
}
