package workflow

import "testing"

func TestAliasAndBugStatuses(t *testing.T) {
	statuses := []StatusInfo{
		{ID: "28", Name: "待确认", DisplayName: "待确认", NameEn: "New"},
		{ID: "100010", Name: "处理中", DisplayName: "处理中", NameEn: "In Progress"},
		{ID: "31", Name: "暂不修复", DisplayName: "暂不修复", NameEn: "Won't Fix"},
	}
	m := BuildBugStatusesMap(statuses)
	if m["confirm"] != "28" || m["processing"] != "100010" || m["wont-fix"] != "31" {
		t.Fatalf("%v", m)
	}
	snip := BuildProfileSnippet(SnippetInput{
		TypeID:          "bug-type",
		TypeName:        "缺陷",
		Category:        "Bug",
		WorkflowID:      "wf1",
		WorkflowName:    "缺陷的默认工作流",
		DefaultStatusID: "28",
		Statuses:        statuses,
		Edges:           map[string][]string{"28": {"100010"}},
	})
	if snip.BugStatuses["confirm"] != "28" || len(snip.BugEdges["28"]) != 1 {
		t.Fatalf("%+v", snip)
	}
	if snip.Workflow.TypeID != "bug-type" || snip.Workflow.Name != "缺陷" {
		t.Fatalf("workflow meta: %+v", snip.Workflow)
	}
	if snip.Workflow.Statuses["待确认"] != "28" || snip.Workflow.Statuses["confirm"] != "28" {
		t.Fatalf("statuses: %v", snip.Workflow.Statuses)
	}
	if snip.Workflow.Edges["28"][0] != "100010" {
		t.Fatalf("edges: %v", snip.Workflow.Edges)
	}
}

func TestBuildProfileSnippetReq(t *testing.T) {
	statuses := []StatusInfo{
		{ID: "100005", Name: "待处理", DisplayName: "待处理"},
		{ID: "100014", Name: "已完成", DisplayName: "已完成"},
	}
	snip := BuildProfileSnippet(SnippetInput{
		TypeID:   "req-type",
		TypeName: "产品类需求",
		Category: "Req",
		Statuses: statuses,
		Edges:    map[string][]string{"100005": {"100014"}},
	})
	if snip.BugStatuses != nil {
		t.Fatalf("expected no bug_statuses for Req, got %v", snip.BugStatuses)
	}
	if snip.Workflow.Category != "Req" || snip.Workflow.Statuses["待处理"] != "100005" {
		t.Fatalf("%+v", snip.Workflow)
	}
}

func TestResolveUniqueStatus(t *testing.T) {
	statuses := []StatusInfo{
		{ID: "141230", Name: "已取消", DisplayName: "已取消", NameEn: "Canceled"},
		{ID: "100010", Name: "处理中", DisplayName: "处理中"},
	}
	id, err := ResolveUniqueStatus("已取消", statuses)
	if err != nil || id != "141230" {
		t.Fatalf("%q %v", id, err)
	}
	id, err = ResolveUniqueStatus("Canceled", statuses)
	if err != nil || id != "141230" {
		t.Fatalf("en %q %v", id, err)
	}
	id, err = ResolveUniqueStatus("141230", statuses)
	if err != nil || id != "141230" {
		t.Fatalf("id %q %v", id, err)
	}
	if _, err := ResolveUniqueStatus("nope", statuses); err == nil {
		t.Fatal("expected miss")
	}
}


func TestBuildProfileSnippetHintedEdges(t *testing.T) {
	snip := BuildProfileSnippet(SnippetInput{
		TypeID:   "req-type",
		Category: "Req",
		Statuses: []StatusInfo{{ID: "s1", Name: "待处理", DisplayName: "待处理"}},
		Edges:    map[string][]string{"s1": {"s2"}},
		HintedEdges: map[string][]string{"s1": {"s3"}},
	})
	if len(snip.Workflow.Edges["s1"]) != 1 || snip.Workflow.Edges["s1"][0] != "s2" {
		t.Fatalf("verified edges: %v", snip.Workflow.Edges)
	}
	if len(snip.Workflow.HintedEdges["s1"]) != 1 || snip.Workflow.HintedEdges["s1"][0] != "s3" {
		t.Fatalf("hinted edges: %v", snip.Workflow.HintedEdges)
	}
}
