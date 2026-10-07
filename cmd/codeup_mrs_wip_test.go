package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// mrsWipServer backs `codeup mrs update --wip/--unwip` (#97): it serves the MR
// with a mutable title, records PUT bodies, and answers the numeric-repo
// ownership GET.
type mrsWipServer struct {
	mu       sync.Mutex
	title    string
	mrStatus int // non-zero: the MR GET answers with this HTTP status
	mrGETs   int
	puts     []map[string]any
}

func newMrsWipServer(t *testing.T, title string) *mrsWipServer {
	t.Helper()
	s := &mrsWipServer{title: title}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/repositories/4951320") :
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "4951320", "name": "demo"})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/changeRequests/125"):
			s.mrGETs++
			w.Header().Set("Content-Type", "application/json")
			if s.mrStatus != 0 {
				w.WriteHeader(s.mrStatus)
				_, _ = w.Write([]byte(`{"errorCode":"AccessDenied","errorMessage":"no mr"}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"localId": 125, "title": s.title, "status": "OPEN",
				"sourceBranch": "feat/x", "targetBranch": "master", "projectId": "4951320",
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/changeRequests/125"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			s.puts = append(s.puts, body)
			if t, ok := body["title"].(string); ok {
				s.title = t
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"localId": 125, "title": s.title, "status": "OPEN",
				"sourceBranch": "feat/x", "targetBranch": "master", "projectId": "4951320",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"unexpected ` + r.Method + ` ` + r.URL.Path + `"}`))
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-mrs-update-wip-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-update-wip-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

// runMrsUpdateWip runs `codeup mrs update` with processExit intercepted and
// returns raw stdout, stderr and the exit code (0 when the command did not exit).
func runMrsUpdateWip(t *testing.T, dryRun bool, extra ...string) (string, string, int) {
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

	resetStringFlags(t, codeupMrsUpdateCmd, "repo", "local-id", "title", "description", "work-item", "full", "wip", "unwip")
	// Later tests reuse this shared command and only reset the flags they know;
	// never leak --wip/--unwip into them.
	t.Cleanup(func() {
		resetStringFlags(t, codeupMrsUpdateCmd, "title", "description", "work-item", "wip", "unwip")
	})
	args := append([]string{"codeup", "mrs", "update", "--repo", "4951320", "--local-id", "125"}, extra...)
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

func decodeMrsUpdateEnvelope(t *testing.T, raw string) output.Envelope {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("JSON envelope: %v / %s", err, raw)
	}
	return env
}

// TestMrsUpdateWipToggle covers #97 real-run behavior end to end: prefix add/strip
// PUT bodies, idempotent no-ops skip the PUT entirely, meta.wip_* keys.
func TestMrsUpdateWipToggle(t *testing.T) {
	cases := []struct {
		name        string
		title       string
		args        []string
		wantPUTKeys map[string]any // expected sole PUT body; nil = no PUT expected
		wantAction  string
		wantChanged bool
		wantNoop    bool // no-op: ok:true, no PUT, data keeps current title
	}{
		{
			name: "unwip strips WIP: prefix", title: "WIP: xxx", args: []string{"--unwip"},
			wantPUTKeys: map[string]any{"title": "xxx"}, wantAction: "unwip", wantChanged: true,
		},
		{
			name: "unwip strips lowercase wip without colon", title: "wip xxx", args: []string{"--unwip"},
			wantPUTKeys: map[string]any{"title": "xxx"}, wantAction: "unwip", wantChanged: true,
		},
		{
			name: "unwip strips compact Wip:x", title: "Wip:x", args: []string{"--unwip"},
			wantPUTKeys: map[string]any{"title": "x"}, wantAction: "unwip", wantChanged: true,
		},
		{
			name: "unwip on prefix-less title is a no-op", title: "xxx", args: []string{"--unwip"},
			wantAction: "unwip", wantNoop: true,
		},
		{
			name: "unwip keeps WIPfix (no separator)", title: "WIPfix bug", args: []string{"--unwip"},
			wantAction: "unwip", wantNoop: true,
		},
		{
			name: "wip adds WIP: prefix", title: "xxx", args: []string{"--wip"},
			wantPUTKeys: map[string]any{"title": "WIP: xxx"}, wantAction: "wip", wantChanged: true,
		},
		{
			name: "wip is idempotent on prefixed title", title: "WIP: xxx", args: []string{"--wip"},
			wantAction: "wip", wantNoop: true,
		},
		{
			name: "wip combined with description PUTs both", title: "xxx", args: []string{"--wip", "--description", "d"},
			wantPUTKeys: map[string]any{"title": "WIP: xxx", "description": "d"}, wantAction: "wip", wantChanged: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newMrsWipServer(t, tc.title)
			stdout, stderr, code := runMrsUpdateWip(t, false, tc.args...)
			if code != 0 {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			env := decodeMrsUpdateEnvelope(t, stdout)
			if !env.OK || env.DryRun {
				t.Fatalf("envelope: %+v", env)
			}
			if env.Meta["wip_action"] != tc.wantAction {
				t.Fatalf("meta.wip_action=%#v want %q: %#v", env.Meta["wip_action"], tc.wantAction, env.Meta)
			}
			changed, _ := env.Meta["wip_changed"].(bool)
			if changed != tc.wantChanged {
				t.Fatalf("meta.wip_changed=%#v want %v: %#v", env.Meta["wip_changed"], tc.wantChanged, env.Meta)
			}
			if tc.wantNoop {
				if len(s.puts) != 0 {
					t.Fatalf("idempotent no-op must not PUT: %#v", s.puts)
				}
				if s.mrGETs != 1 {
					t.Fatalf("mrGETs=%d want 1", s.mrGETs)
				}
				data, _ := env.Data.(map[string]any)
				if data == nil || data["title"] != tc.title {
					t.Fatalf("no-op data must keep the current title: %#v", env.Data)
				}
				return
			}
			if len(s.puts) != 1 {
				t.Fatalf("puts=%#v mrGETs=%d", s.puts, s.mrGETs)
			}
			if s.mrGETs != 1 {
				t.Fatalf("mrGETs=%d want 1 (title fetch)", s.mrGETs)
			}
			got := s.puts[0]
			if len(got) != len(tc.wantPUTKeys) {
				t.Fatalf("PUT body=%#v want keys %v", got, tc.wantPUTKeys)
			}
			for k, v := range tc.wantPUTKeys {
				if got[k] != v {
					t.Fatalf("PUT body=%#v want %s=%v", got, k, v)
				}
			}
			data, _ := env.Data.(map[string]any)
			if data == nil || data["title"] != tc.wantPUTKeys["title"] {
				t.Fatalf("brief data title=%#v want %v", env.Data, tc.wantPUTKeys["title"])
			}
		})
	}
}

// Dry-run (#97): the title-resolving GET still runs, nothing is PUT, the preview
// body carries the resolved title and request.resolved shows before/after.
func TestMrsUpdateUnwipDryRun(t *testing.T) {
	s := newMrsWipServer(t, "WIP: xxx")
	stdout, stderr, code := runMrsUpdateWip(t, true, "--unwip", "--dry-run")
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	env := decodeMrsUpdateEnvelope(t, stdout)
	if !env.OK || !env.DryRun {
		t.Fatalf("envelope: %+v", env)
	}
	req := decodeDryRunRequest(t, stdout)
	body, _ := req["body"].(map[string]any)
	if body["title"] != "xxx" {
		t.Fatalf("dry-run body=%#v", body)
	}
	resolved, _ := req["resolved"].(map[string]any)
	if resolved["before"] != "WIP: xxx" || resolved["after"] != "xxx" || resolved["changed"] != true || resolved["action"] != "unwip" {
		t.Fatalf("resolved=%#v", resolved)
	}
	if s.mrGETs != 1 || len(s.puts) != 0 {
		t.Fatalf("mrGETs=%d puts=%#v (dry-run must fetch but not write)", s.mrGETs, s.puts)
	}
}

// Flag combination contract (#97): --wip/--unwip reject --title and each other,
// and a flag-less update keeps the "nothing to do" error (now mentioning the
// toggles). None of these may reach the network with a PUT.
func TestMrsUpdateWipFlagConflicts(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantInErr []string
	}{
		{name: "wip with title", args: []string{"--wip", "--title", "t"}, wantInErr: []string{"--wip", "--title"}},
		{name: "unwip with title", args: []string{"--unwip", "--title", "t"}, wantInErr: []string{"--unwip", "--title"}},
		{name: "wip with unwip", args: []string{"--wip", "--unwip"}, wantInErr: []string{"mutually exclusive"}},
		{name: "nothing to do", args: nil, wantInErr: []string{"--title", "--description", "--work-item", "--wip"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newMrsWipServer(t, "WIP: xxx")
			stdout, stderr, code := runMrsUpdateWip(t, true, tc.args...)
			if code != 1 {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			for _, want := range tc.wantInErr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("stderr missing %q: %s", want, stderr)
				}
			}
			if len(s.puts) != 0 {
				t.Fatalf("validation errors must not PUT: %#v", s.puts)
			}
		})
	}
}

