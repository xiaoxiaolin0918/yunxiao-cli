package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func pipeCUBodySetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pipe-cu-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pipe-cu-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, pipelineCreateCmd, "name", "content", "file")
	resetStringFlags(t, pipelineUpdateCmd, "id", "name", "content", "file", "validate", "check")
	return stdout, hits
}

func pipeCUBodyAssert(t *testing.T, stdout *bytes.Buffer, hits *int, wantMethod, pathSub string, check func(t *testing.T, body map[string]any, req map[string]any)) {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	if env.Risk != string(risk.HighRiskWrite) && env.Risk != "high-risk-write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, pathSub) {
		t.Fatalf("url=%q want %q", url, pathSub)
	}
	if req["method"] != wantMethod {
		t.Fatalf("method=%v want %s", req["method"], wantMethod)
	}
	body, _ := req["body"].(map[string]any)
	if body == nil {
		t.Fatalf("body nil: %v", req)
	}
	if check != nil {
		check(t, body, req)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func pipeCUNum(t *testing.T, v any) float64 {
	t.Helper()
	n, ok := v.(float64)
	if !ok {
		t.Fatalf("not number: %#v", v)
	}
	return n
}

func TestPipelineCreateContentBodyDryRun(t *testing.T) {
	stdout, hits := pipeCUBodySetup(t, "create-content")
	yaml := "sources: []\nstages: []\n"
	rootCmd.SetArgs([]string{
		"pipeline", "create",
		"--name", "pipe-batch-a",
		"--content", yaml,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeCUBodyAssert(t, stdout, hits, "POST", "/pipelines", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["name"] != "pipe-batch-a" {
			t.Fatalf("body=%v", body)
		}
		if pipeCUNum(t, body["content_bytes"]) != float64(len(yaml)) {
			t.Fatalf("content_bytes=%v want %d", body["content_bytes"], len(yaml))
		}
		if body["content_preview"] != yaml {
			t.Fatalf("content_preview=%v", body["content_preview"])
		}
		if _, has := body["content"]; has {
			t.Fatalf("raw content must not be in preview body: %#v", body)
		}
	})
}

func TestPipelineCreateNameOnlyDiffDryRun(t *testing.T) {
	stdout, hits := pipeCUBodySetup(t, "create-name")
	yaml := "sources: []\n"
	rootCmd.SetArgs([]string{
		"pipeline", "create",
		"--name", "other-name",
		"--content", yaml,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeCUBodyAssert(t, stdout, hits, "POST", "/pipelines", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["name"] != "other-name" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestPipelineCreateFileBodyDryRun(t *testing.T) {
	stdout, hits := pipeCUBodySetup(t, "create-file")
	fp := "pipe-cu-batch-create.yml"
	yaml := "sources: []\nstages:\n  - name: build\n"
	if err := os.WriteFile(fp, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(fp) })
	rootCmd.SetArgs([]string{
		"pipeline", "create",
		"--name", "from-file",
		"--file", fp,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeCUBodyAssert(t, stdout, hits, "POST", "/pipelines", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["name"] != "from-file" {
			t.Fatalf("body=%v", body)
		}
		if pipeCUNum(t, body["content_bytes"]) != float64(len(yaml)) {
			t.Fatalf("content_bytes=%v", body["content_bytes"])
		}
		if body["content_preview"] != yaml {
			t.Fatalf("preview=%v", body["content_preview"])
		}
	})
}

func TestPipelineCreateLongContentTruncatesDryRun(t *testing.T) {
	stdout, hits := pipeCUBodySetup(t, "create-long")
	long := strings.Repeat("a", 250)
	rootCmd.SetArgs([]string{
		"pipeline", "create",
		"--name", "long-yaml",
		"--content", long,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeCUBodyAssert(t, stdout, hits, "POST", "/pipelines", func(t *testing.T, body map[string]any, _ map[string]any) {
		if pipeCUNum(t, body["content_bytes"]) != 250 {
			t.Fatalf("content_bytes=%v", body["content_bytes"])
		}
		want := strings.Repeat("a", 200) + "\u2026"
		if body["content_preview"] != want {
			t.Fatalf("preview=%q want prefix+ellipsis", body["content_preview"])
		}
	})
}

func TestPipelineUpdateContentBodyDryRun(t *testing.T) {
	stdout, hits := pipeCUBodySetup(t, "upd-content")
	yaml := "sources: []\nstages: []\n"
	rootCmd.SetArgs([]string{
		"pipeline", "update",
		"--id", "pipe-cu-1",
		"--name", "upd-a",
		"--content", yaml,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeCUBodyAssert(t, stdout, hits, "PUT", "/pipelines/pipe-cu-1", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["name"] != "upd-a" {
			t.Fatalf("body=%v", body)
		}
		if pipeCUNum(t, body["content_bytes"]) != float64(len(yaml)) {
			t.Fatalf("content_bytes=%v", body["content_bytes"])
		}
		if body["content_preview"] != yaml {
			t.Fatalf("preview=%v", body["content_preview"])
		}
	})
}

func TestPipelineUpdateFileBodyDryRun(t *testing.T) {
	stdout, hits := pipeCUBodySetup(t, "upd-file")
	fp := "pipe-cu-batch-update.yml"
	yaml := "sources: []\n"
	if err := os.WriteFile(fp, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(fp) })
	rootCmd.SetArgs([]string{
		"pipeline", "update",
		"--id", "pipe-cu-2",
		"--name", "upd-file",
		"--file", fp,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeCUBodyAssert(t, stdout, hits, "PUT", "/pipelines/pipe-cu-2", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["name"] != "upd-file" || pipeCUNum(t, body["content_bytes"]) != float64(len(yaml)) {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestPipelineUpdateNamePathDryRun(t *testing.T) {
	stdout, hits := pipeCUBodySetup(t, "upd-name")
	rootCmd.SetArgs([]string{
		"pipeline", "update",
		"--id", "pipe-cu-9",
		"--name", "renamed",
		"--content", "x: 1\n",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeCUBodyAssert(t, stdout, hits, "PUT", "/pipelines/pipe-cu-9", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["name"] != "renamed" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestPipelineUpdateLongContentTruncatesDryRun(t *testing.T) {
	stdout, hits := pipeCUBodySetup(t, "upd-long")
	long := strings.Repeat("b", 220)
	rootCmd.SetArgs([]string{
		"pipeline", "update",
		"--id", "pipe-cu-3",
		"--name", "long-upd",
		"--content", long,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeCUBodyAssert(t, stdout, hits, "PUT", "/pipelines/pipe-cu-3", func(t *testing.T, body map[string]any, _ map[string]any) {
		if pipeCUNum(t, body["content_bytes"]) != 220 {
			t.Fatalf("content_bytes=%v", body["content_bytes"])
		}
		want := strings.Repeat("b", 200) + "\u2026"
		if body["content_preview"] != want {
			t.Fatalf("preview=%q", body["content_preview"])
		}
	})
}
