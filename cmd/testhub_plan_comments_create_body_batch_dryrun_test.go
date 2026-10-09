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

func thPlanCommentCreateSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-pc-create-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-pc-create-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, testhubPlanCommentsCreateCmd,
		"plan-id", "testcase-id", "content", "parent-id", "format-type",
	)
	return stdout, hits
}

func thPlanCommentCreateAssert(t *testing.T, stdout *bytes.Buffer, hits *int, check func(t *testing.T, body map[string]any, req map[string]any)) {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	if env.Risk != string(risk.Write) && env.Risk != "write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/testPlans/plan-batch/testcases/tc-batch/comments") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
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

func TestTesthubPlanCommentsCreateContentOnlyDryRun(t *testing.T) {
	stdout, hits := thPlanCommentCreateSetup(t, "content")
	rootCmd.SetArgs([]string{
		"testhub", "plan-comments", "create",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--content", "hello plan comment",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlanCommentCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["content"] != "hello plan comment" {
			t.Fatalf("content=%v", body["content"])
		}
		if _, has := body["parentId"]; has {
			t.Fatalf("parentId must be omitted: %#v", body)
		}
		if _, has := body["formatType"]; has {
			t.Fatalf("formatType must be omitted: %#v", body)
		}
	})
}

func TestTesthubPlanCommentsCreateParentIdDryRun(t *testing.T) {
	stdout, hits := thPlanCommentCreateSetup(t, "parent")
	rootCmd.SetArgs([]string{
		"testhub", "plan-comments", "create",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--content", "reply",
		"--parent-id", "cmt-parent-1",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlanCommentCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["content"] != "reply" || body["parentId"] != "cmt-parent-1" {
			t.Fatalf("body=%v", body)
		}
		if _, has := body["formatType"]; has {
			t.Fatalf("formatType must be omitted: %#v", body)
		}
	})
}

func TestTesthubPlanCommentsCreateFormatMarkdownDryRun(t *testing.T) {
	stdout, hits := thPlanCommentCreateSetup(t, "md")
	rootCmd.SetArgs([]string{
		"testhub", "plan-comments", "create",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--content", "md body",
		"--format-type", "MARKDOWN",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlanCommentCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["content"] != "md body" || body["formatType"] != "MARKDOWN" {
			t.Fatalf("body=%v", body)
		}
		if _, has := body["parentId"]; has {
			t.Fatalf("parentId must be omitted: %#v", body)
		}
	})
}

func TestTesthubPlanCommentsCreateFormatRichtextDryRun(t *testing.T) {
	stdout, hits := thPlanCommentCreateSetup(t, "rich")
	rootCmd.SetArgs([]string{
		"testhub", "plan-comments", "create",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--content", "<b>rich</b>",
		"--format-type", "RICHTEXT",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlanCommentCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["content"] != "<b>rich</b>" || body["formatType"] != "RICHTEXT" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestTesthubPlanCommentsCreateParentAndFormatDryRun(t *testing.T) {
	stdout, hits := thPlanCommentCreateSetup(t, "combo")
	rootCmd.SetArgs([]string{
		"testhub", "plan-comments", "create",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--content", "combo reply",
		"--parent-id", "cmt-9",
		"--format-type", "MARKDOWN",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlanCommentCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["content"] != "combo reply" || body["parentId"] != "cmt-9" || body["formatType"] != "MARKDOWN" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestTesthubPlanCommentsCreateMultilineContentDryRun(t *testing.T) {
	stdout, hits := thPlanCommentCreateSetup(t, "multi")
	content := "line1\nline2\nline3"
	rootCmd.SetArgs([]string{
		"testhub", "plan-comments", "create",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--content", content,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlanCommentCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["content"] != content {
			t.Fatalf("content=%q", body["content"])
		}
	})
}

func TestTesthubPlanCommentsCreateEmptyParentOmitsDryRun(t *testing.T) {
	// Explicit empty --parent-id / --format-type must still omit optional body keys.
	stdout, hits := thPlanCommentCreateSetup(t, "empty-opt")
	rootCmd.SetArgs([]string{
		"testhub", "plan-comments", "create",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--content", "only-content",
		"--parent-id", "",
		"--format-type", "",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlanCommentCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["content"] != "only-content" {
			t.Fatalf("content=%v", body["content"])
		}
		if _, has := body["parentId"]; has {
			t.Fatalf("empty parent-id must omit parentId: %#v", body)
		}
		if _, has := body["formatType"]; has {
			t.Fatalf("empty format-type must omit formatType: %#v", body)
		}
	})
}

func TestTesthubPlanCommentsCreatePathIdsDryRun(t *testing.T) {
	stdout, hits := thPlanCommentCreateSetup(t, "path-ids")
	// Distinct ids to assert path wiring (setup assert uses plan-batch/tc-batch).
	resetStringFlags(t, testhubPlanCommentsCreateCmd,
		"plan-id", "testcase-id", "content", "parent-id", "format-type",
	)
	rootCmd.SetArgs([]string{
		"testhub", "plan-comments", "create",
		"--plan-id", "plan-batch",
		"--testcase-id", "tc-batch",
		"--content", "path check",
		"--format-type", "RICHTEXT",
		"--parent-id", "p0",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlanCommentCreateAssert(t, stdout, hits, func(t *testing.T, body map[string]any, req map[string]any) {
		url, _ := req["url"].(string)
		if !strings.Contains(url, "/testPlans/plan-batch/") || !strings.Contains(url, "/testcases/tc-batch/") {
			t.Fatalf("url=%q", url)
		}
		if body["content"] != "path check" || body["parentId"] != "p0" || body["formatType"] != "RICHTEXT" {
			t.Fatalf("body=%v", body)
		}
	})
}