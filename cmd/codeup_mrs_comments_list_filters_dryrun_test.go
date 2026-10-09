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

// Dry-run for codeup mrs comments list --state/--file-path/--patchset-biz-ids/--resolved body filters.
func TestCodeupMrsCommentsListFiltersDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-comments-filters-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-comments-filters")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsCommentsListCmd, "repo", "local-id", "comment-type", "state", "file-path", "patchset-biz-ids", "sort")
	_ = codeupMrsCommentsListCmd.Flags().Set("resolved", "true")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "comments", "list",
		"--repo", "4951320",
		"--local-id", "77",
		"--comment-type", "INLINE_COMMENT",
		"--state", "OPENED",
		"--file-path", "cmd/root.go",
		"--patchset-biz-ids", "ps-a,ps-b",
		"--resolved",
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
	if !strings.Contains(url, "/repositories/4951320/changeRequests/77/comments/list") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	bodyRaw, _ := json.Marshal(req["body"])
	var body map[string]any
	_ = json.Unmarshal(bodyRaw, &body)
	if body["commentType"] != "INLINE_COMMENT" {
		t.Fatalf("commentType=%v body=%v", body["commentType"], body)
	}
	if body["state"] != "OPENED" {
		t.Fatalf("state=%v body=%v", body["state"], body)
	}
	if body["filePath"] != "cmd/root.go" {
		t.Fatalf("filePath=%v body=%v", body["filePath"], body)
	}
	if body["resolved"] != true {
		t.Fatalf("resolved=%v body=%v", body["resolved"], body)
	}
	psRaw, _ := json.Marshal(body["patchSetBizIds"])
	ps := string(psRaw)
	if !strings.Contains(ps, "ps-a") || !strings.Contains(ps, "ps-b") {
		t.Fatalf("patchSetBizIds=%v", body["patchSetBizIds"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}