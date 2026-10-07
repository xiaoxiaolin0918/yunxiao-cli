package zhiyi

import (
	"reflect"
	"testing"
)

func TestNormalizeMergeType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"ff-only", "ff-only"},
		{"Fast-Forward-Only", "ff-only"},
		{"no-fast-forward", "no-fast-forward"},
		{"NO_FAST_FORWARD", "no-fast-forward"},
		{"创建合并节点", "no-fast-forward"},
		{"squash", "squash"},
		{"Squash-Merge", "squash"},
		{"rebase", "rebase"},
		{"REBASE_MERGE", "rebase"},
		{"  rebase  ", "rebase"},
		{"", ""},
		{"  ", ""},
		{"turbo-merge", "turbomerge"}, // unknown passes through normalized for reporting
	}
	for _, tc := range cases {
		if got := NormalizeMergeType(tc.in); got != tc.want {
			t.Errorf("NormalizeMergeType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMergeTypesFromMR(t *testing.T) {
	cases := []struct {
		name       string
		mr         map[string]any
		wantList   []string
		wantSource string
	}{
		{name: "absent", mr: map[string]any{"status": "TO_BE_MERGED"}, wantList: nil, wantSource: ""},
		{
			name:       "mergeTypes top-level",
			mr:         map[string]any{"mergeTypes": []any{"Fast-forward-only", "创建合并节点"}},
			wantList:   []string{"ff-only", "no-fast-forward"},
			wantSource: "mergeTypes",
		},
		{
			name:       "supported_merge_types snake",
			mr:         map[string]any{"supported_merge_types": []any{"rebase", "squash", "squash"}},
			wantList:   []string{"rebase", "squash"},
			wantSource: "supportedMergeTypes",
		},
		{
			name:       "mergeSetting nested",
			mr:         map[string]any{"merge_setting": map[string]any{"merge_types": []any{"no-fast-forward"}}},
			wantList:   []string{"no-fast-forward"},
			wantSource: "merge_setting.merge_types",
		},
		{name: "empty list ignored", mr: map[string]any{"mergeTypes": []any{}}, wantList: nil, wantSource: ""},
		{name: "garbage entries dropped", mr: map[string]any{"mergeTypes": []any{"", nil, "squash"}}, wantList: []string{"squash"}, wantSource: "mergeTypes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, src := MergeTypesFromMR(tc.mr)
			if src != tc.wantSource {
				t.Fatalf("source=%q want %q", src, tc.wantSource)
			}
			if !reflect.DeepEqual(got, tc.wantList) {
				t.Fatalf("list=%v want %v", got, tc.wantList)
			}
		})
	}
}

func TestMRConflictCheckStatusAndBool(t *testing.T) {
	mr := map[string]any{
		"conflictCheckStatus":         "has_conflict",
		"mergeable":                   true,
		"support_merge_ff_only_dummy": "yes",
	}
	if got := MRConflictCheckStatus(mr); got != "HAS_CONFLICT" {
		t.Fatalf("MRConflictCheckStatus=%q", got)
	}
	snake := map[string]any{"conflict_check_status": "No_Conflict"}
	if got := MRConflictCheckStatus(snake); got != "NO_CONFLICT" {
		t.Fatalf("snake=%q", got)
	}
	if v, present := MRBool(mr, "mergeable"); !v || !present {
		t.Fatalf("MRBool(mergeable)=%v,%v", v, present)
	}
	if v, present := MRBool(mr, "absent"); v || present {
		t.Fatalf("MRBool(absent)=%v,%v", v, present)
	}
	str := map[string]any{"mergeable": "false"}
	if v, present := MRBool(str, "mergeable"); v || !present {
		t.Fatalf("MRBool(string false)=%v,%v", v, present)
	}
}
