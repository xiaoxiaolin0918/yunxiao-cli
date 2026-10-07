package cmd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/config"
)

// #124 fixtures: merge POST answers 405 SYSTEM_FORBIDDEN_ERROR (the push-review
// WIP block); the detail GET carries the MR state the CLI attaches to the error.
const mrsMerge405Body = `{"errorCode":"SYSTEM_FORBIDDEN_ERROR","errorMessage":"该状态下的评审不允许合并"}`

const mrsMergeDetailUnderDev = `{
 "localId":125,"title":"feat: push review","status":"UNDER_DEV",
 "ahead":2,"behind":0,"allRequirementsPass":false,
 "todoList":{"requirementCheckItems":[{"itemType":"COMMENTS_CHECK","pass":false}]},
 "detailUrl":"https://codeup.aliyun.com/org/repo/change/125"
}`

const mrsMergeDetailTitleWip = `{
 "localId":125,"title":"WIP: fix docs","status":"TO_BE_MERGED",
 "ahead":1,"behind":0,"allRequirementsPass":true
}`

const mrsMergeDetailRequirements = `{
 "localId":125,"title":"fix docs","status":"TO_BE_MERGED",
 "allRequirementsPass":false,
 "todoList":{"requirementCheckItems":[
   {"itemType":"MERGE_CONFLICT_CHECK","pass":false},
   {"itemType":"CI_CHECK","pass":false}]}
}`

const mrsMergeDetailReady = `{
 "localId":125,"title":"fix docs","status":"TO_BE_MERGED","allRequirementsPass":true
}`

type mrsMergeServer struct {
	mu           sync.Mutex
	mergePOSTs   int
	mergeStatus  int // default 405
	mergeBody    string
	detailGETs   int
	detailStatus int // non-zero: detail GET fails with this status
	// when non-zero, detail GETs beyond this count fail with detailStatus —
	// lets the #130 precheck GET succeed while the #124 refresh GET fails.
	detailGETsBeforeFail int
	detailBody           string
}

