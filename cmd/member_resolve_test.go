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

func TestLooksLikeUserID(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"user-1", true},
		{"u1", true},
		{"abcDEF0123", true},
		{" hex_id ", true},
		{"崔健", false},
		{" 崔健 ", false},
		{"Alice Bob", false},
		{"", false},
		{"user@x", false},
	}
	for _, tc := range cases {
		if got := looksLikeUserID(tc.in); got != tc.want {
			t.Fatalf("looksLikeUserID(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestFilterExactMemberHits(t *testing.T) {
	items := []any{
		map[string]any{"id": "u1", "name": "崔健", "email": "cui@ex.com"},
		map[string]any{"id": "u2", "name": "其他人"},
		map[string]any{"id": "u3", "name": " 崔健 "},
		map[string]any{"id": "u1", "name": "崔健"}, // dup id
		"skip",
	}
	hits := filterExactMemberHits(items, "崔健")
	if len(hits) != 2 {
		t.Fatalf("hits=%+v", hits)
	}
	if hits[0].ID != "u1" || hits[0].Email != "cui@ex.com" {
		t.Fatalf("first=%+v", hits[0])
	}
	if hits[1].ID != "u3" {
		t.Fatalf("second=%+v", hits[1])
	}
}

type memberResolveServer struct {
	mu       sync.Mutex
	members  string
	status   int
	searches int
	bodies   []map[string]any
}

func newMemberResolveServer(t *testing.T, members string) *memberResolveServer {
	t.Helper()
	s := &memberResolveServer{members: members}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/members:search") {
			s.searches++
			b, _ := io.ReadAll(r.Body)
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			s.bodies = append(s.bodies, m)
			if s.status != 0 {
				w.WriteHeader(s.status)
				_, _ = io.WriteString(w, `{"errorMessage":"boom"}`)
				return
			}
			_, _ = io.WriteString(w, s.members)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-member-resolve-not-real")
	t.Setenv(config.EnvOrganizationID, "org-member-resolve")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

func runMemberResolve(t *testing.T, dryRun bool, extra ...string) (string, string, int) {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	prevDry := globalDryRun
	globalDryRun = dryRun
	t.Cleanup(func() { globalDryRun = prevDry })
	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	prevExit := processExit
	code := 0
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })

	resetStringFlags(t, orgMemberResolveCmd, "name", "pick")
	args := append([]string{"organization", "+member-resolve"}, extra...)
	rootCmd.SetArgs(args)
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
	if execErr != nil && code == 0 {
		t.Fatalf("execute: %v\nstdout=%s stderr=%s", execErr, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String(), code
}

func TestOrganizationMemberResolveDryRun(t *testing.T) {
	s := newMemberResolveServer(t, `{"members":[]}`)
	stdout, stderr, code := runMemberResolve(t, true, "--name", "崔健")
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout)
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/members:search") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body["query"] != "崔健" {
		t.Fatalf("body=%v", body)
	}
	if s.searches != 0 {
		t.Fatalf("searches=%d", s.searches)
	}
}

func TestOrganizationMemberResolveUnique(t *testing.T) {
	s := newMemberResolveServer(t, `{"members":[{"id":"user-cui","name":"崔健","email":"cui@ex.com"},{"id":"user-other","name":"其他人"}]}`)
	stdout, stderr, code := runMemberResolve(t, false, "--name", "崔健")
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout)
	}
	if !env.OK || env.DryRun {
		t.Fatalf("%+v", env)
	}
	data, _ := env.Data.(map[string]any)
	if data["userId"] != "user-cui" || data["name"] != "崔健" || data["email"] != "cui@ex.com" {
		t.Fatalf("data=%v", data)
	}
	if s.searches != 1 {
		t.Fatalf("searches=%d", s.searches)
	}
}

func TestOrganizationMemberResolveAmbiguous(t *testing.T) {
	_ = newMemberResolveServer(t, `{"members":[{"id":"user-cui-1","name":"崔健"},{"id":"user-cui-2","name":"崔健","email":"c2@ex.com"}]}`)
	_, stderr, code := runMemberResolve(t, false, "--name", "崔健")
	if code != 1 {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stderr), &env); err != nil || env.Error == nil {
		t.Fatalf("stderr JSON: %v / %s", err, stderr)
	}
	msg := env.Error.Message
	if !strings.Contains(msg, "ambiguous") || !strings.Contains(msg, "user-cui-1") || !strings.Contains(msg, "user-cui-2") {
		t.Fatalf("message=%q", msg)
	}
}

func TestOrganizationMemberResolvePickFirst(t *testing.T) {
	_ = newMemberResolveServer(t, `{"members":[{"id":"user-cui-1","name":"崔健"},{"id":"user-cui-2","name":"崔健"}]}`)
	stdout, stderr, code := runMemberResolve(t, false, "--name", "崔健", "--pick", "first")
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout)
	}
	data, _ := env.Data.(map[string]any)
	if data["userId"] != "user-cui-1" {
		t.Fatalf("data=%v", data)
	}
}

func TestOrganizationMemberResolveNoMatch(t *testing.T) {
	_ = newMemberResolveServer(t, `{"members":[{"id":"user-cui","name":"崔健"}]}`)
	_, stderr, code := runMemberResolve(t, false, "--name", "不存在")
	if code != 1 {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "no organization member named") {
		t.Fatalf("stderr=%s", stderr)
	}
}

func TestResolveAssignedToRefPassthroughAndName(t *testing.T) {
	if !looksLikeUserID("user-assignee") {
		t.Fatal("expected id-shaped")
	}
	if looksLikeUserID("崔健") {
		t.Fatal("expected name-shaped")
	}
}
