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
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func TestPipelineFlowVariableGroupsCreateDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pipe-fvg-create-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipe-fvg-create")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	varsJSON := `[{"name":"K","value":"V","isEncrypted":false}]`
	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, pipelineFlowVGCreateCmd, "name", "variables", "description")
	rootCmd.SetArgs([]string{
		"pipeline", "flow-variable-groups", "create",
		"--name", "vg-new",
		"--variables", varsJSON,
		"--description", "desc-cu",
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
	u, _ := req["url"].(string)
	if !strings.Contains(u, "/variableGroups") {
		t.Fatalf("url=%q", u)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	q := parsed.Query()
	if q.Get("name") != "vg-new" || q.Get("variables") != varsJSON || q.Get("description") != "desc-cu" {
		t.Fatalf("query=%v", q)
	}
	if req["body"] != nil {
		t.Fatalf("body should be nil for query-only create, got %v", req["body"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestPipelineFlowVariableGroupsUpdateDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pipe-fvg-update-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipe-fvg-update")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	varsJSON := `[{"name":"K2","value":"V2","isEncrypted":true}]`
	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, pipelineFlowVGUpdateCmd, "id", "name", "variables", "description")
	rootCmd.SetArgs([]string{
		"pipeline", "flow-variable-groups", "update",
		"--id", "vg-42",
		"--name", "vg-renamed",
		"--variables", varsJSON,
		"--description", "desc-up",
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
	u, _ := req["url"].(string)
	if !strings.Contains(u, "/variableGroups/vg-42") {
		t.Fatalf("url=%q", u)
	}
	if req["method"] != "PUT" {
		t.Fatalf("method=%v", req["method"])
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	q := parsed.Query()
	if q.Get("name") != "vg-renamed" || q.Get("variables") != varsJSON || q.Get("description") != "desc-up" {
		t.Fatalf("query=%v", q)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}