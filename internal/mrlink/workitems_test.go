package mrlink

import (
	"strings"
	"testing"
)

func TestAttachedWorkItemIDs(t *testing.T) {
	mr := map[string]any{
		"workItemIds": []any{"a", "b"},
		"relatedWorkItems": []any{
			map[string]any{"id": "b"},
			map[string]any{"workItemId": "c", "serialNumber": "ZYPT-9"},
		},
	}
	got := AttachedWorkItemIDs(mr)
	// a,b,c,ZYPT-9
	if len(got) < 4 {
		t.Fatalf("got %#v", got)
	}
	nested := map[string]any{"data": map[string]any{"workItemIds": []any{"x"}}}
	if AttachedWorkItemIDs(nested)[0] != "x" {
		t.Fatal(AttachedWorkItemIDs(nested))
	}
}

func TestMissingWorkItemIDs_matchesSerial(t *testing.T) {
	wanted := []ResolvedWorkItem{{
		InternalID: "internal-a",
		MatchKeys:  []string{"internal-a", "ZYPT-1", "ZYPT-1"},
	}}
	// response only has serial
	missing := MissingWorkItemIDs(wanted, []string{"ZYPT-1"})
	if len(missing) != 0 {
		t.Fatalf("expected no missing, got %#v", missing)
	}
	missing = MissingWorkItemIDs(wanted, []string{"other"})
	if len(missing) != 1 || missing[0] != "internal-a" {
		t.Fatalf("%#v", missing)
	}
}

func TestMissingWorkItemIDs_dedupe(t *testing.T) {
	wanted := []ResolvedWorkItem{
		{InternalID: "a", MatchKeys: []string{"a"}},
		{InternalID: "a", MatchKeys: []string{"a"}},
	}
	missing := MissingWorkItemIDs(wanted, nil)
	if len(missing) != 1 || missing[0] != "a" {
		t.Fatalf("%#v", missing)
	}
}

func TestSpaceIDFromWorkItem(t *testing.T) {
	if SpaceIDFromWorkItem(map[string]any{"spaceId": "S1"}) != "S1" {
		t.Fatal("spaceId")
	}
	if SpaceIDFromWorkItem(map[string]any{"space": map[string]any{"id": "S2"}}) != "S2" {
		t.Fatal("nested")
	}
	// projectId alone should NOT be treated as space (review Minor #3)
	if SpaceIDFromWorkItem(map[string]any{"projectId": "P1"}) != "" {
		t.Fatal("projectId must not count as space")
	}
}

func TestMatchKeysFromWorkItem(t *testing.T) {
	keys := MatchKeysFromWorkItem(map[string]any{
		"id": "i1", "serialNumber": "ZYPT-2",
	}, "ZYPT-2")
	if len(keys) < 2 {
		t.Fatalf("%#v", keys)
	}
}

func TestFormatMissingLinkWarning(t *testing.T) {
	if FormatMissingLinkWarning(nil) != "" {
		t.Fatal("expected empty")
	}
	w := FormatMissingLinkWarning([]string{"z1"})
	if w == "" || !contains(w, "z1") {
		t.Fatal(w)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestWorkItemIDsCSV(t *testing.T) {
	if got := WorkItemIDsCSV(nil); got != "" {
		t.Fatalf("nil: %q", got)
	}
	if got := WorkItemIDsCSV([]string{"a", "b"}); got != "a,b" {
		t.Fatalf("join: %q", got)
	}
	if got := WorkItemIDsCSV([]string{" a ", "", "b"}); got != "a,b" {
		t.Fatalf("trim: %q", got)
	}
}

func TestFormatMissingLinkError(t *testing.T) {
	msg := FormatMissingLinkError([]string{"abc"}, "https://example/mr/1")
	if msg == "" {
		t.Fatal("empty")
	}
	if !strings.Contains(msg, "abc") || !strings.Contains(msg, "https://example/mr/1") {
		t.Fatalf("msg=%q", msg)
	}
	if !strings.Contains(msg, "yunxiao codeup mrs link") {
		t.Fatalf("should suggest mrs link: %q", msg)
	}
}

func TestRelationRecordID(t *testing.T) {
	if got := RelationRecordID(map[string]any{"relationRecordId": "rr-1"}); got != "rr-1" {
		t.Fatalf("relationRecordId: %q", got)
	}
	if got := RelationRecordID(map[string]any{"id": "legacy-id"}); got != "legacy-id" {
		t.Fatalf("id fallback: %q", got)
	}
	if got := RelationRecordID(map[string]any{"relationRecordId": "rr", "id": "other"}); got != "rr" {
		t.Fatalf("prefer relationRecordId: %q", got)
	}
	if got := RelationRecordID(nil); got != "" {
		t.Fatalf("nil: %q", got)
	}
}
