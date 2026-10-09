package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

func TestPipelineListStatusListDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pipeline-list-status-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipeline-list-status")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, pipelineListCmd, "name", "status-list")
	for _, name := range []string{"page", "per-page"} {
		f := pipelineListCmd.Flags().Lookup(name)
		if f != nil {
			_ = pipelineListCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	if f := pipelineListCmd.Flags().Lookup("all"); f != nil {
		_ = pipelineListCmd.Flags().Set("all", "false")
		f.Changed = false
	}
	rootCmd.SetArgs([]string{
		"pipeline", "list",
		"--name", "demo-pipe",
		"--status-list", "SUCCESS,RUNNING",
		"--page", "2",
		"--per-page", "30",
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
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/pipelines") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "pipelineName=demo-pipe") {
		t.Fatalf("pipelineName missing url=%q", url)
	}
	if !strings.Contains(url, "statusList=") || !strings.Contains(url, "SUCCESS") {
		t.Fatalf("statusList missing url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}