package mrlink

import "testing"

func TestAttachedWorkItemIDs(t *testing.T) {
	mr := map[string]any{
		"workItemIds": []any{"a", "b"},
		"relatedWorkItems": []any{
			map[string]any{"id": "b"},
			map[string]any{"workItemId": "c"},
		},
	}
	got := AttachedWorkItemIDs(mr)
	if len(got) != 3 {
		t.Fatalf("got %#v", got)
	}
	nested := map[string]any{"data": map[string]any{"workItemIds": []any{"x"}}}
	if AttachedWorkItemIDs(nested)[0] != "x" {
		t.Fatal(AttachedWorkItemIDs(nested))
	}
}

func TestMissingWorkItemIDs(t *testing.T) {
	missing := MissingWorkItemIDs([]string{"a", "b", "a", ""}, []string{"b", "c"})
	if len(missing) != 1 || missing[0] != "a" {
		t.Fatalf("%#v", missing)
	}
	if FormatMissingLinkWarning(nil) != "" {
		t.Fatal("expected empty")
	}
	w := FormatMissingLinkWarning([]string{"z1"})
	if w == "" || !contains(w, "z1") {
		t.Fatal(w)
	}
}

func TestSpaceIDFromWorkItem(t *testing.T) {
	if SpaceIDFromWorkItem(map[string]any{"spaceId": "S1"}) != "S1" {
		t.Fatal("spaceId")
	}
	if SpaceIDFromWorkItem(map[string]any{"space": map[string]any{"id": "S2"}}) != "S2" {
		t.Fatal("nested")
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