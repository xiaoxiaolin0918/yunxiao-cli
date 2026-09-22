package workflow

import (
	"fmt"
	"testing"
	"time"
)

func TestExploreTransitionsOneWayPartial(t *testing.T) {
	// One-way chain: after leaving a we may not rediscover all a→* edges.
	allowed := map[string]map[string]bool{
		"a": {"b": true, "c": true},
		"b": {"c": true},
		"c": {},
	}
	current := "a"
	res, err := ExploreTransitions(ProbeOptions{
		StatusIDs:     []string{"a", "b", "c"},
		StartStatus:   "a",
		DefaultStatus: "a",
		Sleep:         func(d time.Duration) {},
		Get:           func() (string, error) { return current, nil },
		Put: func(to string) error {
			from := current
			if allowed[from][to] {
				current = to
				return nil
			}
			return fmt.Errorf("当前状态:%s不能流转到目标状态:%s", from, to)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(res.Edges, "a", "b") && !hasEdge(res.Edges, "a", "c") {
		t.Fatalf("expected at least one outbound from a: %v", res.Edges)
	}
	if !hasEdge(res.Edges, "b", "c") && CountEdges(res.Edges) < 1 {
		t.Fatalf("%v", res.Edges)
	}
}

func TestExploreTransitionsWithReturnEdges(t *testing.T) {
	// Bidirectional enough to finish all pairs from a.
	allowed := map[string]map[string]bool{
		"a": {"b": true, "c": true},
		"b": {"a": true, "c": true},
		"c": {"a": true},
	}
	current := "a"
	res, err := ExploreTransitions(ProbeOptions{
		StatusIDs:     []string{"a", "b", "c"},
		StartStatus:   "a",
		DefaultStatus: "a",
		Sleep:         func(d time.Duration) {},
		Get:           func() (string, error) { return current, nil },
		Put: func(to string) error {
			from := current
			if allowed[from][to] {
				current = to
				return nil
			}
			return fmt.Errorf("当前状态:%s不能流转到目标状态:%s", from, to)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range [][2]string{{"a", "b"}, {"a", "c"}, {"b", "c"}, {"b", "a"}, {"c", "a"}} {
		if !hasEdge(res.Edges, e[0], e[1]) {
			t.Fatalf("missing %s→%s in %v", e[0], e[1], res.Edges)
		}
	}
}

func TestExploreNeedsFieldsCountsAsEdge(t *testing.T) {
	// Legacy name: needs_fields still recorded, but only as hinted (issue 61).
	current := "a"
	res, err := ExploreTransitions(ProbeOptions{
		StatusIDs:   []string{"a", "b"},
		StartStatus: "a",
		Sleep:       func(d time.Duration) {},
		Get:         func() (string, error) { return current, nil },
		Put: func(to string) error {
			if current == "a" && to == "b" {
				return fmt.Errorf("字段【计划完成时间】不能为空")
			}
			return fmt.Errorf("当前状态不能流转到目标状态")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if hasEdge(res.Edges, "a", "b") {
		t.Fatalf("needs_fields must not be verified: %v", res.Edges)
	}
	if !hasEdge(res.HintedEdges, "a", "b") {
		t.Fatalf("expected hinted needs_fields edge, got hinted=%v hints=%v", res.HintedEdges, res.RequiredHints)
	}
	if res.RequiredHints["b"] == "" && res.RequiredHints["a→b"] == "" {
		t.Fatalf("hints=%v", res.RequiredHints)
	}
}

func TestExploreTransitionsResetUnlocksSource(t *testing.T) {
	allowed := map[string]map[string]bool{
		"a": {"b": true, "c": true},
		"b": {"c": true},
		"c": {},
	}
	current := "a"
	resets := 0
	res, err := ExploreTransitions(ProbeOptions{
		StatusIDs:     []string{"a", "b", "c"},
		StartStatus:   "a",
		DefaultStatus: "a",
		Sleep:         func(d time.Duration) {},
		Get:           func() (string, error) { return current, nil },
		Put: func(to string) error {
			from := current
			if allowed[from][to] {
				current = to
				return nil
			}
			return fmt.Errorf("当前状态:%s不能流转到目标状态:%s", from, to)
		},
		Reset: func() (string, error) {
			resets++
			current = "a"
			return "a", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resets == 0 {
		t.Fatal("expected reset")
	}
	if !hasEdge(res.Edges, "a", "b") || !hasEdge(res.Edges, "a", "c") || !hasEdge(res.Edges, "b", "c") {
		t.Fatalf("edges=%v resets=%d", res.Edges, resets)
	}
}

func TestExploreNeedsFieldsAreHintedNotVerified(t *testing.T) {
	current := "a"
	res, err := ExploreTransitions(ProbeOptions{
		StatusIDs:   []string{"a", "b"},
		StartStatus: "a",
		Sleep:       func(d time.Duration) {},
		Get:         func() (string, error) { return current, nil },
		Put: func(to string) error {
			if current == "a" && to == "b" {
				return fmt.Errorf(`API error: {"errorMessage":"字段【计划完成时间】不能为空"}`)
			}
			return fmt.Errorf("当前状态不能流转到目标状态")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if hasEdge(res.Edges, "a", "b") {
		t.Fatalf("needs_fields must NOT be verified edges: %v", res.Edges)
	}
	if !hasEdge(res.HintedEdges, "a", "b") {
		t.Fatalf("expected hinted a→b, got hinted=%v hints=%v", res.HintedEdges, res.RequiredHints)
	}
	if res.RequiredHints["a→b"] == "" && res.RequiredHints["b"] == "" {
		t.Fatalf("hints=%v", res.RequiredHints)
	}
	miss := res.MissingFields["a→b"]
	if len(miss) != 1 || miss[0] != "计划完成时间" {
		t.Fatalf("missing_fields=%v", res.MissingFields)
	}
}

func TestExploreVerifiedAndHintedSplit(t *testing.T) {
	allowed := map[string]map[string]bool{"a": {"b": true}, "b": {"a": true}, "c": {}}
	needs := map[string]map[string]bool{"a": {"c": true}}
	current := "a"
	res, err := ExploreTransitions(ProbeOptions{
		StatusIDs:     []string{"a", "b", "c"},
		StartStatus:   "a",
		DefaultStatus: "a",
		Sleep:         func(d time.Duration) {},
		Get:           func() (string, error) { return current, nil },
		Put: func(to string) error {
			from := current
			if allowed[from][to] {
				current = to
				return nil
			}
			if needs[from][to] {
				return fmt.Errorf("字段【迭代】不能为空")
			}
			return fmt.Errorf("当前状态:%s不能流转到目标状态:%s", from, to)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(res.Edges, "a", "b") {
		t.Fatalf("verified missing a→b: %v", res.Edges)
	}
	if hasEdge(res.Edges, "a", "c") {
		t.Fatalf("a→c must not be verified: %v", res.Edges)
	}
	if !hasEdge(res.HintedEdges, "a", "c") {
		t.Fatalf("hinted missing a→c: %v", res.HintedEdges)
	}
}

func TestExtractMissingFieldNames(t *testing.T) {
	got := ExtractMissingFieldNames(`字段【计划完成时间】不能为空；字段【迭代】必填`)
	if len(got) != 2 || got[0] != "计划完成时间" || got[1] != "迭代" {
		t.Fatalf("%v", got)
	}
	if ExtractMissingFieldNames("invalid transition") != nil {
		t.Fatal("expected nil")
	}
}
