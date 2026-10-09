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

func pkgArtGDSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pkg-art-gd-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pkg-art-gd-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, packagesArtifactsGetCmd, "repo-id", "repo-type", "id")
	resetStringFlags(t, packagesArtifactsDeleteCmd, "repo-id", "repo-type", "id", "version-id")
	return stdout, hits
}

func pkgArtGDAssert(t *testing.T, stdout *bytes.Buffer, hits *int, wantMethod, wantRisk string, checkURL func(t *testing.T, url string)) {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	if env.Risk != wantRisk && env.Risk != string(risk.Read) && env.Risk != string(risk.HighRiskWrite) {
		// allow exact match below
	}
	if env.Risk != wantRisk {
		t.Fatalf("risk=%q want %q", env.Risk, wantRisk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if req["method"] != wantMethod {
		t.Fatalf("method=%v want %s", req["method"], wantMethod)
	}
	if checkURL != nil {
		checkURL(t, url)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func TestPackagesArtifactsGetMavenDryRun(t *testing.T) {
	stdout, hits := pkgArtGDSetup(t, "get-maven")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "get",
		"--repo-id", "repo-gd",
		"--repo-type", "MAVEN",
		"--id", "art-1",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtGDAssert(t, stdout, hits, "GET", string(risk.Read), func(t *testing.T, url string) {
		if !strings.Contains(url, "/repositories/repo-gd/artifacts/art-1") {
			t.Fatalf("url=%q", url)
		}
		if !strings.Contains(url, "repoType=MAVEN") {
			t.Fatalf("missing repoType: url=%q", url)
		}
	})
}

func TestPackagesArtifactsGetNpmDryRun(t *testing.T) {
	stdout, hits := pkgArtGDSetup(t, "get-npm")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "get",
		"--repo-id", "repo-gd",
		"--repo-type", "NPM",
		"--id", "art-npm",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtGDAssert(t, stdout, hits, "GET", string(risk.Read), func(t *testing.T, url string) {
		if !strings.Contains(url, "/artifacts/art-npm") || !strings.Contains(url, "repoType=NPM") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPackagesArtifactsGetGenericDryRun(t *testing.T) {
	stdout, hits := pkgArtGDSetup(t, "get-generic")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "get",
		"--repo-id", "repo-gd",
		"--repo-type", "GENERIC",
		"--id", "art-gen",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtGDAssert(t, stdout, hits, "GET", string(risk.Read), func(t *testing.T, url string) {
		if !strings.Contains(url, "repoType=GENERIC") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPackagesArtifactsGetPypiDryRun(t *testing.T) {
	stdout, hits := pkgArtGDSetup(t, "get-pypi")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "get",
		"--repo-id", "repo-gd",
		"--repo-type", "PYPI",
		"--id", "art-py",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtGDAssert(t, stdout, hits, "GET", string(risk.Read), func(t *testing.T, url string) {
		if !strings.Contains(url, "repoType=PYPI") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPackagesArtifactsDeleteWholeMavenDryRun(t *testing.T) {
	stdout, hits := pkgArtGDSetup(t, "del-whole")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "delete",
		"--repo-id", "repo-gd",
		"--repo-type", "MAVEN",
		"--id", "art-del",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtGDAssert(t, stdout, hits, "DELETE", string(risk.HighRiskWrite), func(t *testing.T, url string) {
		if !strings.Contains(url, "/repositories/repo-gd/artifacts/art-del") {
			t.Fatalf("url=%q", url)
		}
		if strings.Contains(url, "/art-del/") {
			t.Fatalf("version path must be absent: url=%q", url)
		}
		if !strings.Contains(url, "repoType=MAVEN") {
			t.Fatalf("missing repoType: url=%q", url)
		}
	})
}

func TestPackagesArtifactsDeleteVersionNpmDryRun(t *testing.T) {
	stdout, hits := pkgArtGDSetup(t, "del-ver-npm")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "delete",
		"--repo-id", "repo-gd",
		"--repo-type", "NPM",
		"--id", "art-del",
		"--version-id", "1.2.3",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtGDAssert(t, stdout, hits, "DELETE", string(risk.HighRiskWrite), func(t *testing.T, url string) {
		if !strings.Contains(url, "/artifacts/art-del/1.2.3") {
			t.Fatalf("url=%q", url)
		}
		if !strings.Contains(url, "repoType=NPM") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPackagesArtifactsDeleteWholeGenericDryRun(t *testing.T) {
	stdout, hits := pkgArtGDSetup(t, "del-gen")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "delete",
		"--repo-id", "repo-gd",
		"--repo-type", "GENERIC",
		"--id", "blob-9",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtGDAssert(t, stdout, hits, "DELETE", string(risk.HighRiskWrite), func(t *testing.T, url string) {
		if !strings.Contains(url, "/artifacts/blob-9") || !strings.Contains(url, "repoType=GENERIC") {
			t.Fatalf("url=%q", url)
		}
		if strings.Contains(url, "/blob-9/") {
			t.Fatalf("unexpected version: url=%q", url)
		}
	})
}

func TestPackagesArtifactsDeleteVersionDockerDryRun(t *testing.T) {
	stdout, hits := pkgArtGDSetup(t, "del-docker")
	rootCmd.SetArgs([]string{
		"packages", "artifacts", "delete",
		"--repo-id", "repo-gd",
		"--repo-type", "DOCKER",
		"--id", "img-1",
		"--version-id", "sha256-abc",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pkgArtGDAssert(t, stdout, hits, "DELETE", string(risk.HighRiskWrite), func(t *testing.T, url string) {
		if !strings.Contains(url, "/artifacts/img-1/sha256-abc") {
			t.Fatalf("url=%q", url)
		}
		if !strings.Contains(url, "repoType=DOCKER") {
			t.Fatalf("url=%q", url)
		}
	})
}
