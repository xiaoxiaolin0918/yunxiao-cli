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

func TestSprintCreateDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-sprint-create-not-real")
	t.Setenv(config.EnvOrganizationID, "org-sprint-create")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, sprintCreateCmd, "space-id", "name", "owners", "start-date", "end-date", "description")
	rootCmd.SetArgs([]string{
		"sprint", "create",
		"--space-id", "space-s1",
		"--name", "Sprint 42",
		"--owners", "u-1,u-2",
		"--start-date", "2026-10-01",
		"--end-date", "2026-10-14",
		"--description", "dry-run sprint",
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
	if env.Risk != string(risk.Write) && env.Risk != "write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/projects/space-s1/sprints") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["name"] != "Sprint 42" || body["startDate"] != "2026-10-01" || body["endDate"] != "2026-10-14" {
		t.Fatalf("body=%v", body)
	}
	owners, _ := body["owners"].([]any)
	if len(owners) != 2 || owners[0] != "u-1" || owners[1] != "u-2" {
		t.Fatalf("owners=%v", owners)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestSprintUpdateDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-sprint-update-not-real")
	t.Setenv(config.EnvOrganizationID, "org-sprint-update")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, sprintUpdateCmd, "space-id", "id", "name", "owners", "start-date", "end-date", "description", "status")
	rootCmd.SetArgs([]string{
		"sprint", "update",
		"--space-id", "space-s2",
		"--id", "sp-9",
		"--name", "Sprint 43",
		"--owners", "u-9",
		"--status", "DOING",
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
	if env.Risk != string(risk.Write) && env.Risk != "write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/projects/space-s2/sprints/sp-9") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "PUT" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["name"] != "Sprint 43" || body["status"] != "DOING" {
		t.Fatalf("body=%v", body)
	}
	owners, _ := body["owners"].([]any)
	if len(owners) != 1 || owners[0] != "u-9" {
		t.Fatalf("owners=%v", owners)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}