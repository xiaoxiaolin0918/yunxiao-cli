package zhiyi

import "testing"

func TestTestCaseURL(t *testing.T) {
	u := TestCaseURL("sid", "rid", "HEGL-1688")
	want := "https://devops.aliyun.com/testhub/case/HEGL-1688"
	if u != want {
		t.Fatalf("got %q want %q", u, want)
	}
	// No reliable official hash on the internal case id; fall back to the repo page.
	fallback := TestCaseURL("sid", "rid", "")
	wantRepo := "https://devops.aliyun.com/testhub/repo/rid/case"
	if fallback != wantRepo {
		t.Fatalf("fallback got %q want %q", fallback, wantRepo)
	}
	if TestCaseURL("", "", "C") != "https://devops.aliyun.com/testhub/case/C" {
		t.Fatalf("customCode alone should be enough, got %q", TestCaseURL("", "", "C"))
	}
	if TestCaseURL("sid", "", "") != "" || TestCaseURL("", "", "") != "" {
		t.Fatal("empty expected when both repoId and customCode are missing")
	}
}

func TestTestRepoURL(t *testing.T) {
	u := TestRepoURL("sid", "627d7f45b45be0ece02f93d246")
	want := "https://devops.aliyun.com/testhub/repo/627d7f45b45be0ece02f93d246/case"
	if u != want {
		t.Fatalf("got %q want %q", u, want)
	}
	if TestRepoURL("sid", "") != "" || TestRepoURL("", "") != "" {
		t.Fatal("repo URL requires repoId, not spaceId")
	}
}

func TestTestDirectoryURL(t *testing.T) {
	u := TestDirectoryURL("sid", "627d7f45b45be0ece02f93d246", "e82550461056cea35168705b02")
	want := "https://devops.aliyun.com/testhub/repo/627d7f45b45be0ece02f93d246/case#selectedTreeIdentifier=e82550461056cea35168705b02"
	if u != want {
		t.Fatalf("got %q want %q", u, want)
	}
	if TestDirectoryURL("sid", "rid", "") != "https://devops.aliyun.com/testhub/repo/rid/case" {
		t.Fatalf("empty dir should be repo page, got %q", TestDirectoryURL("sid", "rid", ""))
	}
	if TestDirectoryURL("sid", "", "d") != "" {
		t.Fatal("directory URL requires repoId")
	}
}

func TestTestPlanURL(t *testing.T) {
	u := TestPlanURL("sid", "caa1821c698424d2a972bb72fc", "")
	want := "https://devops.aliyun.com/testhub/plan/caa1821c698424d2a972bb72fc/case"
	if u != want {
		t.Fatalf("got %q want %q", u, want)
	}
	withDir := TestPlanURL("sid", "caa1821c698424d2a972bb72fc", "03d997825eaf15c0bec271a907")
	wantDir := "https://devops.aliyun.com/testhub/plan/caa1821c698424d2a972bb72fc/case#selectedTreeIdentifier=03d997825eaf15c0bec271a907"
	if withDir != wantDir {
		t.Fatalf("got %q want %q", withDir, wantDir)
	}
	if TestPlanURL("sid", "", "") != "" || TestPlanURL("", "", "d") != "" {
		t.Fatal("plan URL requires planId, not spaceId")
	}
	if TestPlanURL("", "pid", "") != "https://devops.aliyun.com/testhub/plan/pid/case" {
		t.Fatalf("planId alone should be enough, got %q", TestPlanURL("", "pid", ""))
	}
}

func TestAttachTestRepoIDsAndURLs(t *testing.T) {
	out := AttachTestRepoIDsAndURLs([]any{
		map[string]any{"testRepoIdentifier": "r1", "name": "lib"},
	}, "sid")
	m := out.([]any)[0].(map[string]any)
	if m["id"] != "r1" {
		t.Fatalf("id=%v", m["id"])
	}
	want := "https://devops.aliyun.com/testhub/repo/r1/case"
	if m["url"] != want {
		t.Fatalf("url=%v want %v", m["url"], want)
	}
}

func TestAttachTestDirectoryURLs(t *testing.T) {
	out := AttachTestDirectoryURLs([]any{
		map[string]any{"id": "e82550461056cea35168705b02", "name": "dir"},
	}, "sid", "627d7f45b45be0ece02f93d246")
	m := out.([]any)[0].(map[string]any)
	want := "https://devops.aliyun.com/testhub/repo/627d7f45b45be0ece02f93d246/case#selectedTreeIdentifier=e82550461056cea35168705b02"
	if m["url"] != want {
		t.Fatalf("url=%v", m["url"])
	}
}

