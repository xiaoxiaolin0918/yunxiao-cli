package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

// Dry-run for pipeline +pending --all-pipelines previews the pipelines list GET
// (single-pipeline path covered by TestPipelinePendingDryRun).
func TestPipelinePendingAllPipelinesDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pipe-pending-all-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipe-pending-all")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, pipelinePendingShortcut, "pipeline-id")
	_ = pipelinePendingShortcut.Flags().Set("all-pipelines", "true")
	_ = pipelinePendingShortcut.Flags().Set("include-running", "false")
	_ = pipelinePendingShortcut.Flags().Set("page", "1")
	_ = pipelinePendingShortcut.Flags().Set("per-page", "20")
	t.Cleanup(func() {
		_ = pipelinePendingShortcut.Flags().Set("all-pipelines", "false")
	})
	rootCmd.SetArgs([]string{
		"pipeline", "+pending",
		"--all-pipelines",
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
	if env.Risk != string(risk.Read) && env.Risk != "read" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/pipelines") || strings.Contains(url, "/runs") {
		t.Fatalf("want pipelines list preview, url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}