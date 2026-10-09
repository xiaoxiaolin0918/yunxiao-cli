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

func TestCodeupMrsListQueryDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-codeup-mrs-list-q-not-real")
	t.Setenv(config.EnvOrganizationID, "org-codeup-mrs-list-q")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsListCmd, "state", "search", "status", "repo", "sort", "source", "target")
	for _, name := range []string{"page", "per-page"} {
		f := codeupMrsListCmd.Flags().Lookup(name)
		if f != nil {
			_ = codeupMrsListCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	if f := codeupMrsListCmd.Flags().Lookup("all"); f != nil {
		_ = codeupMrsListCmd.Flags().Set("all", "false")
		f.Changed = false
	}
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "list",
		"--repo", "4951346",
		"--state", "opened",
		"--search", "feat",
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
	if !strings.Contains(url, "/changeRequests") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "projectIds=4951346") {
		t.Fatalf("projectIds missing url=%q", url)
	}
	if !strings.Contains(url, "state=opened") {
		t.Fatalf("state missing url=%q", url)
	}
	if !strings.Contains(url, "search=feat") {
		t.Fatalf("search missing url=%q", url)
	}
	if !strings.Contains(url, "page=2") || !strings.Contains(url, "perPage=30") {
		t.Fatalf("pagination missing url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}