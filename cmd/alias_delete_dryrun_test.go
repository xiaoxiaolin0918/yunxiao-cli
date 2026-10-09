package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Dedicated dry-run for alias delete (set dry-run covered in TestAliasSetDryRun).
func TestAliasDeleteDryRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout, stderr, code := runBrowseAliasRoot(t, false, "alias", "set", "todelete", "pipeline", "list")
	if code != 0 {
		t.Fatalf("set exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	p := filepath.Join(dir, "yunxiao", "aliases.json")
	b, err := os.ReadFile(p)
	if err != nil || !strings.Contains(string(b), "todelete") {
		t.Fatalf("aliases missing todelete: %v %s", err, b)
	}

	stdout, stderr, code = runBrowseAliasRoot(t, true, "alias", "delete", "todelete", "--dry-run")
	if code != 0 {
		t.Fatalf("delete dry-run exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var env struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil || !env.OK {
		t.Fatalf("json=%v stdout=%s", err, stdout)
	}
	if env.Meta["dry_run"] != true {
		t.Fatalf("meta=%#v", env.Meta)
	}
	if env.Data["deleted"] != "todelete" {
		t.Fatalf("data=%#v", env.Data)
	}
	b2, err := os.ReadFile(p)
	if err != nil || !strings.Contains(string(b2), "todelete") {
		t.Fatalf("dry-run must keep alias on disk: %v %s", err, b2)
	}
}