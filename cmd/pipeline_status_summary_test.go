package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

func TestPipelineStatusShortcutSummaryOmitsGlobalParams(t *testing.T) {
	const body = `{
  "pipelineRunId": 88,
  "status": "SUCCESS",
  "resultStatus": "SUCCESS",
  "createTime": 1710000000000,
  "endTime": 1710000100000,
  "globalParams": [{"key":"NACOS_PASSWORD","value":"nacos"},{"key":"APP_PATH","value":"/opt/app"}],
  "stages":[{"name":"build","status":"SUCCESS","jobs":[{"id":"j1","name":"compile","status":"SUCCESS"}]}]
}`
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/runs/latestPipelineRun") {
			hits++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, body)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pipeline-status-summary-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipeline-status-summary")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout := withCmdJSONCapture(t)
	prevDry := globalDryRun
	globalDryRun = false
	t.Cleanup(func() { globalDryRun = prevDry })
	resetStringFlags(t, pipelineStatusShortcut, "pipeline-id", "full")
	rootCmd.SetArgs([]string{"pipeline", "+status", "--pipeline-id", "4432894"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	if env.Meta == nil || env.Meta["projection"] != "summary" {
		t.Fatalf("meta=%#v", env.Meta)
	}
	raw, _ := json.Marshal(env.Data)
	s := string(raw)
	for _, want := range []string{`"runId":"88"`, `"pipeline_id":"4432894"`, `"status":"SUCCESS"`, `"stages":[`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s: %s", want, s)
		}
	}
	for _, no := range []string{"NACOS_PASSWORD", "globalParams", "APP_PATH"} {
		if strings.Contains(s, no) {
			t.Fatalf("must not contain %s: %s", no, s)
		}
	}
	if hits != 1 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestPipelineStatusShortcutFullKeepsGlobalParamsAndRunID(t *testing.T) {
	const body = `{
  "pipelineRunId": 88,
  "status": "FAIL",
  "globalParams": [{"key":"NACOS_PASSWORD","value":"nacos"}]
}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/runs/latestPipelineRun") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, body)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pipeline-status-full-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipeline-status-full")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout := withCmdJSONCapture(t)
	prevDry := globalDryRun
	globalDryRun = false
	t.Cleanup(func() { globalDryRun = prevDry })
	resetStringFlags(t, pipelineStatusShortcut, "pipeline-id", "full")
	rootCmd.SetArgs([]string{"pipeline", "+status", "--pipeline-id", "4432894", "--full"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	if env.Meta != nil {
		if _, has := env.Meta["projection"]; has {
			t.Fatalf("--full must not set projection: %#v", env.Meta)
		}
	}
	raw, _ := json.Marshal(env.Data)
	s := string(raw)
	if !strings.Contains(s, `"runId":"88"`) {
		t.Fatalf("runId missing: %s", s)
	}
	if !strings.Contains(s, "NACOS_PASSWORD") {
		t.Fatalf("--full should keep globalParams: %s", s)
	}
}

func TestPipelineStatusShortcutDryRunProjection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-pipeline-status-dry-proj-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipeline-status-dry-proj")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	prevDry := globalDryRun
	globalDryRun = true
	t.Cleanup(func() { globalDryRun = prevDry })
	resetStringFlags(t, pipelineStatusShortcut, "pipeline-id", "full")
	rootCmd.SetArgs([]string{"pipeline", "+status", "--pipeline-id", "pipe-1", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	proj, _ := req["projection"].(map[string]any)
	if proj["mode"] != "summary" {
		t.Fatalf("projection=%#v", proj)
	}
}