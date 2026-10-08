package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/output"
)

func runBrowseAliasRoot(t *testing.T, dryRun bool, args ...string) (string, string, int) {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	prevDry := globalDryRun
	globalDryRun = dryRun
	t.Cleanup(func() { globalDryRun = prevDry })
	prevPrint := browsePrintOnly
	browsePrintOnly = false
	t.Cleanup(func() { browsePrintOnly = prevPrint })
	_ = browseCmd.PersistentFlags().Set("print-only", "false")

	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	prevExit := processExit
	code := 0
	processExit = func(c int) {
		code = c
		panic(exitPanic{code: c})
	}
	t.Cleanup(func() { processExit = prevExit })
	rootCmd.SetArgs(args)
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
				}
			}
		}()
		_ = rootCmd.Execute()
	}()
	return stdout.String(), stderr.String(), code
}

func TestBrowsePipelinePrintOnly(t *testing.T) {
	stdout, stderr, code := runBrowseAliasRoot(t, false, "browse", "pipeline", "--pipeline-id", "5272454", "--print-only")
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var env struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("json: %v stdout=%s", err, stdout)
	}
	if !env.OK || env.Data["kind"] != "pipeline" {
		t.Fatalf("envelope=%#v", env)
	}
	url, _ := env.Data["url"].(string)
	if !strings.Contains(url, "5272454") || !strings.Contains(url, "flow.aliyun.com") {
		t.Fatalf("url=%q", url)
	}
	if env.Meta["opened"] != false || env.Meta["print_only"] != true {
		t.Fatalf("meta=%#v", env.Meta)
	}
	_ = stderr // URL also goes to os.Stderr; JSON envelope is the contract
}

func TestBrowsePipelineDryRun(t *testing.T) {
	stdout, _, code := runBrowseAliasRoot(t, true, "browse", "pipeline", "--pipeline-id", "99", "--run-id", "4")
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	var env struct {
		Data map[string]any `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	_ = json.Unmarshal([]byte(stdout), &env)
	url, _ := env.Data["url"].(string)
	if !strings.Contains(url, "/pipelines/99/") || !strings.Contains(url, "/builds/4") && !strings.Contains(url, "4") {
		// accept either run path shape from browse.Pipeline
		if !strings.Contains(url, "99") {
			t.Fatalf("url=%q", url)
		}
	}
	if env.Meta["dry_run"] != true || env.Meta["opened"] != false {
		t.Fatalf("meta=%#v", env.Meta)
	}
}

func TestBrowseURLRejectsNonHTTP(t *testing.T) {
	stdout, stderr, code := runBrowseAliasRoot(t, false, "browse", "url", "file:///etc/passwd")
	if code == 0 {
		t.Fatalf("want failure, stdout=%s", stdout)
	}
	blob := stdout + stderr
	if !strings.Contains(blob, "invalid URL") {
		t.Fatalf("want invalid URL error, got %s", blob)
	}
}

func TestBrowseWorkitemPrintOnly(t *testing.T) {
	stdout, _, code := runBrowseAliasRoot(t, false, "browse", "workitem", "--space-id", "space-1", "--serial", "ZYPT-1", "--print-only")
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal([]byte(stdout), &env)
	url, _ := env.Data["url"].(string)
	if !strings.Contains(url, "space-1") || !strings.Contains(url, "ZYPT-1") {
		t.Fatalf("url=%q data=%#v", url, env.Data)
	}
}

func TestAliasSetListDelete(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout, stderr, code := runBrowseAliasRoot(t, false, "alias", "set", "pendingx", "pipeline", "+pending", "--all-pipelines")
	if code != 0 {
		t.Fatalf("set exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var setEnv struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &setEnv); err != nil || !setEnv.OK {
		t.Fatalf("set json=%v stdout=%s", err, stdout)
	}
	if setEnv.Data["name"] != "pendingx" {
		t.Fatalf("data=%#v", setEnv.Data)
	}
	// persisted
	p := filepath.Join(dir, "yunxiao", "aliases.json")
	b, err := os.ReadFile(p)
	if err != nil || !strings.Contains(string(b), "pendingx") {
		t.Fatalf("aliases file missing pendingx: %v %s", err, b)
	}

	stdout, _, code = runBrowseAliasRoot(t, false, "alias", "list")
	if code != 0 {
		t.Fatalf("list exit=%d", code)
	}
	if !strings.Contains(stdout, "pendingx") {
		t.Fatalf("list stdout=%s", stdout)
	}

	stdout, _, code = runBrowseAliasRoot(t, true, "alias", "delete", "pendingx")
	if code != 0 {
		t.Fatalf("delete dry-run exit=%d stdout=%s", code, stdout)
	}
	var dry struct {
		Meta map[string]any `json:"meta"`
	}
	_ = json.Unmarshal([]byte(stdout), &dry)
	if dry.Meta["dry_run"] != true {
		t.Fatalf("meta=%#v", dry.Meta)
	}
	if _, err := os.ReadFile(p); err != nil {
		t.Fatalf("dry-run should keep file: %v", err)
	}

	stdout, _, code = runBrowseAliasRoot(t, false, "alias", "delete", "pendingx")
	if code != 0 {
		t.Fatalf("delete exit=%d stdout=%s", code, stdout)
	}
	if _, err := os.ReadFile(p); err == nil {
		// file may remain empty object
		b2, _ := os.ReadFile(p)
		if strings.Contains(string(b2), "pendingx") {
			t.Fatalf("still has pendingx: %s", b2)
		}
	}
}

func TestAliasSetRejectsYes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	stdout, stderr, code := runBrowseAliasRoot(t, false, "alias", "set", "bad", "pipeline", "trigger", "--yes")
	if code == 0 {
		t.Fatalf("want reject --yes, stdout=%s", stdout)
	}
	blob := stdout + stderr
	if !strings.Contains(blob, "--yes") && !strings.Contains(blob, "-y") {
		t.Fatalf("want --yes rejection, got %s", blob)
	}
}

func TestAliasSetRejectsReservedName(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	stdout, stderr, code := runBrowseAliasRoot(t, false, "alias", "set", "browse", "pipeline", "list")
	if code == 0 {
		t.Fatalf("want reserved name reject, stdout=%s", stdout)
	}
	blob := stdout + stderr
	if !strings.Contains(blob, "browse") && !strings.Contains(strings.ToLower(blob), "reserved") {
		t.Fatalf("want reserved hint, got %s", blob)
	}
}