func TestAttachTestCaseURLs(t *testing.T) {
	out := AttachTestCaseURLs([]any{
		map[string]any{"id": "cid", "customCode": "HEGL-1688"},
		map[string]any{"id": "cid2"},
	}, "sid", "rid")
	items := out.([]any)
	m := items[0].(map[string]any)
	if m["url"] != "https://devops.aliyun.com/testhub/case/HEGL-1688" {
		t.Fatalf("case with customCode url=%v", m["url"])
	}
	m2 := items[1].(map[string]any)
	if m2["url"] != "https://devops.aliyun.com/testhub/repo/rid/case" {
		t.Fatalf("case without customCode should fall back to repo page, url=%v", m2["url"])
	}
}

func TestAttachTestPlanIDsAndURLs(t *testing.T) {
	out := AttachTestPlanIDsAndURLs([]any{
		map[string]any{"testPlanIdentifier": "p1", "spaceIdentifier": "sid", "name": "plan"},
	})
	m := out.([]any)[0].(map[string]any)
	if m["id"] != "p1" {
		t.Fatalf("id=%v", m["id"])
	}
	want := "https://devops.aliyun.com/testhub/plan/p1/case"
	if m["url"] != want {
		t.Fatalf("url=%v want %v", m["url"], want)
	}
}

func TestAttachTestPlanIDsAndURLsWithDirectory(t *testing.T) {
	out := AttachTestPlanIDsAndURLs([]any{
		map[string]any{"id": "p1", "directoryId": "d1"},
	})
	m := out.([]any)[0].(map[string]any)
	want := "https://devops.aliyun.com/testhub/plan/p1/case#selectedTreeIdentifier=d1"
	if m["url"] != want {
		t.Fatalf("url=%v", m["url"])
	}
}

func TestEnrichTestPlanMeta(t *testing.T) {
	meta := EnrichTestPlanMeta(map[string]any{"risk": "read"}, "sid", "", "")
	if meta["url"] != nil || meta["webPathHint"] == nil {
		t.Fatalf("expected hint without planId: %v", meta)
	}
	meta2 := EnrichTestPlanMeta(map[string]any{"risk": "read"}, "", "pid", "")
	if meta2["url"] != "https://devops.aliyun.com/testhub/plan/pid/case" || meta2["webPathHint"] != nil {
		t.Fatalf("%v", meta2)
	}
	meta3 := EnrichTestPlanMeta(map[string]any{"risk": "read"}, "", "pid", "d1")
	if meta3["url"] != "https://devops.aliyun.com/testhub/plan/pid/case#selectedTreeIdentifier=d1" {
		t.Fatalf("%v", meta3)
	}
}

func TestEnrichTestCaseMetaHint(t *testing.T) {
	meta := EnrichTestCaseMeta(map[string]any{"risk": "read"}, "", "", "")
	if meta["webPathHint"] == nil || meta["url"] != nil {
		t.Fatalf("expected hint without url: %v", meta)
	}
	meta2 := EnrichTestCaseMeta(map[string]any{"risk": "read"}, "sid", "rid", "HEGL-1")
	if meta2["url"] != "https://devops.aliyun.com/testhub/case/HEGL-1" || meta2["webPathHint"] != nil {
		t.Fatalf("%v", meta2)
	}
	meta3 := EnrichTestCaseMeta(map[string]any{"risk": "read"}, "", "rid", "")
	if meta3["url"] != "https://devops.aliyun.com/testhub/repo/rid/case" || meta3["webPathHint"] != nil {
		t.Fatalf("fallback without customCode: %v", meta3)
	}
}

func TestEnrichTestRepoMetaUsesRepoID(t *testing.T) {
	meta := EnrichTestRepoMeta(map[string]any{"risk": "read"}, "sid", "rid")
	if meta["url"] != "https://devops.aliyun.com/testhub/repo/rid/case" {
		t.Fatalf("%v", meta)
	}
	metaList := EnrichTestRepoMeta(map[string]any{"risk": "read"}, "sid", "")
	if metaList["url"] != nil || metaList["webPathHint"] == nil {
		t.Fatalf("list without repoId should hint, got %v", metaList)
	}
}

func TestAssignedToPresent(t *testing.T) {
	if AssignedToPresent(nil) || AssignedToPresent(map[string]any{}) {
		t.Fatal("empty")
	}
	if !AssignedToPresent(map[string]any{"assignedTo": "u1"}) {
		t.Fatal("string")
	}
	if !AssignedToPresent(map[string]any{"assignedTo": map[string]any{"id": "u1"}}) {
		t.Fatal("obj")
	}
	if AssignedToPresent(map[string]any{"assignedTo": ""}) {
		t.Fatal("blank")
	}
}
