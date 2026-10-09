package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBrowseWorkitemDryRun(t *testing.T) {
	stdout, _, code := runBrowseAliasRoot(t, true,
		"browse", "workitem",
		"--space-id", "space-9",
		"--serial", "ZYPT-42",
		"--category", "Bug",
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
	if !env.OK || env.Data["kind"] != "workitem" {
		t.Fatalf("envelope=%#v", env)
	}
	url, _ := env.Data["url"].(string)
	if !strings.Contains(url, "space-9") || !strings.Contains(url, "ZYPT-42") {
		t.Fatalf("url=%q", url)
	}
	if env.Meta["dry_run"] != true || env.Meta["opened"] != false {
		t.Fatalf("meta=%#v", env.Meta)
	}
}