// A failing title fetch surfaces as a wrapped context error and writes nothing.
func TestMrsUpdateWipFetchFails(t *testing.T) {
	s := newMrsWipServer(t, "WIP: xxx")
	s.mrStatus = http.StatusForbidden
	_, stderr, code := runMrsUpdateWip(t, false, "--unwip")
	if code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "fetch MR 125 title for --wip/--unwip") {
		t.Fatalf("stderr missing fetch context: %s", stderr)
	}
	if len(s.puts) != 0 {
		t.Fatalf("must not PUT after a failed fetch: %#v", s.puts)
	}
}

// Help contract: semantics, idempotent no-op behavior and mutual exclusion are
// all declared in --help.
func TestMrsUpdateWipHelp(t *testing.T) {
	long := codeupMrsUpdateCmd.Long
	for _, want := range []string{"--wip/--unwip", "idempotent", "Mutually exclusive", "meta.wip_action", "no PUT is", "WIPfix"} {
		if !strings.Contains(long, want) {
			t.Fatalf("mrs update help missing %q:\n%s", want, long)
		}
	}
	for _, f := range []string{"wip", "unwip"} {
		if codeupMrsUpdateCmd.Flags().Lookup(f) == nil {
			t.Fatalf("--%s flag not registered", f)
		}
	}
}

