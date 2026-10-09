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

// Dry-run for appstack change-orders by-origin --env-name query.
func TestAppstackChangeOrdersByOriginEnvDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-appstack-co-byorigin-env-not-real")
	t.Setenv(config.EnvOrganizationID, "org-appstack-co-byorigin-env")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackCOByOriginCmd, "origin-type", "origin-id", "app", "env-name")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "by-origin",
		"--origin-type", "CHANGE_REQUEST",
		"--origin-id", "cr-42",
		"--app", "demo-app",
		"--env-name", "prod",
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
	if !strings.Contains(url, "/changeOrders:byOrigin") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "originType=CHANGE_REQUEST") || !strings.Contains(url, "originId=cr-42") {
		t.Fatalf("origin query missing url=%q", url)
	}
	if !strings.Contains(url, "appName=demo-app") || !strings.Contains(url, "envName=prod") {
		t.Fatalf("app/env missing url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}