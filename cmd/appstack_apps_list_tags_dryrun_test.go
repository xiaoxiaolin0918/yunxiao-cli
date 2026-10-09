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

// Dry-run for appstack apps list --tags / --next-token / --sort
// (order-by alone covered by TestAppstackAppsListDryRun).
func TestAppstackAppsListTagsNextTokenDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-apps-tags-not-real")
	t.Setenv(config.EnvOrganizationID, "org-apps-tags")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackAppsListCmd, "next-token", "order-by", "sort", "tags")
	rootCmd.SetArgs([]string{
		"appstack", "apps", "list",
		"--tags", "prod,core",
		"--next-token", "tok-9",
		"--order-by", "name",
		"--sort", "desc",
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
	if !strings.Contains(url, "/apps:search") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "tags=prod") || !strings.Contains(url, "core") {
		t.Fatalf("missing tags: url=%q", url)
	}
	if !strings.Contains(url, "nextToken=tok-9") {
		t.Fatalf("missing nextToken: url=%q", url)
	}
	if !strings.Contains(url, "orderBy=name") || !strings.Contains(url, "sort=desc") {
		t.Fatalf("missing order/sort: url=%q", url)
	}
	if !strings.Contains(url, "pagination=keyset") {
		t.Fatalf("missing pagination=keyset: url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}