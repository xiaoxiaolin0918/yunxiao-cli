package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

const mrsReviewAuthor403Body = `{"code":403,"errorCode":"SYSTEM_FORBIDDEN_ERROR","errorDescription":"can not submit review opinion as the author"}`

type mrsReviewAuthorServer struct {
	mu           sync.Mutex
	reviewPOSTs  int
	detailGETs   int
	userGETs     int
	other        []string
	mrFixture    string
	userFixture  string
	reviewStatus int
	reviewBody   string
	detailStatus int
	userStatus   int
}

func newMrsReviewAuthorServer(t *testing.T, mrFixture, userFixture string) *mrsReviewAuthorServer {
	t.Helper()
	s := &mrsReviewAuthorServer{mrFixture: mrFixture, userFixture: userFixture}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/platform/user"):
			s.userGETs++
			if s.userStatus != 0 {
				w.WriteHeader(s.userStatus)
				_, _ = io.WriteString(w, `{"errorCode":"SystemError","errorMessage":"boom"}`)
				return
			}
			_, _ = io.WriteString(w, s.userFixture)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/changeRequests/3051") && !strings.HasSuffix(r.URL.Path, "/review"):
			s.detailGETs++
			if s.detailStatus != 0 {
				w.WriteHeader(s.detailStatus)
				_, _ = io.WriteString(w, `{"errorCode":"SystemError","errorMessage":"boom"}`)
				return
			}
			_, _ = io.WriteString(w, s.mrFixture)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/changeRequests/3051/review"):
			s.reviewPOSTs++
			status := s.reviewStatus
			body := s.reviewBody
			if status == 0 {
				status = http.StatusOK
				body = `{"result":true}`
			}
			w.WriteHeader(status)
			_, _ = io.WriteString(w, body)
		default:
			s.other = append(s.other, r.Method+" "+r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-mrs-review-author-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-review-author-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

func runMrsReviewAuthorCmd(t *testing.T, yes, dryRun bool, extra ...string) (string, string, int) {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	globalDryRun = dryRun
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

	resetStringFlags(t, codeupMrsReviewCmd, "repo", "local-id", "opinion", "comment")
	args := append([]string{"codeup", "mrs", "review", "--repo", "4951346", "--local-id", "3051", "--opinion", "PASS"}, extra...)
	if dryRun {
		args = append(args, "--dry-run")
	}
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

func TestMrAuthorUserIDAndPlatformUserID(t *testing.T) {
	if got := mrAuthorUserID(map[string]any{"author": map[string]any{"userId": "abc", "id": "ignored"}}); got != "abc" {
		t.Fatalf("prefer userId: %q", got)
	}
	if got := mrAuthorUserID(map[string]any{"author": map[string]any{"id": "fallback"}}); got != "fallback" {
		t.Fatalf("fallback id: %q", got)
	}
	if got := mrAuthorUserID(nil); got != "" {
		t.Fatalf("nil: %q", got)
	}
	if got := platformUserID(map[string]any{"id": "me"}); got != "me" {
		t.Fatalf("platform id: %q", got)
	}
	if !identitiesEqual("AbC", "abc") || identitiesEqual("", "x") || identitiesEqual("a", "b") {
		t.Fatal("identitiesEqual")
	}
}

func TestIsAuthorSelfReviewForbidden(t *testing.T) {
	ok := &client.APIError{Status: 403, Body: mrsReviewAuthor403Body}
	if !isAuthorSelfReviewForbidden(ok) {
		t.Fatal("want match")
	}
	if isAuthorSelfReviewForbidden(&client.APIError{Status: 403, Body: `{"errorDescription":"other"}`}) {
		t.Fatal("other 403")
	}
	if isAuthorSelfReviewForbidden(&client.APIError{Status: 405, Body: mrsReviewAuthor403Body}) {
		t.Fatal("wrong status")
	}
	if isAuthorSelfReviewForbidden(errors.New("nope")) {
		t.Fatal("non-api")
	}
}

func TestReviewAuthorSuggestedActions(t *testing.T) {
	actions := reviewAuthorSuggestedActions("4951346", "3051")
	joined := strings.Join(actions, "\n")
	for _, want := range []string{
		"yunxiao codeup mrs reviewers add --repo 4951346 --local-id 3051 --reviewer <otherUserId>",
		"switch CLI identity",
		"yunxiao codeup mrs get --repo 4951346 --local-id 3051",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %s", want, joined)
		}
	}
}

func TestMrsReviewPrecheckBlocksAuthorSelf(t *testing.T) {
	s := newMrsReviewAuthorServer(t,
		`{"localId":3051,"title":"feat","status":"UNDER_REVIEW","author":{"userId":"user-author","name":"Me"}}`,
		`{"id":"user-author","name":"Me"}`,
	)
	stdout, stderr, code := runMrsReviewAuthorCmd(t, true, false)
	if code != 1 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stderr), &env); err != nil {
		t.Fatalf("stderr json: %v / %s", err, stderr)
	}
	if env.OK || env.Error == nil || env.Error.Subtype != "review_as_author" {
		t.Fatalf("env=%+v", env)
	}
	if !strings.Contains(env.Error.Message, "author") {
		t.Fatalf("message=%q", env.Error.Message)
	}
	if !strings.Contains(env.Error.Hint, "reviewers add") {
		t.Fatalf("hint=%q", env.Error.Hint)
	}
	raw, _ := json.Marshal(env.Error.Details["suggested_actions"])
	if !strings.Contains(string(raw), "reviewers add") {
		t.Fatalf("suggested_actions=%s", raw)
	}
	if s.reviewPOSTs != 0 {
		t.Fatalf("must not POST review: %d", s.reviewPOSTs)
	}
	if s.detailGETs < 1 || s.userGETs < 1 {
		t.Fatalf("gets detail=%d user=%d", s.detailGETs, s.userGETs)
	}
}

