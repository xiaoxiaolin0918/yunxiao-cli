package zhiyi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

// Fixture status map matching constants.ts / zhiyi.example.json
func testStatuses() map[string]string {
	return map[string]string{
		"confirm":           "28",
		"reopen":            "30",
		"processing":        "100010",
		"deploy-test":       "b2c5c60b4428974ef10483c800",
		"testing":           "42d423acc3f60646a533bac489",
		"deploy-prod":       "4b895bdc3433104d6bf73e4c17",
		"acceptance":        "c0efc47304cc56508ac4314680",
		"fixed":             "29",
		"regression":        "2458e2d912449b731aa6797f70",
		"deferred":          "34",
		"closed-fixed":      "33",
		"wont-fix":          "31",
		"cancelled-nofix":   "03cf4f3d160e37aaa13a6c27c8",
		"cancelled-wontfix": "30bdbe7fc10889cab222915375",
		"closed-unfixed":    "013a823767244591639ea5a7",
	}
}

func testAll(statuses map[string]string) map[string]bool {
	m := map[string]bool{}
	for _, id := range statuses {
		m[id] = true
	}
	return m
}

func TestResolveBugStatusId(t *testing.T) {
	st := testStatuses()
	if got := ResolveBugStatusId("processing", st); got != "100010" {
		t.Fatalf("got %s", got)
	}
	if got := ResolveBugStatusId("100010", st); got != "100010" {
		t.Fatalf("passthrough got %s", got)
	}
	if got := ResolveBugStatusId("deploy-test", st); got != "b2c5c60b4428974ef10483c800" {
		t.Fatalf("got %s", got)
	}
}

func TestTransitionStepsConfirmToTesting(t *testing.T) {
	st := testStatuses()
	edges := profile.DeriveBugEdges(st)
	all := testAll(st)
	steps, err := TransitionSteps(st["confirm"], st["testing"], edges, all)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{st["processing"], st["deploy-test"], st["testing"]}
	if len(steps) != len(want) {
		t.Fatalf("steps=%v want %v", steps, want)
	}
	for i := range want {
		if steps[i] != want[i] {
			t.Fatalf("steps=%v want %v", steps, want)
		}
	}
}

func TestTransitionStepsSameStatusEmpty(t *testing.T) {
	st := testStatuses()
	edges := profile.DeriveBugEdges(st)
	all := testAll(st)
	steps, err := TransitionSteps(st["processing"], st["processing"], edges, all)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 0 {
		t.Fatalf("want empty, got %v", steps)
	}
}

func TestTransitionStepsUnreachableThrows(t *testing.T) {
	st := testStatuses()
	edges := profile.DeriveBugEdges(st)
	all := testAll(st)
	// closed-fixed → confirm: both on-graph keys, but no path back to confirm
	_, err := TransitionSteps(st["closed-fixed"], st["confirm"], edges, all)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "图内无实证边") {
		t.Fatalf("msg=%v", err)
	}
}

func TestTransitionStepsSideBranchSingleHop(t *testing.T) {
	st := testStatuses()
	edges := profile.DeriveBugEdges(st)
	all := testAll(st)
	// deferred is not an edge key → single hop
	steps, err := TransitionSteps(st["confirm"], st["deferred"], edges, all)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0] != st["deferred"] {
		t.Fatalf("got %v", steps)
	}
	// from deferred (not key) to processing (key) → single hop
	steps, err = TransitionSteps(st["deferred"], st["processing"], edges, all)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0] != st["processing"] {
		t.Fatalf("got %v", steps)
	}
}

