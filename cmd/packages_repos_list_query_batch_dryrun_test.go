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

func pkgReposListSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pkg-repos-q-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pkg-repos-q-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, packagesReposListCmd, "repo-types", "repo-categories", "page", "per-page")
	return stdout, hits
}

func pkgReposListAssert(t *testing.T, stdout *bytes.Buffer, hits *int, checkURL func(t *testing.T, url string)) {
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
	if !strings.Contains(url, "/repositories") {
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

func TestPackagesReposListRepoTypesDryRun(t *testing.T) {
	stdout, hits := pkgReposListSetup(t, "types")
	rootCmd.SetArgs([]string{
		"packages", "repos", "list",
		"--repo-types", "MAVEN",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgReposListAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "repoTypes=MAVEN") {
			t.Fatalf("missing repoTypes: url=%q", url)
		}
		if strings.Contains(url, "repoCategories=") {
			t.Fatalf("unexpected repoCategories: url=%q", url)
		}
	})
}

func TestPackagesReposListRepoCategoriesDryRun(t *testing.T) {
	stdout, hits := pkgReposListSetup(t, "cats")
	rootCmd.SetArgs([]string{
		"packages", "repos", "list",
		"--repo-categories", "PRIVATE",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgReposListAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "repoCategories=PRIVATE") {
			t.Fatalf("missing repoCategories: url=%q", url)
		}
		if strings.Contains(url, "repoTypes=") {
			t.Fatalf("unexpected repoTypes: url=%q", url)
		}
	})
}

func TestPackagesReposListTypesAndCategoriesDryRun(t *testing.T) {
	stdout, hits := pkgReposListSetup(t, "both")
	rootCmd.SetArgs([]string{
		"packages", "repos", "list",
		"--repo-types", "NPM",
		"--repo-categories", "PUBLIC",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgReposListAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "repoTypes=NPM") || !strings.Contains(url, "repoCategories=PUBLIC") {
			t.Fatalf("filters missing: url=%q", url)
		}
	})
}

func TestPackagesReposListPagePerPageDryRun(t *testing.T) {
	stdout, hits := pkgReposListSetup(t, "page")
	rootCmd.SetArgs([]string{
		"packages", "repos", "list",
		"--page", "3",
		"--per-page", "15",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgReposListAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "page=3") || !strings.Contains(url, "perPage=15") {
			t.Fatalf("page query missing: url=%q", url)
		}
	})
}

func TestPackagesReposListDefaultPageDryRun(t *testing.T) {
	stdout, hits := pkgReposListSetup(t, "defaults")
	rootCmd.SetArgs([]string{
		"packages", "repos", "list",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgReposListAssert(t, stdout, hits, func(t *testing.T, url string) {
		// defaults: page=1 perPage=20; optional filters omitted
		if !strings.Contains(url, "page=1") || !strings.Contains(url, "perPage=20") {
			t.Fatalf("default page query missing: url=%q", url)
		}
		if strings.Contains(url, "repoTypes=") || strings.Contains(url, "repoCategories=") {
			t.Fatalf("unexpected filters: url=%q", url)
		}
	})
}

func TestPackagesReposListMultiRepoTypesDryRun(t *testing.T) {
	stdout, hits := pkgReposListSetup(t, "multi-types")
	rootCmd.SetArgs([]string{
		"packages", "repos", "list",
		"--repo-types", "GENERIC,MAVEN",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgReposListAssert(t, stdout, hits, func(t *testing.T, url string) {
		// URL-encoded comma may be %2C or literal depending on Values encoding
		if !strings.Contains(url, "repoTypes=GENERIC") || !strings.Contains(url, "MAVEN") {
			t.Fatalf("multi repoTypes missing: url=%q", url)
		}
	})
}

func TestPackagesReposListCategoriesAndPageDryRun(t *testing.T) {
	stdout, hits := pkgReposListSetup(t, "cats-page")
	rootCmd.SetArgs([]string{
		"packages", "repos", "list",
		"--repo-categories", "PRIVATE",
		"--page", "2",
		"--per-page", "50",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgReposListAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "repoCategories=PRIVATE") || !strings.Contains(url, "page=2") || !strings.Contains(url, "perPage=50") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPackagesReposListFullQueryDryRun(t *testing.T) {
	stdout, hits := pkgReposListSetup(t, "full")
	rootCmd.SetArgs([]string{
		"packages", "repos", "list",
		"--repo-types", "DOCKER",
		"--repo-categories", "PRIVATE",
		"--page", "4",
		"--per-page", "10",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgReposListAssert(t, stdout, hits, func(t *testing.T, url string) {
		for _, want := range []string{"repoTypes=DOCKER", "repoCategories=PRIVATE", "page=4", "perPage=10"} {
			if !strings.Contains(url, want) {
				t.Fatalf("missing %q in url=%q", want, url)
			}
		}
	})
}