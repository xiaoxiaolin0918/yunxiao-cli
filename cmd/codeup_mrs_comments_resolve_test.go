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

func TestMrsCommentsResolveRegisteredAndHelp(t *testing.T) {
	foundR, foundO := false, false
	for _, c := range codeupMrsCommentsCmd.Commands() {
		switch c.Name() {
		case "resolve":
			foundR = true
		case "reopen":
			foundO = true
		}
	}
	if !foundR || !foundO {
		t.Fatalf("resolve/reopen not registered: resolve=%v reopen=%v", foundR, foundO)
	}
	h := codeupMrsCommentsResolveCmd.Long
	if !strings.Contains(h, "UpdateChangeRequestComment") || !strings.Contains(h, "resolved") {
		t.Fatalf("resolve Long missing OpenAPI hint: %s", h)
	}
}

func TestMrsCommentsResolveDryRunBody(t *testing.T) {
	var gotMethods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethods = append(gotMethods, r.Method)
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-comment-resolve-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-comment-resolve-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsCommentsResolveCmd, "repo", "local-id", "comment-biz-id")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "comments", "resolve",
		"--repo", "4951320",
		"--local-id", "125",
		"--comment-biz-id", "biz-abc",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("envelope: %+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	if req["method"] != "PUT" {
		t.Fatalf("method=%v", req["method"])
	}
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/comments/biz-abc") {
		t.Fatalf("url=%q", url)
	}
	body, _ := req["body"].(map[string]any)
	if body["resolved"] != true {
		t.Fatalf("body=%#v", body)
	}
	for _, m := range gotMethods {
		if m == http.MethodPut || m == http.MethodPost {
			t.Fatalf("mutating during dry-run: %v", gotMethods)
		}
	}
}

func TestMrsCommentsReopenDryRunBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-comment-reopen-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-comment-reopen-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsCommentsReopenCmd, "repo", "local-id", "comment-biz-id")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "comments", "reopen",
		"--repo", "4951320",
		"--local-id", "125",
		"--comment-biz-id", "biz-abc",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout.Bytes())
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	if req["method"] != "PUT" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body["resolved"] != false {
		t.Fatalf("body=%#v want resolved=false", body)
	}
}
