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

func TestOrganizationMembersSearchPageDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-org-members-search-page-not-real")
	t.Setenv(config.EnvOrganizationID, "org-members-search-page")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, orgMembersSearchCmd, "query")
	for _, name := range []string{"page", "per-page"} {
		f := orgMembersSearchCmd.Flags().Lookup(name)
		if f != nil {
			_ = orgMembersSearchCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	if f := orgMembersSearchCmd.Flags().Lookup("include-aliyun-uid"); f != nil {
		_ = orgMembersSearchCmd.Flags().Set("include-aliyun-uid", "false")
		f.Changed = false
	}
	rootCmd.SetArgs([]string{
		"organization", "members", "search",
		"--query", "bob",
		"--page", "3",
		"--per-page", "40",
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
	if !strings.Contains(url, "/members:search") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["query"] != "bob" {
		t.Fatalf("body=%v", body)
	}
	page, _ := body["page"].(float64)
	perPage, _ := body["perPage"].(float64)
	if page != 3 || perPage != 40 {
		t.Fatalf("page=%v perPage=%v", body["page"], body["perPage"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}