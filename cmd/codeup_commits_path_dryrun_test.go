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

func TestCodeupCommitsListPathSearchDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-codeup-commits-path-not-real")
	t.Setenv(config.EnvOrganizationID, "org-codeup-commits-path")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupCommitsListCmd, "repo", "ref", "search", "path", "sort")
	for _, name := range []string{"page", "per-page"} {
		f := codeupCommitsListCmd.Flags().Lookup(name)
		if f != nil {
			_ = codeupCommitsListCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	rootCmd.SetArgs([]string{
		"codeup", "commits", "list",
		"--repo", "4951346",
		"--ref", "develop",
		"--search", "fix",
		"--path", "cmd/root.go",
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
	if !strings.Contains(url, "/repositories/4951346/commits") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "refName=develop") {
		t.Fatalf("refName missing url=%q", url)
	}
	if !strings.Contains(url, "search=fix") {
		t.Fatalf("search missing url=%q", url)
	}
	if !strings.Contains(url, "path=cmd") {
		t.Fatalf("path missing url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}