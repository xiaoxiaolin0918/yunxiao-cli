package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

func TestRedactPackagesRepoDescriptorNestedProxies(t *testing.T) {
	desc := `{
  "hybridDescriptor": {
    "proxies": [
      {"url": "https://mirror.example", "username": "u-plain", "password": "p-plain"},
      {"url": "https://other", "username": "", "password": null}
    ]
  },
  "isOverwriteArtifact": true
}`
	in := []any{
		map[string]any{
			"repoId":         "r1",
			"repoName":       "maven-release",
			"repoType":       "MAVEN",
			"repoDescriptor": desc,
			"username":       "top-user",
			"password":       "top-pass",
		},
	}
	out := redactPackagesSensitive(in)
	items, ok := out.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("out=%T %#v", out, out)
	}
	item := items[0].(map[string]any)
	if item["repoName"] != "maven-release" {
		t.Fatalf("repoName clobbered: %#v", item["repoName"])
	}
	if item["username"] != "***" || item["password"] != "***" {
		t.Fatalf("top-level secrets: username=%v password=%v", item["username"], item["password"])
	}
	rd, _ := item["repoDescriptor"].(string)
	if rd == "" || strings.Contains(rd, "p-plain") || strings.Contains(rd, "u-plain") {
		t.Fatalf("repoDescriptor still has plaintext: %s", rd)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(rd), &parsed); err != nil {
		t.Fatalf("redacted descriptor not JSON: %v / %s", err, rd)
	}
	proxies := parsed["hybridDescriptor"].(map[string]any)["proxies"].([]any)
	p0 := proxies[0].(map[string]any)
	if p0["username"] != "***" || p0["password"] != "***" {
		t.Fatalf("proxy0=%#v", p0)
	}
	if p0["url"] != "https://mirror.example" {
		t.Fatalf("url clobbered: %#v", p0["url"])
	}
	p1 := proxies[1].(map[string]any)
	if p1["username"] != "" || p1["password"] != nil {
		t.Fatalf("empty/null should stay: %#v", p1)
	}
	raw, _ := json.Marshal(out)
	for _, bad := range []string{"p-plain", "u-plain", "top-pass", "top-user"} {
		if strings.Contains(string(raw), bad) {
			t.Fatalf("plaintext %q still in output: %s", bad, raw)
		}
	}
}

func TestPackagesReposListRedactsPasswordInOutput(t *testing.T) {
	descObj := map[string]any{
		"hybridDescriptor": map[string]any{
			"proxies": []any{
				map[string]any{"username": "secret-user", "password": "secret-pass"},
			},
		},
	}
	descBytes, _ := json.Marshal(descObj)
	payload := []any{
		map[string]any{
			"repoId":         "repo-npm",
			"repoName":       "npm-registry",
			"repoType":       "NPM",
			"repoDescriptor": string(descBytes),
		},
	}
	body, _ := json.Marshal(payload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.Contains(r.URL.Path, "/repositories") {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pkg-redact-386-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pkg-redact-386")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, packagesReposListCmd, "repo-types", "repo-categories", "page", "per-page")
	prevDry := globalDryRun
	globalDryRun = false
	t.Cleanup(func() { globalDryRun = prevDry })
	rootCmd.SetArgs([]string{"packages", "repos", "list", "--per-page", "20"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	out := stdout.String()
	for _, bad := range []string{"secret-pass", "secret-user"} {
		if strings.Contains(out, bad) {
			t.Fatalf("plaintext %q leaked in stdout: %s", bad, out)
		}
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK {
		t.Fatalf("envelope=%+v", env)
	}
	raw, _ := json.Marshal(env.Data)
	if strings.Contains(string(raw), "secret-pass") || strings.Contains(string(raw), "secret-user") {
		t.Fatalf("data still has secrets: %s", raw)
	}
	if !strings.Contains(string(raw), "***") {
		t.Fatalf("expected *** mask in data: %s", raw)
	}
}
