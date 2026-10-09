package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func thCreateBodySetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-create-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-create-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, testhubDirectoriesCreateCmd, "repo-id", "name", "parent-id")
	resetStringFlags(t, testhubCasesCreateCmd, "repo-id", "subject", "directory-id", "data", "data-file")
	return stdout, hits
}

func thCreateBodyAssert(t *testing.T, stdout *bytes.Buffer, hits *int, wantMethod, pathSub string, check func(t *testing.T, body map[string]any, req map[string]any)) {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	if env.Risk != string(risk.Write) && env.Risk != "write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, pathSub) {
		t.Fatalf("url=%q want %q", url, pathSub)
	}
	if req["method"] != wantMethod {
		t.Fatalf("method=%v want %s", req["method"], wantMethod)
	}
	body, _ := req["body"].(map[string]any)
	if body == nil {
		t.Fatalf("body nil: %v", req)
	}
	if check != nil {
		check(t, body, req)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func TestTesthubDirectoriesCreateNameOnlyDryRun(t *testing.T) {
	stdout, hits := thCreateBodySetup(t, "dir-name")
	rootCmd.SetArgs([]string{
		"testhub", "directories", "create",
		"--repo-id", "repo-batch",
		"--name", "Suite A",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thCreateBodyAssert(t, stdout, hits, "POST", "/testRepos/repo-batch/directories", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["name"] != "Suite A" {
			t.Fatalf("body=%v", body)
		}
		if _, has := body["parentIdentifier"]; has {
			t.Fatalf("parentIdentifier must be omitted: %#v", body)
		}
	})
}

func TestTesthubDirectoriesCreateParentIdDryRun(t *testing.T) {
	stdout, hits := thCreateBodySetup(t, "dir-parent")
	rootCmd.SetArgs([]string{
		"testhub", "directories", "create",
		"--repo-id", "repo-batch",
		"--name", "Child",
		"--parent-id", "dir-parent-1",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thCreateBodyAssert(t, stdout, hits, "POST", "/testRepos/repo-batch/directories", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["name"] != "Child" || body["parentIdentifier"] != "dir-parent-1" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestTesthubDirectoriesCreateEmptyParentOmitsDryRun(t *testing.T) {
	stdout, hits := thCreateBodySetup(t, "dir-empty-parent")
	rootCmd.SetArgs([]string{
		"testhub", "directories", "create",
		"--repo-id", "repo-batch",
		"--name", "Rootish",
		"--parent-id", "",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thCreateBodyAssert(t, stdout, hits, "POST", "/testRepos/repo-batch/directories", func(t *testing.T, body map[string]any, _ map[string]any) {
		if _, has := body["parentIdentifier"]; has {
			t.Fatalf("empty parent must omit: %#v", body)
		}
	})
}

func TestTesthubCasesCreateSubjectOnlyDryRun(t *testing.T) {
	stdout, hits := thCreateBodySetup(t, "case-subj")
	rootCmd.SetArgs([]string{
		"testhub", "cases", "create",
		"--repo-id", "repo-batch",
		"--subject", "login happy path",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thCreateBodyAssert(t, stdout, hits, "POST", "/testRepos/repo-batch/testcases", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["subject"] != "login happy path" {
			t.Fatalf("body=%v", body)
		}
		if _, has := body["directoryId"]; has {
			t.Fatalf("directoryId must be omitted: %#v", body)
		}
	})
}

func TestTesthubCasesCreateSubjectAndDirectoryDryRun(t *testing.T) {
	stdout, hits := thCreateBodySetup(t, "case-dir")
	rootCmd.SetArgs([]string{
		"testhub", "cases", "create",
		"--repo-id", "repo-batch",
		"--subject", "checkout",
		"--directory-id", "dir-9",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thCreateBodyAssert(t, stdout, hits, "POST", "/testRepos/repo-batch/testcases", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["subject"] != "checkout" || body["directoryId"] != "dir-9" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestTesthubCasesCreateDataJSONDryRun(t *testing.T) {
	stdout, hits := thCreateBodySetup(t, "case-data")
	rootCmd.SetArgs([]string{
		"testhub", "cases", "create",
		"--repo-id", "repo-batch",
		"--data", `{"priority":"P1","tags":["smoke"]}`,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thCreateBodyAssert(t, stdout, hits, "POST", "/testRepos/repo-batch/testcases", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["priority"] != "P1" {
			t.Fatalf("body=%v", body)
		}
		tags, _ := body["tags"].([]any)
		if len(tags) != 1 || tags[0] != "smoke" {
			t.Fatalf("tags=%v", body["tags"])
		}
	})
}

func TestTesthubCasesCreateDataAndSubjectOverrideDryRun(t *testing.T) {
	stdout, hits := thCreateBodySetup(t, "case-override")
	rootCmd.SetArgs([]string{
		"testhub", "cases", "create",
		"--repo-id", "repo-batch",
		"--data", `{"subject":"from-data","priority":"P2"}`,
		"--subject", "from-flag",
		"--directory-id", "dir-2",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thCreateBodyAssert(t, stdout, hits, "POST", "/testRepos/repo-batch/testcases", func(t *testing.T, body map[string]any, _ map[string]any) {
		// flags applied after data merge — subject/directoryId from flags win
		if body["subject"] != "from-flag" || body["directoryId"] != "dir-2" {
			t.Fatalf("body=%v", body)
		}
		if body["priority"] != "P2" {
			t.Fatalf("priority from data lost: %v", body)
		}
	})
}

func TestTesthubCasesCreateDataFileDryRun(t *testing.T) {
	stdout, hits := thCreateBodySetup(t, "case-file")
	dir := t.TempDir()
	fp := filepath.Join(dir, "case.json")
	if err := os.WriteFile(fp, []byte(`{"subject":"from-file","steps":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{
		"testhub", "cases", "create",
		"--repo-id", "repo-batch",
		"--data-file", fp,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thCreateBodyAssert(t, stdout, hits, "POST", "/testRepos/repo-batch/testcases", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["subject"] != "from-file" {
			t.Fatalf("body=%v", body)
		}
		// JSON numbers decode as float64
		if body["steps"] != float64(3) {
			t.Fatalf("steps=%v", body["steps"])
		}
	})
}