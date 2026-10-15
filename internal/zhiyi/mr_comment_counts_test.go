package zhiyi

import "testing"

func TestStripNullCommentCounts(t *testing.T) {
	mr := map[string]any{
		"totalCommentCount":      nil,
		"unResolvedCommentCount": nil,
		"title":                  "x",
	}
	if !StripNullCommentCounts(mr) {
		t.Fatal("expected strip")
	}
	if _, ok := mr["totalCommentCount"]; ok {
		t.Fatalf("totalCommentCount still present: %#v", mr)
	}
	if _, ok := mr["unResolvedCommentCount"]; ok {
		t.Fatalf("unResolvedCommentCount still present: %#v", mr)
	}
	if mr["title"] != "x" {
		t.Fatalf("title lost: %#v", mr)
	}

	mr2 := map[string]any{
		"totalCommentCount":      float64(3),
		"unResolvedCommentCount": float64(0),
	}
	if StripNullCommentCounts(mr2) {
		t.Fatal("must not strip non-null counts")
	}
	if mr2["totalCommentCount"] != float64(3) || mr2["unResolvedCommentCount"] != float64(0) {
		t.Fatalf("counts mutated: %#v", mr2)
	}

	if StripNullCommentCounts(nil) {
		t.Fatal("nil mr")
	}
}