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

func TestCodeupMrsPlusCreateDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-plus-create-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-plus-create")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsPlusCreateCmd, "repo", "source", "target", "title", "description", "work-item", "reviewer")
	_ = codeupMrsPlusCreateCmd.Flags().Set("wip", "false")
	_ = codeupMrsPlusCreateCmd.Flags().Set("full", "false")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "+create",
		"--repo", "4951320",
		"--source", "feat/quota-burn",
		"--target", "master",
		"--title", "feat: quota burn",
		"--reviewer", "uid-a,uid-b",
		"--wip",
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
	if !strings.Contains(url, "/repositories/4951320/changeRequests") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	bodyRaw, _ := json.Marshal(req["body"])
	var body map[string]any
	_ = json.Unmarshal(bodyRaw, &body)
	title, _ := body["title"].(string)
	if !strings.HasPrefix(title, "WIP:") {
		t.Fatalf("want WIP title, got %q", title)
	}
	reviewers, _ := body["reviewerUserIds"].([]any)
	if len(reviewers) != 2 {
		t.Fatalf("reviewers=%v", body["reviewerUserIds"])
	}
	if body["sourceBranch"] != "feat/quota-burn" || body["targetBranch"] != "master" {
		t.Fatalf("branches=%v / %v", body["sourceBranch"], body["targetBranch"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}
