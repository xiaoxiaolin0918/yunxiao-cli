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

func asVGCUSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-as-vg-cu-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-as-vg-cu-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, appstackVGCreateCmd, "app", "name", "from-revision-sha", "display-name", "message", "branch-name", "vars")
	resetStringFlags(t, appstackVGUpdateCmd, "app", "name", "from-revision-sha", "new-name", "display-name", "message", "branch-name", "vars")
	return stdout, hits
}

func asVGCUAssert(t *testing.T, stdout *bytes.Buffer, hits *int, wantMethod, pathSub string, check func(t *testing.T, body map[string]any, req map[string]any)) {
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

func TestAppstackVGCreateMinimalDryRun(t *testing.T) {
	stdout, hits := asVGCUSetup(t, "create-min")
	rootCmd.SetArgs([]string{
		"appstack", "variable-groups", "create",
		"--app", "demo-app",
		"--from-revision-sha", "sha-base",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asVGCUAssert(t, stdout, hits, "POST", "/apps/demo-app/variableGroup", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["fromRevisionSha"] != "sha-base" {
			t.Fatalf("fromRevisionSha=%v", body["fromRevisionSha"])
		}
		for _, k := range []string{"name", "displayName", "message", "branchName", "vars"} {
			if _, has := body[k]; has {
				t.Fatalf("optional %s must be omitted: %#v", k, body)
			}
		}
	})
}

func TestAppstackVGCreateNameDisplayDryRun(t *testing.T) {
	stdout, hits := asVGCUSetup(t, "create-name")
	rootCmd.SetArgs([]string{
		"appstack", "variable-groups", "create",
		"--app", "demo-app",
		"--from-revision-sha", "sha-1",
		"--name", "vg-batch",
		"--display-name", "Batch VG",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asVGCUAssert(t, stdout, hits, "POST", "/apps/demo-app/variableGroup", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["name"] != "vg-batch" || body["displayName"] != "Batch VG" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAppstackVGCreateMessageBranchDryRun(t *testing.T) {
	stdout, hits := asVGCUSetup(t, "create-msg")
	rootCmd.SetArgs([]string{
		"appstack", "variable-groups", "create",
		"--app", "demo-app",
		"--from-revision-sha", "sha-2",
		"--message", "init vg",
		"--branch-name", "develop",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asVGCUAssert(t, stdout, hits, "POST", "/apps/demo-app/variableGroup", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["message"] != "init vg" || body["branchName"] != "develop" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAppstackVGCreateVarsDryRun(t *testing.T) {
	stdout, hits := asVGCUSetup(t, "create-vars")
	vars := `[{"key":"K1","value":"v1","description":"d1"}]`
	rootCmd.SetArgs([]string{
		"appstack", "variable-groups", "create",
		"--app", "demo-app",
		"--from-revision-sha", "sha-3",
		"--vars", vars,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asVGCUAssert(t, stdout, hits, "POST", "/apps/demo-app/variableGroup", func(t *testing.T, body map[string]any, _ map[string]any) {
		arr, ok := body["vars"].([]any)
		if !ok || len(arr) != 1 {
			t.Fatalf("vars=%#v", body["vars"])
		}
		m, _ := arr[0].(map[string]any)
		if m["key"] != "K1" || m["value"] != "v1" {
			t.Fatalf("var0=%v", m)
		}
	})
}

func TestAppstackVGUpdateMinimalDryRun(t *testing.T) {
	stdout, hits := asVGCUSetup(t, "update-min")
	rootCmd.SetArgs([]string{
		"appstack", "variable-groups", "update",
		"--app", "demo-app",
		"--name", "vg-old",
		"--from-revision-sha", "sha-u0",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asVGCUAssert(t, stdout, hits, "PUT", "/apps/demo-app/variableGroup/vg-old", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["fromRevisionSha"] != "sha-u0" {
			t.Fatalf("fromRevisionSha=%v", body["fromRevisionSha"])
		}
		for _, k := range []string{"name", "displayName", "message", "branchName", "vars"} {
			if _, has := body[k]; has {
				t.Fatalf("optional %s must be omitted: %#v", k, body)
			}
		}
	})
}

func TestAppstackVGUpdateNewNameDisplayDryRun(t *testing.T) {
	stdout, hits := asVGCUSetup(t, "update-rename")
	rootCmd.SetArgs([]string{
		"appstack", "variable-groups", "update",
		"--app", "demo-app",
		"--name", "vg-old",
		"--from-revision-sha", "sha-u1",
		"--new-name", "vg-new",
		"--display-name", "Renamed",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asVGCUAssert(t, stdout, hits, "PUT", "/apps/demo-app/variableGroup/vg-old", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["name"] != "vg-new" || body["displayName"] != "Renamed" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAppstackVGUpdateMessageBranchVarsDryRun(t *testing.T) {
	stdout, hits := asVGCUSetup(t, "update-vars")
	vars := `[{"key":"K2","value":"v2"}]`
	rootCmd.SetArgs([]string{
		"appstack", "variable-groups", "update",
		"--app", "demo-app",
		"--name", "vg-old",
		"--from-revision-sha", "sha-u2",
		"--message", "bump",
		"--branch-name", "master",
		"--vars", vars,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asVGCUAssert(t, stdout, hits, "PUT", "/apps/demo-app/variableGroup/vg-old", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["message"] != "bump" || body["branchName"] != "master" {
			t.Fatalf("body=%v", body)
		}
		arr, ok := body["vars"].([]any)
		if !ok || len(arr) != 1 {
			t.Fatalf("vars=%#v", body["vars"])
		}
	})
}

func TestAppstackVGCreateFullBodyDryRun(t *testing.T) {
	stdout, hits := asVGCUSetup(t, "create-full")
	vars := `[{"key":"A","value":"1","description":"x"},{"key":"B","value":"2"}]`
	rootCmd.SetArgs([]string{
		"appstack", "variable-groups", "create",
		"--app", "demo-app",
		"--from-revision-sha", "sha-full",
		"--name", "vg-full",
		"--display-name", "Full",
		"--message", "all fields",
		"--branch-name", "feature/vg",
		"--vars", vars,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asVGCUAssert(t, stdout, hits, "POST", "/apps/demo-app/variableGroup", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["fromRevisionSha"] != "sha-full" || body["name"] != "vg-full" || body["displayName"] != "Full" {
			t.Fatalf("body=%v", body)
		}
		if body["message"] != "all fields" || body["branchName"] != "feature/vg" {
			t.Fatalf("body=%v", body)
		}
		arr, ok := body["vars"].([]any)
		if !ok || len(arr) != 2 {
			t.Fatalf("vars=%#v", body["vars"])
		}
	})
}