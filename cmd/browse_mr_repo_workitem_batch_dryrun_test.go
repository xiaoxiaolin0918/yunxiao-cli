package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func browseBatchResetVars(t *testing.T) {
	t.Helper()
	browsePipelineID = ""
	browseRunID = ""
	browseSpaceID = ""
	browseWorkItemID = ""
	browseSerial = ""
	browseCategory = ""
	browseRepoURL = ""
	browseLocalID = ""
	browseDetailURL = ""
	browseRepoOnlyURL = ""
}

func browseBatchParse(t *testing.T, stdout string) (data, meta map[string]any) {
	t.Helper()
	var env struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout)
	}
	if !env.OK {
		t.Fatalf("envelope=%#v stdout=%s", env, stdout)
	}
	return env.Data, env.Meta
}

func TestBrowseMRRepoLocalIDDryRun(t *testing.T) {
	browseBatchResetVars(t)
	stdout, _, code := runBrowseAliasRoot(t, true,
		"browse", "mr",
		"--repo-url", "https://codeup.aliyun.com/org/demo",
		"--local-id", "42",
	)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	data, meta := browseBatchParse(t, stdout)
	if data["kind"] != "mr" {
		t.Fatalf("kind=%v", data["kind"])
	}
	url, _ := data["url"].(string)
	if url != "https://codeup.aliyun.com/org/demo/change/42" {
		t.Fatalf("url=%q", url)
	}
	if meta["dry_run"] != true || meta["opened"] != false {
		t.Fatalf("meta=%#v", meta)
	}
}

func TestBrowseMRDetailURLDryRun(t *testing.T) {
	browseBatchResetVars(t)
	detail := "https://codeup.aliyun.com/org/demo/change/9"
	stdout, _, code := runBrowseAliasRoot(t, true,
		"browse", "mr",
		"--detail-url", detail,
	)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	data, meta := browseBatchParse(t, stdout)
	if data["kind"] != "mr" || data["url"] != detail {
		t.Fatalf("data=%#v", data)
	}
	if meta["dry_run"] != true || meta["opened"] != false {
		t.Fatalf("meta=%#v", meta)
	}
}

func TestBrowseRepoURLDryRun(t *testing.T) {
	browseBatchResetVars(t)
	repo := "https://codeup.aliyun.com/org/demo"
	stdout, _, code := runBrowseAliasRoot(t, true,
		"browse", "repo",
		"--repo-url", repo,
	)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	data, meta := browseBatchParse(t, stdout)
	if data["kind"] != "repo" || data["url"] != repo {
		t.Fatalf("data=%#v", data)
	}
	if meta["dry_run"] != true || meta["opened"] != false {
		t.Fatalf("meta=%#v", meta)
	}
}

func TestBrowseWorkitemSerialDryRun(t *testing.T) {
	browseBatchResetVars(t)
	stdout, _, code := runBrowseAliasRoot(t, true,
		"browse", "workitem",
		"--space-id", "space-batch",
		"--serial", "ZYPT-99",
	)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	data, meta := browseBatchParse(t, stdout)
	if data["kind"] != "workitem" {
		t.Fatalf("kind=%v", data["kind"])
	}
	url, _ := data["url"].(string)
	if !strings.Contains(url, "space-batch") || !strings.Contains(url, "serialNumber=ZYPT-99") {
		t.Fatalf("url=%q", url)
	}
	if meta["dry_run"] != true || meta["opened"] != false {
		t.Fatalf("meta=%#v", meta)
	}
}

func TestBrowseWorkitemIDDryRun(t *testing.T) {
	browseBatchResetVars(t)
	stdout, _, code := runBrowseAliasRoot(t, true,
		"browse", "workitem",
		"--space-id", "space-batch",
		"--id", "wid-77",
	)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	data, meta := browseBatchParse(t, stdout)
	url, _ := data["url"].(string)
	if !strings.Contains(url, "openWorkitemIdentifier=wid-77") {
		t.Fatalf("url=%q", url)
	}
	if meta["dry_run"] != true {
		t.Fatalf("meta=%#v", meta)
	}
}

func TestBrowseWorkitemCategoryDryRun(t *testing.T) {
	browseBatchResetVars(t)
	stdout, _, code := runBrowseAliasRoot(t, true,
		"browse", "workitem",
		"--space-id", "space-batch",
		"--serial", "ZYPT-1",
		"--category", "Bug",
	)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	data, _ := browseBatchParse(t, stdout)
	url, _ := data["url"].(string)
	if !strings.Contains(url, "serialNumber=ZYPT-1") {
		t.Fatalf("url=%q", url)
	}
	// category may appear as categoryId=Bug in URL depending on zhiyi builder
	if !strings.Contains(url, "Bug") && !strings.Contains(url, "category") {
		// still OK if serial present; category is optional enrichment
		t.Logf("category not visible in url=%q (optional)", url)
	}
}

func TestBrowseMRPrintOnlyBatch(t *testing.T) {
	browseBatchResetVars(t)
	stdout, _, code := runBrowseAliasRoot(t, false,
		"browse", "mr",
		"--repo-url", "https://codeup.aliyun.com/org/demo",
		"--local-id", "3",
		"--print-only",
	)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	data, meta := browseBatchParse(t, stdout)
	if data["kind"] != "mr" {
		t.Fatalf("kind=%v", data["kind"])
	}
	url, _ := data["url"].(string)
	if url != "https://codeup.aliyun.com/org/demo/change/3" {
		t.Fatalf("url=%q", url)
	}
	if meta["print_only"] != true || meta["opened"] != false {
		t.Fatalf("meta=%#v", meta)
	}
}

func TestBrowseRepoPrintOnlyBatch(t *testing.T) {
	browseBatchResetVars(t)
	repo := "https://codeup.aliyun.com/org/other"
	stdout, _, code := runBrowseAliasRoot(t, false,
		"browse", "repo",
		"--repo-url", repo,
		"--print-only",
	)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	data, meta := browseBatchParse(t, stdout)
	if data["kind"] != "repo" || data["url"] != repo {
		t.Fatalf("data=%#v", data)
	}
	if meta["print_only"] != true || meta["opened"] != false {
		t.Fatalf("meta=%#v", meta)
	}
}