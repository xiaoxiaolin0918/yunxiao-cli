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

func TestWorkitemRelationsDeleteDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-rel-del-not-real")
	t.Setenv(config.EnvOrganizationID, "org-rel-del")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemRelationsDeleteCmd, "id", "related-id", "relation-type")
	rootCmd.SetArgs([]string{
		"workitem", "relations", "delete",
		"--id", "wi-1",
		"--related-id", "wi-9",
		"--relation-type", "DEPEND_ON",
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
	if !strings.Contains(url, "/workitems/wi-1/relationRecords") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "DELETE" {
		t.Fatalf("method=%v", req["method"])
	}
	bodyRaw, _ := json.Marshal(req["body"])
	if !strings.Contains(string(bodyRaw), `"relationType":"DEPEND_ON"`) || !strings.Contains(string(bodyRaw), `"workitemId":"wi-9"`) {
		t.Fatalf("body=%s", bodyRaw)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestPipelineResourceMembersUpdateTransferDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-rm-upd-xfer-not-real")
	t.Setenv(config.EnvOrganizationID, "org-rm-upd-xfer")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, pipelineRMUpdateCmd, "resource-type", "resource-id", "role-name", "user-id")
	rootCmd.SetArgs([]string{
		"pipeline", "resource-members", "update",
		"--resource-type", "hostGroup",
		"--resource-id", "hg-3",
		"--role-name", "developer",
		"--user-id", "u-77",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("update: %v\n%s", err, stdout.String())
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
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/resourceMembers/resourceTypes/hostGroup/resourceIds/hg-3") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "roleName=developer") || !strings.Contains(url, "userId=u-77") {
		t.Fatalf("query missing: url=%q", url)
	}
	if req["method"] != "PUT" {
		t.Fatalf("method=%v", req["method"])
	}

	stdout2 := withCmdJSONCapture(t)
	resetStringFlags(t, pipelineRMTransferCmd, "resource-type", "resource-id", "new-owner-id")
	rootCmd.SetArgs([]string{
		"pipeline", "resource-members", "transfer-owner",
		"--resource-type", "pipeline",
		"--resource-id", "pipe-5",
		"--new-owner-id", "u-99",
		"--dry-run",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("transfer: %v\n%s", err, stdout2.String())
	}
	var env2 output.Envelope
	if err := json.Unmarshal(stdout2.Bytes(), &env2); err != nil {
		t.Fatalf("json2: %v / %s", err, stdout2.Bytes())
	}
	if !env2.OK || !env2.DryRun {
		t.Fatalf("%+v", env2)
	}
	if env2.Risk != string(risk.HighRiskWrite) && env2.Risk != "high-risk-write" {
		t.Fatalf("risk2=%q", env2.Risk)
	}
	raw2, _ := json.Marshal(env2.Request)
	var req2 map[string]any
	_ = json.Unmarshal(raw2, &req2)
	url2, _ := req2["url"].(string)
	if !strings.Contains(url2, "/resourceMembers/resourceTypes/pipeline/resourceIds/pipe-5/transfer/owner") {
		t.Fatalf("transfer url=%q", url2)
	}
	if !strings.Contains(url2, "newOwnerId=u-99") {
		t.Fatalf("transfer query missing: url=%q", url2)
	}
	if req2["method"] != "POST" {
		t.Fatalf("method2=%v", req2["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}