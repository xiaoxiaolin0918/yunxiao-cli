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

// Dry-run for codeup mrs list --search/--state (status dry-run exists without these query keys).
func TestCodeupMrsListSearchStateDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-list-search-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-list-search")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsListCmd, "repo", "search", "state", "status", "source", "target", "sort")
	_ = codeupMrsListCmd.Flags().Set("all", "false")
	_ = codeupMrsListCmd.Flags().Set("page", "1")
	_ = codeupMrsListCmd.Flags().Set("per-page", "20")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "list",
		"--repo", "4951320",
		"--search", "fix",
		"--state", "opened",
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
	if env.Risk != string(risk.Read) && env.Risk != "read" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/changeRequests") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "search=fix") || !strings.Contains(url, "state=opened") {
		t.Fatalf("missing search/state: url=%q", url)
	}
	if !strings.Contains(url, "projectIds=4951320") {
		t.Fatalf("missing projectIds: url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}