func newMrsMergeServer(t *testing.T, detailBody string) *mrsMergeServer {
	t.Helper()
	s := &mrsMergeServer{mergeStatus: http.StatusMethodNotAllowed, mergeBody: mrsMerge405Body, detailBody: detailBody}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/changeRequests/125/merge"):
			s.mergePOSTs++
			w.Header().Set("Content-Type", "application/json")
			if s.mergeStatus != 0 {
				w.WriteHeader(s.mergeStatus)
			}
			_, _ = io.WriteString(w, s.mergeBody)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/changeRequests/125"):
			s.detailGETs++
			w.Header().Set("Content-Type", "application/json")
			if s.detailStatus != 0 && (s.detailGETsBeforeFail == 0 || s.detailGETs > s.detailGETsBeforeFail) {
				w.WriteHeader(s.detailStatus)
				_, _ = io.WriteString(w, `{"errorCode":"SystemError","errorMessage":"boom"}`)
				return
			}
			_, _ = io.WriteString(w, s.detailBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-mrs-merge-status-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-merge-status-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

func runMrsMergeCmd(t *testing.T, dryRun, yes bool, extra ...string) (string, string, int) {
	t.Helper()
	resetStringFlags(t, codeupMrsMergeCmd, "repo", "local-id", "merge-type", "message", "remove-source-branch")
	args := append([]string{
		"codeup", "mrs", "merge", "--repo", "4951320", "--local-id", "125",
		"--merge-type", "ff-only",
	}, extra...)
	return runMrsStatusCmd(t, dryRun, yes, args...)
}

// High-risk gate: without --yes the merge aborts with exit 10 before any call.
func TestMrsMergeGateWithoutYes(t *testing.T) {
	s := newMrsMergeServer(t, mrsMergeDetailUnderDev)
	_, stderr, code := runMrsMergeCmd(t, false, false)
	if code != 10 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "confirmation_required") {
		t.Fatalf("stderr=%s", stderr)
	}
	if s.mergePOSTs != 0 || s.detailGETs != 0 {
		t.Fatalf("no HTTP calls before the gate: POSTs=%d GETs=%d", s.mergePOSTs, s.detailGETs)
	}
}

// Dry-run previews the POST plus the #130 read-only precheck (one detail GET);
// the merge itself is never sent.
func TestMrsMergeDryRun(t *testing.T) {
	s := newMrsMergeServer(t, mrsMergeDetailUnderDev)
	stdout, stderr, code := runMrsMergeCmd(t, true, false, "--dry-run")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	req := decodeDryRunRequest(t, stdout)
	if req["method"] != "POST" || !strings.Contains(fmtURL(req["url"]), "/changeRequests/125/merge") {
		t.Fatalf("request=%#v", req)
	}
	if s.mergePOSTs != 0 || s.detailGETs != 1 {
		t.Fatalf("dry-run: POSTs=%d (must be 0) GETs=%d (one precheck, #130)", s.mergePOSTs, s.detailGETs)
	}
}

// #124: a 405 on an UNDER_DEV (push-review WIP) MR gains details.mr + a hint that
// names the web-UI 取消 WIP workaround (no OpenAPI exists) and the retry command.
func TestMrsMerge405UnderDevEnriched(t *testing.T) {
	s := newMrsMergeServer(t, mrsMergeDetailUnderDev)
	stdout, stderr, code := runMrsMergeCmd(t, false, true, "--yes")
	if code != 1 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if eb.Type != "api" || eb.Code != http.StatusMethodNotAllowed {
		t.Fatalf("error body=%+v", eb)
	}
	if !strings.HasPrefix(eb.Message, "merge MR 125 blocked: ") || !strings.Contains(eb.Message, "SYSTEM_FORBIDDEN_ERROR") {
		t.Fatalf("message=%q", eb.Message)
	}
	for _, want := range []string{"UNDER_DEV", "开发中", "no OpenAPI to cancel WIP", "取消 WIP", "mrs merge --repo 4951320 --local-id 125"} {
		if !strings.Contains(eb.Hint, want) {
			t.Fatalf("hint missing %q: %q", want, eb.Hint)
		}
	}
	// failing requirement checks are surfaced alongside the WIP explanation
	if !strings.Contains(eb.Hint, "COMMENTS_CHECK") {
		t.Fatalf("hint should list failing todo items: %q", eb.Hint)
	}
	mr, _ := eb.Details["mr"].(map[string]any)
	if mr == nil {
		t.Fatalf("details: %#v", eb.Details)
	}
	if mr["status"] != "UNDER_DEV" || mr["wip"] != true || mr["ahead"] != float64(2) || mr["title"] != "feat: push review" {
		t.Fatalf("details.mr=%#v", mr)
	}
	todo, _ := mr["todo"].([]any)
	if len(todo) != 1 {
		t.Fatalf("details.mr.todo=%#v", mr["todo"])
	}
	if s.mergePOSTs != 1 || s.detailGETs != 2 {
		t.Fatalf("POSTs=%d GETs=%d (one #130 precheck + one refresh after the failed merge)", s.mergePOSTs, s.detailGETs)
	}
}

// A failing status refresh never masks the original merge error (degrade silently).
// The first detail GET (the #130 precheck) succeeds; only the refresh after the
// failed POST hits the 500.
func TestMrsMergeDegradesWhenRefreshFails(t *testing.T) {
	t.Cleanup(client.SetRetrySleepForTest(func(ctx context.Context, d time.Duration) error { return nil }))
	s := newMrsMergeServer(t, mrsMergeDetailUnderDev)
	s.detailStatus = http.StatusInternalServerError
	s.detailGETsBeforeFail = 1
	stdout, stderr, code := runMrsMergeCmd(t, false, true, "--yes")
	if code != 1 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if eb.Type != "api" || eb.Code != http.StatusMethodNotAllowed {
		t.Fatalf("error body=%+v", eb)
	}
	if strings.Contains(eb.Message, "blocked") {
		t.Fatalf("no enrichment when the refresh fails: %q", eb.Message)
	}
	if len(eb.Details) != 0 {
		t.Fatalf("details must stay empty on degrade: %#v", eb.Details)
	}
}

// Title-prefix WIP (distinct from push-review UNDER_DEV; PR #135 adds --wip/--unwip
// toggles for it) is hinted as a rename, not as the web 取消 WIP action.
func TestMrsMergeTitlePrefixWipHint(t *testing.T) {
	s := newMrsMergeServer(t, mrsMergeDetailTitleWip)
	_, stderr, code := runMrsMergeCmd(t, false, true, "--yes")
	if code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if s.mergePOSTs != 1 || s.detailGETs != 2 {
		t.Fatalf("POSTs=%d GETs=%d (precheck + refresh)", s.mergePOSTs, s.detailGETs)
	}
	if !strings.Contains(eb.Hint, `"WIP:"`) || !strings.Contains(eb.Hint, "mrs update") || !strings.Contains(eb.Hint, `--title "fix docs"`) {
		t.Fatalf("hint=%q", eb.Hint)
	}
	if strings.Contains(eb.Hint, "取消 WIP") {
		t.Fatalf("title-prefix WIP must not get the UNDER_DEV web-UI hint: %q", eb.Hint)
	}
	mr, _ := eb.Details["mr"].(map[string]any)
	if mr["status"] != "TO_BE_MERGED" || mr["wip"] != false {
		t.Fatalf("details.mr=%#v", mr)
	}
}

// Non-WIP blockers: failing merge requirements are listed (with the failing check types).
func TestMrsMergeRequirementsHint(t *testing.T) {
	s := newMrsMergeServer(t, mrsMergeDetailRequirements)
	_, stderr, code := runMrsMergeCmd(t, false, true, "--yes")
	if code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if s.mergePOSTs != 1 || s.detailGETs != 2 {
		t.Fatalf("POSTs=%d GETs=%d (precheck + refresh)", s.mergePOSTs, s.detailGETs)
	}
	if !strings.Contains(eb.Hint, "merge requirements not met: MERGE_CONFLICT_CHECK, CI_CHECK") {
		t.Fatalf("hint=%q", eb.Hint)
	}
	if strings.Contains(eb.Hint, "UNDER_DEV") {
		t.Fatalf("status is TO_BE_MERGED — no WIP hint: %q", eb.Hint)
	}
	if !strings.Contains(eb.Hint, "mrs comments resolve") {
		t.Fatalf("hint=%q", eb.Hint)
	}
}

// Success: raw API data passthrough + meta.url (additive), no refresh GET.
func TestMrsMergeSuccessKeepsRawOut(t *testing.T) {
	s := newMrsMergeServer(t, mrsMergeDetailUnderDev)
	s.mergeStatus = 0 // OK
	s.mergeBody = `{"result":true,"localId":125,"status":"MERGED","detailUrl":"https://codeup.aliyun.com/org/repo/change/125"}`
	stdout, stderr, code := runMrsMergeCmd(t, false, true, "--yes")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env := decodeMrsStatusEnvelope(t, stdout)
	if !env.OK {
		t.Fatalf("env=%+v", env)
	}
	data, _ := env.Data.(map[string]any)
	if data["result"] != true || data["status"] != "MERGED" {
		t.Fatalf("data must stay the raw API object: %#v", env.Data)
	}
	if u, _ := env.Meta["url"].(string); !strings.Contains(u, "/change/125") {
		t.Fatalf("meta.url: %#v", env.Meta)
	}
	if env.Meta["risk"] != "high-risk-write" {
		t.Fatalf("meta.risk: %#v", env.Meta)
	}
	if env.Meta["precheck"] == nil {
		t.Fatalf("meta.precheck from the #130 precheck: %#v", env.Meta)
	}
	if s.detailGETs != 1 {
		t.Fatalf("one #130 precheck GET, no refresh on success, got %d", s.detailGETs)
	}
}

func TestMrsMergeHelpDocumentsWIP(t *testing.T) {
	h := codeupMrsMergeCmd.Long
	for _, want := range []string{"UNDER_DEV", "取消 WIP", "error.details.mr", "+push-review-status"} {
		if !strings.Contains(h, want) {
			t.Fatalf("merge help missing %q: %s", want, h)
		}
	}
}
