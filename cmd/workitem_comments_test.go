package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func TestWorkitemCommentsHelpDocumentsRPCDeleteUpdate(t *testing.T) {
	h := workitemCommentsCmd.Long
	if !strings.Contains(h, "deleteComent") && !strings.Contains(h, "DeleteWorkitemComment") {
		t.Fatalf("comments Long should mention DeleteWorkitemComment/deleteComent: %s", h)
	}
	if !strings.Contains(h, "AccessKey") && !strings.Contains(h, "ALIBABA_CLOUD_ACCESS_KEY") {
		t.Fatalf("comments Long should mention AccessKey auth: %s", h)
	}
	if !strings.Contains(h, "OAPI") {
		t.Fatalf("comments Long should contrast OAPI: %s", h)
	}
	ch := workitemCommentCmd.Long
	if !strings.Contains(ch, "content-file") {
		t.Fatalf("comment Long should mention --content-file: %s", ch)
	}
	if workitemCommentCmd.Flags().Lookup("content-file") == nil {
		t.Fatal("missing --content-file flag")
	}
	var hasDelete, hasUpdate bool
	for _, c := range workitemCommentsCmd.Commands() {
		switch c.Name() {
		case "delete":
			hasDelete = true
		case "update":
			hasUpdate = true
		}
	}
	if !hasDelete || !hasUpdate {
		t.Fatalf("expected comments delete+update subcommands, got delete=%v update=%v", hasDelete, hasUpdate)
	}
	if !strings.Contains(workitemCommentsDeleteCmd.Long, "high-risk") && !strings.Contains(workitemCommentsDeleteCmd.Long, "--yes") {
		t.Fatalf("delete Long should mention high-risk / --yes: %s", workitemCommentsDeleteCmd.Long)
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
	for _, m := range gotMethods {
		if m == http.MethodPost || m == http.MethodPut || m == http.MethodDelete {
			t.Fatalf("mutating during dry-run: %v", gotMethods)
		}
	}
}

func TestWorkitemCommentsDeleteDryRunRPC(t *testing.T) {
	var oapiHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oapiHits++
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`should not be called on dry-run hex id`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-wi-comment-del-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-comment-del")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "LTAI_test_not_real")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "secret_test_not_real")
	t.Setenv("ALIBABA_CLOUD_REGION_ID", "cn-hangzhou")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemCommentsDeleteCmd, "id", "comment-id")
	rootCmd.SetArgs([]string{
		"workitem", "comments", "delete",
		"--id", "wi-hex-abc",
		"--comment-id", "42",
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
	if env.Risk != string(risk.HighRiskWrite) && env.Risk != "high-risk-write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/workitems/deleteComent") {
		t.Fatalf("url=%q", url)
	}
	if strings.Contains(url, "deleteComment") {
		t.Fatal("must keep official typo deleteComent")
	}
	if req["action"] != "DeleteWorkitemComment" {
		t.Fatalf("action=%v", req["action"])
	}
	body, _ := req["body"].(map[string]any)
	if body["identifier"] != "wi-hex-abc" {
		t.Fatalf("body=%v", body)
	}
	if oapiHits != 0 {
		t.Fatalf("OAPI should not be hit for non-serial dry-run, hits=%d", oapiHits)
	}
}

func TestWorkitemCommentsDeleteMissingAK(t *testing.T) {
	t.Setenv(config.EnvAccessToken, "test-token-wi-comment-del-ak")
	t.Setenv(config.EnvOrganizationID, "org-wi-comment-del-ak")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, "http://127.0.0.1:9")
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "")
	t.Setenv("ALICLOUD_ACCESS_KEY_ID", "")
	t.Setenv("ALICLOUD_ACCESS_KEY_SECRET", "")
	t.Setenv("ALIYUN_ACCESS_KEY_ID", "")
	t.Setenv("ALIYUN_ACCESS_KEY_SECRET", "")

	prevExit := processExit
	var code int
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })

	stdout := withCmdJSONCapture(t)
	// withCmdJSONCapture installs a discard stderr; re-capture for Fail JSON
	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })

	resetStringFlags(t, workitemCommentsDeleteCmd, "id", "comment-id")
	rootCmd.SetArgs([]string{
		"workitem", "comments", "delete",
		"--id", "wi-hex-upd",
		"--comment-id", "9",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	var execErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
			}
			}
		}()
		execErr = rootCmd.Execute()
	}()
	_ = execErr
	if code != 1 {
		t.Fatalf("exit code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	out := stdout.String() + stderr.String()
	if !strings.Contains(out, "ALIBABA_CLOUD_ACCESS_KEY") && !strings.Contains(out, "AccessKey") {
		t.Fatalf("expected AK hint in error: %s", out)
	}
}

func TestWorkitemCommentsUpdateDryRunRPC(t *testing.T) {
	t.Setenv(config.EnvAccessToken, "test-token-wi-comment-upd")
	t.Setenv(config.EnvOrganizationID, "org-wi-comment-upd")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, "http://127.0.0.1:9")
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "LTAI_upd")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "secret_upd")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemCommentsUpdateCmd, "id", "comment-id", "content", "content-file", "format-type")
	rootCmd.SetArgs([]string{
		"workitem", "comments", "update",
		"--id", "wi-hex-upd",
		"--comment-id", "7",
		"--content", "修订后的评论",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout: %v", err)
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/workitems/commentUpdate") {
		t.Fatalf("url=%q", url)
	}
	body, _ := req["body"].(map[string]any)
	if body["content"] != "修订后的评论" {
		t.Fatalf("body=%v", body)
	}
}
