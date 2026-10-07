package zhiyi

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// plan123Fixture mirrors the template bug graph: a linear chain confirm → processing
// → testing → closed-fixed where closed-fixed has no outgoing edges, plus an
// off-graph status deferred (known in statuses, absent from edges).
func plan123Fixture() (edges map[string][]string, all map[string]bool) {
	edges = map[string][]string{
		"st-confirm":      {"st-processing"},
		"st-processing":   {"st-testing"},
		"st-testing":      {"st-closed-fixed"},
		"st-closed-fixed": {},
	}
	all = map[string]bool{
		"st-confirm": true, "st-processing": true, "st-testing": true,
		"st-closed-fixed": true, "st-deferred": true,
	}
	return edges, all
}

func TestTransitionStepsNoPathIsTypedError(t *testing.T) {
	edges, all := plan123Fixture()
	_, err := TransitionSteps("st-closed-fixed", "st-processing", edges, all)
	if err == nil {
		t.Fatal("expected error")
	}
	var np *NoPathError
	if !errors.As(err, &np) {
		t.Fatalf("want *NoPathError, got %T: %v", err, err)
	}
	if np.Current != "st-closed-fixed" || np.Target != "st-processing" {
		t.Fatalf("NoPathError=%+v", np)
	}
	// Unknown statuses are NOT NoPathError (input problem, not a graph gap).
	_, err = TransitionSteps("st-processing", "st-unknown", edges, all)
	if errors.As(err, &np) {
		t.Fatalf("unknown target must not be NoPathError: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "不在缺陷状态机中") {
		t.Fatalf("unknown target error=%v", err)
	}
}

func TestBugTransitionPlan(t *testing.T) {
	edges, all := plan123Fixture()
	cases := []struct {
		name    string
		current string
		target  string
		direct  bool
		wantErr string // "" when success expected
		want    []string
		mode    string
	}{
		{
			name:    "bfs route follows edges",
			current: "st-confirm", target: "st-testing",
			want: []string{"st-processing", "st-testing"}, mode: TransitionModeProfileBFS,
		},
		{
			name:    "single verified edge still profile_bfs",
			current: "st-confirm", target: "st-processing",
			want: []string{"st-processing"}, mode: TransitionModeProfileBFS,
		},
		{
			name:    "no path falls back to single direct hop",
			current: "st-closed-fixed", target: "st-processing",
			want: []string{"st-processing"}, mode: TransitionModeBFSNoPathDirect,
		},
		{
			name:    "off-graph target single hop is fallback mode",
			current: "st-confirm", target: "st-deferred",
			want: []string{"st-deferred"}, mode: TransitionModeBFSNoPathDirect,
		},
		{
			name:    "off-graph current single hop is fallback mode",
			current: "st-deferred", target: "st-processing",
			want: []string{"st-processing"}, mode: TransitionModeBFSNoPathDirect,
		},
		{
			name:    "direct flag skips bfs even when a route exists",
			current: "st-confirm", target: "st-testing", direct: true,
			want: []string{"st-testing"}, mode: TransitionModeDirectForced,
		},
		{
			name:    "direct flag skips status machine membership",
			current: "st-confirm", target: "st-brand-new", direct: true,
			want: []string{"st-brand-new"}, mode: TransitionModeDirectForced,
		},
		{
			name:    "same status is a noop",
			current: "st-processing", target: "st-processing",
			want: nil, mode: TransitionModeNoop,
		},
		{
			name:    "same status noop wins over direct",
			current: "st-processing", target: "st-processing", direct: true,
			want: nil, mode: TransitionModeNoop,
		},
		{
			name:    "unknown target without direct still errors",
			current: "st-processing", target: "st-unknown",
			wantErr: "不在缺陷状态机中",
		},
		{
			name:    "unknown current still errors",
			current: "st-unknown", target: "st-processing",
			wantErr: "不在缺陷状态机中",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			steps, mode, err := BugTransitionPlan(tc.current, tc.target, edges, all, tc.direct)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mode != tc.mode {
				t.Fatalf("mode=%q want %q (steps=%v)", mode, tc.mode, steps)
			}
			if !reflect.DeepEqual(steps, tc.want) {
				t.Fatalf("steps=%v want %v", steps, tc.want)
			}
		})
	}
}

// Empty edges (profile without bug_edges and no derivable template): every hop is a
// direct fallback — matches the pre-#123 single-hop behavior, now with honest mode.
func TestBugTransitionPlanEmptyEdgesAllDirect(t *testing.T) {
	all := map[string]bool{"st-a": true, "st-b": true}
	steps, mode, err := BugTransitionPlan("st-a", "st-b", map[string][]string{}, all, false)
	if err != nil {
		t.Fatal(err)
	}
	if mode != TransitionModeBFSNoPathDirect || len(steps) != 1 || steps[0] != "st-b" {
		t.Fatalf("steps=%v mode=%q", steps, mode)
	}
}
