package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// #93 fixture: unordered patch sets; MERGE_TARGET has the newest createTime.
const mrsPatchesFixture = `[
 {"patchSetBizId":"ps-target","versionNo":3,"relatedMergeItemType":"MERGE_TARGET","createTime":"2026-09-29T10:00:00Z"},
 {"patchSetBizId":"ps-v2","versionNo":2,"relatedMergeItemType":"MERGE_SOURCE","createTime":"2026-09-28T10:00:00Z"},
 {"patchSetBizId":"ps-v3","versionNo":3,"relatedMergeItemType":"MERGE_SOURCE","createTime":"2026-09-29T09:00:00Z"},
 {"patchSetBizId":"ps-v1","versionNo":1,"relatedMergeItemType":"MERGE_SOURCE","createTime":"2026-09-27T10:00:00Z"}
]`

type mrsCommentServer struct {
	mu        sync.Mutex
	patchGETs int
	posts     []map[string]any
	other     []string
}

func newMrsCommentServer(t *testing.T, patches string) *mrsCommentServer {
	t.Helper()
	s := &mrsCommentServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/changeRequests/125/diffs/patches"):
			s.patchGETs++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, patches)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/changeRequests/125/comments"):
			b, _ := io.ReadAll(r.Body)
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			s.posts = append(s.posts, m)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"commentBizId":"c-new"}`)
		default:
			s.other = append(s.other, r.Method+" "+r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-mrs-comment-create-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-comment-create-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

// runMrsCommentsCreate runs `codeup mrs comments create` and returns stdout,
// stderr and processExit code (0 when the command did not exit).
func runMrsCommentsCreate(t *testing.T, dryRun bool, extra ...string) (string, string, int) {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	globalDryRun = dryRun
	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	prevExit := processExit
	code := 0
	processExit = func(c int) {
		code = c
		panic(exitPanic{code: c})
	}
	t.Cleanup(func() { processExit = prevExit })

	resetStringFlags(t, codeupMrsCommentsCreateCmd, "repo", "local-id", "content", "comment-type", "patchset-biz-id",
		"draft", "resolved", "file-path", "line-number", "from-patchset-biz-id", "to-patchset-biz-id", "parent-comment-biz-id")
	args := append([]string{"codeup", "mrs", "comments", "create", "--repo", "4951320", "--local-id", "125", "--content", "通知：已部署"}, extra...)
	rootCmd.SetArgs(args)
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
				}
			}
		}()
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	}()
	return stdout.String(), stderr.String(), code
}

func decodeDryRunRequest(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout)
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("envelope: %+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	return req
}

func TestMrsCommentsCreateHelpPatchsetOptional(t *testing.T) {
	f := codeupMrsCommentsCreateCmd.Flags().Lookup("patchset-biz-id")
	if f == nil || strings.Contains(f.Usage, "(required)") {
		t.Fatalf("patchset-biz-id usage should not say required: %+v", f)
	}
	if !strings.Contains(codeupMrsCommentsCreateCmd.Long, "latest") {
		t.Fatalf("Long should document latest-patchset default: %s", codeupMrsCommentsCreateCmd.Long)
	}
}

// #93: GLOBAL_COMMENT without --patchset-biz-id resolves the latest MERGE_SOURCE patchset.
func TestMrsCommentsCreateGlobalDefaultsToLatestPatchsetDryRun(t *testing.T) {
	s := newMrsCommentServer(t, mrsPatchesFixture)
	stdout, stderr, code := runMrsCommentsCreate(t, true, "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	req := decodeDryRunRequest(t, stdout)
	body, _ := req["body"].(map[string]any)
	if body["patchset_biz_id"] != "ps-v3" || body["comment_type"] != "GLOBAL_COMMENT" {
		t.Fatalf("body=%#v", body)
	}
	resolved, _ := req["resolved"].(map[string]any)
	if resolved["patchset_biz_id"] != "ps-v3" || resolved["patchset_source"] != "latest" || resolved["version_no"] != float64(3) {
		t.Fatalf("resolved=%#v", resolved)
	}
	if s.patchGETs != 1 || len(s.posts) != 0 {
		t.Fatalf("patchGETs=%d posts=%d other=%v", s.patchGETs, len(s.posts), s.other)
	}
}

func TestMrsCommentsCreateGlobalDefaultsToLatestPatchsetRun(t *testing.T) {
	s := newMrsCommentServer(t, mrsPatchesFixture)
	stdout, stderr, code := runMrsCommentsCreate(t, false)
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if len(s.posts) != 1 || s.posts[0]["patchset_biz_id"] != "ps-v3" || s.posts[0]["content"] != "通知：已部署" {
		t.Fatalf("posts=%#v", s.posts)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout)
	}
	if !env.OK || env.Meta["patchset_biz_id"] != "ps-v3" || env.Meta["patchset_source"] != "latest" {
		t.Fatalf("envelope=%+v", env)
	}
}

func TestMrsCommentsCreateExplicitPatchsetWins(t *testing.T) {
	s := newMrsCommentServer(t, mrsPatchesFixture)
	stdout, stderr, code := runMrsCommentsCreate(t, true, "--patchset-biz-id", "ps-explicit", "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	req := decodeDryRunRequest(t, stdout)
	body, _ := req["body"].(map[string]any)
	if body["patchset_biz_id"] != "ps-explicit" {
		t.Fatalf("body=%#v", body)
	}
	if _, ok := req["resolved"]; ok {
		t.Fatalf("explicit patchset must not report resolved: %#v", req)
	}
	if s.patchGETs != 0 {
		t.Fatalf("explicit patchset must not call diffs/patches (got %d)", s.patchGETs)
	}
}

func TestMrsCommentsCreateInlineStillRequiresPatchset(t *testing.T) {
	s := newMrsCommentServer(t, mrsPatchesFixture)
	_, stderr, code := runMrsCommentsCreate(t, true, "--comment-type", "INLINE_COMMENT",
		"--file-path", "a.go", "--line-number", "3", "--from-patchset-biz-id", "p1", "--to-patchset-biz-id", "p2", "--dry-run")
	if code != 1 || !strings.Contains(stderr, "missing required flag --patchset-biz-id") {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if s.patchGETs != 0 {
		t.Fatalf("INLINE must not auto-resolve (got %d GETs)", s.patchGETs)
	}
}

func TestMrsCommentsCreateNoPatchsetsClearError(t *testing.T) {
	for name, fixture := range map[string]string{
		"empty":       `[]`,
		"target-only": `[{"patchSetBizId":"t","versionNo":1,"relatedMergeItemType":"MERGE_TARGET"}]`,
	} {
		t.Run(name, func(t *testing.T) {
			s := newMrsCommentServer(t, fixture)
			_, stderr, code := runMrsCommentsCreate(t, false)
			if code != 1 {
				t.Fatalf("expected exit 1, got %d stderr=%s", code, stderr)
			}
			// M5: hint carries the actual --repo / --local-id values, not a placeholder.
			for _, want := range []string{"patchset", "--patchset-biz-id", "yunxiao codeup mrs diffs --repo 4951320 --local-id 125"} {
				if !strings.Contains(stderr, want) {
					t.Fatalf("stderr missing %q: %s", want, stderr)
				}
			}
			if len(s.posts) != 0 {
				t.Fatalf("must not POST when no patchset: %#v", s.posts)
			}
		})
	}
}

// Replies without --patchset-biz-id attach to the latest source patchset (documented
// behavior; the OpenAPI does not require the parent's patchset).
func TestMrsCommentsCreateReplyDefaultsToLatestPatchset(t *testing.T) {
	s := newMrsCommentServer(t, mrsPatchesFixture)
	stdout, stderr, code := runMrsCommentsCreate(t, true, "--parent-comment-biz-id", "parent-1", "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	req := decodeDryRunRequest(t, stdout)
	body, _ := req["body"].(map[string]any)
	if body["parent_comment_biz_id"] != "parent-1" || body["patchset_biz_id"] != "ps-v3" {
		t.Fatalf("body=%#v", body)
	}
	if s.patchGETs != 1 {
		t.Fatalf("patchGETs=%d", s.patchGETs)
	}
}

func TestShellArgQuotesOnlyWhenNeeded(t *testing.T) {
	cases := map[string]string{"4951320": "4951320", "group/repo": "group/repo", "my repo": `"my repo"`, "": `""`}
	for in, want := range cases {
		if got := shellArg(in); got != want {
			t.Fatalf("shellArg(%q)=%s want %s", in, got, want)
		}
	}
}
