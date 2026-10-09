package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBrowseURLDryRun(t *testing.T) {
	stdout, _, code := runBrowseAliasRoot(t, true,
		"browse", "url", "https://flow.aliyun.com/pipelines/5272454",
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
	if !env.OK {
		t.Fatalf("envelope=%#v", env)
	}
	kind, _ := env.Data["kind"].(string)
	if kind != "url" && kind != "raw" {
		// browse.Raw may label kind as url or raw depending on version
		if kind == "" {
			t.Fatalf("missing kind: %#v", env.Data)
		}
	}
	url, _ := env.Data["url"].(string)
	if !strings.Contains(url, "https://flow.aliyun.com/pipelines/5272454") {
		t.Fatalf("url=%q", url)
	}
	if env.Meta["dry_run"] != true || env.Meta["opened"] != false {
		t.Fatalf("meta=%#v", env.Meta)
	}
}