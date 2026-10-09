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

func pkgArtListQSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pkg-art-q-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pkg-art-q-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, packagesArtifactsListCmd, "repo-id", "repo-type", "search", "order-by", "sort")
	for _, name := range []string{"page", "per-page"} {
		f := packagesArtifactsListCmd.Flags().Lookup(name)
		if f != nil {
			_ = packagesArtifactsListCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	return stdout, hits
}

func pkgArtListQAssert(t *testing.T, stdout *bytes.Buffer, hits *int, checkURL func(t *testing.T, url string)) {
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
	if !strings.Contains(url, "/repositories/repo-batch/artifacts") {
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

func TestPackagesArtifactsListRepoTypeMavenDryRun(t *testing.T) {
	stdout, hits := pkgArtListQSetup(t, "maven")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "list",
		"--repo-id", "repo-batch",
		"--repo-type", "MAVEN",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "repoType=MAVEN") {
			t.Fatalf("url=%q", url)
		}
		if strings.Contains(url, "search=") {
			t.Fatalf("search must be omitted: %q", url)
		}
	})
}

func TestPackagesArtifactsListRepoTypeDockerDryRun(t *testing.T) {
	stdout, hits := pkgArtListQSetup(t, "docker")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "list",
		"--repo-id", "repo-batch",
		"--repo-type", "DOCKER",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "repoType=DOCKER") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPackagesArtifactsListDefaultOrderSortDryRun(t *testing.T) {
	stdout, hits := pkgArtListQSetup(t, "defaults")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "list",
		"--repo-id", "repo-batch",
		"--repo-type", "GENERIC",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "orderBy=latestUpdate") || !strings.Contains(url, "sort=desc") {
			t.Fatalf("defaults missing: url=%q", url)
		}
		if !strings.Contains(url, "page=1") || !strings.Contains(url, "perPage=20") {
			t.Fatalf("default page missing: url=%q", url)
		}
	})
}

func TestPackagesArtifactsListNoSearchOmitsDryRun(t *testing.T) {
	stdout, hits := pkgArtListQSetup(t, "no-search")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "list",
		"--repo-id", "repo-batch",
		"--repo-type", "NPM",
		"--search", "",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if strings.Contains(url, "search=") {
			t.Fatalf("empty search must omit: %q", url)
		}
		if !strings.Contains(url, "repoType=NPM") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPackagesArtifactsListOrderByGmtCreateDryRun(t *testing.T) {
	stdout, hits := pkgArtListQSetup(t, "order-gmt")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "list",
		"--repo-id", "repo-batch",
		"--repo-type", "GENERIC",
		"--order-by", "gmtCreate",
		"--sort", "asc",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "orderBy=gmtCreate") || !strings.Contains(url, "sort=asc") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPackagesArtifactsListPageOnlyDryRun(t *testing.T) {
	stdout, hits := pkgArtListQSetup(t, "page")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "list",
		"--repo-id", "repo-batch",
		"--repo-type", "GENERIC",
		"--page", "5",
		"--per-page", "8",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "page=5") || !strings.Contains(url, "perPage=8") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPackagesArtifactsListSearchAndPageDryRun(t *testing.T) {
	stdout, hits := pkgArtListQSetup(t, "search-page")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "list",
		"--repo-id", "repo-batch",
		"--repo-type", "MAVEN",
		"--search", "spring",
		"--page", "2",
		"--per-page", "25",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "search=spring") || !strings.Contains(url, "page=2") || !strings.Contains(url, "perPage=25") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPackagesArtifactsListFullQueryDryRun(t *testing.T) {
	stdout, hits := pkgArtListQSetup(t, "full")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "list",
		"--repo-id", "repo-batch",
		"--repo-type", "DOCKER",
		"--search", "busybox",
		"--order-by", "name",
		"--sort", "asc",
		"--page", "3",
		"--per-page", "10",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		for _, want := range []string{"repoType=DOCKER", "search=busybox", "orderBy=name", "sort=asc", "page=3", "perPage=10"} {
			if !strings.Contains(url, want) {
				t.Fatalf("missing %q in url=%q", want, url)
			}
		}
	})
}