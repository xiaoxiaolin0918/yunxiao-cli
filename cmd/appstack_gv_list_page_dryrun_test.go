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

// Dry-run for appstack global-vars list --current/--page-size query (+ search body).
func TestAppstackGlobalVarsListPageDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-appstack-gv-list-page-not-real")
	t.Setenv(config.EnvOrganizationID, "org-appstack-gv-list-page")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackGVListCmd, "search", "data", "data-file")
	for _, name := range []string{"current", "page-size"} {
		f := appstackGVListCmd.Flags().Lookup(name)
		if f != nil {
			_ = appstackGVListCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	rootCmd.SetArgs([]string{
		"appstack", "global-vars", "list",
		"--search", "DB_HOST",
		"--current", "3",
		"--page-size", "50",
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
	if !strings.Contains(url, "/globalVars:search") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "current=3") || !strings.Contains(url, "pageSize=50") {
		t.Fatalf("pagination missing url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["search"] != "DB_HOST" {
		t.Fatalf("body=%v", body)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}