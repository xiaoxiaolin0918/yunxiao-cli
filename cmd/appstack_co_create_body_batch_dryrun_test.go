package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func asCOCreateSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-as-co-create-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-as-co-create-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, appstackCOCreateCmd, "app", "data", "data-file")
	return stdout, hits
}

func asCOCreateAssert(t *testing.T, stdout *bytes.Buffer, hits *int, check func(t *testing.T, body map[string]any, req map[string]any)) {
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
	if !strings.Contains(url, "/apps/demo-co-batch/changeOrders") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
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

func TestAppstackCOCreateDataDeployDryRun(t *testing.T) {
	stdout, hits := asCOCreateSetup(t, "deploy")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "create",
		"--app", "demo-co-batch",
		"--data", "{\"changeOrderName\":\"deploy-1\",\"type\":\"Deploy\"}",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["changeOrderName"] != "deploy-1" || body["type"] != "Deploy" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAppstackCOCreateDataRollbackDryRun(t *testing.T) {
	stdout, hits := asCOCreateSetup(t, "rollback")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "create",
		"--app", "demo-co-batch",
		"--data", "{\"changeOrderName\":\"rb-1\",\"type\":\"Rollback\"}",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["type"] != "Rollback" || body["changeOrderName"] != "rb-1" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAppstackCOCreateDataEnvsDryRun(t *testing.T) {
	stdout, hits := asCOCreateSetup(t, "envs")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "create",
		"--app", "demo-co-batch",
		"--data", "{\"changeOrderName\":\"e1\",\"type\":\"Deploy\",\"envs\":{\"prod\":{\"values\":{\"k\":\"v\"}}}}",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		envs, _ := body["envs"].(map[string]any)
		prod, _ := envs["prod"].(map[string]any)
		vals, _ := prod["values"].(map[string]any)
		if vals["k"] != "v" {
			t.Fatalf("envs=%v", body["envs"])
		}
	})
}

func TestAppstackCOCreateDataShaDryRun(t *testing.T) {
	stdout, hits := asCOCreateSetup(t, "sha")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "create",
		"--app", "demo-co-batch",
		"--data", "{\"changeOrderName\":\"sha-1\",\"type\":\"Deploy\",\"orchestrationRevisionSha\":\"abc123def\"}",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["orchestrationRevisionSha"] != "abc123def" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAppstackCOCreateDataExtraFieldsDryRun(t *testing.T) {
	stdout, hits := asCOCreateSetup(t, "extra")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "create",
		"--app", "demo-co-batch",
		"--data", "{\"changeOrderName\":\"x1\",\"type\":\"Deploy\",\"description\":\"batch\",\"force\":true}",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["description"] != "batch" || body["force"] != true {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAppstackCOCreateDataFileDryRun(t *testing.T) {
	stdout, hits := asCOCreateSetup(t, "file")
	dir := t.TempDir()
	fp := filepath.Join(dir, "co.json")
	if err := os.WriteFile(fp, []byte("{\"changeOrderName\":\"from-file\",\"type\":\"Deploy\"}"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "create",
		"--app", "demo-co-batch",
		"--data-file", fp,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["changeOrderName"] != "from-file" || body["type"] != "Deploy" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAppstackCOCreateDataAtFileDryRun(t *testing.T) {
	stdout, hits := asCOCreateSetup(t, "at-file")
	dir := t.TempDir()
	fp := filepath.Join(dir, "co-at.json")
	if err := os.WriteFile(fp, []byte("{\"changeOrderName\":\"from-at\",\"type\":\"Deploy\",\"note\":\"via-at\"}"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "create",
		"--app", "demo-co-batch",
		"--data", "@" + fp,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["changeOrderName"] != "from-at" || body["note"] != "via-at" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAppstackCOCreateDataNestedArrayDryRun(t *testing.T) {
	stdout, hits := asCOCreateSetup(t, "nested")
	rootCmd.SetArgs([]string{
		"appstack", "change-orders", "create",
		"--app", "demo-co-batch",
		"--data", "{\"changeOrderName\":\"arr-1\",\"type\":\"Deploy\",\"targets\":[\"a\",\"b\"],\"meta\":{\"n\":2}}",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asCOCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		targets, _ := body["targets"].([]any)
		if len(targets) != 2 || targets[0] != "a" || targets[1] != "b" {
			t.Fatalf("targets=%v", body["targets"])
		}
		meta, _ := body["meta"].(map[string]any)
		if meta["n"] != float64(2) {
			t.Fatalf("meta=%v", body["meta"])
		}
	})
}
