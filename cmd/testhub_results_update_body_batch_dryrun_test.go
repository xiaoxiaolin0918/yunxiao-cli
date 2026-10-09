package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func thResultsUpdSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-results-upd-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-results-upd-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, testhubResultsUpdateCmd, "plan-id", "testcase-id", "status", "executor")
	return stdout, hits
}

func thResultsUpdAssert(t *testing.T, stdout *bytes.Buffer, hits *int, check func(t *testing.T, body map[string]any, req map[string]any)) {
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
	if !strings.Contains(url, "/testPlans/plan-batch/testcases/tc-batch") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "PUT" {
		t.Fatalf("method=%v", req["method"])
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

func TestTesthubResultsUpdateFailedDryRun(t *testing.T) {
	stdout, hits := thResultsUpdSetup(t, "failed")
	rootCmd.SetArgs([]string{
		"testhub", "results", "update",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--status", "FAILED",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thResultsUpdAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["status"] != "FAILED" {
			t.Fatalf("status=%v", body["status"])
		}
		if _, has := body["executor"]; has {
			t.Fatalf("executor must be omitted: %#v", body)
		}
	})
}

func TestTesthubResultsUpdateBlockedDryRun(t *testing.T) {
	stdout, hits := thResultsUpdSetup(t, "blocked")
	rootCmd.SetArgs([]string{
		"testhub", "results", "update",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--status", "BLOCKED",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thResultsUpdAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["status"] != "BLOCKED" {
			t.Fatalf("status=%v", body["status"])
		}
	})
}

func TestTesthubResultsUpdateSkippedDryRun(t *testing.T) {
	stdout, hits := thResultsUpdSetup(t, "skipped")
	rootCmd.SetArgs([]string{
		"testhub", "results", "update",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--status", "SKIPPED",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thResultsUpdAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["status"] != "SKIPPED" {
			t.Fatalf("status=%v", body["status"])
		}
	})
}

func TestTesthubResultsUpdateExecutorOnlyDryRun(t *testing.T) {
	stdout, hits := thResultsUpdSetup(t, "exec-only")
	rootCmd.SetArgs([]string{
		"testhub", "results", "update",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--executor", "uid-exec-1",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thResultsUpdAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["executor"] != "uid-exec-1" {
			t.Fatalf("executor=%v", body["executor"])
		}
		if _, has := body["status"]; has {
			t.Fatalf("status must be omitted: %#v", body)
		}
	})
}

func TestTesthubResultsUpdateStatusAndExecutorDryRun(t *testing.T) {
	stdout, hits := thResultsUpdSetup(t, "both")
	rootCmd.SetArgs([]string{
		"testhub", "results", "update",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--status", "PASSED",
		"--executor", "uid-exec-2",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thResultsUpdAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["status"] != "PASSED" || body["executor"] != "uid-exec-2" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestTesthubResultsUpdateEmptyStatusOmitsWithExecutorDryRun(t *testing.T) {
	stdout, hits := thResultsUpdSetup(t, "empty-status")
	rootCmd.SetArgs([]string{
		"testhub", "results", "update",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--status", "",
		"--executor", "uid-only",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thResultsUpdAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["executor"] != "uid-only" {
			t.Fatalf("executor=%v", body["executor"])
		}
		if _, has := body["status"]; has {
			t.Fatalf("empty status must omit: %#v", body)
		}
	})
}

func TestTesthubResultsUpdateEmptyExecutorOmitsWithStatusDryRun(t *testing.T) {
	stdout, hits := thResultsUpdSetup(t, "empty-exec")
	rootCmd.SetArgs([]string{
		"testhub", "results", "update",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--status", "RETEST",
		"--executor", "",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thResultsUpdAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["status"] != "RETEST" {
			t.Fatalf("status=%v", body["status"])
		}
		if _, has := body["executor"]; has {
			t.Fatalf("empty executor must omit: %#v", body)
		}
	})
}

func TestTesthubResultsUpdateFailedWithExecutorDryRun(t *testing.T) {
	stdout, hits := thResultsUpdSetup(t, "failed-exec")
	rootCmd.SetArgs([]string{
		"testhub", "results", "update",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--status", "FAILED",
		"--executor", "uid-fail",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thResultsUpdAssert(t, stdout, hits, func(t *testing.T, body map[string]any, req map[string]any) {
		if body["status"] != "FAILED" || body["executor"] != "uid-fail" {
			t.Fatalf("body=%v", body)
		}
		url, _ := req["url"].(string)
		if !strings.Contains(url, "/testPlans/plan-batch/") || !strings.Contains(url, "/testcases/tc-batch") {
			t.Fatalf("url=%q", url)
		}
	})
}