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

// Dry-run for appstack change-requests list --state/--current/--page-size body filters.
func TestAppstackChangeRequestsListStatePageDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-appstack-cr-state-not-real")
	t.Setenv(config.EnvOrganizationID, "org-appstack-cr-state")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackCRListCmd, "app", "name", "state")
	for _, name := range []string{"current", "page-size"} {
		f := appstackCRListCmd.Flags().Lookup(name)
		if f != nil {
			_ = appstackCRListCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	rootCmd.SetArgs([]string{
		"appstack", "change-requests", "list",
		"--app", "demo-app",
		"--state", "DEVELOPING,INTEGRATING",
		"--current", "2",
		"--page-size", "30",
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
	if !strings.Contains(url, "/apps/demo-app/changeRequests:search") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil {
		t.Fatalf("body nil: %v", req)
	}
	stateRaw, _ := json.Marshal(body["state"])
	state := string(stateRaw)
	if !strings.Contains(state, "DEVELOPING") || !strings.Contains(state, "INTEGRATING") {
		t.Fatalf("state=%v", body["state"])
	}
	current, _ := body["current"].(float64)
	pageSize, _ := body["pageSize"].(float64)
	if current != 2 || pageSize != 30 {
		t.Fatalf("page current=%v pageSize=%v body=%v", body["current"], body["pageSize"], body)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}