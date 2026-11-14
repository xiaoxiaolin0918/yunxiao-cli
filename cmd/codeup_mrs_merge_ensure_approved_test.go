package cmd

import (
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

type ensureMergeServer struct {
	mu          sync.Mutex
	detail      map[string]any
	userID      string
	calls       []string
	mergeStatus int
	mergeBody   string
}

func newEnsureMergeServer(t *testing.T, detail map[string]any, userID string) *ensureMergeServer {
	t.Helper()
	if detail == nil {
		detail = map[string]any{}
	}
	s := &ensureMergeServer{detail: detail, userID: userID, mergeStatus: http.StatusOK, mergeBody: `{"result":true,"localId":21,"status":"MERGED"}`}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/platform/user"):
			s.calls = append(s.calls, "GET /platform/user")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": s.userID, "name": "me"})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/changeRequests/21") && !strings.Contains(r.URL.Path, "/merge"):
			s.calls = append(s.calls, "GET /changeRequests/21")
			_ = json.NewEncoder(w).Encode(s.detail)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/person/REVIEWER"):
			s.calls = append(s.calls, "POST /person/REVIEWER")
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			ids, _ := body["userIds"].([]any)
			revs, _ := s.detail["reviewers"].([]any)
			for _, id := range ids {
				revs = append(revs, map[string]any{"userId": id})
			}
			s.detail["reviewers"] = revs
			_ = json.NewEncoder(w).Encode(map[string]any{"result": true})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/review"):
			s.calls = append(s.calls, "POST /review")
			var body map[string]any
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &body)
			if body["reviewOpinion"] != "PASS" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"errorMessage":"expected PASS"}`)
				return
			}
			s.detail["status"] = "TO_BE_MERGED"
			_ = json.NewEncoder(w).Encode(map[string]any{"result": true})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/merge"):
			s.calls = append(s.calls, "POST /merge")
			if s.mergeStatus != http.StatusOK && s.mergeStatus != 0 {
				w.WriteHeader(s.mergeStatus)
				_, _ = io.WriteString(w, s.mergeBody)
				return
			}
			_, _ = io.WriteString(w, s.mergeBody)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"errorMessage":"unexpected `+r.Method+` `+r.URL.Path+`"}`)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-ensure-approved-not-real")
	t.Setenv(config.EnvOrganizationID, "org-ensure-approved")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

func (s *ensureMergeServer) snapshotCalls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]string(nil), s.calls...)
	return out
}

func runMrsMergeEnsure(t *testing.T, dryRun, yes bool, extra ...string) (string, string, int) {
	t.Helper()
	resetStringFlags(t, codeupMrsMergeCmd, "repo", "local-id", "merge-type", "message", "remove-source-branch", "ensure-approved", "reviewer")
	args := append([]string{"codeup", "mrs", "merge", "--repo", "4951320", "--local-id", "21", "--merge-type", "no-fast-forward"}, extra...)
	return runMrsStatusCmd(t, dryRun, yes, args...)
}

func readyMR(reviewers ...any) map[string]any {
	m := map[string]any{
		"localId":             21,
		"title":               "feat: ticket",
		"status":              "TO_BE_MERGED",
		"mergeable":           true,
		"conflictCheckStatus": "NO_CONFLICT",
		"allRequirementsPass": true,
		"author":              map[string]any{"userId": "author-1"},
	}
	if len(reviewers) > 0 {
		m["reviewers"] = reviewers
	}
	return m
}

func TestMrsMergeEnsureApprovedFlagRegistered(t *testing.T) {
	if codeupMrsMergeCmd.Flags().Lookup("ensure-approved") == nil {
		t.Fatal("--ensure-approved not registered")
	}
	if codeupMrsMergeCmd.Flags().Lookup("ensure-approved").DefValue != "false" {
		t.Fatalf("must default to false, got %q", codeupMrsMergeCmd.Flags().Lookup("ensure-approved").DefValue)
	}
}

