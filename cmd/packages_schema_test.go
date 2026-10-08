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

func TestPackagesHelpDocumentsSurface(t *testing.T) {
	h := packagesCmd.Long
	if !strings.Contains(h, "repos list") || !strings.Contains(h, "artifacts") {
		t.Fatalf("packages Long should mention repos/artifacts: %s", h)
	}
	if !strings.Contains(h, "high-risk") && !strings.Contains(h, "delete") {
		t.Fatalf("packages Long should mention delete risk: %s", h)
	}
	var hasRepos, hasArtifacts, hasDelete bool
	for _, c := range packagesCmd.Commands() {
		switch c.Name() {
		case "repos":
			hasRepos = true
		case "artifacts":
			hasArtifacts = true
			for _, sub := range c.Commands() {
				if sub.Name() == "delete" {
					hasDelete = true
				}
			}
		}
	}
	if !hasRepos || !hasArtifacts || !hasDelete {
		t.Fatalf("expected repos+artifacts+delete, got repos=%v artifacts=%v delete=%v", hasRepos, hasArtifacts, hasDelete)
	}
}

func TestPackagesReposListDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-packages-repos-not-real")
	t.Setenv(config.EnvOrganizationID, "org-packages-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, packagesReposListCmd, "repo-types", "repo-categories")
	rootCmd.SetArgs([]string{
		"packages", "repos", "list",
		"--repo-types", "GENERIC",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("envelope: %+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/repositories") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("HTTP should not hit on dry-run, hits=%d", hits)
	}
}

func TestPackagesArtifactsDeleteDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-packages-del-not-real")
	t.Setenv(config.EnvOrganizationID, "org-packages-del")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, packagesArtifactsDeleteCmd, "repo-id", "repo-type", "id", "version-id")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "delete",
		"--repo-id", "repo-1",
		"--repo-type", "GENERIC",
		"--id", "art-9",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("envelope: %+v", env)
	}
	if env.Risk != string(risk.HighRiskWrite) && env.Risk != "high-risk-write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/repositories/repo-1/artifacts/art-9") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "DELETE" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("HTTP should not hit on dry-run, hits=%d", hits)
	}
}

func TestSchemaListAndLookup(t *testing.T) {
	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, schemaCmd, "domain")
	rootCmd.SetArgs([]string{"schema", "--domain", "project"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("list: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Data)
	if !strings.Contains(string(raw), "workitem.search") {
		t.Fatalf("expected workitem.search in project domain list: %s", raw)
	}

	stdout2 := withCmdJSONCapture(t)
	resetStringFlags(t, schemaCmd, "domain")
	rootCmd.SetArgs([]string{"schema", "workitem.search"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("lookup: %v\n%s", err, stdout2.String())
	}
	var env2 output.Envelope
	if err := json.Unmarshal(stdout2.Bytes(), &env2); err != nil {
		t.Fatalf("json2: %v / %s", err, stdout2.Bytes())
	}
	if !env2.OK {
		t.Fatalf("%+v", env2)
	}
	raw2, _ := json.Marshal(env2.Data)
	if !strings.Contains(string(raw2), "workitem.search") && !strings.Contains(string(raw2), "search") {
		t.Fatalf("expected workitem.search schema: %s", raw2)
	}
}