func TestRequiredFieldUnionMultiStep(t *testing.T) {
	st := testStatuses()
	required := map[string][]string{
		st["processing"]:  {"80", "171ea64c4a9b51542c960a6d7c"},
		st["deploy-test"]: {"4086ed55f8852e203dcf3d0042", "51fdf5114acb39746e4ad08677", "6a32dfdc0288052b44f2b004b2"},
		st["testing"]:     {"80", "171ea64c4a9b51542c960a6d7c"},
	}
	steps := []string{st["processing"], st["deploy-test"], st["testing"]}
	got := RequiredFieldIDs(steps, required)
	want := []string{
		"80", "171ea64c4a9b51542c960a6d7c",
		"4086ed55f8852e203dcf3d0042", "51fdf5114acb39746e4ad08677", "6a32dfdc0288052b44f2b004b2",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestLooksLikeSerial(t *testing.T) {
	if !LooksLikeSerial("ZYPT-5768") {
		t.Fatal("expected true")
	}
	if LooksLikeSerial("abc-1") {
		t.Fatal("lowercase prefix should fail generic")
	}
	if !LooksLikeSerialPrefix("ZYPT-5768", "ZYPT") {
		t.Fatal("prefix")
	}
	if LooksLikeSerial("deadbeef") {
		t.Fatal("internal id")
	}
}

func TestPlanDueDateWire(t *testing.T) {
	if got := PlanDueDateWire("2026-09-20"); got != "2026-09-20T00:00:00+08:00" {
		t.Fatalf("got %s", got)
	}
	if got := PlanDueDateWire("2026-09-20T00:00:00+08:00"); got != "2026-09-20T00:00:00+08:00" {
		t.Fatalf("passthrough got %s", got)
	}
}

func TestCurrentStatusAndIDs(t *testing.T) {
	item := map[string]any{
		"id":           "internal-abc",
		"serialNumber": "ZYPT-5768",
		"status":       map[string]any{"id": "28", "name": "待确认"},
	}
	if CurrentStatusID(item) != "28" {
		t.Fatal(CurrentStatusID(item))
	}
	if InternalID(item) != "internal-abc" {
		t.Fatal(InternalID(item))
	}
	if SerialNumber(item) != "ZYPT-5768" {
		t.Fatal(SerialNumber(item))
	}
}

func TestAggregateBugSprints(t *testing.T) {
	items := []map[string]any{
		{"sprint": map[string]any{"id": "s2", "name": "Sprint B"}},
		{"sprint": map[string]any{"id": "s1", "name": "Sprint A"}},
		{"sprint": map[string]any{"id": "s1", "name": "Sprint A"}},
		{"subject": "no sprint"},
		{"sprint": "bad"},
	}
	got := AggregateBugSprints(items)
	if got.SampleSize != 5 {
		t.Fatalf("sampleSize=%d", got.SampleSize)
	}
	if got.Suggested != "s1" {
		t.Fatalf("suggested=%s", got.Suggested)
	}
	if len(got.Candidates) != 2 {
		t.Fatalf("candidates=%v", got.Candidates)
	}
	if got.Candidates[0].ID != "s1" || got.Candidates[0].Count != 2 {
		t.Fatalf("first=%v", got.Candidates[0])
	}
	if got.Candidates[1].ID != "s2" || got.Candidates[1].Count != 1 {
		t.Fatalf("second=%v", got.Candidates[1])
	}
}

func TestAggregateBugSprintsTieBreakByID(t *testing.T) {
	items := []map[string]any{
		{"sprint": map[string]any{"id": "b", "name": "B"}},
		{"sprint": map[string]any{"id": "a", "name": "A"}},
	}
	got := AggregateBugSprints(items)
	if got.Suggested != "a" {
		t.Fatalf("tie-break want a, got %s", got.Suggested)
	}
}

func TestFormatSprintSuggestion(t *testing.T) {
	empty := FormatSprintSuggestion(SprintCurrentResult{})
	if !strings.Contains(empty, "未找到当前迭代") {
		t.Fatalf("%s", empty)
	}
	msg := FormatSprintSuggestion(SprintCurrentResult{
		Suggested: "s1",
		Candidates: []SprintCandidate{
			{ID: "s1", Name: "A", Count: 3},
			{ID: "s2", Name: "B", Count: 1},
		},
	})
	if !strings.Contains(msg, "建议 --sprint s1") || !strings.Contains(msg, "A(3)") {
		t.Fatalf("%s", msg)
	}
}

func TestWithWipTitle(t *testing.T) {
	if got := WithWipTitle("fix", "master", true); got != "WIP: fix" {
		t.Fatalf("%s", got)
	}
	if got := WithWipTitle("WIP: fix", "master", true); got != "WIP: fix" {
		t.Fatalf("idempotent %s", got)
	}
	if got := WithWipTitle("fix", "develop", true); got != "fix" {
		t.Fatalf("non-master %s", got)
	}
	if got := WithWipTitle("fix", "master", false); got != "fix" {
		t.Fatalf("no wip %s", got)
	}
}

func TestResolveRepositoryID(t *testing.T) {
	repos := map[string]int64{"iipmes_gy": 4951346, "zhiyi-cli": 7287010, "sandbox": 7472289}
	got, err := ResolveRepositoryID("4951346", repos)
	if err != nil || got != "4951346" {
		t.Fatalf("numeric: %s %v", got, err)
	}
	got, err = ResolveRepositoryID("iipmes_gy", repos)
	if err != nil || got != "4951346" {
		t.Fatalf("alias: %s %v", got, err)
	}
	got, err = ResolveRepositoryID("sandbox", repos)
	if err != nil || got != "7472289" {
		t.Fatalf("sandbox alias: %s %v", got, err)
	}
	if _, err := ResolveRepositoryID("unknown", repos); err == nil {
		t.Fatal("expected error")
	}
	got, err = ResolveRepositoryID("7287010", nil)
	if err != nil || got != "7287010" {
		t.Fatalf("numeric without map: %s %v", got, err)
	}
	got, err = ResolveRepositoryID("org/my-repo", nil)
	if err != nil || got != "org/my-repo" {
		t.Fatalf("path: %s %v", got, err)
	}
	got, err = ResolveRepositoryID("org%2Fmy-repo", repos)
	if err != nil || got != "org%2Fmy-repo" {
		t.Fatalf("urlencoded path: %s %v", got, err)
	}
	if _, err := ResolveRepositoryID("sandbox", nil); err == nil {
		t.Fatal("alias without map should fail")
	}
}

func TestBuildCreateBugArgs(t *testing.T) {
	pf := &profile.Profile{
		SpaceID:   "space-1",
		BugTypeID: "bug-type-1",
		BugCreateFields: profile.BugCreateFields{
			Priority: map[string]string{
				"high": "prio-high",
			},
			SeriousLevel: map[string]string{
				"normal": "sev-normal",
			},
			Module:            "mod-fid",
			Environment:       "env-fid",
			ExpCompletionTime: "ExpCompletionTime",
		},
	}
	body, err := BuildCreateBugArgs(CreateBugInput{
		Title:              "t",
		Description:        "d",
		Environment:        "测试环境",
		Priority:           "high",
		SeriousLevel:       "normal",
		Module:             "MES",
		ExpectedCompletion: "2026-09-20",
		Sprint:             "sprint-1",
		AssignedTo:         "user-1",
	}, pf)
	if err != nil {
		t.Fatal(err)
	}
	if body["spaceId"] != "space-1" || body["workitemTypeId"] != "bug-type-1" {
		t.Fatalf("ids: %v", body)
	}
	if body["subject"] != "t" || body["formatType"] != "MARKDOWN" {
		t.Fatalf("subject/format: %v", body)
	}
	cf, ok := body["customFieldValues"].(map[string]any)
	if !ok {
		t.Fatalf("customFieldValues type %T", body["customFieldValues"])
	}
	if cf["priority"] != "prio-high" || cf["seriousLevel"] != "sev-normal" {
		t.Fatalf("priority/serious: %v", cf)
	}
	if cf["mod-fid"] != "MES" || cf["env-fid"] != "测试环境" {
		t.Fatalf("module/env: %v", cf)
	}
	if cf["ExpCompletionTime"] != "2026-09-20" {
		t.Fatalf("exp: %v", cf)
	}
}

func TestBuildCreateBugArgsOmitsOptionalFields(t *testing.T) {
	pf := &profile.Profile{
		SpaceID:   "space-1",
		BugTypeID: "bug-type-1",
		BugCreateFields: profile.BugCreateFields{
			Priority:     map[string]string{"high": "prio-high"},
			SeriousLevel: map[string]string{"normal": "sev-normal"},
			// no module / environment / ExpCompletionTime
		},
	}
	body, err := BuildCreateBugArgs(CreateBugInput{
		Title:        "t",
		Description:  "d",
		Priority:     "high",
		SeriousLevel: "normal",
		Sprint:       "sprint-1",
		AssignedTo:   "user-1",
	}, pf)
	if err != nil {
		t.Fatal(err)
	}
	cf := body["customFieldValues"].(map[string]any)
	if len(cf) != 2 {
		t.Fatalf("expected only priority+seriousLevel, got %v", cf)
	}
	if cf["priority"] != "prio-high" || cf["seriousLevel"] != "sev-normal" {
		t.Fatalf("%v", cf)
	}
}

func TestBuildCreateBugArgsMinimal(t *testing.T) {
	pf := &profile.Profile{
		SpaceID:   "space-1",
		BugTypeID: "bug-type-1",
		BugCreateFields: profile.BugCreateFields{
			Priority:          map[string]string{"high": "prio-high"},
			SeriousLevel:      map[string]string{"normal": "sev-normal"},
			Module:            "mod-fid",
			Environment:       "env-fid",
			ExpCompletionTime: "ExpCompletionTime",
		},
	}
	body, err := BuildCreateBugArgs(CreateBugInput{
		Title:              "t",
		Description:        "d",
		Environment:        "测试环境",
		Priority:           "high",
		SeriousLevel:       "normal",
		Module:             "MES",
		ExpectedCompletion: "2026-09-20",
		Sprint:             "sprint-1",
		AssignedTo:         "user-1",
		Minimal:            true,
	}, pf)
	if err != nil {
		t.Fatal(err)
	}
	cf := body["customFieldValues"].(map[string]any)
	if _, ok := cf["mod-fid"]; ok {
		t.Fatalf("minimal should omit module: %v", cf)
	}
	if _, ok := cf["env-fid"]; ok {
		t.Fatalf("minimal should omit env: %v", cf)
	}
	if _, ok := cf["ExpCompletionTime"]; ok {
		t.Fatalf("minimal should omit exp: %v", cf)
	}
}

func TestExtractSearchItems(t *testing.T) {
	arr := ExtractSearchItems([]any{
		map[string]any{"id": "1"},
		map[string]any{"id": "2"},
	})
	if len(arr) != 2 {
		t.Fatalf("%d", len(arr))
	}
	wrapped := ExtractSearchItems(map[string]any{
		"items": []any{map[string]any{"id": "a"}},
	})
	if len(wrapped) != 1 || wrapped[0]["id"] != "a" {
		t.Fatalf("%v", wrapped)
	}
}

func TestWorkItemURL(t *testing.T) {
	u := WorkItemURL(map[string]any{
		"id":         "abc",
		"categoryId": "Bug",
	}, "space-x")
	if !strings.Contains(u, "/bug#openWorkitemIdentifier=abc") {
		t.Fatalf("%s", u)
	}
	cases := []struct {
		cat string
		seg string
	}{
		{"Req", "req"},
		{"Task", "task"},
		{"Risk", "risk"},
		{"Topic", "topic"},
		{"Request", "req"},
		{"UnknownThing", "req"},
	}
	for _, c := range cases {
		got := WorkItemURL(map[string]any{"id": "x1", "categoryId": c.cat}, "sid")
		want := "/projex/project/sid/" + c.seg + "#openWorkitemIdentifier=x1"
		if !strings.Contains(got, want) {
			t.Fatalf("cat=%s got=%s want substring %s", c.cat, got, want)
		}
	}
	serial := WorkItemURL(map[string]any{"serialNumber": "YXCLI-1", "categoryId": "Req"}, "sid")
	if !strings.Contains(serial, "?serialNumber=YXCLI-1") {
		t.Fatalf("%s", serial)
	}
}

func TestMergeRequestURL(t *testing.T) {
	// Prefer detailUrl (live Codeup webUrl is often repo-only).
	u := MergeRequestURL(map[string]any{
		"detailUrl": "https://codeup.aliyun.com/org/repo/change/3",
		"webUrl":    "https://codeup.aliyun.com/org/repo",
		"localId":   3,
	})
	if u != "https://codeup.aliyun.com/org/repo/change/3" {
		t.Fatalf("%s", u)
	}
	// Construct from path + localId when detailUrl absent and webUrl is repo home.
	u = MergeRequestURL(map[string]any{
		"webUrl":                         "https://codeup.aliyun.com/sanzhi/public/zhiyi_release_log",
		"localId":                        float64(3),
		"targetProjectPathWithNamespace": "sanzhi/public/zhiyi_release_log",
	})
	if u != "https://codeup.aliyun.com/sanzhi/public/zhiyi_release_log/change/3" {
		t.Fatalf("%s", u)
	}
	// webUrl already an MR page
	u = MergeRequestURL(map[string]any{
		"webUrl": "https://codeup.aliyun.com/a/b/change/9",
	})
	if u != "https://codeup.aliyun.com/a/b/change/9" {
		t.Fatalf("%s", u)
	}
}

func TestAttachMergeRequestURLs(t *testing.T) {
	arr := []any{
		map[string]any{
			"detailUrl": "https://codeup.aliyun.com/o/r/change/1",
			"localId":   1,
		},
	}
	out := AttachMergeRequestURLs(arr).([]any)
	m := out[0].(map[string]any)
	if m["url"] != "https://codeup.aliyun.com/o/r/change/1" {
		t.Fatalf("%v", m)
	}
}

func TestResolveSpaceID(t *testing.T) {
	item := map[string]any{"space": map[string]any{"id": "from-item"}}
	if got := ResolveSpaceID(item, "from-profile", "from-flag"); got != "from-profile" {
		t.Fatalf("%s", got)
	}
	if got := ResolveSpaceID(item, "", "from-flag"); got != "from-item" {
		t.Fatalf("%s", got)
	}
	if got := ResolveSpaceID(nil, "", "from-flag"); got != "from-flag" {
		t.Fatalf("%s", got)
	}
}

func TestEnrichWorkItemMeta(t *testing.T) {
	meta := EnrichWorkItemMeta(map[string]any{"risk": "read"}, map[string]any{
		"id":           "ee5e",
		"serialNumber": "YXCLI-49",
		"categoryId":   "Req",
		"space":        map[string]any{"id": "b388"},
	}, "b388", "")
	if meta["url"] == nil || !strings.Contains(fmt.Sprint(meta["url"]), "openWorkitemIdentifier=ee5e") {
		t.Fatalf("%v", meta)
	}
	if meta["serial_number"] != "YXCLI-49" {
		t.Fatalf("%v", meta)
	}
}

func TestBuildCreateBugArgs_UnmappedPriorityAlias(t *testing.T) {
	pf := &profile.Profile{SpaceID: "space-1", BugTypeID: "bug-type-1"}
	_, err := BuildCreateBugArgs(CreateBugInput{
		Title: "t", Description: "d", Priority: "urgent", SeriousLevel: "normal", Sprint: "s1", AssignedTo: "u1",
	}, pf)
	if err == nil || !strings.Contains(err.Error(), "priority alias") {
		t.Fatalf("expected priority alias error, got %v", err)
	}
}

func TestResolveRepositoryID_UnknownAlias(t *testing.T) {
	_, err := ResolveRepositoryID("zhiyi_doc", map[string]int64{"other": 1})
	if err == nil || !strings.Contains(err.Error(), "unknown repository alias") {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "profile.repositories") {
		t.Fatalf("hint missing: %v", err)
	}
}

func TestIsNumericRepositoryID(t *testing.T) {
	if !IsNumericRepositoryID("4951346") {
		t.Fatal("expected numeric")
	}
	if !IsNumericRepositoryID(" 7287010 ") {
		t.Fatal("trim")
	}
	if IsNumericRepositoryID("iipmes_gy") || IsNumericRepositoryID("org/repo") || IsNumericRepositoryID("org%2Frepo") || IsNumericRepositoryID("") {
		t.Fatal("non-numeric should be false")
	}
}

func TestProfileRepositoryIDSet(t *testing.T) {
	got := ProfileRepositoryIDSet(map[string]int64{"a": 1, "b": 2})
	if _, ok := got["1"]; !ok {
		t.Fatalf("missing 1: %v", got)
	}
	if _, ok := got["2"]; !ok {
		t.Fatalf("missing 2: %v", got)
	}
	if ProfileRepositoryIDSet(nil) == nil {
		t.Fatal("nil map should yield empty non-nil set")
	}
	if len(ProfileRepositoryIDSet(nil)) != 0 {
		t.Fatal("expected empty")
	}
}

func TestAddRepositoryIDsFromListItems(t *testing.T) {
	dst := map[string]struct{}{}
	AddRepositoryIDsFromListItems(dst, []any{
		map[string]any{"id": "10", "name": "a"},
		map[string]any{"id": float64(11), "name": "b"},
		map[string]any{"id": int64(12)},
		"skip",
		map[string]any{"name": "no-id"},
	})
	for _, id := range []string{"10", "11", "12"} {
		if _, ok := dst[id]; !ok {
			t.Fatalf("missing %s in %v", id, dst)
		}
	}
}

func TestValidateNumericRepoInAllowlist(t *testing.T) {
	if err := ValidateNumericRepoInAllowlist("skip-alias", map[string]struct{}{}); err != nil {
		t.Fatalf("non-numeric flag should skip: %v", err)
	}
	allowed := map[string]struct{}{"4951346": {}}
	if err := ValidateNumericRepoInAllowlist("4951346", allowed); err != nil {
		t.Fatal(err)
	}
	err := ValidateNumericRepoInAllowlist("9999999", allowed)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	if !strings.Contains(err.Error(), "9999999") || !strings.Contains(err.Error(), "reachable") {
		t.Fatalf("message: %v", err)
	}
	if !strings.Contains(err.Error(), "profile.repositories") {
		t.Fatalf("hint: %v", err)
	}
	err = ValidateNumericRepoInAllowlist("123", map[string]struct{}{})
	if err == nil || !strings.Contains(err.Error(), "reachable") {
		t.Fatalf("empty allowlist should still reject numeric: %v", err)
	}
}
