package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func flowVGCUSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-flowvg-cu-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-flowvg-cu-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, pipelineFlowVGCreateCmd, "name", "variables", "description")
	resetStringFlags(t, pipelineFlowVGUpdateCmd, "id", "name", "variables", "description")
	return stdout, hits
}

func flowVGCUAssert(t *testing.T, stdout *bytes.Buffer, hits *int, wantMethod, pathSub string, checkQuery func(t *testing.T, q url.Values, rawURL string)) {
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
	rawURL, _ := req["url"].(string)
	if !strings.Contains(rawURL, pathSub) {
		t.Fatalf("url=%q want path %q", rawURL, pathSub)
	}
	if req["method"] != wantMethod {
		t.Fatalf("method=%v want %s", req["method"], wantMethod)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if checkQuery != nil {
		checkQuery(t, u.Query(), rawURL)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func TestPipelineFlowVGCreateNameVarsDryRun(t *testing.T) {
	stdout, hits := flowVGCUSetup(t, "create-basic")
	vars := `[{"name":"K1","value":"v1","isEncrypted":false}]`
	rootCmd.SetArgs([]string{
		"pipeline", "flow-variable-groups", "create",
		"--name", "vg-batch",
		"--variables", vars,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	flowVGCUAssert(t, stdout, hits, "POST", "/variableGroups", func(t *testing.T, q url.Values, rawURL string) {
		if q.Get("name") != "vg-batch" {
			t.Fatalf("name=%q url=%q", q.Get("name"), rawURL)
		}
		if q.Get("variables") != vars {
			t.Fatalf("variables=%q want %q", q.Get("variables"), vars)
		}
		if q.Has("description") {
			t.Fatalf("description must be omitted: %v", q)
		}
	})
}

func TestPipelineFlowVGCreateWithDescriptionDryRun(t *testing.T) {
	stdout, hits := flowVGCUSetup(t, "create-desc")
	vars := `[{"name":"A","value":"1","isEncrypted":false}]`
	rootCmd.SetArgs([]string{
		"pipeline", "flow-variable-groups", "create",
		"--name", "vg-desc",
		"--variables", vars,
		"--description", "batch desc",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	flowVGCUAssert(t, stdout, hits, "POST", "/variableGroups", func(t *testing.T, q url.Values, _ string) {
		if q.Get("description") != "batch desc" || q.Get("name") != "vg-desc" {
			t.Fatalf("q=%v", q)
		}
	})
}

func TestPipelineFlowVGCreateMultiVarsDryRun(t *testing.T) {
	stdout, hits := flowVGCUSetup(t, "create-multi")
	vars := `[{"name":"X","value":"1","isEncrypted":false},{"name":"Y","value":"2","isEncrypted":false}]`
	rootCmd.SetArgs([]string{
		"pipeline", "flow-variable-groups", "create",
		"--name", "vg-multi",
		"--variables", vars,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	flowVGCUAssert(t, stdout, hits, "POST", "/variableGroups", func(t *testing.T, q url.Values, _ string) {
		got := q.Get("variables")
		if !strings.Contains(got, `"name":"X"`) || !strings.Contains(got, `"name":"Y"`) {
			t.Fatalf("variables=%q", got)
		}
	})
}

func TestPipelineFlowVGCreateEncryptedVarDryRun(t *testing.T) {
	stdout, hits := flowVGCUSetup(t, "create-enc")
	vars := `[{"name":"SECRET","value":"s3cr3t","isEncrypted":true}]`
	rootCmd.SetArgs([]string{
		"pipeline", "flow-variable-groups", "create",
		"--name", "vg-enc",
		"--variables", vars,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	flowVGCUAssert(t, stdout, hits, "POST", "/variableGroups", func(t *testing.T, q url.Values, _ string) {
		got := q.Get("variables")
		if !strings.Contains(got, `"isEncrypted":true`) || !strings.Contains(got, "SECRET") {
			t.Fatalf("variables=%q", got)
		}
	})
}

func TestPipelineFlowVGUpdateNameVarsDryRun(t *testing.T) {
	stdout, hits := flowVGCUSetup(t, "update-basic")
	vars := `[{"name":"U1","value":"vu","isEncrypted":false}]`
	rootCmd.SetArgs([]string{
		"pipeline", "flow-variable-groups", "update",
		"--id", "vg-42",
		"--name", "vg-upd",
		"--variables", vars,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	flowVGCUAssert(t, stdout, hits, "PUT", "/variableGroups/vg-42", func(t *testing.T, q url.Values, rawURL string) {
		if q.Get("name") != "vg-upd" || q.Get("variables") != vars {
			t.Fatalf("q=%v url=%q", q, rawURL)
		}
		if q.Has("description") {
			t.Fatalf("description must be omitted: %v", q)
		}
	})
}

func TestPipelineFlowVGUpdateWithDescriptionDryRun(t *testing.T) {
	stdout, hits := flowVGCUSetup(t, "update-desc")
	vars := `[{"name":"U2","value":"2","isEncrypted":false}]`
	rootCmd.SetArgs([]string{
		"pipeline", "flow-variable-groups", "update",
		"--id", "vg-7",
		"--name", "vg-upd-d",
		"--variables", vars,
		"--description", "updated",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	flowVGCUAssert(t, stdout, hits, "PUT", "/variableGroups/vg-7", func(t *testing.T, q url.Values, _ string) {
		if q.Get("description") != "updated" || q.Get("name") != "vg-upd-d" {
			t.Fatalf("q=%v", q)
		}
	})
}

func TestPipelineFlowVGUpdateEmptyDescriptionOmitsDryRun(t *testing.T) {
	stdout, hits := flowVGCUSetup(t, "update-empty-desc")
	vars := `[{"name":"U3","value":"3","isEncrypted":false}]`
	rootCmd.SetArgs([]string{
		"pipeline", "flow-variable-groups", "update",
		"--id", "vg-8",
		"--name", "vg-upd-e",
		"--variables", vars,
		"--description", "",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	flowVGCUAssert(t, stdout, hits, "PUT", "/variableGroups/vg-8", func(t *testing.T, q url.Values, _ string) {
		if q.Has("description") {
			t.Fatalf("empty description must omit: %v", q)
		}
	})
}

func TestPipelineFlowVGCreateUpdateComboVarsDryRun(t *testing.T) {
	// Create with description + encrypted + plain vars in one request shape.
	stdout, hits := flowVGCUSetup(t, "create-combo")
	vars := `[{"name":"PLAIN","value":"p","isEncrypted":false},{"name":"ENC","value":"e","isEncrypted":true}]`
	rootCmd.SetArgs([]string{
		"pipeline", "flow-variable-groups", "create",
		"--name", "vg-combo",
		"--variables", vars,
		"--description", "combo",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	flowVGCUAssert(t, stdout, hits, "POST", "/variableGroups", func(t *testing.T, q url.Values, _ string) {
		if q.Get("name") != "vg-combo" || q.Get("description") != "combo" {
			t.Fatalf("q=%v", q)
		}
		got := q.Get("variables")
		if !strings.Contains(got, "PLAIN") || !strings.Contains(got, "ENC") || !strings.Contains(got, `"isEncrypted":true`) {
			t.Fatalf("variables=%q", got)
		}
	})
}