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

func pipeRunListQSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pipe-run-list-q-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipe-run-list-q-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, pipelineRunListCmd, "pipeline-id", "status", "sort")
	for _, name := range []string{"page", "per-page"} {
		f := pipelineRunListCmd.Flags().Lookup(name)
		if f != nil {
			_ = pipelineRunListCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	return stdout, hits
}

func pipeRunListQAssert(t *testing.T, stdout *bytes.Buffer, hits *int, checkURL func(t *testing.T, url string)) {
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
	if !strings.Contains(url, "/pipelines/pipe-batch/runs") {
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

func TestPipelineRunListStatusFailDryRun(t *testing.T) {
	stdout, hits := pipeRunListQSetup(t, "fail")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "list",
		"--pipeline-id", "pipe-batch",
		"--status", "FAIL",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeRunListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "status=FAIL") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPipelineRunListStatusSuccessDryRun(t *testing.T) {
	stdout, hits := pipeRunListQSetup(t, "success")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "list",
		"--pipeline-id", "pipe-batch",
		"--status", "SUCCESS",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeRunListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "status=SUCCESS") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPipelineRunListStatusCanceledDryRun(t *testing.T) {
	stdout, hits := pipeRunListQSetup(t, "canceled")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "list",
		"--pipeline-id", "pipe-batch",
		"--status", "CANCELED",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeRunListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "status=CANCELED") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPipelineRunListNoStatusOmitsDryRun(t *testing.T) {
	stdout, hits := pipeRunListQSetup(t, "no-status")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "list",
		"--pipeline-id", "pipe-batch",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeRunListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if strings.Contains(url, "status=") {
			t.Fatalf("status must be omitted: %q", url)
		}
		if !strings.Contains(url, "page=1") {
			t.Fatalf("default page missing: %q", url)
		}
	})
}

func TestPipelineRunListEmptyStatusOmitsDryRun(t *testing.T) {
	stdout, hits := pipeRunListQSetup(t, "empty-status")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "list",
		"--pipeline-id", "pipe-batch",
		"--status", "",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeRunListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if strings.Contains(url, "status=") {
			t.Fatalf("empty status must omit: %q", url)
		}
	})
}

func TestPipelineRunListPagePerPageDryRun(t *testing.T) {
	stdout, hits := pipeRunListQSetup(t, "page")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "list",
		"--pipeline-id", "pipe-batch",
		"--page", "4",
		"--per-page", "15",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeRunListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "page=4") || !strings.Contains(url, "perPage=15") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPipelineRunListStatusAndPageDryRun(t *testing.T) {
	stdout, hits := pipeRunListQSetup(t, "status-page")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "list",
		"--pipeline-id", "pipe-batch",
		"--status", "RUNNING",
		"--page", "2",
		"--per-page", "30",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeRunListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "status=RUNNING") || !strings.Contains(url, "page=2") || !strings.Contains(url, "perPage=30") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestPipelineRunListFullQueryDryRun(t *testing.T) {
	stdout, hits := pipeRunListQSetup(t, "full")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "list",
		"--pipeline-id", "pipe-batch",
		"--status", "FAIL",
		"--page", "3",
		"--per-page", "10",
		"--sort", "asc",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeRunListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		// --sort is client-side; only assert HTTP query params
		for _, want := range []string{"status=FAIL", "page=3", "perPage=10"} {
			if !strings.Contains(url, want) {
				t.Fatalf("missing %q in url=%q", want, url)
			}
		}
	})
}