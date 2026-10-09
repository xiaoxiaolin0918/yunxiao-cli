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

func coExecJobSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-co-exec-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-co-exec-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, appstackCOExecuteJobCmd, "app", "sn", "job-sn", "action-type", "comment")
	return stdout, hits
}

func coExecJobAssert(t *testing.T, stdout *bytes.Buffer, hits *int, action string, wantComment string, expectContext bool) {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	if env.Risk != string(risk.HighRiskWrite) && env.Risk != "high-risk-write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	urlStr, _ := req["url"].(string)
	if !strings.Contains(urlStr, "/apps/demo-app/changeOrders/CO-batch/jobs/JOB-batch:execute") {
		t.Fatalf("url=%q", urlStr)
	}
	if req["method"] != "PUT" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["actionType"] != action {
		t.Fatalf("body=%v want actionType=%s", body, action)
	}
	ctx, _ := body["context"].(map[string]any)
	if expectContext {
		if ctx == nil || ctx["comment"] != wantComment {
			t.Fatalf("context=%v want comment=%q", body["context"], wantComment)
		}
	} else if _, ok := body["context"]; ok {
		t.Fatalf("unexpected context=%v", body["context"])
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func TestAppstackCOExecuteJobResumeWithCommentDryRun(t *testing.T) {
	stdout, hits := coExecJobSetup(t, "resume-cmt")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "execute-job",
		"--app", "demo-app", "--sn", "CO-batch", "--job-sn", "JOB-batch",
		"--action-type", "RESUME", "--comment", "go-on", "--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	coExecJobAssert(t, stdout, hits, "RESUME", "go-on", true)
}

func TestAppstackCOExecuteJobRollbackWithCommentDryRun(t *testing.T) {
	stdout, hits := coExecJobSetup(t, "rollback-cmt")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "execute-job",
		"--app", "demo-app", "--sn", "CO-batch", "--job-sn", "JOB-batch",
		"--action-type", "ROLLBACK", "--comment", "undo", "--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	coExecJobAssert(t, stdout, hits, "ROLLBACK", "undo", true)
}

func TestAppstackCOExecuteJobStopWithCommentDryRun(t *testing.T) {
	stdout, hits := coExecJobSetup(t, "stop-cmt")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "execute-job",
		"--app", "demo-app", "--sn", "CO-batch", "--job-sn", "JOB-batch",
		"--action-type", "STOP", "--comment", "halt", "--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	coExecJobAssert(t, stdout, hits, "STOP", "halt", true)
}

func TestAppstackCOExecuteJobSuspendNoCommentDryRun(t *testing.T) {
	stdout, hits := coExecJobSetup(t, "suspend-nocmt")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "execute-job",
		"--app", "demo-app", "--sn", "CO-batch", "--job-sn", "JOB-batch",
		"--action-type", "SUSPEND", "--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	coExecJobAssert(t, stdout, hits, "SUSPEND", "", false)
}

func TestAppstackCOExecuteJobResumeNoCommentDryRun(t *testing.T) {
	stdout, hits := coExecJobSetup(t, "resume-nocmt")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "execute-job",
		"--app", "demo-app", "--sn", "CO-batch", "--job-sn", "JOB-batch",
		"--action-type", "RESUME", "--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	coExecJobAssert(t, stdout, hits, "RESUME", "", false)
}

func TestAppstackCOExecuteJobRollbackNoCommentDryRun(t *testing.T) {
	stdout, hits := coExecJobSetup(t, "rollback-nocmt")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "execute-job",
		"--app", "demo-app", "--sn", "CO-batch", "--job-sn", "JOB-batch",
		"--action-type", "ROLLBACK", "--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	coExecJobAssert(t, stdout, hits, "ROLLBACK", "", false)
}

func TestAppstackCOExecuteJobStopNoCommentDryRun(t *testing.T) {
	stdout, hits := coExecJobSetup(t, "stop-nocmt")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "execute-job",
		"--app", "demo-app", "--sn", "CO-batch", "--job-sn", "JOB-batch",
		"--action-type", "STOP", "--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	coExecJobAssert(t, stdout, hits, "STOP", "", false)
}

func TestAppstackCOExecuteJobSuspendEmptyCommentOmitsContextDryRun(t *testing.T) {
	// Empty --comment should not add context (comment != "" gate).
	stdout, hits := coExecJobSetup(t, "suspend-empty-cmt")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "execute-job",
		"--app", "demo-app", "--sn", "CO-batch", "--job-sn", "JOB-batch",
		"--action-type", "SUSPEND", "--comment", "", "--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	coExecJobAssert(t, stdout, hits, "SUSPEND", "", false)
}