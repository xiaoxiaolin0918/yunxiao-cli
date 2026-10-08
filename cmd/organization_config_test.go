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

func TestOrganizationListDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-org-list-not-real")
	t.Setenv(config.EnvOrganizationID, "org-list-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	rootCmd.SetArgs([]string{"organization", "list", "--dry-run"})
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
	if !strings.Contains(url, "/platform/organizations") {
		t.Fatalf("url=%q", url)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestOrganizationMembersListDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-org-members-not-real")
	t.Setenv(config.EnvOrganizationID, "org-members-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	rootCmd.SetArgs([]string{"organization", "members", "list", "--dry-run"})
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
	if !strings.Contains(url, "/members") {
		t.Fatalf("url=%q", url)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestOrganizationMembersHelpMentionsAliyunUID(t *testing.T) {
	h := orgMembersListCmd.Long
	if !strings.Contains(h, "include-aliyun-uid") && !strings.Contains(h, "ALIBABA_CLOUD_ACCESS_KEY") {
		t.Fatalf("members list Long should mention Aliyun UID / AccessKey: %s", h)
	}
}

func TestConfigPathAndShow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(config.EnvAccessToken, "test-token-config-show-not-real")
	t.Setenv(config.EnvOrganizationID, "org-config-show")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, "https://example.invalid")
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	rootCmd.SetArgs([]string{"config", "path"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("path: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Data)
	if !strings.Contains(string(raw), "path") {
		t.Fatalf("data=%s", raw)
	}

	stdout2 := withCmdJSONCapture(t)
	rootCmd.SetArgs([]string{"config", "show"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("show: %v\n%s", err, stdout2.String())
	}
	var env2 output.Envelope
	if err := json.Unmarshal(stdout2.Bytes(), &env2); err != nil {
		t.Fatalf("json2: %v / %s", err, stdout2.Bytes())
	}
	if !env2.OK {
		t.Fatalf("%+v", env2)
	}
	raw2, _ := json.Marshal(env2.Data)
	s := string(raw2)
	if !strings.Contains(s, "token_masked") {
		t.Fatalf("expected masked token: %s", s)
	}
	if strings.Contains(s, "test-token-config-show-not-real") {
		t.Fatalf("raw token leaked: %s", s)
	}
}
