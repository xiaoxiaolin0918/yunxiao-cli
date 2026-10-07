package zhiyi

import (
	"encoding/json"
	"testing"
)

// #132: push-review lifecycle status resolution for list items.
func TestMergeRequestListStatus(t *testing.T) {
	cases := []struct {
		name string
		item map[string]any
		want string
	}{
		{"newVersionState wins", map[string]any{"newVersionState": "UNDER_DEV", "state": "opened"}, "UNDER_DEV"},
		{"detail status", map[string]any{"status": "TO_BE_MERGED"}, "TO_BE_MERGED"},
		{"legacy state fallback", map[string]any{"state": "opened"}, "opened"},
		{"snake_case newVersionState", map[string]any{"new_version_state": "MERGED"}, "MERGED"},
		{"empty", map[string]any{"title": "x"}, ""},
		{"nil", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MergeRequestListStatus(tc.item); got != tc.want {
				t.Fatalf("MergeRequestListStatus(%v)=%q want %q", tc.item, got, tc.want)
			}
		})
	}
}

func TestMergeRequestWIP(t *testing.T) {
	cases := []struct {
		name string
		item map[string]any
		wip  bool
		ok   bool
	}{
		{"UNDER_DEV status", map[string]any{"status": "UNDER_DEV"}, true, true},
		{"newVersionState UNDER_DEV", map[string]any{"newVersionState": "UNDER_DEV"}, true, true},
		{"TO_BE_MERGED not wip", map[string]any{"newVersionState": "TO_BE_MERGED"}, false, true},
		{"workInProgress true", map[string]any{"workInProgress": true}, true, true},
		{"workInProgress false", map[string]any{"workInProgress": false}, false, true},
		{"legacy opened counts as signal", map[string]any{"state": "opened"}, false, true},
		{"no signal", map[string]any{"title": "x"}, false, false},
		{"nil", nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wip, ok := MergeRequestWIP(tc.item)
			if wip != tc.wip || ok != tc.ok {
				t.Fatalf("MergeRequestWIP(%v)=(%v,%v) want (%v,%v)", tc.item, wip, ok, tc.wip, tc.ok)
			}
		})
	}
}

// AttachMergeRequestStatuses injects status/wip per item and leaves legacy state untouched.
func TestAttachMergeRequestStatuses(t *testing.T) {
	data := []any{
		map[string]any{"localId": float64(139), "newVersionState": "UNDER_DEV", "state": "opened", "workInProgress": true},
		map[string]any{"localId": float64(140), "newVersionState": "TO_BE_MERGED"},
		map[string]any{"localId": float64(141), "state": "opened"},
		map[string]any{"localId": float64(142)}, // no signal at all
	}
	out := AttachMergeRequestStatuses(data).([]any)
	m139 := out[0].(map[string]any)
	if m139["status"] != "UNDER_DEV" || m139["wip"] != true {
		t.Fatalf("139: %#v", m139)
	}
	if m139["state"] != "opened" {
		t.Fatalf("legacy state must stay untouched: %#v", m139)
	}
	m140 := out[1].(map[string]any)
	if m140["status"] != "TO_BE_MERGED" || m140["wip"] != false {
		t.Fatalf("140: %#v", m140)
	}
	m141 := out[2].(map[string]any)
	if m141["status"] != "opened" || m141["wip"] != false {
		t.Fatalf("141 (legacy): %#v", m141)
	}
	m142 := out[3].(map[string]any)
	if _, ok := m142["status"]; ok {
		t.Fatalf("142 must not get a fabricated status: %#v", m142)
	}
	if _, ok := m142["wip"]; ok {
		t.Fatalf("142 must not get a fabricated wip: %#v", m142)
	}
}

// The same-named API field is overwritten (same injection convention as #94 latest).
func TestAttachMergeRequestStatusesOverwritesSameNamedField(t *testing.T) {
	data := []any{map[string]any{"newVersionState": "UNDER_DEV", "status": "MERGED"}}
	out := AttachMergeRequestStatuses(data).([]any)
	m := out[0].(map[string]any)
	if m["status"] != "UNDER_DEV" {
		t.Fatalf("CLI-computed status must win: %#v", m)
	}
}

