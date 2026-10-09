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

func TestCodeupCompareStraightDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-codeup-compare-straight-not-real")
	t.Setenv(config.EnvOrganizationID, "org-codeup-compare-straight")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupCompareCmd, "repo", "from", "to", "source-type", "target-type", "straight")
	rootCmd.SetArgs([]string{
		"codeup", "compare",
		"--repo", "4951346",
		"--from", "v1.0.0",
		"--to", "v1.1.0",
		"--source-type", "tag",
		"--target-type", "tag",
		"--straight", "true",
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
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/repositories/4951346/compares") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "from=v1.0.0") || !strings.Contains(url, "to=v1.1.0") {
		t.Fatalf("from/to missing url=%q", url)
	}
	if !strings.Contains(url, "sourceType=tag") || !strings.Contains(url, "targetType=tag") {
		t.Fatalf("types missing url=%q", url)
	}
	if !strings.Contains(url, "straight=true") {
		t.Fatalf("straight missing url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}