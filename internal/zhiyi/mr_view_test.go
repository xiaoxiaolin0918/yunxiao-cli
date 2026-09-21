package zhiyi

import "testing"

func TestStabilizeMergeRequest_MapsStatusToState(t *testing.T) {
	mr := map[string]any{
		"localId":   float64(2765),
		"title":     "feat: x",
		"status":    "UNDER_REVIEW",
		"detailUrl": "https://codeup.aliyun.com/org/repo/change/2765",
		"author":    map[string]any{"state": "active"},
	}
	got := StabilizeMergeRequest(mr)
	if got["state"] != "UNDER_REVIEW" || got["status"] != "UNDER_REVIEW" {
		t.Fatalf("state/status: %#v", got)
	}
	if got["localId"] != "2765" && got["localId"] != float64(2765) {
		// Stabilize may set string localId
		if s, _ := got["localId"].(string); s != "2765" {
			t.Fatalf("localId: %#v", got["localId"])
		}
	}
	brief := BriefMergeRequest(mr)
	if brief["state"] != "UNDER_REVIEW" || brief["title"] != "feat: x" {
		t.Fatalf("brief: %#v", brief)
	}
	if _, ok := brief["author"]; ok {
		t.Fatal("brief should not include author")
	}
}

func TestMRStatus_IgnoresAuthorState(t *testing.T) {
	mr := map[string]any{"author": map[string]any{"state": "active"}}
	if MRStatus(mr) != "" {
		t.Fatalf("got %q", MRStatus(mr))
	}
}

func TestUnwrapMergeRequestPayload(t *testing.T) {
	wrapped := map[string]any{"data": map[string]any{"localId": float64(1), "status": "MERGED", "title": "t"}}
	got := StabilizeMergeRequest(UnwrapMergeRequestPayload(wrapped))
	if got["state"] != "MERGED" || got["title"] != "t" {
		t.Fatalf("%#v", got)
	}
}
