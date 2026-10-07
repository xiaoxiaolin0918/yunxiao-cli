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
// Since #130 the merge also pays one read-only precheck GET before the POST, so the
// detail endpoint serves precheckFixture on the first GET (when set) and mrFixture on
// the diagnosis GET after a failed POST.
const mrsMergeRejectBody = `{"errorCode":"SYSTEM_FORBIDDEN_ERROR","errorMessage":"该状态下的评审不允许合并，请刷新页面后重试","traceId":"0a06dd8617912953944533543e66e0"}`

const mrsMergeDiagURL = "https://codeup.aliyun.com/zhiyi/zhiyi_doc/change/139"

// mrsMergeDiagPrecheckFixture passes the #130 precheck (non-terminal status, no
// conflict/mergeable signals, no merge-type config) for cases whose diag fixture
// itself would be refused before the POST (MERGED / CLOSED).
const mrsMergeDiagPrecheckFixture = `{"localId":139,"title":"feat: pre","status":"TO_BE_MERGED","targetProjectPathWithNamespace":"zhiyi/zhiyi_doc"}`

type mrsMergeDiagServer struct {
	mu          sync.Mutex
	mergePOSTs  int
	mergeBodies []map[string]any
	gets        int
	mergeStatus int    // 0 → default 405 rejection
	mergeResp   string // response body for the merge POST (default mrsMergeRejectBody)
	getStatus   int    // 0 → serve fixtures; non-zero → error JSON with this status
	// when non-zero, GETs beyond this count fail with getStatus — lets the #130
	// precheck GET succeed while the #127 diagnosis GET after the failed POST fails.
	getsBeforeFail int
	precheckFixture string // served on the first GET when non-empty
	mrFixture       string // served on every other GET
	other           []string
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
			if s.gets == 1 && s.precheckFixture != "" {
				_, _ = io.WriteString(w, s.precheckFixture)
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

// runMrsMergeDiag executes `codeup mrs merge --repo 4951320 --local-id 139 --merge-type ff-only`
// and returns stdout, stderr and the processExit code (0 when the command did not exit).
func runMrsMergeDiag(t *testing.T, yes bool, extra ...string) (string, string, int) {
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
		name            string
		mrFixture       string
		precheckFixture string // non-empty: first GET (the #130 precheck) serves this
		wantStatus      any      // expected details.current_status (nil = key omitted)
		noURL           bool     // true = expect details.mr.url to be absent/nil
		wantGap         []string // substrings that must appear in details.state_gap
		wantActions     []string // substrings that must appear somewhere in details.suggested_actions
		wantHint        []string // substrings that must appear in error.hint
		noHint          []string // substrings that must NOT appear in error.hint
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
			name:            "wrapped_payload_and_merge_status_fallback",
			mrFixture:       `{"data":{"localId":139,"title":"feat: y","mergeStatus":"MERGED","detailUrl":"https://codeup.aliyun.com/zhiyi/zhiyi_doc/change/139"}}`,
			precheckFixture: mrsMergeDiagPrecheckFixture, // MERGED would be refused by the #130 precheck before the POST
			wantStatus:      "MERGED",
			wantGap:         []string{"already MERGED"},
			wantActions: []string{
				"yunxiao codeup mrs get --repo 4951320 --local-id 139",
			},
			wantHint: []string{"already MERGED"},
			noHint:   []string{"取消 WIP", "mrs reopen"},
		},
		{
			name:            "closed_suggests_reopen",
			mrFixture:       `{"localId":139,"title":"feat: z","status":"CLOSED","targetProjectPathWithNamespace":"zhiyi/zhiyi_doc"}`,
			precheckFixture: mrsMergeDiagPrecheckFixture, // CLOSED would be refused by the #130 precheck before the POST
			wantStatus:      "CLOSED",
			wantGap:         []string{"CLOSED", "reopen"},
			wantActions: []string{
				"yunxiao codeup mrs reopen --repo 4951320 --local-id 139",
				"yunxiao codeup mrs merge --repo 4951320 --local-id 139 --merge-type ff-only --yes",
			},
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
			s.precheckFixture = tc.precheckFixture
			_, stderr, code := runMrsMergeDiag(t, true)
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
			if !strings.HasPrefix(eb.Message, "merge MR 139: yunxiao API POST") || !strings.Contains(eb.Message, "该状态下的评审不允许合并") {
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
			if src, _ := diag["source"].(string); !strings.HasPrefix(src, "GET ") || !strings.HasSuffix(src, "/changeRequests/139") {
				t.Fatalf("details.diagnose=%#v", diag)
			}
			// one #130 precheck GET + one #127 diagnosis GET after the failed POST
			if s.mergePOSTs != 1 || s.gets != 2 || len(s.other) != 0 {
				t.Fatalf("mergePOSTs=%d gets=%d other=%v", s.mergePOSTs, s.gets, s.other)
			}
			if len(s.mergeBodies) != 1 || s.mergeBodies[0]["mergeType"] != "ff-only" {
				t.Fatalf("mergeBodies=%#v", s.mergeBodies)
			}
		})
	}
}

