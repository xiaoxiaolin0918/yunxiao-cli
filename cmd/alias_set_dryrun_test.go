package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Dedicated dry-run for alias set (delete dry-run covered in TestAliasSetListDelete).
func TestAliasSetDryRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout, stderr, code := runBrowseAliasRoot(t, true, "alias", "set", "drypending", "pipeline", "+pending", "--all-pipelines", "--dry-run")
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
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
	if env.Data["name"] != "drypending" {
		t.Fatalf("data=%#v", env.Data)
	}
	exp, _ := env.Data["expansion"].([]any)
	if len(exp) < 2 || exp[0] != "pipeline" || exp[1] != "+pending" {
		t.Fatalf("expansion=%v", exp)
	}
	p := filepath.Join(dir, "yunxiao", "aliases.json")
	if _, err := os.ReadFile(p); err == nil {
		b, _ := os.ReadFile(p)
		if strings.Contains(string(b), "drypending") {
			t.Fatalf("dry-run must not persist alias: %s", b)
		}
	}
}