package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

func TestWorkitemCommentsHelpDocumentsNoDeleteUpdate(t *testing.T) {
	h := workitemCommentsCmd.Long
	if !strings.Contains(h, "Delete/update are NOT available") && !strings.Contains(h, "not available") {
		// accept either phrasing from Long
		if !strings.Contains(strings.ToLower(h), "delete") || !strings.Contains(h, "OAPI") {
			t.Fatalf("comments Long should document missing delete/update on OAPI: %s", h)
		}
	}
	if !strings.Contains(h, "deleteComent") && !strings.Contains(h, "DeleteWorkitemComment") {
		t.Fatalf("comments Long should mention Aliyun RPC DeleteWorkitemComment/deleteComent: %s", h)
	}
	ch := workitemCommentCmd.Long
	if !strings.Contains(ch, "content-file") {
		t.Fatalf("comment Long should mention --content-file: %s", ch)
	}
	if !strings.Contains(ch, "delete") && !strings.Contains(ch, "OAPI") {
		t.Fatalf("comment Long should note no OAPI delete/update: %s", ch)
	}
	if workitemCommentCmd.Flags().Lookup("content-file") == nil {
		t.Fatal("missing --content-file flag")
	}
	for _, c := range workitemCommentsCmd.Commands() {
		if c.Name() == "delete" || c.Name() == "update" {
			t.Fatalf("unexpected subcommand %q — OAPI does not support it", c.Name())
		}
	}
}

func TestWorkitemCommentContentFileDryRun(t *testing.T) {
	var gotMethods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethods = append(gotMethods, r.Method)
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// UTF-8 with BOM + Chinese — BOM must be stripped in request body
	payload := append([]byte{0xEF, 0xBB, 0xBF}, []byte("进度更新：已修复")...)
	if err := os.WriteFile("note.md", payload, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(config.EnvAccessToken, "test-token-wi-comment-file-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-comment-file-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemCommentCmd, "id", "content", "content-file")
	rootCmd.SetArgs([]string{
		"workitem", "comment",
		"--id", "wi-abc",
		"--content-file", "note.md",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("envelope: %+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/workitems/wi-abc/comments") {
		t.Fatalf("url=%q", url)
	}
	body, _ := req["body"].(map[string]any)
	content, _ := body["content"].(string)
	if content != "进度更新：已修复" {
		t.Fatalf("content=%q want Chinese without BOM", content)
	}
	if strings.HasPrefix(content, "\ufeff") {
		t.Fatal("BOM not stripped")
	}
	for _, m := range gotMethods {
		if m == http.MethodPost || m == http.MethodPut || m == http.MethodDelete {
			t.Fatalf("mutating during dry-run: %v", gotMethods)
		}
	}
}