// Wrapped shapes (ListMergeRequests returns {"result": [...]}) are walked too.
func TestAttachMergeRequestStatusesWrappedShapes(t *testing.T) {
	for _, key := range []string{"items", "list", "data", "changeRequests", "result"} {
		wrapped := map[string]any{key: []any{map[string]any{"newVersionState": "UNDER_DEV"}}}
		AttachMergeRequestStatuses(wrapped)
		inner := wrapped[key].([]any)[0].(map[string]any)
		if inner["status"] != "UNDER_DEV" || inner["wip"] != true {
			t.Fatalf("key %s: %#v", key, inner)
		}
	}
}

func TestFilterMergeRequestListByStatus(t *testing.T) {
	data := []any{
		map[string]any{"localId": float64(1), "newVersionState": "UNDER_DEV"},
		map[string]any{"localId": float64(2), "newVersionState": "TO_BE_MERGED"},
		map[string]any{"localId": float64(3), "state": "opened"},
		"not-a-map", // passes through
	}
	out := FilterMergeRequestListByStatus(data, "under_dev").([]any)
	// matching item plus non-map junk passes through untouched (MR lists are maps;
	// the filter never silently drops what it cannot inspect)
	if len(out) != 2 || out[0].(map[string]any)["localId"] != float64(1) || out[1] != "not-a-map" {
		t.Fatalf("filter: %#v", out)
	}
	out = FilterMergeRequestListByStatus(data, "TO_BE_MERGED").([]any)
	if len(out) != 2 || out[0].(map[string]any)["localId"] != float64(2) {
		t.Fatalf("filter case-insensitive: %#v", out)
	}
	if got := FilterMergeRequestListByStatus(data, ""); len(got.([]any)) != 4 {
		t.Fatalf("empty filter keeps everything: %#v", got)
	}
	if got := FilterMergeRequestListByStatus(data, "MERGED"); len(got.([]any)) != 1 || got.([]any)[0] != "not-a-map" {
		t.Fatalf("no match (junk only): %#v", got)
	}

	wrapped := map[string]any{"result": []any{
		map[string]any{"newVersionState": "UNDER_DEV"},
		map[string]any{"newVersionState": "MERGED"},
	}}
	FilterMergeRequestListByStatus(wrapped, "MERGED")
	if items := wrapped["result"].([]any); len(items) != 1 {
		t.Fatalf("wrapped filter: %#v", wrapped)
	}
}

func TestMRStatusDisplay(t *testing.T) {
	cases := map[string]string{
		"UNDER_DEV":    "开发中(WIP)",
		"UNDER_REVIEW": "评审中",
		"TO_BE_MERGED": "待合并",
		"CLOSED":       "已关闭",
		"MERGED":       "已合并",
		"weird":        "",
	}
	for in, want := range cases {
		if got := MRStatusDisplay(in); got != want {
			t.Fatalf("MRStatusDisplay(%q)=%q want %q", in, got, want)
		}
	}
}

func TestMergeRequestLocalID(t *testing.T) {
	cases := []struct {
		mr   map[string]any
		want string
	}{
		{map[string]any{"localId": float64(139)}, "139"},
		{map[string]any{"local_id": 140}, "140"},
		{map[string]any{"iid": "141"}, "141"},
		{map[string]any{"title": "x"}, ""},
	}
	for _, tc := range cases {
		if got := MergeRequestLocalID(tc.mr); got != tc.want {
			t.Fatalf("MergeRequestLocalID(%v)=%q want %q", tc.mr, got, tc.want)
		}
	}
}

// StabilizeMergeRequest / BriefMergeRequest expose wip (#132).
func TestStabilizeMergeRequestAddsWip(t *testing.T) {
	got := StabilizeMergeRequest(map[string]any{"localId": float64(139), "status": "UNDER_DEV"})
	if got["wip"] != true {
		t.Fatalf("wip: %#v", got)
	}
	brief := BriefMergeRequest(map[string]any{"localId": float64(139), "status": "UNDER_DEV"})
	if brief["wip"] != true || brief["status"] != "UNDER_DEV" {
		t.Fatalf("brief wip: %#v", brief)
	}
	ready := StabilizeMergeRequest(map[string]any{"status": "TO_BE_MERGED"})
	if ready["wip"] != false {
		t.Fatalf("ready wip: %#v", ready)
	}
	if _, ok := StabilizeMergeRequest(map[string]any{"title": "x"})["wip"]; ok {
		t.Fatal("no signal → no wip key")
	}
}

// MRStatus prefers newVersionState for list-shaped objects.
func TestMRStatusReadsNewVersionState(t *testing.T) {
	if got := MRStatus(map[string]any{"newVersionState": "UNDER_DEV", "state": "opened"}); got != "UNDER_DEV" {
		t.Fatalf("MRStatus=%q", got)
	}
}

