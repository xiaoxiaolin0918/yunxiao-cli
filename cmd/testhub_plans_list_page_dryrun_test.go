package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

// Dry-run for testhub plans list pagination (filters covered elsewhere).
func TestTesthubPlansListPageDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-plans-page-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-plans-page")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, testhubPlansListCmd, "project-id", "sprint-id", "name", "status")
	rootCmd.SetArgs([]string{
		"testhub", "plans", "list",
		"--project-id", "proj-page-1",
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
	if env.Risk != string(risk.Read) && env.Risk != "read" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/testPlan/list") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["projectIdentifier"] != "proj-page-1" {
		t.Fatalf("body=%v", body)
	}
	page, _ := body["page"].(float64)
	perPage, _ := body["perPage"].(float64)
	if int(page) != 3 || int(perPage) != 40 {
		t.Fatalf("page fields: page=%v perPage=%v", body["page"], body["perPage"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}