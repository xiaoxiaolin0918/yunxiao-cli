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

// #132 fixtures: a push-review WIP MR (newVersionState UNDER_DEV + workInProgress),
// a ready MR, and an old-version item with only legacy lowercase state.
const mrsListStatusFixture = `[
 {"localId":139,"title":"feat: push review","state":"opened","newVersionState":"UNDER_DEV","workInProgress":true,
  "sourceBranch":"feat/x","targetBranch":"master","targetProjectPathWithNamespace":"org/zhiyi_doc","updatedAt":"2026-10-05T10:00:00Z"},
 {"localId":140,"title":"fix: ready","state":"opened","newVersionState":"TO_BE_MERGED",
  "targetProjectPathWithNamespace":"org/zhiyi_doc","updatedAt":"2026-10-06T10:00:00Z"},
 {"localId":141,"title":"chore: old mr","state":"opened",
  "targetProjectPathWithNamespace":"org/zhiyi_doc","updatedAt":"2026-10-04T10:00:00Z"}
]`

type mrsStatusServer struct {
	mu        sync.Mutex
	listGETs  int
	lastQuery string
}

func newMrsStatusServer(t *testing.T, listBody string) *mrsStatusServer {
	t.Helper()
	s := &mrsStatusServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/changeRequests") {
			s.listGETs++
			s.lastQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, listBody)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-mrs-status-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-status-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

// runMrsStatusCmd executes a codeup command with captured stdout/stderr and the
// processExit code (0 when the command did not exit).
func runMrsStatusCmd(t *testing.T, dryRun, yes bool, args ...string) (string, string, int) {
	t.Helper()
	stdout := withCmdJSONCapture(t) // resets globals (dryRun=true, yes=false, profile/org "")
	prevDry := globalDryRun
	globalDryRun = dryRun
	prevYes := globalYes
	globalYes = yes
	t.Cleanup(func() {
		globalDryRun = prevDry
		globalYes = prevYes
	})
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

func decodeMrsStatusEnvelope(t *testing.T, raw string) output.Envelope {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("JSON: %v / %s", err, raw)
	}
	return env
}

func mrsStatusItem(t *testing.T, env output.Envelope, localID string) map[string]any {
	t.Helper()
	items, ok := env.Data.([]any)
	if !ok {
		t.Fatalf("data not a list: %#v", env.Data)
	}
	for _, it := range items {
		if m, ok := it.(map[string]any); ok && fmtLocalID(m["localId"]) == localID {
			return m
		}
	}
	t.Fatalf("localId %s not found in %#v", localID, env.Data)
	return nil
}

// fmtLocalID renders JSON-decoded localId values (float64 or string) as "139".
func fmtLocalID(v any) string {
	switch t := v.(type) {
	case float64:
		b, _ := json.Marshal(t)
		return strings.TrimSuffix(string(b), ".0")
	case string:
		return t
	default:
		return ""
	}
}

// #132: mrs list injects per-item status/wip next to the existing url injection.
func TestMrsListInjectsStatusAndWip(t *testing.T) {
	s := newMrsStatusServer(t, mrsListStatusFixture)
	resetStringFlags(t, codeupMrsListCmd, "state", "search", "status", "repo", "sort")
	resetStringFlags(t, codeupMrsListCmd, "all", "page", "per-page")
	stdout, stderr, code := runMrsStatusCmd(t, false, false,
		"codeup", "mrs", "list", "--repo", "4951320")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env := decodeMrsStatusEnvelope(t, stdout)
	if !env.OK || s.listGETs != 1 {
		t.Fatalf("ok=%v listGETs=%d env=%+v", env.OK, s.listGETs, env)
	}
	m139 := mrsStatusItem(t, env, "139")
	if m139["status"] != "UNDER_DEV" || m139["wip"] != true {
		t.Fatalf("139 status/wip: %#v", m139)
	}
	if m139["state"] != "opened" {
		t.Fatalf("legacy state must stay untouched: %#v", m139)
	}
	if u, _ := m139["url"].(string); !strings.Contains(u, "/change/139") {
		t.Fatalf("url injection must survive: %#v", m139["url"])
	}
	m140 := mrsStatusItem(t, env, "140")
	if m140["status"] != "TO_BE_MERGED" || m140["wip"] != false {
		t.Fatalf("140: %#v", m140)
	}
	m141 := mrsStatusItem(t, env, "141")
	if m141["status"] != "opened" || m141["wip"] != false {
		t.Fatalf("141 legacy item: %#v", m141)
	}
	// raw API fields kept
	if m139["newVersionState"] != "UNDER_DEV" || m139["workInProgress"] != true {
		t.Fatalf("API fields must be preserved: %#v", m139)
	}
}

