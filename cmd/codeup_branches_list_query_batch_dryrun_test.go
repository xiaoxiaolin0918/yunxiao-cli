package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func brListQSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-br-list-q-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-br-list-q-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, codeupBranchesListCmd, "repo", "search", "page", "per-page")
	return stdout, hits
}

func brListQAssert(t *testing.T, stdout *bytes.Buffer, hits *int, checkURL func(t *testing.T, url string)) {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	if env.Risk != string(risk.Read) && env.Risk != "read" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/repositories/4952001/branches") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if checkURL != nil {
		checkURL(t, url)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func TestCodeupBranchesListDefaultsDryRun(t *testing.T) {
	stdout, hits := brListQSetup(t, "defaults")
	rootCmd.SetArgs([]string{"codeup", "branches", "list", "--repo", "4952001", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	brListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "page=1") || !strings.Contains(url, "perPage=20") {
			t.Fatalf("defaults missing: url=%q", url)
		}
		if strings.Contains(url, "search=") {
			t.Fatalf("search must be omitted: url=%q", url)
		}
	})
}

func TestCodeupBranchesListSearchOnlyDryRun(t *testing.T) {
	stdout, hits := brListQSetup(t, "search")
	rootCmd.SetArgs([]string{"codeup", "branches", "list", "--repo", "4952001", "--search", "feat/", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	brListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "search=feat") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestCodeupBranchesListPageDryRun(t *testing.T) {
	stdout, hits := brListQSetup(t, "page")
	rootCmd.SetArgs([]string{"codeup", "branches", "list", "--repo", "4952001", "--page", "4", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	brListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "page=4") || !strings.Contains(url, "perPage=20") {
			t.Fatalf("url=%q", url)
		}
		if strings.Contains(url, "search=") {
			t.Fatalf("search must be omitted: url=%q", url)
		}
	})
}

func TestCodeupBranchesListPerPageDryRun(t *testing.T) {
	stdout, hits := brListQSetup(t, "perpage")
	rootCmd.SetArgs([]string{"codeup", "branches", "list", "--repo", "4952001", "--per-page", "50", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	brListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "page=1") || !strings.Contains(url, "perPage=50") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestCodeupBranchesListSearchAndPageDryRun(t *testing.T) {
	stdout, hits := brListQSetup(t, "search-page")
	rootCmd.SetArgs([]string{"codeup", "branches", "list", "--repo", "4952001", "--search", "release", "--page", "2", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	brListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "search=release") || !strings.Contains(url, "page=2") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestCodeupBranchesListSearchAndPerPageDryRun(t *testing.T) {
	stdout, hits := brListQSetup(t, "search-pp")
	rootCmd.SetArgs([]string{"codeup", "branches", "list", "--repo", "4952001", "--search", "hotfix", "--per-page", "10", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	brListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "search=hotfix") || !strings.Contains(url, "perPage=10") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestCodeupBranchesListPageAndPerPageDryRun(t *testing.T) {
	stdout, hits := brListQSetup(t, "page-pp")
	rootCmd.SetArgs([]string{"codeup", "branches", "list", "--repo", "4952001", "--page", "3", "--per-page", "30", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	brListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "page=3") || !strings.Contains(url, "perPage=30") {
			t.Fatalf("url=%q", url)
		}
		if strings.Contains(url, "search=") {
			t.Fatalf("search must be omitted: url=%q", url)
		}
	})
}

func TestCodeupBranchesListAllFieldsDryRun(t *testing.T) {
	stdout, hits := brListQSetup(t, "all")
	rootCmd.SetArgs([]string{
		"codeup", "branches", "list",
		"--repo", "4952001",
		"--search", "main",
		"--page", "5",
		"--per-page", "15",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	brListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "search=main") || !strings.Contains(url, "page=5") || !strings.Contains(url, "perPage=15") {
			t.Fatalf("url=%q", url)
		}
	})
}