func TestMrsReviewPrecheckAllowsNonAuthor(t *testing.T) {
	s := newMrsReviewAuthorServer(t,
		`{"localId":3051,"title":"feat","status":"UNDER_REVIEW","author":{"userId":"user-author","name":"Author"}}`,
		`{"id":"user-reviewer","name":"Reviewer"}`,
	)
	s.reviewStatus = http.StatusOK
	s.reviewBody = `{"result":true}`
	stdout, stderr, code := runMrsReviewAuthorCmd(t, true, false)
	if code != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout json: %v / %s", err, stdout)
	}
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	metaRaw, _ := json.Marshal(env.Meta)
	var meta map[string]any
	_ = json.Unmarshal(metaRaw, &meta)
	pre, _ := meta["precheck"].(map[string]any)
	if pre == nil || pre["status"] != "ok" || pre["is_author"] != false {
		t.Fatalf("precheck=%v meta=%s", pre, metaRaw)
	}
	if s.reviewPOSTs != 1 {
		t.Fatalf("POSTs=%d", s.reviewPOSTs)
	}
}

func TestMrsReviewDryRunPrecheckAuthorBlocks(t *testing.T) {
	s := newMrsReviewAuthorServer(t,
		`{"localId":3051,"author":{"userId":"same"},"status":"UNDER_REVIEW"}`,
		`{"id":"same"}`,
	)
	_, stderr, code := runMrsReviewAuthorCmd(t, false, true)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stderr), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stderr)
	}
	if env.Error == nil || env.Error.Subtype != "review_as_author" {
		t.Fatalf("%+v", env)
	}
	if s.reviewPOSTs != 0 {
		t.Fatalf("POSTs=%d", s.reviewPOSTs)
	}
}

func TestMrsReviewDryRunPrecheckOK(t *testing.T) {
	s := newMrsReviewAuthorServer(t,
		`{"localId":3051,"author":{"userId":"author"},"status":"UNDER_REVIEW"}`,
		`{"id":"reviewer"}`,
	)
	stdout, stderr, code := runMrsReviewAuthorCmd(t, false, true)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout)
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
	pre, _ := req["precheck"].(map[string]any)
	if pre == nil || pre["status"] != "ok" || pre["is_author"] != false {
		t.Fatalf("request.precheck=%v raw=%s", pre, raw)
	}
	if s.reviewPOSTs != 0 {
		t.Fatalf("POSTs=%d", s.reviewPOSTs)
	}
}

func TestMrsReview403EnrichedWhenPrecheckSkipped(t *testing.T) {
	// Force skip precheck (detail GET fails), then POST returns author 403 → enrich.
	s := newMrsReviewAuthorServer(t,
		`{"localId":3051,"author":{"userId":"author"}}`,
		`{"id":"author"}`,
	)
	s.detailStatus = http.StatusInternalServerError
	s.reviewStatus = http.StatusForbidden
	s.reviewBody = mrsReviewAuthor403Body
	_, stderr, code := runMrsReviewAuthorCmd(t, true, false)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stderr), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stderr)
	}
	if env.Error == nil || env.Error.Subtype != "review_as_author" {
		t.Fatalf("%+v", env)
	}
	if !strings.Contains(env.Error.Hint, "reviewers add") {
		t.Fatalf("hint=%q", env.Error.Hint)
	}
	if env.Error.Type != "api" && env.Error.Code != 403 {
		// contextError wrapping APIError → type api + subtype
		if env.Error.Code != 403 {
			t.Fatalf("want api 403 body: %+v", env.Error)
		}
	}
	if s.reviewPOSTs != 1 {
		t.Fatalf("POSTs=%d (precheck skipped so POST must run)", s.reviewPOSTs)
	}
}

func TestMrsReviewOther403NotMisAttributed(t *testing.T) {
	s := newMrsReviewAuthorServer(t,
		`{"localId":3051,"author":{"userId":"author"}}`,
		`{"id":"reviewer"}`,
	)
	s.reviewStatus = http.StatusForbidden
	s.reviewBody = `{"errorCode":"SYSTEM_FORBIDDEN_ERROR","errorDescription":"permission denied for other reason"}`
	_, stderr, code := runMrsReviewAuthorCmd(t, true, false)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stderr), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stderr)
	}
	if env.Error != nil && env.Error.Subtype == "review_as_author" {
		t.Fatalf("must not mis-attribute: %+v", env.Error)
	}
	if s.reviewPOSTs != 1 {
		t.Fatalf("POSTs=%d", s.reviewPOSTs)
	}
}
