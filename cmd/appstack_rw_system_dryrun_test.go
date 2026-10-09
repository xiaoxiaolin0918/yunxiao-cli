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

func TestAppstackRWSystemListDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-appstack-rw-sys-list-not-real")
	t.Setenv(config.EnvOrganizationID, "org-appstack-rw-sys-list")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackRWSystemListCmd, "system")
	rootCmd.SetArgs([]string{
		"appstack", "release-workflows", "system", "list",
		"--system", "sys-a",
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
	if !strings.Contains(urlStr, "/systems/sys-a/releaseWorkflows") {
		t.Fatalf("url=%q", urlStr)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestAppstackRWSystemCreateDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-appstack-rw-sys-create-not-real")
	t.Setenv(config.EnvOrganizationID, "org-appstack-rw-sys-create")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackRWSystemCreateCmd, "system", "name", "note", "template-sn", "data", "data-file")
	rootCmd.SetArgs([]string{
		"appstack", "release-workflows", "system", "create",
		"--system", "sys-a",
		"--name", "wf-new",
		"--note", "n1",
		"--template-sn", "tpl-1",
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
	if env.Risk != string(risk.HighRiskWrite) && env.Risk != "high-risk-write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	urlStr, _ := req["url"].(string)
	if !strings.Contains(urlStr, "/systems/sys-a/releaseWorkflows") {
		t.Fatalf("url=%q", urlStr)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	wf, _ := body["workflow"].(map[string]any)
	if wf == nil || wf["name"] != "wf-new" || wf["note"] != "n1" || wf["templateSn"] != "tpl-1" {
		t.Fatalf("body=%v", body)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}