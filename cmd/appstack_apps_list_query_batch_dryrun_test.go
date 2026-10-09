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

func appsListQSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-apps-list-q-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-apps-list-q-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, appstackAppsListCmd, "next-token", "order-by", "sort", "tags", "per-page")
	return stdout, hits
}

func appsListQAssert(t *testing.T, stdout *bytes.Buffer, hits *int, checkURL func(t *testing.T, url string)) {
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
	if !strings.Contains(url, "/apps:search") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if !strings.Contains(url, "pagination=keyset") {
		t.Fatalf("missing pagination=keyset: url=%q", url)
	}
	if checkURL != nil {
		checkURL(t, url)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func TestAppstackAppsListDefaultsDryRun(t *testing.T) {
	stdout, hits := appsListQSetup(t, "defaults")
	rootCmd.SetArgs([]string{"appstack", "apps", "list", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	appsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "orderBy=id") || !strings.Contains(url, "perPage=20") {
			t.Fatalf("defaults missing: url=%q", url)
		}
		if strings.Contains(url, "sort=") || strings.Contains(url, "tags=") || strings.Contains(url, "nextToken=") {
			t.Fatalf("optional must be omitted: url=%q", url)
		}
	})
}

func TestAppstackAppsListOrderByNameDryRun(t *testing.T) {
	stdout, hits := appsListQSetup(t, "order-name")
	rootCmd.SetArgs([]string{"appstack", "apps", "list", "--order-by", "name", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	appsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "orderBy=name") {
			t.Fatalf("url=%q", url)
		}
		if strings.Contains(url, "sort=") {
			t.Fatalf("sort must be omitted: url=%q", url)
		}
	})
}

func TestAppstackAppsListSortAscDryRun(t *testing.T) {
	stdout, hits := appsListQSetup(t, "sort-asc")
	rootCmd.SetArgs([]string{"appstack", "apps", "list", "--sort", "asc", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	appsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "sort=asc") || !strings.Contains(url, "orderBy=id") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestAppstackAppsListSortDescDryRun(t *testing.T) {
	stdout, hits := appsListQSetup(t, "sort-desc")
	rootCmd.SetArgs([]string{"appstack", "apps", "list", "--sort", "desc", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	appsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "sort=desc") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestAppstackAppsListPerPageDryRun(t *testing.T) {
	stdout, hits := appsListQSetup(t, "perpage")
	rootCmd.SetArgs([]string{"appstack", "apps", "list", "--per-page", "50", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	appsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "perPage=50") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestAppstackAppsListPerPageZeroOmitsDryRun(t *testing.T) {
	stdout, hits := appsListQSetup(t, "perpage0")
	rootCmd.SetArgs([]string{"appstack", "apps", "list", "--per-page", "0", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	appsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if strings.Contains(url, "perPage=") {
			t.Fatalf("perPage must be omitted when 0: url=%q", url)
		}
		if !strings.Contains(url, "orderBy=id") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestAppstackAppsListNextTokenOnlyDryRun(t *testing.T) {
	stdout, hits := appsListQSetup(t, "next")
	rootCmd.SetArgs([]string{"appstack", "apps", "list", "--next-token", "tok-only", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	appsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "nextToken=tok-only") {
			t.Fatalf("url=%q", url)
		}
		if strings.Contains(url, "tags=") {
			t.Fatalf("tags must be omitted: url=%q", url)
		}
	})
}

func TestAppstackAppsListTagsOnlyDryRun(t *testing.T) {
	stdout, hits := appsListQSetup(t, "tags")
	rootCmd.SetArgs([]string{"appstack", "apps", "list", "--tags", "edge", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	appsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "tags=edge") {
			t.Fatalf("url=%q", url)
		}
		if strings.Contains(url, "nextToken=") {
			t.Fatalf("nextToken must be omitted: url=%q", url)
		}
	})
}