// #127 + #124: the diagnosis folds the push-review extras in — failing merge
// requirements (todoList) land in suggested_actions and error.details.mr.todo, and
// every rejection ends with the +push-review-status tracking shortcut.
func TestMrsMergeRejectFoldsPushReviewContext(t *testing.T) {
	s := newMrsMergeDiagServer(t, `{
	 "localId":139,"title":"WIP: feat: x","status":"UNDER_DEV","ahead":2,"behind":0,"allRequirementsPass":false,
	 "todoList":{"requirementCheckItems":[{"itemType":"COMMENTS_CHECK","pass":false}]},
	 "targetProjectPathWithNamespace":"zhiyi/zhiyi_doc"}`)
	_, stderr, code := runMrsMergeDiag(t, true)
	if code != 1 {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if eb.Subtype != "merge_rejected" {
		t.Fatalf("subtype=%q", eb.Subtype)
	}
	for _, want := range []string{"merge requirements not met: COMMENTS_CHECK", "mrs comments resolve", "mrs update", "--title \"feat: x\"", "+push-review-status"} {
		if !strings.Contains(eb.Hint, want) {
			t.Fatalf("hint=%q missing %q", eb.Hint, want)
		}
	}
	mr, _ := eb.Details["mr"].(map[string]any)
	if mr["wip"] != true || mr["ahead"] != float64(2) || mr["behind"] != float64(0) || mr["allRequirementsPass"] != false {
		t.Fatalf("details.mr merge-state signals missing: %#v", mr)
	}
	todo, _ := mr["todo"].([]any)
	if len(todo) != 1 {
		t.Fatalf("details.mr.todo=%#v", mr["todo"])
	}
	if s.mergePOSTs != 1 || s.gets != 2 {
		t.Fatalf("mergePOSTs=%d gets=%d", s.mergePOSTs, s.gets)
	}
}

// #127: when the diagnosis GET itself fails, the original API error passes through
// unchanged (no fake status, no misleading 取消 WIP advice). The first detail GET is
// the #130 precheck and must succeed for the POST to happen at all.
func TestMrsMergeRejectDiagGETFails(t *testing.T) {
	t.Cleanup(client.SetRetrySleepForTest(func(ctx context.Context, d time.Duration) error { return nil }))
	s := newMrsMergeDiagServer(t, `{"localId":139,"status":"UNDER_DEV"}`)
	s.getStatus = http.StatusInternalServerError
	s.getsBeforeFail = 1
	_, stderr, code := runMrsMergeDiag(t, true)
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
		// gets: 1 precheck + the diagnosis GET (retried on 5xx by the idempotent policy)
		t.Fatalf("mergePOSTs=%d gets=%d", s.mergePOSTs, s.gets)
	}
}

// The high-risk gate must stay ahead of any diagnosis: without --yes nothing is POSTed
// or GET and the exit stays 10 / confirmation_required.
func TestMrsMergeGateBeforeDiagnosis(t *testing.T) {
	s := newMrsMergeDiagServer(t, `{"localId":139,"status":"UNDER_DEV"}`)
	_, stderr, code := runMrsMergeDiag(t, false)
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

// A successful merge pays only the #130 precheck GET, never the diagnosis GET.
func TestMrsMergeSuccessNoDiagnosis(t *testing.T) {
	s := newMrsMergeDiagServer(t, `{"localId":139,"status":"UNDER_DEV"}`)
	s.mergeStatus = http.StatusOK
	s.mergeResp = `{"result":true}`
	stdout, stderr, code := runMrsMergeDiag(t, true)
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil || !env.OK {
		t.Fatalf("envelope=%+v err=%v", env, err)
	}
	if s.mergePOSTs != 1 || s.gets != 1 {
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

// An API-derived subtype (yaml_validation) wins over the wrapper's Subtype, and the
// wrapper's Details merge on top of API details without clobbering unrelated keys.
func TestHandleErrContextErrorAPISubtypeWins(t *testing.T) {
	body := `{"errorCode":"1209300","errorMessage":"yaml校验失败 {\"errorMessage\":\"validators invalid\",\"path\":\"stages[0].jobs[0]\"}"}`
	ae := &client.APIError{Status: 400, Method: "POST", URL: "https://x/pipelines", Body: body}
	eb, _ := handleErrBody(t, &contextError{
		Context: "ctx",
		Subtype: "merge_rejected",
		Details: map[string]any{"extra": 1},
		Err:     ae,
	})
	if eb.Subtype != "yaml_validation" {
		t.Fatalf("subtype=%q", eb.Subtype)
	}
	if eb.Details["errorCode"] != "1209300" || eb.Details["issues"] == nil || eb.Details["extra"] != float64(1) {
		t.Fatalf("details=%#v", eb.Details)
	}
}
