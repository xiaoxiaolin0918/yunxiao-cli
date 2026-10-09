package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func TestCodeupReposCreateDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-codeup-repos-create-not-real")
	t.Setenv(config.EnvOrganizationID, "org-codeup-repos-create")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupReposCreateCmd, "name", "path", "description", "visibility", "avatar-url", "readme-type")
	rootCmd.SetArgs([]string{
		"codeup", "repos", "create",
		"--name", "demo-repo",
		"--path", "demo-repo",
		"--visibility", "private",
		"--description", "dry-run demo",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
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
	if !strings.Contains(url, "/codeup/organizations/org-codeup-repos-create/repositories") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "createParentPath=true") {
		t.Fatalf("missing createParentPath in url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["name"] != "demo-repo" || body["path"] != "demo-repo" {
		t.Fatalf("body=%v", body)
	}
	if body["visibility"] != "private" || body["description"] != "dry-run demo" {
		t.Fatalf("body extras=%v", body)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}