func TestMrsMergeEnsureApprovedDryRunListsStepsAndWritesNothing(t *testing.T) {
	s := newEnsureMergeServer(t, readyMR(), "user-me")
	s.detail["status"] = "UNDER_REVIEW"
	stdout, stderr, code := runMrsMergeEnsure(t, true, false, "--ensure-approved", "--dry-run")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil || !env.DryRun || !env.OK {
		t.Fatalf("env=%+v err=%v", env, err)
	}
	req, _ := env.Request.(map[string]any)
	if req["method"] != "POST" || !strings.Contains(fmtURL(req["url"]), "/changeRequests/21/merge") {
		t.Fatalf("merge preview missing: %#v", req)
	}
	ea, _ := req["ensure_approved"].(map[string]any)
	steps, _ := ea["steps"].([]any)
	if len(steps) < 3 {
		t.Fatalf("ensure_approved.steps=%#v", ea)
	}
	joined := ""
	for _, st := range steps {
		m, _ := st.(map[string]any)
		joined += m["action"].(string) + ":" + m["status"].(string) + ","
	}
	if !strings.Contains(joined, "reviewers_add:will_run") || !strings.Contains(joined, "review_pass:will_run") || !strings.Contains(joined, "merge:will_run") {
		t.Fatalf("steps=%q req=%#v", joined, req)
	}
	for _, c := range s.snapshotCalls() {
		if strings.HasPrefix(c, "POST ") {
			t.Fatalf("dry-run must not write: %v", s.snapshotCalls())
		}
	}
}

func TestMrsMergeEnsureApprovedDryRunIdempotentSkips(t *testing.T) {
	s := newEnsureMergeServer(t, readyMR(map[string]any{"userId": "user-me", "reviewOpinionStatus": "PASS"}), "user-me")
	stdout, stderr, code := runMrsMergeEnsure(t, true, false, "--ensure-approved", "--dry-run")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	req := decodeDryRunRequest(t, stdout)
	ea, _ := req["ensure_approved"].(map[string]any)
	steps, _ := ea["steps"].([]any)
	joined := ""
	for _, st := range steps {
		m, _ := st.(map[string]any)
		joined += m["action"].(string) + ":" + m["status"].(string) + ","
	}
	if !strings.Contains(joined, "reviewers_add:skip_already_reviewer") || !strings.Contains(joined, "review_pass:skip_already_pass") || !strings.Contains(joined, "merge:will_run") {
		t.Fatalf("steps=%q", joined)
	}
	for _, c := range s.snapshotCalls() {
		if strings.HasPrefix(c, "POST ") {
			t.Fatalf("dry-run wrote: %v", s.snapshotCalls())
		}
	}
}

func TestMrsMergeEnsureApprovedLiveAddPassMerge(t *testing.T) {
	detail := readyMR()
	detail["status"] = "UNDER_REVIEW"
	s := newEnsureMergeServer(t, detail, "user-me")
	stdout, stderr, code := runMrsMergeEnsure(t, false, true, "--ensure-approved", "--yes")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	calls := strings.Join(s.snapshotCalls(), ",")
	if !strings.Contains(calls, "POST /person/REVIEWER") || !strings.Contains(calls, "POST /review") || !strings.Contains(calls, "POST /merge") {
		t.Fatalf("calls=%s", calls)
	}
	// Order: add before PASS before merge.
	if strings.Index(calls, "POST /person/REVIEWER") > strings.Index(calls, "POST /review") || strings.Index(calls, "POST /review") > strings.Index(calls, "POST /merge") {
		t.Fatalf("wrong order: %s", calls)
	}
	var env output.Envelope
	_ = json.Unmarshal([]byte(stdout), &env)
	if !env.OK {
		t.Fatalf("env=%+v", env)
	}
	ea, _ := env.Meta["ensure_approved"].(map[string]any)
	if ea == nil {
		t.Fatalf("meta.ensure_approved missing: %#v", env.Meta)
	}
}

func TestMrsMergeEnsureApprovedLiveSkipsWhenAlreadyApproved(t *testing.T) {
	s := newEnsureMergeServer(t, readyMR(map[string]any{"userId": "user-me", "reviewOpinionStatus": "PASS"}), "user-me")
	_, stderr, code := runMrsMergeEnsure(t, false, true, "--ensure-approved", "--yes")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	calls := strings.Join(s.snapshotCalls(), ",")
	if strings.Contains(calls, "POST /person/REVIEWER") || strings.Contains(calls, "POST /review") {
		t.Fatalf("should skip add/PASS: %s", calls)
	}
	if !strings.Contains(calls, "POST /merge") {
		t.Fatalf("must still merge: %s", calls)
	}
}

func TestMrsMergeEnsureApprovedAuthorRefusedBeforeMerge(t *testing.T) {
	detail := readyMR()
	detail["status"] = "UNDER_REVIEW"
	detail["author"] = map[string]any{"userId": "user-me"}
	s := newEnsureMergeServer(t, detail, "user-me")
	_, stderr, code := runMrsMergeEnsure(t, false, true, "--ensure-approved", "--yes")
	if code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if eb.Subtype != "review_as_author" {
		t.Fatalf("subtype=%q body=%+v", eb.Subtype, eb)
	}
	for _, c := range s.snapshotCalls() {
		if c == "POST /merge" || c == "POST /review" {
			t.Fatalf("must not review/merge as author: %v", s.snapshotCalls())
		}
	}
}

