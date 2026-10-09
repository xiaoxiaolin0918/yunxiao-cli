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

func TestVersionsCreateDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-versions-create-not-real")
	t.Setenv(config.EnvOrganizationID, "org-versions-create")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, versionsCreateCmd, "space-id", "name", "owners", "start-date", "publish-date")
	rootCmd.SetArgs([]string{
		"versions", "create",
		"--space-id", "space-vc1",
		"--name", "v1.2.3",
		"--owners", "u-1,u-2",
		"--start-date", "2026-10-01",
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
	if !strings.Contains(url, "/projects/space-vc1/versions") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["name"] != "v1.2.3" || body["startDate"] != "2026-10-01" {
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

func TestVersionsUpdateDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-versions-update-not-real")
	t.Setenv(config.EnvOrganizationID, "org-versions-update")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, versionsUpdateCmd, "space-id", "id", "name", "owners", "start-date", "publish-date")
	rootCmd.SetArgs([]string{
		"versions", "update",
		"--space-id", "space-vu1",
		"--id", "ver-7",
		"--name", "v1.2.4",
		"--owners", "u-9",
		"--publish-date", "2026-11-01",
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
	if !strings.Contains(url, "/projects/space-vu1/versions/ver-7") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "PUT" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["name"] != "v1.2.4" || body["publishDate"] != "2026-11-01" {
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
