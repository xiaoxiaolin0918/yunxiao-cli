package cmd

import "testing"

func TestWorkitemCategoryForSearch(t *testing.T) {
	if got := workitemCategoryForSearch(map[string]any{"categoryId": "Bug"}); got != "Bug" {
		t.Fatalf("categoryId=%q", got)
	}
	if got := workitemCategoryForSearch(map[string]any{"workitemType": map[string]any{"category": "Task"}}); got != "Task" {
		t.Fatalf("wt.category=%q", got)
	}
	if got := workitemCategoryForSearch(map[string]any{}); got != "" {
		t.Fatalf("empty=%q", got)
	}
}