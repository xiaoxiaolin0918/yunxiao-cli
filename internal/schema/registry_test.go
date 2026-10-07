package schema

import (
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func TestFindAndList(t *testing.T) {
	m := Find("codeup.mrs.create")
	if m == nil || m.Risk != risk.HighRiskWrite {
		t.Fatalf("%+v", m)
	}
	for _, id := range []string{"codeup.mrs.merge", "codeup.mrs.close", "codeup.mrs.reopen", "codeup.files.delete", "pipeline.run.cancel", "pipeline.job.retry", "pipeline.job.pass", "pipeline.job.refuse", "pipeline.create", "pipeline.update", "workitem.delete", "packages.artifacts.delete", "appstack.tags.create", "appstack.tags.bind", "appstack.variable_groups.create", "codeup.branches.create", "codeup.branches.delete", "appstack.apps.create", "versions.delete", "codeup.repos.create", "codeup.tags.create", "codeup.tags.delete", "codeup.protected_branches.create", "codeup.protected_branches.delete", "pipeline.vm_deploy.stop", "pipeline.resource_members.create", "appstack.deploy.add_hosts"} {
		mm := Find(id)
		if mm == nil || mm.Risk != risk.HighRiskWrite {
			t.Fatalf("%s %+v", id, mm)
		}
	}
	if Find("workitem.create") == nil || Find("workitem.create").Risk != risk.Write {
		t.Fatal("workitem.create")
	}
	if Find("workitem.transition") == nil || Find("workitem.transition").Risk != risk.Write {
		t.Fatal("workitem.transition")
	}
	if Find("codeup.tags.list") == nil || Find("codeup.protected_branches.list") == nil {
		t.Fatal("codeup tags/protect reads")
	}
	for _, id := range []string{"codeup.mrs.comments.create", "codeup.mrs.labels.attach", "codeup.mrs.reviewers.add", "testhub.results.update", "workitem.relations.create", "workitem.relations.delete"} {
		mm := Find(id)
		if mm == nil || mm.Risk != risk.Write {
			t.Fatalf("%s %+v", id, mm)
		}
	}
	if Find("appstack.orchestrations.list") == nil || Find("appstack.change_orders.job_logs") == nil {
		t.Fatal("appstack reads")
	}
	if Find("pipeline.get") == nil || Find("workitem.attachments.create") == nil || Find("workitem.attachments.create").Risk != risk.Write {
		t.Fatal("v0.7 reads/writes")
	}
	if Find("sprint.list") == nil || Find("organization.departments.list") == nil || Find("testhub.cases.search") == nil {
		t.Fatal("v0.8 gaps")
	}
	if Find("programs.search") == nil || Find("pipeline.vm_deploy.get") == nil || Find("appstack.release_workflows.list") == nil || Find("codeup.repos.create") == nil || Find("codeup.repos.create").Risk != risk.HighRiskWrite {
		t.Fatal("v0.9 gaps")
	}
	list := List("pipeline")
	if len(list) < 3 {
		t.Fatalf("pipeline methods=%d", len(list))
	}
	if Find("nope") != nil {
		t.Fatal()
	}
}

func TestFindWorkitemSearchAliases(t *testing.T) {
	m := Find("workitem.search")
	if m == nil || m.ID != "workitem.search" {
		t.Fatalf("%+v", m)
	}
	for _, alias := range []string{"project.searchWorkitems", "search_workitems", "searchWorkitems"} {
		a := Find(alias)
		if a == nil || a.ID != "workitem.search" {
			t.Fatalf("alias %s -> %+v", alias, a)
		}
	}
	hasAsItems := false
	for _, param := range m.Params {
		if param.Name == "as-items" {
			hasAsItems = true
		}
	}
	if !hasAsItems {
		t.Fatal("workitem.search missing as-items param")
	}
}

func TestFindUpdate(t *testing.T) {
	m := Find("update")
	if m == nil || m.ID != "update" || m.Risk != risk.Write || m.Domain != "cli" {
		t.Fatalf("%+v", m)
	}
	a := Find("self-update")
	if a == nil || a.ID != "update" {
		t.Fatalf("self-update alias -> %+v", a)
	}
	hasCheck := false
	for _, param := range m.Params {
		if param.Name == "check" {
			hasCheck = true
		}
	}
	if !hasCheck {
		t.Fatal("update missing check param")
	}
}

func TestMrsCreateReviewerParam(t *testing.T) {
	m := Find("codeup.mrs.create")
	if m == nil {
		t.Fatal("missing codeup.mrs.create")
	}
	hasReviewer := false
	for _, param := range m.Params {
		if param.Name == "reviewer" {
			hasReviewer = true
			if param.Required {
				t.Fatal("reviewer should be optional")
			}
		}
	}
	if !hasReviewer {
		t.Fatal("codeup.mrs.create missing reviewer param")
	}
	if m.Example == "" || !strings.Contains(m.Example, "--reviewer") {
		t.Fatalf("example should mention --reviewer: %q", m.Example)
	}
}

func TestMrsReviewersAddSchema(t *testing.T) {
	m := Find("codeup.mrs.reviewers.add")
	if m == nil {
		t.Fatal("missing codeup.mrs.reviewers.add")
	}
	if m.Risk != risk.Write {
		t.Fatalf("risk=%v", m.Risk)
	}
	if m.HTTPMethod != "POST" || !strings.Contains(m.Path, "/person/REVIEWER") {
		t.Fatalf("method/path=%s %s", m.HTTPMethod, m.Path)
	}
	hasReviewer := false
	for _, param := range m.Params {
		if param.Name == "reviewer" {
			hasReviewer = true
			if !param.Required {
				t.Fatal("reviewer should be required")
			}
		}
	}
	if !hasReviewer {
		t.Fatal("codeup.mrs.reviewers.add missing reviewer param")
	}
	if m.Example == "" || !strings.Contains(m.Example, "reviewers add") {
		t.Fatalf("example: %q", m.Example)
	}
}


func TestWorkitemCreateSchemaDocumentsFileFlags(t *testing.T) {
	m := Find("workitem.create")
	if m == nil {
		t.Fatal("missing workitem.create")
	}
	want := []string{"subject-file", "description-file", "custom-fields-file"}
	found := map[string]bool{}
	for _, p := range m.Params {
		for _, name := range want {
			if p.Name == name {
				found[name] = true
				if p.Required {
					t.Fatalf("%s should not be required", name)
				}
				if !strings.Contains(strings.ToLower(p.Desc), "windows") {
					t.Fatalf("%s desc should mention Windows: %q", name, p.Desc)
				}
			}
		}
	}
	for _, name := range want {
		if !found[name] {
			t.Fatalf("missing param %s", name)
		}
	}
	if !strings.Contains(m.Example, "-file") {
		t.Fatalf("example should show *-file: %s", m.Example)
	}
}


func TestWorkitemBugCreateSchemaDocumentsFileFlags(t *testing.T) {
	m := Find("workitem.bug_create")
	if m == nil {
		t.Fatal("missing workitem.bug_create")
	}
	want := []string{"title-file", "description-file"}
	found := map[string]bool{}
	for _, p := range m.Params {
		for _, name := range want {
			if p.Name == name {
				found[name] = true
				if p.Required {
					t.Fatalf("%s should not be required", name)
				}
				if !strings.Contains(strings.ToLower(p.Desc), "windows") {
					t.Fatalf("%s desc should mention Windows: %q", name, p.Desc)
				}
			}
		}
	}
	for _, name := range want {
		if !found[name] {
			t.Fatalf("missing param %s", name)
		}
	}
	if !strings.Contains(m.Example, "title-file") {
		t.Fatalf("example should show title-file: %s", m.Example)
	}
}

func TestWorkitemCommentContentFileParam(t *testing.T) {
	m := Find("workitem.comment")
	if m == nil {
		t.Fatal("missing workitem.comment")
	}
	if !strings.Contains(m.Description, "delete/update") && !strings.Contains(m.Description, "AccessKey") {
		t.Fatalf("description should mention delete/update RPC: %s", m.Description)
	}
	var hasFile, contentNotRequired bool
	for _, p := range m.Params {
		if p.Name == "content-file" {
			hasFile = true
		}
		if p.Name == "content" && !p.Required {
			contentNotRequired = true
		}
	}
	if !hasFile {
		t.Fatal("missing content-file param")
	}
	if !contentNotRequired {
		t.Fatal("content should not be required when content-file is allowed")
	}
	list := Find("workitem.comments.list")
	if list == nil {
		t.Fatal("missing workitem.comments.list")
	}
	del := Find("workitem.comments.delete")
	if del == nil || del.Risk != risk.HighRiskWrite {
		t.Fatalf("delete: %+v", del)
	}
	if !strings.Contains(del.Path, "deleteComent") {
		t.Fatalf("delete path typo: %s", del.Path)
	}
	upd := Find("workitem.comments.update")
	if upd == nil || upd.Risk != risk.Write {
		t.Fatalf("update: %+v", upd)
	}
	if !strings.Contains(upd.Path, "commentUpdate") {
		t.Fatalf("update path: %s", upd.Path)
	}
}

// #93: GLOBAL_COMMENT defaults to the latest patchset, so the flag is optional in schema.
func TestCodeupMrsCommentsCreatePatchsetOptional(t *testing.T) {
	m := Find("codeup.mrs.comments.create")
	if m == nil {
		t.Fatal("missing codeup.mrs.comments.create")
	}
	found := false
	for _, p := range m.Params {
		if p.Name == "patchset-biz-id" {
			found = true
			if p.Required {
				t.Fatal("patchset-biz-id should not be required")
			}
			if !strings.Contains(p.Desc, "latest") || !strings.Contains(p.Desc, "INLINE_COMMENT") {
				t.Fatalf("desc should document latest default + INLINE requirement: %q", p.Desc)
			}
		}
	}
	if !found {
		t.Fatal("missing param patchset-biz-id")
	}
	if strings.Contains(m.Example, "--patchset-biz-id") {
		t.Fatalf("example should omit --patchset-biz-id: %s", m.Example)
	}
}

// #132: push-review status shortcut is a read; mrs list documents the
// client-side --status filter.
func TestCodeupMrsPushReviewStatusSchema(t *testing.T) {
	m := Find("codeup.mrs.push_review_status")
	if m == nil {
		t.Fatal("missing codeup.mrs.push_review_status")
	}
	if m.Risk != risk.Read || m.HTTPMethod != "GET" {
		t.Fatalf("%+v", m)
	}
	hasLocalID := false
	for _, p := range m.Params {
		if p.Name == "local-id" {
			hasLocalID = true
		}
	}
	if !hasLocalID {
		t.Fatal("missing local-id param")
	}
	if m.Example == "" || !strings.Contains(m.Example, "+push-review-status") {
		t.Fatalf("example: %s", m.Example)
	}

	list := Find("codeup.mrs.list")
	if list == nil {
		t.Fatal("missing codeup.mrs.list")
	}
	hasStatus := false
	for _, p := range list.Params {
		if p.Name == "status" {
			hasStatus = true
			if !strings.Contains(p.Desc, "client-side") {
				t.Fatalf("status desc should say client-side: %q", p.Desc)
			}
		}
	}
	if !hasStatus {
		t.Fatal("codeup.mrs.list missing status param")
	}
	merge := Find("codeup.mrs.merge")
	if merge == nil || !strings.Contains(merge.Description, "details.mr") {
		t.Fatalf("merge schema should document the #124 error enrichment: %+v", merge)
	}
}
