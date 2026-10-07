package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// #127 fixtures: the real 405 rejection body observed on zhiyi_doc MR #139, and the
// post-failure diagnosis GET payload shapes (bare object and {"data":{...}} wrapper).
const mrsMergeRejectBody = `{"errorCode":"SYSTEM_FORBIDDEN_ERROR","errorMessage":"该状态下的评审不允许合并，请刷新页面后重试","traceId":"0a06dd8617912953944533543e66e0"}`

const mrsMergeDiagURL = "https://codeup.aliyun.com/zhiyi/zhiyi_doc/change/139"

type mrsMergeDiagServer struct {
	mu          sync.Mutex
	mergePOSTs  int
	mergeBodies []map[string]any
	gets        int
	mergeStatus int    // 0 → default 405 rejection
	mergeResp   string // response body for the merge POST (default mrsMergeRejectBody)
	getStatus   int    // 0 → serve mrFixture; non-zero → error JSON with this status
	// when non-zero, GETs beyond this count fail — lets #130 precheck succeed while
	// the #124/#127 refresh GET fails.
	getsBeforeFail int
	mrFixture   string
	other       []string
}

func newMrsMergeDiagServer(t *testing.T, mrFixture string) *mrsMergeDiagServer {
	t.Helper()
	s := &mrsMergeDiagServer{mrFixture: mrFixture}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/changeRequests/139/merge"):
			s.mergePOSTs++
			b, _ := io.ReadAll(r.Body)
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			s.mergeBodies = append(s.mergeBodies, m)
			w.Header().Set("Content-Type", "application/json")
			status := s.mergeStatus
			resp := s.mergeResp
			if status == 0 {
				status = http.StatusMethodNotAllowed
				resp = mrsMergeRejectBody
			}
			w.WriteHeader(status)
			_, _ = io.WriteString(w, resp)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/changeRequests/139"):
			s.gets++
			w.Header().Set("Content-Type", "application/json")
			if s.getStatus != 0 && (s.getsBeforeFail == 0 || s.gets > s.getsBeforeFail) {
				w.WriteHeader(s.getStatus)
				_, _ = io.WriteString(w, `{"errorCode":"SystemError","errorMessage":"boom"}`)
				return
			}
			_, _ = io.WriteString(w, s.mrFixture)
		default:
			s.other = append(s.other, r.Method+" "+r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-mrs-merge-diag-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-merge-diag-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

// runMrsMergeDiagCmd executes `codeup mrs merge --repo 4951320 --local-id 139 --merge-type ff-only`
// and returns stdout, stderr and the processExit code (0 when the command did not exit).
func runMrsMergeDiagCmd(t *testing.T, yes bool, extra ...string) (string, string, int) {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	globalDryRun = false
	globalYes = yes
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

	resetStringFlags(t, codeupMrsMergeCmd, "repo", "local-id", "merge-type", "message")
	args := append([]string{"codeup", "mrs", "merge", "--repo", "4951320", "--local-id", "139", "--merge-type", "ff-only"}, extra...)
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

// #127: a rejected merge enriches the error envelope with the current MR status, the
// state gap, and concrete next steps (web 取消 WIP for UNDER_DEV, reopen for CLOSED, ...).
func TestMrsMergeRejectEnriched(t *testing.T) {
	cases := []struct {
		name        string
		mrFixture   string
		wantStatus  any      // expected details.current_status (nil = key omitted)
		noURL       bool     // true = expect details.mr.url to be absent/nil
		wantGap     []string // substrings that must appear in details.state_gap
		wantActions []string // substrings that must appear somewhere in details.suggested_actions
		wantHint    []string // substrings that must appear in error.hint
		noHint      []string // substrings that must NOT appear in error.hint
	}{
		{
			name:      "under_dev_wip_needs_unwip_via_web",
			mrFixture: `{"localId":139,"title":"feat: x","status":"UNDER_DEV","targetProjectPathWithNamespace":"zhiyi/zhiyi_doc"}`,
			wantStatus: "UNDER_DEV",
			wantGap:    []string{"UNDER_DEV", "取消 WIP"},
			wantActions: []string{
				"取消 WIP",
				"yunxiao codeup mrs merge --repo 4951320 --local-id 139 --merge-type ff-only --yes",
			},
			wantHint: []string{"UNDER_DEV", "取消 WIP", "mrs merge --repo 4951320 --local-id 139 --merge-type ff-only --yes"},
		},
		{
			name:      "under_review_suggests_review_pass",
			mrFixture: `{"localId":139,"title":"feat: r","status":"UNDER_REVIEW","targetProjectPathWithNamespace":"zhiyi/zhiyi_doc"}`,
			wantStatus: "UNDER_REVIEW",
			wantGap:    []string{"UNDER_REVIEW", "review"},
			wantActions: []string{
				"yunxiao codeup mrs review --repo 4951320 --local-id 139 --opinion PASS",
			},
		},
		{
			name:      "unknown_status_generic_guidance",
			mrFixture: `{"localId":139,"title":"feat: u","status":"SOME_NEW_STATE","targetProjectPathWithNamespace":"zhiyi/zhiyi_doc"}`,
			wantStatus: "SOME_NEW_STATE",
			wantGap:    []string{"SOME_NEW_STATE"},
			wantActions: []string{
				"yunxiao codeup mrs get --repo 4951320 --local-id 139",
				"yunxiao codeup mrs merge --repo 4951320 --local-id 139 --merge-type ff-only --yes",
			},
			noHint: []string{"取消 WIP"},
		},
		{
			name:       "no_status_field_omits_current_status",
			mrFixture:  `{"localId":139,"title":"feat: n"}`,
			wantStatus: nil,
			noURL:      true,
			wantGap:    []string{"no recognizable status"},
			wantActions: []string{
				"yunxiao codeup mrs get --repo 4951320 --local-id 139",
			},
			noHint: []string{"取消 WIP"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newMrsMergeDiagServer(t, tc.mrFixture)
			_, stderr, code := runMrsMergeDiagCmd(t, true)
			if code != 1 {
				t.Fatalf("exit %d stderr=%s", code, stderr)
			}
			eb := decodeErrorBody(t, stderr)
			if eb.Type != "api" || eb.Code != http.StatusMethodNotAllowed {
				t.Fatalf("error body=%+v", eb)
			}
			if eb.Subtype != "merge_rejected" {
				t.Fatalf("subtype=%q", eb.Subtype)
			}
			if !strings.HasPrefix(eb.Message, "merge MR 139 blocked: yunxiao API POST") || !strings.Contains(eb.Message, "该状态下的评审不允许合并") {
				t.Fatalf("message=%q", eb.Message)
			}
			if got, ok := eb.Details["current_status"]; ok && tc.wantStatus == nil {
				t.Fatalf("current_status must be omitted, got %#v", got)
			} else if tc.wantStatus != nil && got != tc.wantStatus {
				t.Fatalf("current_status=%#v want %#v", got, tc.wantStatus)
			}
			gap, _ := eb.Details["state_gap"].(string)
			for _, want := range tc.wantGap {
				if !strings.Contains(gap, want) {
					t.Fatalf("state_gap=%q missing %q", gap, want)
				}
			}
			rawActions, _ := json.Marshal(eb.Details["suggested_actions"])
			for _, want := range tc.wantActions {
				if !strings.Contains(string(rawActions), want) {
					t.Fatalf("suggested_actions=%s missing %q", rawActions, want)
				}
			}
			for _, want := range tc.wantHint {
				if !strings.Contains(eb.Hint, want) {
					t.Fatalf("hint=%q missing %q", eb.Hint, want)
				}
			}
			for _, banned := range tc.noHint {
				if strings.Contains(eb.Hint, banned) {
					t.Fatalf("hint=%q must not contain %q", eb.Hint, banned)
				}
			}
			// details.mr carries the clickable MR context (localId normalized to string,
			// title/status, and the constructed/known page URL when resolvable).
			var wantURL any = mrsMergeDiagURL
			if tc.noURL {
				wantURL = nil
			}
			mr, _ := eb.Details["mr"].(map[string]any)
			if mr == nil || mr["localId"] != "139" || mr["url"] != wantURL {
				t.Fatalf("details.mr=%#v want url=%#v", mr, wantURL)
			}
			diag, _ := eb.Details["diagnose"].(map[string]any)
			if src, _ := diag["source"].(string); !strings.Contains(src, "changeRequests/139") {
				t.Fatalf("details.diagnose=%#v", diag)
			}
			if s.mergePOSTs != 1 || s.gets != 2 || len(s.other) != 0 {
				t.Fatalf("mergePOSTs=%d gets=%d other=%v", s.mergePOSTs, s.gets, s.other)
			}
			if len(s.mergeBodies) != 1 || s.mergeBodies[0]["mergeType"] != "ff-only" {
				t.Fatalf("mergeBodies=%#v", s.mergeBodies)
			}
		})
	}
}

// #127: when the diagnosis GET itself fails, the original API error passes through
// unchanged (no fake status, no misleading 取消 WIP advice).
func TestMrsMergeRejectDiagGETFails(t *testing.T) {
	t.Cleanup(client.SetRetrySleepForTest(func(ctx context.Context, d time.Duration) error { return nil }))
	s := newMrsMergeDiagServer(t, `{"localId":139,"status":"UNDER_DEV"}`)
	s.getStatus = http.StatusInternalServerError
	s.getsBeforeFail = 1
	_, stderr, code := runMrsMergeDiagCmd(t, true)
	if code != 1 {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if eb.Type != "api" || eb.Code != http.StatusMethodNotAllowed || eb.Subtype != "" || eb.Details != nil {
		t.Fatalf("error body=%+v", eb)
	}
	if !strings.Contains(eb.Message, "该状态下的评审不允许合并") || strings.Contains(eb.Hint, "取消 WIP") {
		t.Fatalf("message=%q hint=%q", eb.Message, eb.Hint)
	}
	if s.mergePOSTs != 1 || s.gets < 2 {
		t.Fatalf("mergePOSTs=%d gets=%d", s.mergePOSTs, s.gets)
	}
}

// The high-risk gate must stay ahead of any diagnosis: without --yes nothing is POSTed
// or GET and the exit stays 10 / confirmation_required.
func TestMrsMergeGateBeforeDiagnosis(t *testing.T) {
	s := newMrsMergeDiagServer(t, `{"localId":139,"status":"UNDER_DEV"}`)
	_, stderr, code := runMrsMergeDiagCmd(t, false)
	if code != 10 {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if eb.Subtype != "confirmation_required" || eb.Risk != "high-risk-write" {
		t.Fatalf("error body=%+v", eb)
	}
	if s.mergePOSTs != 0 || s.gets != 0 {
		t.Fatalf("mergePOSTs=%d gets=%d — gate must not leak API calls", s.mergePOSTs, s.gets)
	}
}

// A successful merge never pays the diagnosis GET.
func TestMrsMergeSuccessNoDiagnosis(t *testing.T) {
	s := newMrsMergeDiagServer(t, `{"localId":139,"status":"TO_BE_MERGED","mergeable":true}`)
	s.mergeStatus = http.StatusOK
	s.mergeResp = `{"result":true}`
	stdout, stderr, code := runMrsMergeDiagCmd(t, true)
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil || !env.OK {
		t.Fatalf("envelope=%+v err=%v", env, err)
	}
	if s.mergePOSTs != 1 || s.gets != 1 {
		t.Fatalf("mergePOSTs=%d gets=%d (one #130 precheck only)", s.mergePOSTs, s.gets)
	}
}


// MERGED/CLOSED are refused by #130 precheck before POST; #127 helpers still describe them.
func TestMrsMergeDiagHelpersTerminalStatuses(t *testing.T) {
	if gap := mergeStateGap("MERGED"); !strings.Contains(gap, "already MERGED") {
		t.Fatalf("gap=%q", gap)
	}
	if gap := mergeStateGap("CLOSED"); !strings.Contains(gap, "CLOSED") || !strings.Contains(gap, "reopen") {
		t.Fatalf("gap=%q", gap)
	}
	actions := mergeSuggestedActions("4951320", "139", "ff-only", "CLOSED")
	raw, _ := json.Marshal(actions)
	if !strings.Contains(string(raw), "mrs reopen") {
		t.Fatalf("actions=%s", raw)
	}
}

// #130 precheck refuses MERGED before POST (no merge_rejected enrichment needed).
func TestMrsMergeRejectMergedCaughtByPrecheck(t *testing.T) {
	s := newMrsMergeDiagServer(t, `{"data":{"localId":139,"title":"feat: y","mergeStatus":"MERGED","detailUrl":"https://codeup.aliyun.com/zhiyi/zhiyi_doc/change/139"}}`)
	_, stderr, code := runMrsMergeDiagCmd(t, true)
	if code != 1 {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if eb.Subtype != "mr_already_merged" {
		t.Fatalf("subtype=%q body=%+v", eb.Subtype, eb)
	}
	if s.mergePOSTs != 0 || s.gets != 1 {
		t.Fatalf("mergePOSTs=%d gets=%d", s.mergePOSTs, s.gets)
	}
}

// contextError Subtype/Details merge into handleErr's body for both API and CLI causes.
func TestHandleErrContextErrorSubtypeDetails(t *testing.T) {
	ae := &client.APIError{Status: 405, Method: "POST", URL: "https://x/changeRequests/139/merge", Body: mrsMergeRejectBody}
	eb, code := handleErrBody(t, &contextError{
		Context: "merge MR 139",
		Subtype: "merge_rejected",
		Hint:    "gap; next: web 取消 WIP",
		Details: map[string]any{"current_status": "UNDER_DEV"},
		Err:     ae,
	})
	if code != 1 || eb.Type != "api" || eb.Code != 405 || eb.Subtype != "merge_rejected" {
		t.Fatalf("code=%d body=%+v", code, eb)
	}
	if eb.Details["current_status"] != "UNDER_DEV" {
		t.Fatalf("details=%#v", eb.Details)
	}
	cli, _ := handleErrBody(t, &contextError{
		Context: "merge MR 139",
		Subtype: "merge_rejected",
		Details: map[string]any{"current_status": "UNDER_DEV"},
		Err:     errors.New("dial tcp: refused"),
	})
	if cli.Type != "cli" || cli.Subtype != "merge_rejected" || cli.Details["current_status"] != "UNDER_DEV" {
		t.Fatalf("cli body=%+v", cli)
	}
}

