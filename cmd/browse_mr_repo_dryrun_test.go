package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBrowseMRDryRun(t *testing.T) {
	stdout, _, code := runBrowseAliasRoot(t, true,
		"browse", "mr",
		"--repo-url", "https://codeup.aliyun.com/org/demo",
		"--local-id", "42",
	)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	var env struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout)
	}
	if !env.OK || env.Data["kind"] != "mr" {
		t.Fatalf("envelope=%#v", env)
	}
	url, _ := env.Data["url"].(string)
	if !strings.Contains(url, "42") || !strings.Contains(url, "codeup.aliyun.com") {
		t.Fatalf("url=%q", url)
	}
	if env.Meta["dry_run"] != true || env.Meta["opened"] != false {
		t.Fatalf("meta=%#v", env.Meta)
	}
}

func TestBrowseRepoDryRun(t *testing.T) {
	stdout, _, code := runBrowseAliasRoot(t, true,
		"browse", "repo",
		"--repo-url", "https://codeup.aliyun.com/org/demo",
	)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	var env struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout)
	}
	if !env.OK || env.Data["kind"] != "repo" {
		t.Fatalf("envelope=%#v", env)
	}
	url, _ := env.Data["url"].(string)
	if !strings.Contains(url, "codeup.aliyun.com/org/demo") {
		t.Fatalf("url=%q", url)
	}
	if env.Meta["dry_run"] != true || env.Meta["opened"] != false {
		t.Fatalf("meta=%#v", env.Meta)
	}
}