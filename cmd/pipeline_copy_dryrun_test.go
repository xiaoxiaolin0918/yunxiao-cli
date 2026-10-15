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

func TestPipelineCopyDryRun(t *testing.T) {
	var gets, posts int
	yaml := "sources: []\nstages:\n  build:\n    name: build\n    jobs: {}\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pipelines/src-99"):
			gets++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   "src-99",
				"name": "source-pipe",
				"pipelineConfig": map[string]any{
					"flow": yaml,
				},
			})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/pipelines"):
			posts++
			w.WriteHeader(500)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pipeline-copy-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipeline-copy")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, pipelineCopyCmd, "id", "name")
	rootCmd.SetArgs([]string{
		"pipeline", "copy",
		"--id", "src-99",
		"--name", "copied-pipe",
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
	if env.Risk != string(risk.HighRiskWrite) && env.Risk != "high-risk-write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/pipelines") {
		t.Fatalf("url=%q", url)
	}
	if strings.Contains(url, "/pipelines/src-99") {
		t.Fatalf("create URL must not include source id: %q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil {
		t.Fatalf("body nil: %v", req)
	}
	if body["name"] != "copied-pipe" {
		t.Fatalf("name=%v", body["name"])
	}
	if body["source_pipeline_id"] != "src-99" {
		t.Fatalf("source_pipeline_id=%v", body["source_pipeline_id"])
	}
	if body["source_name"] != "source-pipe" {
		t.Fatalf("source_name=%v", body["source_name"])
	}
	bytesF, ok := body["content_bytes"].(float64)
	if !ok || int(bytesF) != len(yaml) {
		t.Fatalf("content_bytes=%v want %d", body["content_bytes"], len(yaml))
	}
	if _, has := body["content"]; has {
		t.Fatalf("raw content must not be in preview: %#v", body)
	}
	if gets != 1 {
		t.Fatalf("gets=%d want 1", gets)
	}
	if posts != 0 {
		t.Fatalf("posts=%d want 0", posts)
	}
}

func TestPipelineCopyCloneAliasDryRun(t *testing.T) {
	var gets int
	yaml := "sources: []\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name": "n",
				"pipelineConfig": map[string]any{"flow": yaml},
			})
			return
		}
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pipeline-clone-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipeline-clone")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, pipelineCopyCmd, "id", "name")
	rootCmd.SetArgs([]string{
		"pipeline", "clone",
		"--id", "1",
		"--name", "cloned",
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
	if gets != 1 {
		t.Fatalf("gets=%d", gets)
	}
}
