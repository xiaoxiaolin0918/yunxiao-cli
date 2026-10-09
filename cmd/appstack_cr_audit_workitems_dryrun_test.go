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

func TestAppstackChangeRequestsAuditItemsDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-appstack-cr-audit-not-real")
	t.Setenv(config.EnvOrganizationID, "org-appstack-cr-audit")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackCRAuditCmd, "app", "sn", "ref-type")
	rootCmd.SetArgs([]string{
		"appstack", "change-requests", "audit-items",
		"--app", "demo-app",
		"--sn", "CR-9",
		"--ref-type", "CODE",
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
	if !strings.Contains(urlStr, "/apps/demo-app/changeRequests/CR-9/auditItems") {
		t.Fatalf("url=%q", urlStr)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	parsed, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Query().Get("refType") != "CODE" {
		t.Fatalf("query=%v", parsed.Query())
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestAppstackChangeRequestsWorkItemsDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-appstack-cr-wi-not-real")
	t.Setenv(config.EnvOrganizationID, "org-appstack-cr-wi")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackCRWorkItemsCmd, "app", "sn")
	rootCmd.SetArgs([]string{
		"appstack", "change-requests", "work-items",
		"--app", "demo-app",
		"--sn", "CR-9",
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
	if !strings.Contains(urlStr, "/apps/demo-app/changeRequests/CR-9/workItems") {
		t.Fatalf("url=%q", urlStr)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}