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

func asCOVerQSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-as-co-ver-q-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-as-co-ver-q-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, appstackCOVersionsCmd, "app", "env-names", "creators")
	for _, name := range []string{"current", "page-size"} {
		f := appstackCOVersionsCmd.Flags().Lookup(name)
		if f != nil {
			_ = appstackCOVersionsCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	return stdout, hits
}

func asCOVerQAssert(t *testing.T, stdout *bytes.Buffer, hits *int, checkURL func(t *testing.T, url string)) {
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
	if !strings.Contains(url, "/apps/demo-batch/changeOrders/versions") {
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

func TestAppstackCOVersionsEnvNamesDryRun(t *testing.T) {
	stdout, hits := asCOVerQSetup(t, "env")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "versions",
		"--app", "demo-batch",
		"--env-names", "prod,staging",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOVerQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "envNames=prod") || !strings.Contains(url, "staging") {
			t.Fatalf("url=%q", url)
		}
		if strings.Contains(url, "creators=") {
			t.Fatalf("creators must be omitted: %q", url)
		}
	})
}

func TestAppstackCOVersionsCreatorsDryRun(t *testing.T) {
	stdout, hits := asCOVerQSetup(t, "creators")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "versions",
		"--app", "demo-batch",
		"--creators", "u-a,u-b",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOVerQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "creators=u-a") || !strings.Contains(url, "u-b") {
			t.Fatalf("url=%q", url)
		}
		if strings.Contains(url, "envNames=") {
			t.Fatalf("envNames must be omitted: %q", url)
		}
	})
}

func TestAppstackCOVersionsEnvAndCreatorsDryRun(t *testing.T) {
	stdout, hits := asCOVerQSetup(t, "both")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "versions",
		"--app", "demo-batch",
		"--env-names", "dev",
		"--creators", "u-1",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOVerQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "envNames=dev") || !strings.Contains(url, "creators=u-1") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestAppstackCOVersionsPageDryRun(t *testing.T) {
	stdout, hits := asCOVerQSetup(t, "page")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "versions",
		"--app", "demo-batch",
		"--current", "3",
		"--page-size", "25",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOVerQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "current=3") || !strings.Contains(url, "pageSize=25") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestAppstackCOVersionsDefaultPageDryRun(t *testing.T) {
	stdout, hits := asCOVerQSetup(t, "defaults")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "versions",
		"--app", "demo-batch",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOVerQAssert(t, stdout, hits, func(t *testing.T, url string) {
		// defaults current=1 page-size=20
		if !strings.Contains(url, "current=1") || !strings.Contains(url, "pageSize=20") {
			t.Fatalf("defaults missing: url=%q", url)
		}
		if strings.Contains(url, "envNames=") || strings.Contains(url, "creators=") {
			t.Fatalf("filters must be omitted: %q", url)
		}
	})
}

func TestAppstackCOVersionsEmptyFiltersOmitsDryRun(t *testing.T) {
	stdout, hits := asCOVerQSetup(t, "empty")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "versions",
		"--app", "demo-batch",
		"--env-names", "",
		"--creators", "",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOVerQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if strings.Contains(url, "envNames=") || strings.Contains(url, "creators=") {
			t.Fatalf("empty filters must omit: %q", url)
		}
	})
}

func TestAppstackCOVersionsEnvAndPageDryRun(t *testing.T) {
	stdout, hits := asCOVerQSetup(t, "env-page")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "versions",
		"--app", "demo-batch",
		"--env-names", "prod",
		"--current", "2",
		"--page-size", "10",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOVerQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "envNames=prod") || !strings.Contains(url, "current=2") || !strings.Contains(url, "pageSize=10") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestAppstackCOVersionsFullQueryDryRun(t *testing.T) {
	stdout, hits := asCOVerQSetup(t, "full")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "versions",
		"--app", "demo-batch",
		"--env-names", "prod,dev",
		"--creators", "alice,bob",
		"--current", "4",
		"--page-size", "15",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOVerQAssert(t, stdout, hits, func(t *testing.T, url string) {
		for _, want := range []string{"envNames=", "creators=", "current=4", "pageSize=15"} {
			if !strings.Contains(url, want) {
				t.Fatalf("missing %q in url=%q", want, url)
			}
		}
		if !strings.Contains(url, "prod") || !strings.Contains(url, "alice") {
			t.Fatalf("filter values missing: url=%q", url)
		}
	})
}