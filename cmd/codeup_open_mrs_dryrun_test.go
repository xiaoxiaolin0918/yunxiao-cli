package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

func TestCodeupOpenMrsDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-codeup-open-mrs-not-real")
	t.Setenv(config.EnvOrganizationID, "org-codeup-open-mrs")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupOpenMrsShortcut, "state", "search", "repo")
	_ = codeupOpenMrsShortcut.Flags().Set("page", "1")
	_ = codeupOpenMrsShortcut.Flags().Set("per-page", "20")
	rootCmd.SetArgs([]string{
		"codeup", "+open-mrs",
		"--repo", "4951320",
		"--search", "feat",
		"--page", "2",
		"--per-page", "10",
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
	urlStr, _ := req["url"].(string)
	if !strings.Contains(urlStr, "/changeRequests") {
		t.Fatalf("url=%q", urlStr)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	parsed, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := parsed.Query()
	if q.Get("state") != "opened" {
		t.Fatalf("state=%q query=%v", q.Get("state"), q)
	}
	if q.Get("projectIds") != "4951320" {
		t.Fatalf("projectIds=%q", q.Get("projectIds"))
	}
	if q.Get("search") != "feat" || q.Get("page") != "2" || q.Get("perPage") != "10" {
		t.Fatalf("query=%v", q)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}