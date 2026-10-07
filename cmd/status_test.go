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
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

type statusServer struct {
	mu          sync.Mutex
	userGETs    int
	searchPOSTs int
	mrsGETs     int
	runsGETs    int
	runDetail   int
	lastSearch  string
	lastMrsQ    string
	failMrs     bool
}

func newStatusServer(t *testing.T) *statusServer {
	t.Helper()
	s := &statusServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/platform/user"):
			s.userGETs++
			_, _ = io.WriteString(w, `{"id":"u-status-1","name":"Status User"}`)
		case r.Method == http.MethodPost && strings.Contains(path, "/workitems:search"):
			s.searchPOSTs++
			body, _ := io.ReadAll(r.Body)
			s.lastSearch = string(body)
			_, _ = io.WriteString(w, `[{"id":"wi-1","serialNumber":"REQ-1","subject":"open item"}]`)
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/changeRequests"):
			s.mrsGETs++
			s.lastMrsQ = r.URL.RawQuery
			if s.failMrs {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, `{"error":"boom"}`)
				return
			}
			_, _ = io.WriteString(w, `[
 {"localId":139,"title":"feat: push review","state":"opened","newVersionState":"UNDER_DEV","workInProgress":true,
  "targetProjectPathWithNamespace":"org/demo"},
 {"localId":140,"title":"fix: ready","state":"opened","newVersionState":"TO_BE_MERGED",
  "targetProjectPathWithNamespace":"org/demo"}
]`)
		case r.Method == http.MethodGet && strings.Contains(path, "/pipelines/") && strings.HasSuffix(path, "/runs"):
			s.runsGETs++
			_, _ = io.WriteString(w, `[{"pipelineRunId":"run-9","status":"WAITING"}]`)
		case r.Method == http.MethodGet && strings.Contains(path, "/pipelines/") && strings.Contains(path, "/runs/"):
			s.runDetail++
			_, _ = io.WriteString(w, `{
 "pipelineRunId":"run-9",
 "status":"WAITING",
 "stages":[{"jobs":[{"id":"j1","name":"manual","actions":[{"type":"pass"},{"type":"refuse"}]}]}]
}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{}`)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-status-not-real")
	t.Setenv(config.EnvOrganizationID, "org-status-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	// Install a tiny profile so resolveSpaceIDFlag works without --space-id.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	pf := &profile.Profile{Name: "statusplay", OrganizationID: "org-status-test", SpaceID: "space-status"}
	if _, err := pf.Save(); err != nil {
		t.Fatalf("save profile: %v", err)
	}
	t.Setenv(profile.EnvProfile, "statusplay")
	return s
}

func runStatusCmd(t *testing.T, dryRun bool, args ...string) (string, string, int) {
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
	processExit = func(c int) {
		code = c
		panic(exitPanic{code: c})
	}
	t.Cleanup(func() { processExit = prevExit })
	resetStringFlags(t, statusCmd, "skip-workitems", "skip-mrs", "category", "space-id", "status-stage",
		"repo", "pipeline-id", "all-pipelines", "include-running", "page", "per-page")
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

func decodeStatusEnvelope(t *testing.T, raw string) (output.Envelope, map[string]any) {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("JSON: %v / %s", err, raw)
	}
	data, _ := env.Data.(map[string]any)
	if data == nil {
		t.Fatalf("data not object: %#v", env.Data)
	}
	return env, data
}

