package cmd

import (
	"context"
	"encoding/json"
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

// #132 fixtures: two open MRs — a push-review WIP MR (UNDER_DEV) and a ready one.
const pushReviewListFixture = `[
 {"localId":139,"title":"feat: push review","state":"opened","newVersionState":"UNDER_DEV","workInProgress":true,
  "sourceBranch":"feat/x","targetBranch":"master","targetProjectPathWithNamespace":"org/zhiyi_doc","updatedAt":"2026-10-05T10:00:00Z"},
 {"localId":140,"title":"fix: ready","state":"opened","newVersionState":"TO_BE_MERGED",
  "targetProjectPathWithNamespace":"org/zhiyi_doc","updatedAt":"2026-10-06T10:00:00Z"}
]`

const pushReviewDetail139 = `{
 "localId":139,"title":"feat: push review","status":"UNDER_DEV",
 "sourceBranch":"feat/x","targetBranch":"master","ahead":3,"behind":1,
 "allRequirementsPass":false,
 "todoList":{"requirementCheckItems":[
   {"itemType":"REVIEWER_APPROVED_CHECK","pass":true},
   {"itemType":"COMMENTS_CHECK","pass":false}]},
 "reviewers":[{"name":"张三","username":"zhangsan","reviewOpinionStatus":"PASS","hasReviewed":true}],
 "author":{"name":"李四","username":"lisi"},"createFrom":"COMMAND_LINE",
 "detailUrl":"https://codeup.aliyun.com/org/zhiyi_doc/change/139"
}`

const pushReviewDetail140 = `{
 "localId":140,"title":"fix: ready","status":"TO_BE_MERGED",
 "ahead":1,"behind":0,"allRequirementsPass":true,
 "reviewers":[{"name":"张三","username":"zhangsan","reviewOpinionStatus":"PASS"}],
 "detailUrl":"https://codeup.aliyun.com/org/zhiyi_doc/change/140"
}`

type pushReviewServer struct {
	mu           sync.Mutex
	listGETs     int
	detailGETs   map[string]int
	detailStatus int // non-zero: every detail GET answers with this status
	lastQuery    string
}

func newPushReviewServer(t *testing.T) *pushReviewServer {
	t.Helper()
	s := &pushReviewServer{detailGETs: map[string]int{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/changeRequests"):
			s.listGETs++
			s.lastQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, pushReviewListFixture)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/changeRequests/139"):
			s.detailGETs["139"]++
			w.Header().Set("Content-Type", "application/json")
			if s.detailStatus != 0 {
				w.WriteHeader(s.detailStatus)
				_, _ = io.WriteString(w, `{"errorCode":"SystemError","errorMessage":"boom"}`)
				return
			}
			_, _ = io.WriteString(w, pushReviewDetail139)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/changeRequests/140"):
			s.detailGETs["140"]++
			w.Header().Set("Content-Type", "application/json")
			if s.detailStatus != 0 {
				w.WriteHeader(s.detailStatus)
				_, _ = io.WriteString(w, `{"errorCode":"SystemError","errorMessage":"boom"}`)
				return
			}
			_, _ = io.WriteString(w, pushReviewDetail140)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-push-review-not-real")
	t.Setenv(config.EnvOrganizationID, "org-push-review-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

func runPushReviewStatus(t *testing.T, dryRun bool, extra ...string) (string, string, int) {
	t.Helper()
	resetStringFlags(t, codeupMrsPushReviewStatusCmd, "repo", "local-id", "all", "per-page")
	args := append([]string{"codeup", "mrs", "+push-review-status", "--repo", "4951320"}, extra...)
	return runMrsStatusCmd(t, dryRun, false, args...)
}

// #132: list open MRs + one detail GET each; composed view carries everything the
// "push → check MR state → decide" loop needs.
func TestPushReviewStatusComposesViews(t *testing.T) {
	s := newPushReviewServer(t)
	stdout, stderr, code := runPushReviewStatus(t, false)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env := decodeMrsStatusEnvelope(t, stdout)
	if !env.OK {
		t.Fatalf("env=%+v", env)
	}
	items, _ := env.Data.([]any)
	if len(items) != 2 {
		t.Fatalf("data=%#v", env.Data)
	}
	m139 := mrsStatusItem(t, env, "139")
	if m139["status"] != "UNDER_DEV" || m139["state"] != "UNDER_DEV" || m139["status_display"] != "开发中(WIP)" || m139["wip"] != true {
		t.Fatalf("139 status fields: %#v", m139)
	}
	if m139["ahead"] != float64(3) || m139["behind"] != float64(1) {
		t.Fatalf("139 ahead/behind: %#v", m139)
	}
	if m139["mergeable"] != false {
		t.Fatalf("139 mergeable: %#v", m139)
	}
	todo, _ := m139["todo"].([]any)
	if len(todo) != 2 {
		t.Fatalf("139 todo: %#v", m139["todo"])
	}
	reviewers, _ := m139["reviewers"].([]any)
	if len(reviewers) != 1 || reviewers[0].(map[string]any)["review_opinion_status"] != "PASS" {
		t.Fatalf("139 reviewers: %#v", m139["reviewers"])
	}
	if m139["review_passed"] != true {
		t.Fatalf("139 review_passed: %#v", m139)
	}
	if m139["author_name"] != "李四" || m139["creation_method"] != "COMMAND_LINE" {
		t.Fatalf("139 author/creation: %#v", m139)
	}
	if u, _ := m139["url"].(string); !strings.Contains(u, "/change/139") {
		t.Fatalf("139 url: %#v", m139["url"])
	}
	m140 := mrsStatusItem(t, env, "140")
	if m140["status"] != "TO_BE_MERGED" || m140["wip"] != false || m140["mergeable"] != true {
		t.Fatalf("140: %#v", m140)
	}
	if env.Meta["repository_id"] != "4951320" || env.Meta["count"] != float64(2) || env.Meta["wip_count"] != float64(1) {
		t.Fatalf("meta: %#v", env.Meta)
	}
	if s.listGETs != 1 || s.detailGETs["139"] != 1 || s.detailGETs["140"] != 1 {
		t.Fatalf("listGETs=%d detailGETs=%#v", s.listGETs, s.detailGETs)
	}
	if !strings.Contains(s.lastQuery, "state=opened") || !strings.Contains(s.lastQuery, "projectIds=4951320") {
		t.Fatalf("query=%q", s.lastQuery)
	}
}

// --local-id: single detail GET, no list call.
func TestPushReviewStatusLocalIdSingle(t *testing.T) {
	s := newPushReviewServer(t)
	stdout, stderr, code := runPushReviewStatus(t, false, "--local-id", "139")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env := decodeMrsStatusEnvelope(t, stdout)
	if !env.OK || len(env.Data.([]any)) != 1 {
		t.Fatalf("env=%+v", env)
	}
	m := mrsStatusItem(t, env, "139")
	if m["status"] != "UNDER_DEV" || m["wip"] != true || m["ahead"] != float64(3) {
		t.Fatalf("139: %#v", m)
	}
	if env.Meta["local_id"] != "139" || env.Meta["count"] != float64(1) || env.Meta["wip_count"] != float64(1) {
		t.Fatalf("meta: %#v", env.Meta)
	}
	if s.listGETs != 0 || s.detailGETs["139"] != 1 || s.detailGETs["140"] != 0 {
		t.Fatalf("listGETs=%d detailGETs=%#v", s.listGETs, s.detailGETs)
	}
}

// Dry-run previews the reads only; nothing is fetched.
func TestPushReviewStatusDryRun(t *testing.T) {
	s := newPushReviewServer(t)
	stdout, stderr, code := runPushReviewStatus(t, true, "--dry-run")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("JSON: %v / %s", err, stdout)
	}
	if !env.OK || !env.DryRun || env.Risk != "read" {
		t.Fatalf("env=%+v", env)
	}
	req, _ := env.Request.(map[string]any)
	list, _ := req["list"].(map[string]any)
	if list["method"] != "GET" || !strings.Contains(fmtURL(list["url"]), "/changeRequests") {
		t.Fatalf("request.list: %#v", req["list"])
	}
	if !strings.Contains(fmtURL(list["url"]), "state=opened") {
		t.Fatalf("list url must carry state=opened: %#v", list["url"])
	}
	detail, _ := req["detail"].(map[string]any)
	if detail["skipped"] != true {
		t.Fatalf("request.detail: %#v", detail)
	}
	if s.listGETs != 0 || len(s.detailGETs) != 0 {
		t.Fatalf("dry-run must not call the API: listGETs=%d detailGETs=%#v", s.listGETs, s.detailGETs)
	}
}

// --local-id dry-run previews the single detail GET instead of the list.
func TestPushReviewStatusDryRunLocalId(t *testing.T) {
	s := newPushReviewServer(t)
	stdout, stderr, code := runPushReviewStatus(t, true, "--local-id", "139", "--dry-run")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil || !env.DryRun {
		t.Fatalf("env=%+v err=%v", env, err)
	}
	req, _ := env.Request.(map[string]any)
	list, _ := req["list"].(map[string]any)
	if list["method"] != "GET" || !strings.Contains(fmtURL(list["url"]), "/changeRequests/139") {
		t.Fatalf("request.list (single-MR mode): %#v", req["list"])
	}
	if s.listGETs != 0 || len(s.detailGETs) != 0 {
		t.Fatalf("dry-run must not call the API: listGETs=%d detailGETs=%#v", s.listGETs, s.detailGETs)
	}
}

// A failing detail GET fails the shortcut (context-prefixed API error); it never
// reports a half-composed status as success.
func TestPushReviewStatusDetailErrorFails(t *testing.T) {
	t.Cleanup(client.SetRetrySleepForTest(func(ctx context.Context, d time.Duration) error { return nil }))
	s := newPushReviewServer(t)
	s.detailStatus = http.StatusInternalServerError
	stdout, stderr, code := runPushReviewStatus(t, false)
	if code != 1 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if eb.Type != "api" || eb.Code != http.StatusInternalServerError {
		t.Fatalf("error body=%+v", eb)
	}
	if !strings.HasPrefix(eb.Message, "refresh MR 139 push-review status: ") {
		t.Fatalf("message=%q", eb.Message)
	}
}

// Empty open list is still ok with zero counts.
func TestPushReviewStatusEmptyOpenList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-push-review-empty-not-real")
	t.Setenv(config.EnvOrganizationID, "org-push-review-empty")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	stdout, stderr, code := runPushReviewStatus(t, false)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env := decodeMrsStatusEnvelope(t, stdout)
	if !env.OK || len(env.Data.([]any)) != 0 {
		t.Fatalf("env=%+v", env)
	}
	if env.Meta["count"] != float64(0) || env.Meta["wip_count"] != float64(0) {
		t.Fatalf("meta: %#v", env.Meta)
	}
}

func fmtURL(v any) string {
	s, _ := v.(string)
	return s
}

func TestPushReviewStatusHelp(t *testing.T) {
	h := codeupMrsPushReviewStatusCmd.Long
	for _, want := range []string{"Risk: read", "UNDER_DEV", "取消 WIP", "--local-id", "--all", "ahead/behind"} {
		if !strings.Contains(h, want) {
			t.Fatalf("help missing %q: %s", want, h)
		}
	}
	found := false
	for _, c := range codeupMrsCmd.Commands() {
		if c.Name() == "+push-review-status" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("+push-review-status not registered on mrs")
	}
}