func TestBuildMRSummaryView(t *testing.T) {
	list := map[string]any{
		"localId":         float64(139),
		"title":           "feat: push",
		"state":           "opened",
		"newVersionState": "UNDER_DEV",
		"workInProgress":  true,
		"sourceBranch":    "feat/x",
		"targetBranch":    "master",
		"author":          map[string]any{"name": "李四", "username": "lisi"},
		"creationMethod":  "COMMAND_LINE",
	}
	detail := map[string]any{
		"localId":             float64(139),
		"status":              "UNDER_DEV",
		"ahead":               float64(3),
		"behind":              float64(1),
		"allRequirementsPass": false,
		"todoList": map[string]any{"requirementCheckItems": []any{
			map[string]any{"itemType": "REVIEWER_APPROVED_CHECK", "pass": true},
			map[string]any{"itemType": "COMMENTS_CHECK", "pass": false},
		}},
		"reviewers": []any{
			map[string]any{"name": "张三", "username": "zhangsan", "reviewOpinionStatus": "PASS", "hasReviewed": true},
		},
		"detailUrl": "https://codeup.aliyun.com/org/repo/change/139",
	}
	view := BuildMRSummaryView(list, detail)
	if view.LocalID != "139" || view.Title != "feat: push" {
		t.Fatalf("view: %+v", view)
	}
	if view.Status != "UNDER_DEV" || view.State != "UNDER_DEV" || view.StatusDisplay != "开发中(WIP)" {
		t.Fatalf("status fields: %+v", view)
	}
	if view.WIP == nil || !*view.WIP {
		t.Fatalf("wip: %+v", view)
	}
	if view.Ahead != float64(3) || view.Behind != float64(1) {
		t.Fatalf("ahead/behind: %+v", view)
	}
	if view.Mergeable == nil || *view.Mergeable {
		t.Fatalf("mergeable: %+v", view)
	}
	if len(view.Todo) != 2 {
		t.Fatalf("todo: %+v", view)
	}
	if len(view.Reviewers) != 1 || view.Reviewers[0]["review_opinion_status"] != "PASS" {
		t.Fatalf("reviewers: %+v", view.Reviewers)
	}
	if view.ReviewPassed == nil || !*view.ReviewPassed {
		t.Fatalf("review_passed: %+v", view)
	}
	if view.AuthorName != "李四" || view.CreationMethod != "COMMAND_LINE" {
		t.Fatalf("author/creation: %+v", view)
	}
	if view.DetailURL != "https://codeup.aliyun.com/org/repo/change/139" {
		t.Fatalf("url: %+v", view)
	}
	// JSON shape: keys agents rely on.
	b, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"localId", "title", "status", "state", "status_display", "wip", "sourceBranch", "targetBranch", "ahead", "behind", "mergeable", "todo", "reviewers", "review_passed", "url"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("json missing key %q: %s", k, b)
		}
	}
}

// List-only view (no detail): ahead/behind/mergeable/todo are omitted, not zeroed.
func TestBuildMRSummaryViewListOnly(t *testing.T) {
	view := BuildMRSummaryView(map[string]any{"localId": float64(1), "newVersionState": "UNDER_DEV"}, nil)
	if view.Status != "UNDER_DEV" || view.WIP == nil || !*view.WIP {
		t.Fatalf("view: %+v", view)
	}
	if view.Ahead != nil || view.Behind != nil || view.Mergeable != nil || view.Todo != nil || view.ReviewPassed != nil {
		t.Fatalf("detail fields must stay nil without a detail GET: %+v", view)
	}
}

func TestMrReviewPassed(t *testing.T) {
	mixed := map[string]any{"reviewers": []any{
		map[string]any{"reviewOpinionStatus": "PASS"},
		map[string]any{"reviewOpinionStatus": "NOT_PASS"},
	}}
	if p := mrReviewPassed(mixed); p == nil || *p {
		t.Fatalf("NOT_PASS must fail the set: %v", p)
	}
	allPass := map[string]any{"reviewers": []any{map[string]any{"reviewOpinionStatus": "PASS"}}}
	if p := mrReviewPassed(allPass); p == nil || !*p {
		t.Fatalf("all PASS: %v", p)
	}
	noOpinion := map[string]any{"reviewers": []any{map[string]any{"name": "n"}}}
	if p := mrReviewPassed(noOpinion); p != nil {
		t.Fatalf("no opinion → nil: %v", p)
	}
}
