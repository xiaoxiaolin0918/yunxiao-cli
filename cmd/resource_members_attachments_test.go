package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func TestPipelineResourceMembersListDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-rm-list-not-real")
	t.Setenv(config.EnvOrganizationID, "org-rm-list")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, pipelineRMListCmd, "resource-type", "resource-id")
	rootCmd.SetArgs([]string{
		"pipeline", "resource-members", "list",
		"--resource-type", "pipeline",
		"--resource-id", "pipe-1",
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
	if !strings.Contains(url, "/resourceMembers/resourceTypes/pipeline/resourceIds/pipe-1") {
		t.Fatalf("url=%q", url)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestPipelineResourceMembersCreateDeleteDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-rm-write-not-real")
	t.Setenv(config.EnvOrganizationID, "org-rm-write")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, pipelineRMCreateCmd, "resource-type", "resource-id", "role-name", "user-id")
	rootCmd.SetArgs([]string{
		"pipeline", "resource-members", "create",
		"--resource-type", "pipeline",
		"--resource-id", "pipe-1",
		"--role-name", "admin",
		"--user-id", "u-42",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("create: %v\n%s", err, stdout.String())
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
	if !strings.Contains(url, "/resourceMembers/resourceTypes/pipeline/resourceIds/pipe-1") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "roleName=admin") || !strings.Contains(url, "userId=u-42") {
		t.Fatalf("query missing: url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}

	stdout2 := withCmdJSONCapture(t)
	resetStringFlags(t, pipelineRMDeleteCmd, "resource-type", "resource-id", "user-id")
	rootCmd.SetArgs([]string{
		"pipeline", "resource-members", "delete",
		"--resource-type", "pipeline",
		"--resource-id", "pipe-1",
		"--user-id", "u-42",
		"--dry-run",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("delete: %v\n%s", err, stdout2.String())
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
	if !strings.Contains(url2, "/resourceMembers/resourceTypes/pipeline/resourceIds/pipe-1") {
		t.Fatalf("delete url=%q", url2)
	}
	if !strings.Contains(url2, "userId=u-42") {
		t.Fatalf("delete query missing: url=%q", url2)
	}
	if req2["method"] != "DELETE" {
		t.Fatalf("method2=%v", req2["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestWorkitemAttachmentsListDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-attach-list-not-real")
	t.Setenv(config.EnvOrganizationID, "org-attach-list")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemAttachmentsListCmd, "id")
	rootCmd.SetArgs([]string{"workitem", "attachments", "list", "--id", "wi-1", "--dry-run"})
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
	if !strings.Contains(url, "/workitems/wi-1/attachments") {
		t.Fatalf("url=%q", url)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestWorkitemAttachmentsCreateDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-attach-create-not-real")
	t.Setenv(config.EnvOrganizationID, "org-attach-create")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	dir := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })
	if err := os.WriteFile("note.txt", []byte("hello-attach"), 0o644); err != nil {
		t.Fatal(err)
	}

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemAttachmentsCreateCmd, "id", "file", "file-name", "operator-id")
	rootCmd.SetArgs([]string{
		"workitem", "attachments", "create",
		"--id", "wi-1",
		"--file", "note.txt",
		"--file-name", "upload.txt",
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
	if !strings.Contains(url, "/workitems/wi-1/attachments") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	bodyRaw, _ := json.Marshal(req["body"])
	bodyStr := string(bodyRaw)
	if !strings.Contains(bodyStr, `"multipart":true`) || !strings.Contains(bodyStr, `"filename":"upload.txt"`) {
		t.Fatalf("body=%s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"size":12`) {
		t.Fatalf("expected size 12 for hello-attach, body=%s cwd=%s", bodyStr, dir)
	}
	_ = filepath.Base("note.txt")
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestWorkitemRelationsListCreateDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-rel-not-real")
	t.Setenv(config.EnvOrganizationID, "org-rel")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemRelationsListCmd, "id", "relation-type")
	rootCmd.SetArgs([]string{
		"workitem", "relations", "list",
		"--id", "wi-1",
		"--relation-type", "ASSOCIATED",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("list: %v\n%s", err, stdout.String())
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
	if !strings.Contains(url, "/workitems/wi-1/relationRecords") {
		t.Fatalf("list url=%q", url)
	}
	if !strings.Contains(url, "relationType=ASSOCIATED") {
		t.Fatalf("list query missing: url=%q", url)
	}

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout2 := withCmdJSONCapture(t)
	resetStringFlags(t, workitemRelationsCreateCmd, "id", "related-id", "relation-type")
	rootCmd.SetArgs([]string{
		"workitem", "relations", "create",
		"--id", "wi-1",
		"--related-id", "wi-2",
		"--relation-type", "ASSOCIATED",
		"--dry-run",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("create: %v\n%s", err, stdout2.String())
	}
	var env2 output.Envelope
	if err := json.Unmarshal(stdout2.Bytes(), &env2); err != nil {
		t.Fatalf("json2: %v / %s", err, stdout2.Bytes())
	}
	if !env2.OK || !env2.DryRun {
		t.Fatalf("%+v", env2)
	}
	if env2.Risk != string(risk.Write) && env2.Risk != "write" {
		t.Fatalf("risk2=%q", env2.Risk)
	}
	raw2, _ := json.Marshal(env2.Request)
	var req2 map[string]any
	_ = json.Unmarshal(raw2, &req2)
	url2, _ := req2["url"].(string)
	if !strings.Contains(url2, "/workitems/wi-1/relationRecords") {
		t.Fatalf("create url=%q", url2)
	}
	if req2["method"] != "POST" {
		t.Fatalf("method2=%v", req2["method"])
	}
	bodyRaw, _ := json.Marshal(req2["body"])
	if !strings.Contains(string(bodyRaw), `"relationType":"ASSOCIATED"`) || !strings.Contains(string(bodyRaw), `"workitemId":"wi-2"`) {
		t.Fatalf("body=%s", bodyRaw)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}