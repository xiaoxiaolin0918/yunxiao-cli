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

func TestPipelineHostGroupsListIdsDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pipe-hg-ids-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipe-hg-ids")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, pipelineHGListCmd, "name", "ids")
	for _, name := range []string{"page", "per-page"} {
		f := pipelineHGListCmd.Flags().Lookup(name)
		if f != nil {
			_ = pipelineHGListCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	rootCmd.SetArgs([]string{
		"pipeline", "host-groups", "list",
		"--name", "ci-hosts",
		"--ids", "hg-1,hg-2",
		"--page", "2",
		"--per-page", "25",
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
	if !strings.Contains(url, "/hostGroups") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "name=ci-hosts") {
		t.Fatalf("name missing url=%q", url)
	}
	if !strings.Contains(url, "ids=hg-1") {
		t.Fatalf("ids missing url=%q", url)
	}
	if !strings.Contains(url, "page=2") || !strings.Contains(url, "perPage=25") {
		t.Fatalf("pagination missing url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}