// Prefix helpers (#97): case-insensitive, colon optional, separator required.
func TestApplyWipToggle(t *testing.T) {
	cases := []struct {
		name    string
		title   string
		wip     bool
		want    string
		changed bool
	}{
		{name: "wip plain", title: "xxx", wip: true, want: "WIP: xxx", changed: true},
		{name: "wip already prefixed", title: "WIP: xxx", wip: true, want: "WIP: xxx"},
		{name: "wip lowercase prefix", title: "wip: xxx", wip: true, want: "wip: xxx"},
		{name: "wip trims leading spaces", title: "  xxx", wip: true, want: "WIP: xxx", changed: true},
		{name: "unwip colon", title: "WIP: xxx", want: "xxx", changed: true},
		{name: "unwip colon no space", title: "WIP:xxx", want: "xxx", changed: true},
		{name: "unwip no colon", title: "WIP xxx", want: "xxx", changed: true},
		{name: "unwip mixed case", title: "wIp xxx", want: "xxx", changed: true},
		{name: "unwip leading spaces", title: "  WIP: xxx", want: "xxx", changed: true},
		{name: "unwip absent", title: "xxx", want: "xxx"},
		{name: "unwip glued word is not a prefix", title: "WIPfix bug", want: "WIPfix bug"},
		{name: "unwip prefix-only title stays", title: "WIP:", want: "WIP:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := applyWipToggle(tc.title, tc.wip)
			if got != tc.want || changed != tc.changed {
				t.Fatalf("applyWipToggle(%q,%v)=(%q,%v) want (%q,%v)", tc.title, tc.wip, got, changed, tc.want, tc.changed)
			}
		})
	}
}
