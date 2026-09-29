package mrpatchset

import (
	"encoding/json"
	"errors"
	"testing"
)

func decode(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// Unordered list as returned by GET .../diffs/patches; MERGE_TARGET has the
// newest createTime but must never be picked for comments (#93/#94).
const unordered = `[
 {"patchSetBizId":"ps-target","versionNo":3,"relatedMergeItemType":"MERGE_TARGET","createTime":"2026-09-29T10:00:00Z"},
 {"patchSetBizId":"ps-v2","versionNo":2,"relatedMergeItemType":"MERGE_SOURCE","createTime":"2026-09-28T10:00:00Z"},
 {"patchSetBizId":"ps-v3","versionNo":3,"relatedMergeItemType":"MERGE_SOURCE","createTime":"2026-09-29T09:00:00Z","patchSetName":"版本3","commitId":"abc"},
 {"patchSetBizId":"ps-v1","versionNo":1,"relatedMergeItemType":"MERGE_SOURCE","createTime":"2026-09-27T10:00:00Z"}
]`

func TestLatestPicksHighestSourceVersion(t *testing.T) {
	sets, err := Parse(decode(t, unordered))
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 4 {
		t.Fatalf("parsed %d sets", len(sets))
	}
	got, err := Latest(sets)
	if err != nil {
		t.Fatal(err)
	}
	if got.BizID != "ps-v3" || got.VersionNo != 3 || got.Index != 2 || got.Name != "版本3" || got.CommitID != "abc" {
		t.Fatalf("got %+v", got)
	}
}

func TestLatestTieBreaksOnCreateTime(t *testing.T) {
	sets, err := Parse(decode(t, `[
	 {"patchSetBizId":"a","versionNo":2,"relatedMergeItemType":"MERGE_SOURCE","createTime":"2026-09-29T09:00:00Z"},
	 {"patchSetBizId":"b","versionNo":2,"relatedMergeItemType":"MERGE_SOURCE","createTime":"2026-09-29T11:00:00+08:00"},
	 {"patchSetBizId":"c","versionNo":2,"relatedMergeItemType":"MERGE_SOURCE","createTime":"2026-09-29T08:00:00Z"}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Latest(sets)
	if err != nil || got.BizID != "a" {
		// b is 03:00Z, earlier than a (09:00Z)
		t.Fatalf("got %+v err=%v", got, err)
	}
}

func TestLatestLegacyFieldsAndWrapper(t *testing.T) {
	// Older API shape: {result:[...]} with patchSetNo / createdAt (local time).
	sets, err := Parse(decode(t, `{"result":[
	 {"patchSetBizId":"old","patchSetNo":"1","relatedMergeItemType":"MERGE_SOURCE","createdAt":"2022-03-18 14:24:54"},
	 {"patchSetBizId":"new","patchSetNo":"2","relatedMergeItemType":"MERGE_SOURCE","createdAt":"2022-03-19 09:00:00"}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Latest(sets)
	if err != nil || got.BizID != "new" || got.VersionNo != 2 {
		t.Fatalf("got %+v err=%v", got, err)
	}
}

func TestLatestFallsBackToCreateTimeWithoutVersion(t *testing.T) {
	sets, _ := Parse(decode(t, `[
	 {"patchSetBizId":"x","relatedMergeItemType":"MERGE_SOURCE","createTime":"2026-09-29T09:00:00Z"},
	 {"patchSetBizId":"y","relatedMergeItemType":"MERGE_SOURCE","createTime":"2026-09-29T10:00:00Z"}
	]`))
	got, err := Latest(sets)
	if err != nil || got.BizID != "y" {
		t.Fatalf("got %+v err=%v", got, err)
	}
}

func TestLatestUntypedItemsWhenNoSourceTyped(t *testing.T) {
	sets, _ := Parse(decode(t, `[
	 {"patchSetBizId":"u1","versionNo":1},
	 {"patchSetBizId":"u2","versionNo":2},
	 {"patchSetBizId":"t","versionNo":9,"relatedMergeItemType":"MERGE_TARGET"}
	]`))
	got, err := Latest(sets)
	if err != nil || got.BizID != "u2" {
		t.Fatalf("got %+v err=%v", got, err)
	}
}

func TestLatestErrors(t *testing.T) {
	sets, err := Parse(decode(t, `[]`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Latest(sets); !errors.Is(err, ErrNoPatchSets) {
		t.Fatalf("empty: err=%v", err)
	}
	sets, _ = Parse(decode(t, `[{"patchSetBizId":"t","versionNo":1,"relatedMergeItemType":"MERGE_TARGET"}]`))
	if _, err := Latest(sets); !errors.Is(err, ErrNoSourcePatchSet) {
		t.Fatalf("target only: err=%v", err)
	}
	// Items without a biz id are ignored.
	sets, _ = Parse(decode(t, `[{"versionNo":1,"relatedMergeItemType":"MERGE_SOURCE"}]`))
	if _, err := Latest(sets); !errors.Is(err, ErrNoPatchSets) {
		t.Fatalf("no biz id: err=%v", err)
	}
}

func TestParseRejectsNonList(t *testing.T) {
	if _, err := Parse(decode(t, `{"foo":1}`)); err == nil {
		t.Fatal("expected error for non-list payload")
	}
	if _, err := Parse(decode(t, `"x"`)); err == nil {
		t.Fatal("expected error for scalar payload")
	}
}
