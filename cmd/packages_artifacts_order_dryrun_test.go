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

func TestPackagesArtifactsListOrderSortDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-packages-art-order-not-real")
	t.Setenv(config.EnvOrganizationID, "org-packages-art-order")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, packagesArtifactsListCmd, "repo-id", "repo-type", "search", "order-by", "sort")
	for _, name := range []string{"page", "per-page"} {
		f := packagesArtifactsListCmd.Flags().Lookup(name)
		if f != nil {
			_ = packagesArtifactsListCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "list",
		"--repo-id", "repo-99",
		"--repo-type", "NPM",
		"--search", "cli",
		"--order-by", "name",
		"--sort", "asc",
		"--page", "3",
		"--per-page", "15",
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
	if !strings.Contains(url, "/repositories/repo-99/artifacts") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "repoType=NPM") {
		t.Fatalf("repoType missing url=%q", url)
	}
	if !strings.Contains(url, "search=cli") {
		t.Fatalf("search missing url=%q", url)
	}
	if !strings.Contains(url, "orderBy=name") {
		t.Fatalf("orderBy missing url=%q", url)
	}
	if !strings.Contains(url, "sort=asc") {
		t.Fatalf("sort missing url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}