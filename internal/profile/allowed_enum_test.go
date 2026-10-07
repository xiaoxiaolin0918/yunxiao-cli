package profile

import (
	"reflect"
	"testing"
)

// #121: allowed_* snapshots drift when tenants edit field options.
// DiffAllowedEnum reports both directions: stale snapshot values (sent → API 400)
// and live labels the snapshot would block at the client gate.
func TestDiffAllowedEnum(t *testing.T) {
	cases := []struct {
		name         string
		snapshot     []string
		accepted     []string
		labels       []string
		wantStale    []string
		wantUnlisted []string
	}{
		{
			name:         "issue_121_module_drift",
			snapshot:     []string{"MES", "OMS", "PDM", "公共组件"},
			accepted:     []string{"MES", "OMS", "PDM", "系统服务"},
			labels:       []string{"MES", "OMS", "PDM", "系统服务"},
			wantStale:    []string{"公共组件"},
			wantUnlisted: []string{"系统服务"},
		},
		{
			name:     "in_sync",
			snapshot: []string{"生产环境", "测试环境"},
			accepted: []string{"生产环境", "测试环境"},
			labels:   []string{"生产环境", "测试环境"},
		},
		{
			name:         "option_id_matches_display",
			snapshot:     []string{"MES"},
			accepted:     []string{"m-a", "MES"}, // id + value tokens both accepted
			labels:       []string{"MES"},
			wantUnlisted: nil,
		},
		{
			// Pure-function semantics: an empty snapshot accepts nothing, so every live
			// label is unlisted. Callers that treat an empty snapshot as "gate disabled"
			// (profile doctor, +bug-create) skip the diff instead.
			name:         "empty_snapshot_lists_all_labels",
			snapshot:     nil,
			accepted:     []string{"MES"},
			labels:       []string{"MES"},
			wantUnlisted: []string{"MES"},
		},
		{
			name:         "blank_and_duplicate_inputs_deduped",
			snapshot:     []string{"MES", "MES", " ", ""},
			accepted:     []string{"MES", ""},
			labels:       []string{"MES", "MES", " "},
			wantStale:    nil,
			wantUnlisted: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stale, unlisted := DiffAllowedEnum(tc.snapshot, tc.accepted, tc.labels)
			if !reflect.DeepEqual(stale, tc.wantStale) {
				t.Fatalf("stale=%v want %v", stale, tc.wantStale)
			}
			if !reflect.DeepEqual(unlisted, tc.wantUnlisted) {
				t.Fatalf("unlisted=%v want %v", unlisted, tc.wantUnlisted)
			}
		})
	}
}
