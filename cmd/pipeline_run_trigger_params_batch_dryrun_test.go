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

func pipeTriggerSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pipe-trigger-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipe-trigger-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, pipelineRunTriggerCmd, "pipeline-id", "branch", "params", "comment")
	return stdout, hits
}

func pipeTriggerAssert(t *testing.T, stdout *bytes.Buffer, hits *int, checkParams func(t *testing.T, params any, body map[string]any)) {
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
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/pipelines/pipe-trig/runs") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil {
		t.Fatalf("body nil: %v", req)
	}
	if checkParams != nil {
		checkParams(t, body["params"], body)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func decodeParamsObject(t *testing.T, params any) map[string]any {
	t.Helper()
	switch v := params.(type) {
	case string:
		var m map[string]any
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			t.Fatalf("params string not JSON object: %q err=%v", v, err)
		}
		return m
	case map[string]any:
		return v
	default:
		t.Fatalf("unexpected params type %T=%v", params, params)
		return nil
	}
}

func TestPipelineRunTriggerBranchCommentDryRun(t *testing.T) {
	stdout, hits := pipeTriggerSetup(t, "branch-comment")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "trigger",
		"--pipeline-id", "pipe-trig",
		"--branch", "feature/x",
		"--comment", "batch-comment",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeTriggerAssert(t, stdout, hits, func(t *testing.T, params any, _ map[string]any) {
		m := decodeParamsObject(t, params)
		if m["comment"] != "batch-comment" {
			t.Fatalf("comment=%v m=%v", m["comment"], m)
		}
		brRaw, _ := json.Marshal(m["branchModeBranchs"])
		if !strings.Contains(string(brRaw), "feature/x") {
			t.Fatalf("branchModeBranchs=%v", m["branchModeBranchs"])
		}
	})
}

func TestPipelineRunTriggerBranchOnlyDryRun(t *testing.T) {
	stdout, hits := pipeTriggerSetup(t, "branch-only")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "trigger",
		"--pipeline-id", "pipe-trig",
		"--branch", "main",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeTriggerAssert(t, stdout, hits, func(t *testing.T, params any, _ map[string]any) {
		m := decodeParamsObject(t, params)
		brRaw, _ := json.Marshal(m["branchModeBranchs"])
		if !strings.Contains(string(brRaw), "main") {
			t.Fatalf("branchModeBranchs=%v", m["branchModeBranchs"])
		}
		if _, ok := m["comment"]; ok {
			t.Fatalf("unexpected comment in %v", m)
		}
	})
}

func TestPipelineRunTriggerCommentOnlyDryRun(t *testing.T) {
	stdout, hits := pipeTriggerSetup(t, "comment-only")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "trigger",
		"--pipeline-id", "pipe-trig",
		"--comment", "only-cmt",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeTriggerAssert(t, stdout, hits, func(t *testing.T, params any, _ map[string]any) {
		m := decodeParamsObject(t, params)
		if m["comment"] != "only-cmt" {
			t.Fatalf("comment=%v", m["comment"])
		}
	})
}

func TestPipelineRunTriggerParamsObjectDryRun(t *testing.T) {
	stdout, hits := pipeTriggerSetup(t, "params-obj")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "trigger",
		"--pipeline-id", "pipe-trig",
		"--params", `{"env":"prod","retry":2}`,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeTriggerAssert(t, stdout, hits, func(t *testing.T, params any, _ map[string]any) {
		m := decodeParamsObject(t, params)
		if m["env"] != "prod" {
			t.Fatalf("env=%v m=%v", m["env"], m)
		}
		// JSON numbers decode as float64
		retry, _ := m["retry"].(float64)
		if retry != 2 {
			t.Fatalf("retry=%v m=%v", m["retry"], m)
		}
	})
}

func TestPipelineRunTriggerParamsStringDryRun(t *testing.T) {
	stdout, hits := pipeTriggerSetup(t, "params-str")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "trigger",
		"--pipeline-id", "pipe-trig",
		"--params", `"raw-params-string"`,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeTriggerAssert(t, stdout, hits, func(t *testing.T, params any, _ map[string]any) {
		if params != "raw-params-string" {
			t.Fatalf("params=%v (%T)", params, params)
		}
	})
}

func TestPipelineRunTriggerParamsObjectPlusBranchDryRun(t *testing.T) {
	stdout, hits := pipeTriggerSetup(t, "params-branch")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "trigger",
		"--pipeline-id", "pipe-trig",
		"--params", `{"env":"staging"}`,
		"--branch", "release/1",
		"--comment", "ship",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeTriggerAssert(t, stdout, hits, func(t *testing.T, params any, _ map[string]any) {
		m := decodeParamsObject(t, params)
		if m["env"] != "staging" || m["comment"] != "ship" {
			t.Fatalf("m=%v", m)
		}
		brRaw, _ := json.Marshal(m["branchModeBranchs"])
		if !strings.Contains(string(brRaw), "release/1") {
			t.Fatalf("branchModeBranchs=%v", m["branchModeBranchs"])
		}
	})
}

func TestPipelineRunTriggerNoExtraFlagsDryRun(t *testing.T) {
	stdout, hits := pipeTriggerSetup(t, "empty")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "trigger",
		"--pipeline-id", "pipe-trig",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeTriggerAssert(t, stdout, hits, func(t *testing.T, params any, body map[string]any) {
		if _, ok := body["params"]; ok {
			t.Fatalf("unexpected params=%v body=%v", params, body)
		}
	})
}

func TestPipelineRunTriggerParamsStringIgnoresBranchMergeDryRun(t *testing.T) {
	// When --params is a JSON string, body["params"] is set directly and branch/comment
	// are NOT merged into paramsObj (see pipeline.go: if _, ok := body["params"]; !ok ...)
	stdout, hits := pipeTriggerSetup(t, "str-ignores-branch")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "trigger",
		"--pipeline-id", "pipe-trig",
		"--params", `"keep-me"`,
		"--branch", "should-not-merge",
		"--comment", "should-not-merge",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeTriggerAssert(t, stdout, hits, func(t *testing.T, params any, _ map[string]any) {
		if params != "keep-me" {
			t.Fatalf("params=%v want keep-me (branch/comment must not rewrite string params)", params)
		}
	})
}