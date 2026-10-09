package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// +failed shares Run with pipeline run failed; cover the shortcut entry path.
func TestPipelineFailedShortcutDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pipeline-failed-sc-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipeline-failed-sc")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, pipelineFailedShortcut, "pipeline-id")
	resetStringFlags(t, pipelineRunFailedCmd, "pipeline-id")
	_ = pipelineFailedShortcut.Flags().Set("per-page", "5")
	_ = pipelineRunFailedCmd.Flags().Set("per-page", "5")
	rootCmd.SetArgs([]string{
		"pipeline", "+failed",
		"--pipeline-id", "pipe-9",
		"--per-page", "3",
		"--dry-run",
	})
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
	urlStr, _ := req["url"].(string)
	if !strings.Contains(urlStr, "/pipelines/pipe-9/runs") {
		t.Fatalf("url=%q", urlStr)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	parsed, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := parsed.Query()
	if q.Get("status") != "FAIL" {
		t.Fatalf("status=%q query=%v", q.Get("status"), q)
	}
	if q.Get("perPage") != "3" {
		t.Fatalf("perPage=%q", q.Get("perPage"))
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}