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

func TestCodeupCommitsFilesCompareHelpSurface(t *testing.T) {
	if !strings.Contains(codeupCmd.Long, "commits list") {
		t.Fatalf("codeup Long should mention commits list: %s", codeupCmd.Long)
	}
	if !strings.Contains(codeupCmd.Long, "compare") {
		t.Fatalf("codeup Long should mention compare: %s", codeupCmd.Long)
	}
	var hasFiles, hasCommits, hasCompare bool
	for _, c := range codeupCmd.Commands() {
		switch c.Name() {
		case "files":
			hasFiles = true
			var tree, get bool
			for _, sub := range c.Commands() {
				switch sub.Name() {
				case "tree":
					tree = true
				case "get":
					get = true
				}
			}
			if !tree || !get {
				t.Fatalf("files expected tree+get, tree=%v get=%v", tree, get)
			}
		case "commits":
			hasCommits = true
			var list bool
			for _, sub := range c.Commands() {
				if sub.Name() == "list" {
					list = true
				}
			}
			if !list {
				t.Fatal("commits expected list")
			}
		case "compare":
			hasCompare = true
		}
	}
	if !hasFiles || !hasCommits || !hasCompare {
		t.Fatalf("expected files+commits+compare, got files=%v commits=%v compare=%v", hasFiles, hasCommits, hasCompare)
	}
}

func TestCodeupCommitsListDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-codeup-commits-list-not-real")
	t.Setenv(config.EnvOrganizationID, "org-codeup-commits")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupCommitsListCmd, "repo", "ref", "search", "path", "sort")
	rootCmd.SetArgs([]string{
		"codeup", "commits", "list",
		"--repo", "4951346",
		"--ref", "main",
		"--search", "fix",
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
	if !strings.Contains(url, "/repositories/4951346/commits") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "refName=main") {
		t.Fatalf("missing refName in url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestCodeupCompareDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-codeup-compare-not-real")
	t.Setenv(config.EnvOrganizationID, "org-codeup-compare")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupCompareCmd, "repo", "from", "to", "source-type", "target-type", "straight")
	rootCmd.SetArgs([]string{
		"codeup", "compare",
		"--repo", "4951346",
		"--from", "main",
		"--to", "feature/x",
		"--source-type", "branch",
		"--target-type", "branch",
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
	if !strings.Contains(url, "/repositories/4951346/compares") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "from=main") || !strings.Contains(url, "to=feature") {
		t.Fatalf("missing from/to in url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestCodeupFilesTreeDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-codeup-files-tree-not-real")
	t.Setenv(config.EnvOrganizationID, "org-codeup-files-tree")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupFilesTreeCmd, "repo", "path", "ref", "type")
	rootCmd.SetArgs([]string{
		"codeup", "files", "tree",
		"--repo", "4951346",
		"--path", "cmd",
		"--ref", "main",
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
	if !strings.Contains(url, "/repositories/4951346/files/tree") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestCodeupFilesGetDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-codeup-files-get-not-real")
	t.Setenv(config.EnvOrganizationID, "org-codeup-files-get")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupFilesGetCmd, "repo", "path", "ref")
	rootCmd.SetArgs([]string{
		"codeup", "files", "get",
		"--repo", "4951346",
		"--path", "README.md",
		"--ref", "main",
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
	if !strings.Contains(url, "/repositories/4951346/files/") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "README.md") && !strings.Contains(url, "README%2Emd") && !strings.Contains(url, "README") {
		t.Fatalf("missing file path in url=%q", url)
	}
	if !strings.Contains(url, "ref=main") {
		t.Fatalf("missing ref in url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}