func TestMrsMergeEnsureApprovedPushPermissionClassified(t *testing.T) {
	s := newEnsureMergeServer(t, readyMR(map[string]any{"userId": "user-me", "reviewOpinionStatus": "PASS"}), "user-me")
	s.mergeStatus = http.StatusForbidden
	s.mergeBody = `{"errorCode":"SYSTEM_FORBIDDEN_ERROR","errorMessage":"` + "\u65e0\u6743\u9650\u63a8\u9001\u5230\u4fdd\u62a4\u5206\u652f" + `"}`
	_, stderr, code := runMrsMergeEnsure(t, false, true, "--ensure-approved", "--yes")
	if code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if eb.Subtype != "merge_push_permission" {
		t.Fatalf("subtype=%q body=%+v", eb.Subtype, eb)
	}
	if eb.Details["rejection_class"] != "push_permission" {
		t.Fatalf("details=%#v", eb.Details)
	}
	if !strings.Contains(eb.Hint, "\u4fdd\u62a4") && !strings.Contains(strings.ToLower(eb.Hint), "protect") && !strings.Contains(eb.Hint, "\u63a8\u9001") {
		t.Fatalf("hint should distinguish push/protection vs PASS: %q", eb.Hint)
	}
	if strings.Contains(eb.Hint, "mrs review --opinion PASS") {
		t.Fatalf("must not suggest another PASS: %q", eb.Hint)
	}
}

func TestMrsMergeWithoutEnsureApprovedDoesNotAutoReview(t *testing.T) {
	detail := readyMR()
	detail["status"] = "UNDER_REVIEW"
	s := newEnsureMergeServer(t, detail, "user-me")
	stdout, stderr, code := runMrsMergeEnsure(t, false, true, "--yes")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	calls := strings.Join(s.snapshotCalls(), ",")
	if strings.Contains(calls, "POST /person/REVIEWER") || strings.Contains(calls, "POST /review") || strings.Contains(calls, "GET /platform/user") {
		t.Fatalf("default merge must not auto-approve: %s", calls)
	}
	if !strings.Contains(calls, "POST /merge") {
		t.Fatalf("should merge: %s", calls)
	}
}

func TestMrsMergeEnsureApprovedStillHonorsPrecheck(t *testing.T) {
	detail := readyMR(map[string]any{"userId": "user-me", "reviewOpinionStatus": "PASS"})
	detail["conflictCheckStatus"] = "HAS_CONFLICT"
	s := newEnsureMergeServer(t, detail, "user-me")
	_, stderr, code := runMrsMergeEnsure(t, false, true, "--ensure-approved", "--yes")
	if code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if eb.Subtype != "mr_conflict" {
		t.Fatalf("subtype=%q", eb.Subtype)
	}
	for _, c := range s.snapshotCalls() {
		if strings.HasPrefix(c, "POST ") {
			t.Fatalf("precheck must block writes: %v", s.snapshotCalls())
		}
	}
}

func TestMrsMergeEnsureApprovedGateBeforeWrites(t *testing.T) {
	s := newEnsureMergeServer(t, readyMR(), "user-me")
	_, stderr, code := runMrsMergeEnsure(t, false, false, "--ensure-approved")
	if code != 10 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	if len(s.snapshotCalls()) != 0 {
		t.Fatalf("gate must run first: %v", s.snapshotCalls())
	}
}

func TestMrsMergeEnsureApprovedHelp(t *testing.T) {
	h := codeupMrsMergeCmd.Long
	for _, want := range []string{"--ensure-approved", "reviewers add", "PASS", "idempotent", "push_permission"} {
		if !strings.Contains(h, want) {
			t.Fatalf("help missing %q:\n%s", want, h)
		}
	}
	if strings.Contains(h, "default on") || strings.Contains(h, "defaults to true") {
		t.Fatalf("must not enable ensure-approved by default: %s", h)
	}
}

func TestMrsMergeReviewerRequiresEnsureApproved(t *testing.T) {
	s := newEnsureMergeServer(t, readyMR(), "user-me")
	_, stderr, code := runMrsMergeEnsure(t, true, false, "--reviewer", "other", "--dry-run")
	if code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "--ensure-approved") {
		t.Fatalf("stderr=%s", stderr)
	}
	if len(s.snapshotCalls()) != 0 {
		t.Fatalf("validation must be local: %v", s.snapshotCalls())
	}
}