// #132: +open-mrs injects the same status/wip fields and sends state=opened.
func TestOpenMrsInjectsStatusAndWip(t *testing.T) {
	s := newMrsStatusServer(t, mrsListStatusFixture)
	resetStringFlags(t, codeupOpenMrsShortcut, "state", "search", "repo", "page", "per-page")
	stdout, stderr, code := runMrsStatusCmd(t, false, false,
		"codeup", "+open-mrs", "--repo", "4951320")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env := decodeMrsStatusEnvelope(t, stdout)
	if !env.OK {
		t.Fatalf("env=%+v", env)
	}
	m := mrsStatusItem(t, env, "139")
	if m["status"] != "UNDER_DEV" || m["wip"] != true {
		t.Fatalf("139: %#v", m)
	}
	if !strings.Contains(s.lastQuery, "state=opened") || !strings.Contains(s.lastQuery, "projectIds=4951320") {
		t.Fatalf("query=%q", s.lastQuery)
	}
}

// --status filters client-side (case-insensitive) and records itself in meta.
func TestMrsListStatusFilterClientSide(t *testing.T) {
	s := newMrsStatusServer(t, mrsListStatusFixture)
	resetStringFlags(t, codeupMrsListCmd, "state", "search", "status", "repo", "sort")
	resetStringFlags(t, codeupMrsListCmd, "all", "page", "per-page")
	stdout, stderr, code := runMrsStatusCmd(t, false, false,
		"codeup", "mrs", "list", "--repo", "4951320", "--status", "under_dev")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env := decodeMrsStatusEnvelope(t, stdout)
	items := env.Data.([]any)
	if len(items) != 1 {
		t.Fatalf("want only the UNDER_DEV MR, got %#v", env.Data)
	}
	if items[0].(map[string]any)["localId"] != float64(139) {
		t.Fatalf("item: %#v", items[0])
	}
	if env.Meta["status_filter"] != "UNDER_DEV" || env.Meta["status_filter_scope"] != "client-side" {
		t.Fatalf("meta: %#v", env.Meta)
	}
	// the filter is client-side: the server still receives no status param
	if strings.Contains(s.lastQuery, "status=") {
		t.Fatalf("server query must not carry status: %q", s.lastQuery)
	}
}

func TestMrsListStatusFilterNoMatch(t *testing.T) {
	newMrsStatusServer(t, mrsListStatusFixture)
	resetStringFlags(t, codeupMrsListCmd, "state", "search", "status", "repo", "sort")
	resetStringFlags(t, codeupMrsListCmd, "all", "page", "per-page")
	stdout, stderr, code := runMrsStatusCmd(t, false, false,
		"codeup", "mrs", "list", "--repo", "4951320", "--status", "MERGED")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env := decodeMrsStatusEnvelope(t, stdout)
	if !env.OK || len(env.Data.([]any)) != 0 {
		t.Fatalf("env=%+v", env)
	}
}

// Dry-run previews without executing (read path).
func TestMrsListStatusDryRunNoCalls(t *testing.T) {
	s := newMrsStatusServer(t, mrsListStatusFixture)
	resetStringFlags(t, codeupMrsListCmd, "state", "search", "status", "repo", "sort")
	resetStringFlags(t, codeupMrsListCmd, "all", "page", "per-page")
	stdout, stderr, code := runMrsStatusCmd(t, true, false,
		"codeup", "mrs", "list", "--repo", "4951320")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env := decodeMrsStatusEnvelope(t, stdout)
	if !env.OK || !env.DryRun {
		t.Fatalf("env=%+v", env)
	}
	if s.listGETs != 0 {
		t.Fatalf("dry-run must not call the API, got %d", s.listGETs)
	}
}

func TestMrsListHelpDocumentsStatusInjection(t *testing.T) {
	h := codeupMrsListCmd.Long
	for _, want := range []string{"UNDER_DEV", "newVersionState", "client-side", "status_filter"} {
		if !strings.Contains(h, want) {
			t.Fatalf("list help missing %q: %s", want, h)
		}
	}
	if !strings.Contains(codeupOpenMrsShortcut.Long, "UNDER_DEV") {
		t.Fatalf("open-mrs help: %s", codeupOpenMrsShortcut.Long)
	}
}
