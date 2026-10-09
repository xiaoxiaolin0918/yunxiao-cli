package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// +my-open-items / +created-by-me always resolve self via GET /platform/user,
// then dry-run the workitems:search without sending it.
func TestProjectMyOpenItemsDryRun(t *testing.T) {
	var userHits, searchHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/platform/user"):
			userHits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"u-shortcut-1","name":"Shortcut User"}`))
		case strings.Contains(r.URL.Path, "workitems:search"):
			searchHits.Add(1)
			w.WriteHeader(500)
		default:
			w.WriteHeader(500)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-project-my-open-not-real")
	t.Setenv(config.EnvOrganizationID, "org-project-my-open")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, projectMyOpenItemsCmd, "category", "space-id", "status-stage")
	rootCmd.SetArgs([]string{
		"project", "+my-open-items",
		"--space-id", "space-s1",
		"--category", "Task",
		"--status-stage", "1,2",
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
	if !strings.Contains(url, "/workitems:search") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["category"] != "Task" || body["spaceId"] != "space-s1" {
		t.Fatalf("body=%v", body)
	}
	conds, _ := body["conditions"].(string)
	if !strings.Contains(conds, "assignedTo") || !strings.Contains(conds, "u-shortcut-1") {
		t.Fatalf("conditions=%q", conds)
	}
	if userHits.Load() != 1 {
		t.Fatalf("userHits=%d", userHits.Load())
	}
	if searchHits.Load() != 0 {
		t.Fatalf("searchHits=%d", searchHits.Load())
	}
}

func TestProjectCreatedByMeDryRun(t *testing.T) {
	var userHits, searchHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/platform/user"):
			userHits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"u-created-9","name":"Creator"}`))
		case strings.Contains(r.URL.Path, "workitems:search"):
			searchHits.Add(1)
			w.WriteHeader(500)
		default:
			w.WriteHeader(500)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-project-created-not-real")
	t.Setenv(config.EnvOrganizationID, "org-project-created")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, projectCreatedByMeCmd, "category", "space-id", "status-stage")
	rootCmd.SetArgs([]string{
		"project", "+created-by-me",
		"--space-id", "space-c1",
		"--category", "Req",
		"--status-stage", "1,2",
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
	if !strings.Contains(url, "/workitems:search") {
		t.Fatalf("url=%q", url)
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["spaceId"] != "space-c1" {
		t.Fatalf("body=%v", body)
	}
	conds, _ := body["conditions"].(string)
	if !strings.Contains(conds, "creator") || !strings.Contains(conds, "u-created-9") {
		t.Fatalf("conditions=%q", conds)
	}
	if !strings.Contains(conds, "statusStage") {
		t.Fatalf("expected statusStage filter: %q", conds)
	}
	if userHits.Load() != 1 {
		t.Fatalf("userHits=%d", userHits.Load())
	}
	if searchHits.Load() != 0 {
		t.Fatalf("searchHits=%d", searchHits.Load())
	}
}