package zhiyi

import "testing"

func TestTestCaseURL(t *testing.T) {
	u := TestCaseURL("sid", "cid")
	want := "https://devops.aliyun.com/projex/project/sid/testcase#openTestcaseIdentifier=cid"
	if u != want {
		t.Fatalf("got %q", u)
	}
	if TestCaseURL("", "c") != "" || TestCaseURL("s", "") != "" {
		t.Fatal("empty expected")
	}
}

func TestTestPlanURL(t *testing.T) {
	u := TestPlanURL("sid", "pid")
	want := "https://devops.aliyun.com/projex/project/sid/testplan/pid"
	if u != want {
		t.Fatalf("got %q", u)
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
	if m["url"] == nil || m["url"] == "" {
		t.Fatalf("url missing: %v", m)
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
	want := "https://devops.aliyun.com/projex/project/sid/testplan/p1"
	if m["url"] != want {
		t.Fatalf("url=%v", m["url"])
	}
}

func TestEnrichTestCaseMetaHint(t *testing.T) {
	meta := EnrichTestCaseMeta(map[string]any{"risk": "read"}, "", "cid")
	if meta["webPathHint"] == nil || meta["url"] != nil {
		t.Fatalf("%v", meta)
	}
	meta2 := EnrichTestCaseMeta(map[string]any{"risk": "read"}, "sid", "cid")
	if meta2["url"] == nil || meta2["webPathHint"] != nil {
		t.Fatalf("%v", meta2)
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