func TestStatusAggregatesWorkitemsAndMrs(t *testing.T) {
	s := newStatusServer(t)
	stdout, stderr, code := runStatusCmd(t, false, "status", "--space-id", "space-status")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	env, data := decodeStatusEnvelope(t, stdout)
	if !env.OK {
		t.Fatalf("envelope not ok: %+v", env)
	}
	if s.userGETs < 1 || s.searchPOSTs != 1 || s.mrsGETs != 1 {
		t.Fatalf("GETs user=%d search=%d mrs=%d", s.userGETs, s.searchPOSTs, s.mrsGETs)
	}
	wi, _ := data["workitems"].(map[string]any)
	mrs, _ := data["mrs"].(map[string]any)
	pg, _ := data["pending_gates"].(map[string]any)
	if wi == nil || wi["ok"] != true || wi["count"] != float64(1) {
		t.Fatalf("workitems: %#v", wi)
	}
	if mrs == nil || mrs["ok"] != true || mrs["count"] != float64(2) {
		t.Fatalf("mrs: %#v", mrs)
	}
	items, _ := mrs["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("mrs items: %#v", items)
	}
	m0, _ := items[0].(map[string]any)
	if m0["status"] != "UNDER_DEV" {
		t.Fatalf("expected status inject: %#v", m0)
	}
	if pg == nil || pg["skipped"] != true {
		t.Fatalf("pending should be skipped by default: %#v", pg)
	}
	if !strings.Contains(s.lastMrsQ, "state=opened") {
		t.Fatalf("mrs query: %s", s.lastMrsQ)
	}
	if env.Meta["sections_ok"] != true {
		t.Fatalf("meta.sections_ok=%v", env.Meta["sections_ok"])
	}
}

func TestStatusPendingGatesOptional(t *testing.T) {
	s := newStatusServer(t)
	stdout, stderr, code := runStatusCmd(t, false,
		"status", "--skip-workitems", "--skip-mrs", "--pipeline-id", "pipe-1")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	_, data := decodeStatusEnvelope(t, stdout)
	pg, _ := data["pending_gates"].(map[string]any)
	if pg == nil || pg["ok"] != true || pg["skipped"] == true {
		t.Fatalf("pending_gates: %#v", pg)
	}
	if pg["count"] != float64(1) {
		t.Fatalf("pending count: %#v", pg)
	}
	if s.runsGETs < 1 || s.runDetail < 1 {
		t.Fatalf("runs=%d detail=%d", s.runsGETs, s.runDetail)
	}
}

func TestStatusSoftFailMrs(t *testing.T) {
	s := newStatusServer(t)
	s.failMrs = true
	stdout, stderr, code := runStatusCmd(t, false, "status", "--space-id", "space-status")
	if code != 1 {
		t.Fatalf("want exit 1, got %d stderr=%s stdout=%s", code, stderr, stdout)
	}
	env, data := decodeStatusEnvelope(t, stdout)
	if !env.OK {
		// Success still prints before ExitError; ok may be true with sections_ok false.
	}
	wi, _ := data["workitems"].(map[string]any)
	mrs, _ := data["mrs"].(map[string]any)
	if wi == nil || wi["ok"] != true {
		t.Fatalf("workitems should still succeed: %#v", wi)
	}
	if mrs == nil || mrs["ok"] != false {
		t.Fatalf("mrs should fail soft: %#v", mrs)
	}
	if env.Meta["sections_ok"] != false {
		t.Fatalf("sections_ok=%v", env.Meta["sections_ok"])
	}
}

func TestStatusDryRunPreviews(t *testing.T) {
	_ = newStatusServer(t)
	stdout, stderr, code := runStatusCmd(t, true,
		"status", "--space-id", "space-status", "--pipeline-id", "pipe-1", "--dry-run")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("JSON: %v / %s", err, stdout)
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("want ok dry-run envelope: %+v / %s", env, stdout)
	}
	req, _ := env.Request.(map[string]any)
	if req == nil || req["action"] != "status" {
		t.Fatalf("dry-run request: %#v stdout=%s", env.Request, stdout)
	}
	previews, _ := req["previews"].([]any)
	if len(previews) != 3 {
		t.Fatalf("want 3 previews, got %#v", previews)
	}
	sections := map[string]bool{}
	for _, p := range previews {
		m, _ := p.(map[string]any)
		sec, _ := m["section"].(string)
		sections[sec] = true
	}
	for _, want := range []string{"workitems", "mrs", "pending_gates"} {
		if !sections[want] {
			t.Fatalf("missing preview section %s in %#v", want, previews)
		}
	}
}

func TestStatusHelpMentionsGap(t *testing.T) {
	if !strings.Contains(statusCmd.Long, "yx status") {
		t.Fatalf("Long should mention yx status gap: %s", statusCmd.Long)
	}
	if !strings.Contains(statusCmd.Long, "pending_gates") {
		t.Fatalf("Long should describe pending_gates: %s", statusCmd.Long)
	}
}
