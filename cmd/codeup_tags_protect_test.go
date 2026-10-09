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

func TestCodeupTagsHelpSurface(t *testing.T) {
	if !strings.Contains(codeupTagsCmd.Long, "high-risk") {
		t.Fatalf("tags Long should mention high-risk: %s", codeupTagsCmd.Long)
	}
	if !strings.Contains(codeupProtectedBranchesCmd.Long, "protected") {
		t.Fatalf("protected Long unexpected: %s", codeupProtectedBranchesCmd.Long)
	}
}

func TestCodeupTagsListDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-tags-list-not-real")
	t.Setenv(config.EnvOrganizationID, "org-tags-list")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupTagsListCmd, "repo", "search", "sort", "order-by")
	rootCmd.SetArgs([]string{"codeup", "tags", "list", "--repo", "4951320", "--search", "v1", "--dry-run"})
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
	if !strings.Contains(url, "/repositories/4951320/tags") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "search=v1") {
		t.Fatalf("missing search query: url=%q", url)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestCodeupTagsCreateDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-tags-create-not-real")
	t.Setenv(config.EnvOrganizationID, "org-tags-create")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupTagsCreateCmd, "repo", "tag-name", "ref", "message")
	rootCmd.SetArgs([]string{
		"codeup", "tags", "create",
		"--repo", "4951320",
		"--tag-name", "v9.9.9",
		"--ref", "master",
		"--message", "cut",
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
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/repositories/4951320/tags") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "tagName=v9.9.9") || !strings.Contains(url, "ref=master") {
		t.Fatalf("query missing: url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestCodeupTagsDeleteDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-tags-del-not-real")
	t.Setenv(config.EnvOrganizationID, "org-tags-del")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupTagsDeleteCmd, "repo", "tag-name")
	rootCmd.SetArgs([]string{
		"codeup", "tags", "delete",
		"--repo", "4951320",
		"--tag-name", "v9.9.9",
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
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/repositories/4951320/tags/v9.9.9") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "DELETE" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestCodeupProtectedListGetDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-protect-list-not-real")
	t.Setenv(config.EnvOrganizationID, "org-protect-list")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupProtectedListCmd, "repo")
	rootCmd.SetArgs([]string{"codeup", "protected-branches", "list", "--repo", "4951320", "--dry-run"})
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
	if !strings.Contains(url, "/repositories/4951320/protectedBranches") {
		t.Fatalf("list url=%q", url)
	}

	stdout2 := withCmdJSONCapture(t)
	resetStringFlags(t, codeupProtectedGetCmd, "repo", "id")
	rootCmd.SetArgs([]string{"codeup", "protected-branches", "get", "--repo", "4951320", "--id", "pb-1", "--dry-run"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("get: %v\n%s", err, stdout2.String())
	}
	var env2 output.Envelope
	if err := json.Unmarshal(stdout2.Bytes(), &env2); err != nil {
		t.Fatalf("json2: %v / %s", err, stdout2.Bytes())
	}
	if !env2.OK || !env2.DryRun {
		t.Fatalf("%+v", env2)
	}
	raw2, _ := json.Marshal(env2.Request)
	var req2 map[string]any
	_ = json.Unmarshal(raw2, &req2)
	url2, _ := req2["url"].(string)
	if !strings.Contains(url2, "/repositories/4951320/protectedBranches/pb-1") {
		t.Fatalf("get url=%q", url2)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestCodeupProtectedCreateDeleteDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-protect-write-not-real")
	t.Setenv(config.EnvOrganizationID, "org-protect-write")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupProtectedCreateCmd, "repo", "branch", "allow-push-roles", "allow-merge-roles", "body")
	rootCmd.SetArgs([]string{
		"codeup", "protected-branches", "create",
		"--repo", "4951320",
		"--branch", "master",
		"--allow-push-roles", "40,30",
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
	if !strings.Contains(url, "/repositories/4951320/protectedBranches") {
		t.Fatalf("create url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	bodyRaw, _ := json.Marshal(req["body"])
	if !strings.Contains(string(bodyRaw), `"branch":"master"`) {
		t.Fatalf("body=%s", bodyRaw)
	}

	stdout2 := withCmdJSONCapture(t)
	resetStringFlags(t, codeupProtectedDeleteCmd, "repo", "id")
	rootCmd.SetArgs([]string{
		"codeup", "protected-branches", "delete",
		"--repo", "4951320",
		"--id", "pb-9",
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
	if !strings.Contains(url2, "/repositories/4951320/protectedBranches/pb-9") {
		t.Fatalf("delete url=%q", url2)
	}
	if req2["method"] != "DELETE" {
		t.Fatalf("method2=%v", req2["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}