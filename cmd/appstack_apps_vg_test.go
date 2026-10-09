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

func TestAppstackAppsVGHelpSurface(t *testing.T) {
	if !strings.Contains(appstackCmd.Long, "apps list") {
		t.Fatalf("appstack Long should mention apps list: %s", appstackCmd.Long)
	}
	if !strings.Contains(appstackCmd.Long, "variable-groups") {
		t.Fatalf("appstack Long should mention variable-groups: %s", appstackCmd.Long)
	}
	var hasApps, hasVG bool
	for _, c := range appstackCmd.Commands() {
		switch c.Name() {
		case "apps":
			hasApps = true
			var list, get bool
			for _, sub := range c.Commands() {
				switch sub.Name() {
				case "list":
					list = true
				case "get":
					get = true
				}
			}
			if !list || !get {
				t.Fatalf("apps expected list+get, list=%v get=%v", list, get)
			}
		case "variable-groups":
			hasVG = true
			var list bool
			for _, sub := range c.Commands() {
				if sub.Name() == "list" {
					list = true
				}
			}
			if !list {
				t.Fatal("variable-groups expected list")
			}
		}
	}
	if !hasApps || !hasVG {
		t.Fatalf("expected apps+variable-groups, got apps=%v vg=%v", hasApps, hasVG)
	}
}

func TestAppstackAppsListDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-appstack-apps-list-not-real")
	t.Setenv(config.EnvOrganizationID, "org-appstack-apps")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackAppsListCmd, "next-token", "order-by", "sort", "tags")
	rootCmd.SetArgs([]string{
		"appstack", "apps", "list",
		"--order-by", "id",
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
	if !strings.Contains(url, "/appstack/organizations/org-appstack-apps/apps:search") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestAppstackAppsGetDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-appstack-apps-get-not-real")
	t.Setenv(config.EnvOrganizationID, "org-appstack-apps-get")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackAppsGetCmd, "name")
	rootCmd.SetArgs([]string{"appstack", "apps", "get", "--name", "demo-app", "--dry-run"})
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
	if !strings.Contains(url, "/apps/demo-app") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestAppstackVariableGroupsListDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-appstack-vg-list-not-real")
	t.Setenv(config.EnvOrganizationID, "org-appstack-vg")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackVGListCmd, "app")
	rootCmd.SetArgs([]string{
		"appstack", "variable-groups", "list",
		"--app", "demo-app",
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
	if !strings.Contains(url, "/apps/demo-app/variableGroups") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}