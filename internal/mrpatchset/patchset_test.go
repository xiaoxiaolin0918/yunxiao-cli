package mrpatchset

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
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

// Untyped items are a fallback only for payloads that carry no relatedMergeItemType at all.
func TestLatestUntypedItemsWhenNoSourceTyped(t *testing.T) {
	sets, _ := Parse(decode(t, `[
	 {"patchSetBizId":"u1","versionNo":1},
	 {"patchSetBizId":"u2","versionNo":2}
	]`))
	got, err := Latest(sets)
	if err != nil || got.BizID != "u2" {
		t.Fatalf("got %+v err=%v", got, err)
	}
}

// m2: once MERGE_TARGET entries exist the payload is typed; untyped items are not
// trusted as source and the missing MERGE_SOURCE is an error.
func TestLatestUntypedIgnoredWhenTargetPresent(t *testing.T) {
	sets, _ := Parse(decode(t, `[
	 {"patchSetBizId":"u1","versionNo":1},
	 {"patchSetBizId":"u2","versionNo":2},
	 {"patchSetBizId":"t","versionNo":9,"relatedMergeItemType":"MERGE_TARGET"}
	]`))
	if got, err := Latest(sets); !errors.Is(err, ErrNoSourcePatchSet) {
		t.Fatalf("got %+v err=%v, want ErrNoSourcePatchSet", got, err)
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
	if _, err := Parse(decode(t, `{"foo":1}`)); err == nil || !strings.Contains(err.Error(), "result/data/items/patchSets") {
		t.Fatalf("expected error naming all wrapper keys, got %v", err)
	}
	if _, err := Parse(decode(t, `"x"`)); err == nil {
		t.Fatal("expected error for scalar payload")
	}
}

func latestOf(t *testing.T, payload string) PatchSet {
	t.Helper()
	sets, err := Parse(decode(t, payload))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Latest(sets)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// I3: string versions compare numerically ("10" > "9"), independent of order.
func TestLatestStringVersionsCompareNumerically(t *testing.T) {
	v10 := `{"patchSetBizId":"v10","versionNo":"10","relatedMergeItemType":"MERGE_SOURCE"}`
	v9 := `{"patchSetBizId":"v9","versionNo":"9","relatedMergeItemType":"MERGE_SOURCE"}`
	for _, payload := range []string{"[" + v10 + "," + v9 + "]", "[" + v9 + "," + v10 + "]"} {
		if got := latestOf(t, payload); got.BizID != "v10" || got.VersionNo != 10 {
			t.Fatalf("%s: got %+v", payload, got)
		}
	}
}

// m3: version forms. Integral floats ("3.0", 3.0) parse; fractional / garbage → 0 (unknown).
func TestParseVersionForms(t *testing.T) {
	cases := map[string]int64{`3`: 3, `"3"`: 3, `"3.0"`: 3, `3.0`: 3, `" 4 "`: 4, `"3.5"`: 0, `"abc"`: 0, `null`: 0}
	for raw, want := range cases {
		sets, err := Parse(decode(t, `[{"patchSetBizId":"a","versionNo":`+raw+`}]`))
		if err != nil {
			t.Fatal(err)
		}
		if sets[0].VersionNo != want {
			t.Fatalf("versionNo %s → %d, want %d", raw, sets[0].VersionNo, want)
		}
	}
}

// m3: numeric epoch-millisecond createTime is kept as digits (no 1.7e+12) and parsed.
func TestParseNumericEpochMillisCreateTime(t *testing.T) {
	sets, err := Parse(decode(t, `[{"patchSetBizId":"a","createTime":1727600000123},{"patchSetBizId":"b","createTime":"1727600000"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if sets[0].CreateTime != "1727600000123" {
		t.Fatalf("CreateTime=%q", sets[0].CreateTime)
	}
	if want := time.UnixMilli(1727600000123).UTC(); !sets[0].Created.Equal(want) {
		t.Fatalf("Created=%v want %v", sets[0].Created, want)
	}
	// 10-digit value = epoch seconds.
	if want := time.Unix(1727600000, 0).UTC(); !sets[1].Created.Equal(want) {
		t.Fatalf("seconds Created=%v want %v", sets[1].Created, want)
	}
}

// m3: json.Number (decoder.UseNumber) is handled like float64.
func TestParseJSONNumber(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader(`[{"patchSetBizId":"a","versionNo":10,"createTime":1727600000123}]`))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	sets, err := Parse(v)
	if err != nil {
		t.Fatal(err)
	}
	if sets[0].VersionNo != 10 || sets[0].CreateTime != "1727600000123" || !sets[0].Created.Equal(time.UnixMilli(1727600000123)) {
		t.Fatalf("got %+v", sets[0])
	}
}

// m3: zoneless timestamps are UTC, not the machine's local zone.
func TestParseZonelessTimeIsUTC(t *testing.T) {
	sets, _ := Parse(decode(t, `[{"patchSetBizId":"a","createTime":"2022-03-18 14:24:54"},{"patchSetBizId":"b","createTime":"2022-03-18T14:24:54"}]`))
	want := time.Date(2022, 3, 18, 14, 24, 54, 0, time.UTC)
	for _, ps := range sets {
		if !ps.Created.Equal(want) {
			t.Fatalf("%s: Created=%v want %v", ps.BizID, ps.Created, want)
		}
	}
}

func TestLatestTieBreaksOnNumericCreateTime(t *testing.T) {
	got := latestOf(t, `[
	 {"patchSetBizId":"new","versionNo":1,"relatedMergeItemType":"MERGE_SOURCE","createTime":1727600000999},
	 {"patchSetBizId":"old","versionNo":1,"relatedMergeItemType":"MERGE_SOURCE","createTime":1727600000123}
	]`)
	if got.BizID != "new" {
		t.Fatalf("got %+v", got)
	}
}

// m1: total order (version, has-parsed-time, time, index). An unparseable createTime
// must not make the pick depend on response order.
func TestLatestTieBreakIsOrderIndependent(t *testing.T) {
	items := []string{
		`{"patchSetBizId":"late","versionNo":2,"relatedMergeItemType":"MERGE_SOURCE","createTime":"2026-09-29T10:00:00Z"}`,
		`{"patchSetBizId":"bad","versionNo":2,"relatedMergeItemType":"MERGE_SOURCE","createTime":"not-a-time"}`,
		`{"patchSetBizId":"early","versionNo":2,"relatedMergeItemType":"MERGE_SOURCE","createTime":"2026-09-29T08:00:00Z"}`,
	}
	perms := [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
	for _, p := range perms {
		payload := "[" + items[p[0]] + "," + items[p[1]] + "," + items[p[2]] + "]"
		if got := latestOf(t, payload); got.BizID != "late" {
			t.Fatalf("perm %v: got %s, want late", p, got.BizID)
		}
	}
}

// Full ties (same version, no usable time or equal time) fall back to the later position.
func TestLatestFullTieUsesLaterPosition(t *testing.T) {
	got := latestOf(t, `[
	 {"patchSetBizId":"first","versionNo":2,"relatedMergeItemType":"MERGE_SOURCE"},
	 {"patchSetBizId":"second","versionNo":2,"relatedMergeItemType":"MERGE_SOURCE"}
	]`)
	if got.BizID != "second" {
		t.Fatalf("got %+v", got)
	}
}

// Only long numerics (>= 10 digits) are epoch s/ms; date-like "20260929" is not an epoch.
func TestParseShortNumericIsNotEpoch(t *testing.T) {
	sets, err := Parse(decode(t, `[{"patchSetBizId":"a","createTime":"20260929"},{"patchSetBizId":"b","createTime":20260929},{"patchSetBizId":"c","createTime":"172760000"},{"patchSetBizId":"d","createTime":"1727600000"}]`))
	if err != nil {
		t.Fatal(err)
	}
	for _, ps := range sets[:3] {
		if !ps.Created.IsZero() {
			t.Fatalf("%s (%s) must not parse as epoch: %v", ps.BizID, ps.CreateTime, ps.Created)
		}
	}
	if want := time.Unix(1727600000, 0).UTC(); !sets[3].Created.Equal(want) {
		t.Fatalf("10-digit seconds: %v", sets[3].Created)
	}
}

// Any typed entry (not only MERGE_TARGET) disables the untyped fallback.
func TestLatestUntypedIgnoredWhenAnyTypePresent(t *testing.T) {
	sets, _ := Parse(decode(t, `[{"patchSetBizId":"u","versionNo":2},{"patchSetBizId":"x","versionNo":1,"relatedMergeItemType":"SOMETHING_ELSE"}]`))
	if got, err := Latest(sets); !errors.Is(err, ErrNoSourcePatchSet) {
		t.Fatalf("got %+v err=%v", got, err)
	}
}

// A typed entry without patchSetBizId still marks the payload as typed (no untyped fallback).
func TestLatestTypedEntryWithoutBizIDDisablesFallback(t *testing.T) {
	sets, _ := Parse(decode(t, `[{"patchSetBizId":"","versionNo":2,"relatedMergeItemType":"MERGE_TARGET"},{"patchSetBizId":"u","versionNo":1}]`))
	if got, err := Latest(sets); !errors.Is(err, ErrNoSourcePatchSet) {
		t.Fatalf("got %+v err=%v", got, err)
	}
}

// A signed value is not an all-digit epoch.
func TestParseSignedNumericIsNotEpoch(t *testing.T) {
	sets, err := Parse(decode(t, `[{"patchSetBizId":"a","createTime":"+1727600000"},{"patchSetBizId":"b","createTime":"-1727600000"}]`))
	if err != nil {
		t.Fatal(err)
	}
	for _, ps := range sets {
		if !ps.Created.IsZero() {
			t.Fatalf("%s (%s) must not parse as epoch: %v", ps.BizID, ps.CreateTime, ps.Created)
		}
	}
}
