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
	"github.com/yunxiao-cli/yunxiao/internal/profile"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// #123 fixture server: one bug workitem whose status is server-tracked so the
// post-transition refresh GET reflects applied PUTs. PUT failures are configurable
// to simulate platform rejection.
type bugTransitionServer struct {
	mu        sync.Mutex
	status    string // current status id (mutated by successful PUTs)
	putStatus int    // non-zero: every PUT answers with this HTTP status
	puts      []map[string]any
	gets      int
}

func newBugTransitionServer(t *testing.T, startStatus string) *bugTransitionServer {
	t.Helper()
	s := &bugTransitionServer{status: startStatus}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/workitems/"):
			s.gets++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "wi-5000",
				"serialNumber":   "ZYPT-5000",
				"spaceId":        "space-123",
				"categoryId":     "Bug",
				"workitemTypeId": "type-bug-1",
				"status":         map[string]any{"id": s.status, "displayName": s.status},
			})
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/workitems/"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			s.puts = append(s.puts, body)
			w.Header().Set("Content-Type", "application/json")
			if s.putStatus != 0 {
				w.WriteHeader(s.putStatus)
				_, _ = w.Write([]byte(`{"errorCode":"InvalidFlowError","errorMessage":"不能流转到目标状态"}`))
				return
			}
			if st, ok := body["status"].(string); ok && st != "" {
				s.status = st
			}
			_, _ = w.Write([]byte(`{"id":"wi-5000"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"unexpected ` + r.Method + ` ` + r.URL.Path + `"}`))
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-bug-transition-123-not-real")
	t.Setenv(config.EnvOrganizationID, "org-123")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "t123")
	return s
}

// setupBugTransitionEnv123 points XDG_CONFIG_HOME at a fresh temp dir and returns
// it; the t123 profile must be written there.
func setupBugTransitionEnv123(t *testing.T) string {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	return xdg
}

// writeBugTransitionProfile123 installs profile t123 with the template graph
// confirm → processing → testing → closed-fixed (closed-fixed has no outgoing
// edges, so closed-fixed → processing has no BFS route).
func writeBugTransitionProfile123(t *testing.T, xdg string, required map[string][]string) {
	t.Helper()
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           "t123",
		OrganizationID: "org-123",
		SpaceID:        "space-123",
		BugTypeID:      "type-bug-1",
		BugStatuses: map[string]string{
			"confirm":      "st-confirm",
			"processing":   "st-processing",
			"testing":      "st-testing",
			"closed-fixed": "st-closed-fixed",
		},
		BugEdges: map[string][]string{
			"st-confirm":      {"st-processing"},
			"st-processing":   {"st-testing"},
			"st-testing":      {"st-closed-fixed"},
			"st-closed-fixed": {},
		},
		BugFields:             map[string]string{"developer": "f-dev"},
		BugTransitionRequired: required,
	})
}

// runBugTransition123 runs `workitem +bug-transition` and returns stdout, stderr and
// the processExit code (0 when the command did not exit).
func runBugTransition123(t *testing.T, dryRun bool, args ...string) (string, string, int) {
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

	resetStringFlags(t, workitemBugTransitionCmd, "id", "to", "plan-due-date", "developer",
		"responsible-person", "bug-reason", "bug-impact-scope", "direct")
	globalProfile = "t123"
	rootCmd.SetArgs(append([]string{"workitem", "+bug-transition", "--id", "ZYPT-5000"}, args...))
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

// decodeBugTransitionSuccess decodes a success envelope (dry-run or real) and
// returns its top-level payload (request for dry-run, data for real runs).
func decodeBugTransitionSuccess(t *testing.T, stdout, context string) (output.Envelope, map[string]any) {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("%s: stdout JSON: %v / %s", context, err, stdout)
	}
	if !env.OK {
		t.Fatalf("%s: want ok:true envelope, got %+v", context, env)
	}
	raw, _ := json.Marshal(env.Request)
	if env.DryRun {
		// dry-run payload
	} else {
		raw, _ = json.Marshal(env.Data)
	}
	var payload map[string]any
	_ = json.Unmarshal(raw, &payload)
	return env, payload
}

func putStatuses(s *bugTransitionServer) []string {
	out := make([]string, 0, len(s.puts))
	for _, p := range s.puts {
		out = append(out, p["status"].(string))
	}
	return out
}

// Table-driven #123 coverage: BFS fallback, --direct, error cause split, dry-run
// warnings, and the no-fallback input errors (unknown status, missing required).
func TestBugTransitionBFSFallbackTable(t *testing.T) {
	cases := []struct {
		name         string
		startStatus  string
		required     map[string][]string // profile bug_transition_required (nil: none)
		putStatus    int                 // non-zero: platform rejects PUTs
		args         []string            // after --id (always ZYPT-5000)
		wantExit     int
		wantMode     string // expected transition_mode ("" to skip)
		wantPUTs     []string
		wantErrParts []string
		wantErrType  string
		wantErrCode  int
	}{
		{
			name:        "bfs-route-multi-step-regression",
			startStatus: "st-confirm",
			args:        []string{"--to", "testing", "--yes"},
			wantExit:    0,
			wantMode:    zhiyi.TransitionModeProfileBFS,
			wantPUTs:    []string{"st-processing", "st-testing"},
		},
		{
			name:        "no-path-falls-back-to-direct-and-succeeds",
			startStatus: "st-closed-fixed",
			args:        []string{"--to", "processing", "--yes"},
			wantExit:    0,
			wantMode:    zhiyi.TransitionModeBFSNoPathDirect,
			wantPUTs:    []string{"st-processing"},
		},
		{
			name:         "no-path-fallback-rejected-distinguishes-both-causes",
			startStatus:  "st-closed-fixed",
			putStatus:    http.StatusBadRequest,
			args:         []string{"--to", "processing", "--yes"},
			wantExit:     1,
			wantPUTs:     []string{"st-processing"},
			wantErrType:  "api",
			wantErrCode:  http.StatusBadRequest,
			wantErrParts: []string{"no path in profile edges", "platform rejected", "不能流转到目标状态", "+explore-workflow"},
		},
		{
			name:        "direct-flag-skips-bfs-single-put",
			startStatus: "st-confirm",
			args:        []string{"--to", "testing", "--direct", "--yes"},
			wantExit:    0,
			wantMode:    zhiyi.TransitionModeDirectForced,
			wantPUTs:    []string{"st-testing"},
		},
		{
			name:         "direct-flag-rejected",
			startStatus:  "st-confirm",
			putStatus:    http.StatusBadRequest,
			args:         []string{"--to", "testing", "--direct", "--yes"},
			wantExit:     1,
			wantPUTs:     []string{"st-testing"},
			wantErrType:  "api",
			wantErrCode:  http.StatusBadRequest,
			wantErrParts: []string{"--direct", "platform rejected"},
		},
		{
			name:         "graph-backed-step-rejected-is-platform-only",
			startStatus:  "st-confirm",
			putStatus:    http.StatusBadRequest,
			args:         []string{"--to", "testing", "--yes"},
			wantExit:     1,
			wantPUTs:     []string{"st-processing"},
			wantErrType:  "api",
			wantErrCode:  http.StatusBadRequest,
			wantErrParts: []string{"第 1/2 步", "platform rejected"},
		},
		{
			name:        "dry-run-no-path-shows-fallback-plan-and-warning",
			startStatus: "st-closed-fixed",
			args:        []string{"--to", "processing", "--dry-run"},
			wantExit:    0,
			wantMode:    zhiyi.TransitionModeBFSNoPathDirect,
			wantPUTs:    []string{"st-processing"},
		},
		{
			name:        "dry-run-direct-flag",
			startStatus: "st-confirm",
			args:        []string{"--to", "testing", "--direct", "--dry-run"},
			wantExit:    0,
			wantMode:    zhiyi.TransitionModeDirectForced,
			wantPUTs:    []string{"st-testing"},
		},
		{
			name:         "unknown-target-still-errors-no-fallback",
			startStatus:  "st-confirm",
			args:         []string{"--to", "nonexistent", "--yes"},
			wantExit:     1,
			wantErrType:  "cli",
			wantErrParts: []string{"不在缺陷状态机中"},
		},
		{
			name:         "fallback-still-enforces-target-required-fields",
			startStatus:  "st-closed-fixed",
			required:     map[string][]string{"st-processing": {"f-dev"}},
			args:         []string{"--to", "processing", "--yes"},
			wantExit:     1,
			wantPUTs:     []string{},
			wantErrParts: []string{"云效要求必填"},
		},
		{
			name:        "fallback-required-fields-satisfied-by-flags",
			startStatus: "st-closed-fixed",
			required:    map[string][]string{"st-processing": {"f-dev"}},
			args:        []string{"--to", "processing", "--developer", "u-1", "--yes"},
			wantExit:    0,
			wantMode:    zhiyi.TransitionModeBFSNoPathDirect,
			wantPUTs:    []string{"st-processing"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeBugTransitionProfile123(t, setupBugTransitionEnv123(t), tc.required)
			s := newBugTransitionServer(t, tc.startStatus)
			s.putStatus = tc.putStatus

			dryRun := false
			for _, a := range tc.args {
				if a == "--dry-run" {
					dryRun = true
				}
			}
			stdout, stderr, code := runBugTransition123(t, dryRun, tc.args...)

			if code != tc.wantExit {
				t.Fatalf("exit=%d want %d; stdout=%s stderr=%s", code, tc.wantExit, stdout, stderr)
			}
			if tc.wantExit != 0 {
				eb := decodeErrorBody(t, stderr)
				if tc.wantErrType != "" && eb.Type != tc.wantErrType {
					t.Fatalf("error type=%q want %q: %+v", eb.Type, tc.wantErrType, eb)
				}
				if tc.wantErrCode != 0 && eb.Code != tc.wantErrCode {
					t.Fatalf("error code=%d want %d: %+v", eb.Code, tc.wantErrCode, eb)
				}
				for _, want := range tc.wantErrParts {
					if !strings.Contains(eb.Message, want) && !strings.Contains(eb.Hint, want) {
						t.Fatalf("error missing %q: message=%q hint=%q", want, eb.Message, eb.Hint)
					}
				}
				if tc.name == "graph-backed-step-rejected-is-platform-only" && strings.Contains(eb.Message, "no path") {
					t.Fatalf("graph-backed rejection must not claim no-path: %q", eb.Message)
				}
				if len(s.puts) != len(tc.wantPUTs) {
					t.Fatalf("PUTs=%v want %v", putStatuses(s), tc.wantPUTs)
				}
				return
			}

			env, payload := decodeBugTransitionSuccess(t, stdout, tc.name)
			if !env.DryRun && tc.wantMode != "" {
				if env.Meta["transition_mode"] != tc.wantMode {
					t.Fatalf("meta.transition_mode=%v want %q", env.Meta["transition_mode"], tc.wantMode)
				}
				if payload["transition_mode"] != tc.wantMode {
					t.Fatalf("data.transition_mode=%v want %q", payload["transition_mode"], tc.wantMode)
				}
			}
			if env.DryRun {
				if !env.DryRun || env.Risk != "write" {
					t.Fatalf("dry-run envelope=%+v", env)
				}
				steps, _ := payload["steps"].([]any)
				if len(steps) != len(tc.wantPUTs) {
					t.Fatalf("dry-run steps=%v want %v", payload["steps"], tc.wantPUTs)
				}
				for i, want := range tc.wantPUTs {
					if steps[i] != want {
						t.Fatalf("dry-run steps=%v want %v", payload["steps"], tc.wantPUTs)
					}
				}
				warning, _ := payload["warning"].(string)
				switch tc.wantMode {
				case zhiyi.TransitionModeBFSNoPathDirect:
					if !strings.Contains(warning, "no path in profile edges") || !strings.Contains(warning, "+explore-workflow") {
						t.Fatalf("dry-run warning=%q", warning)
					}
				case zhiyi.TransitionModeDirectForced:
					if !strings.Contains(warning, "--direct") {
						t.Fatalf("dry-run warning=%q", warning)
					}
				}
			}
			if env.DryRun && len(s.puts) != 0 {
				t.Fatalf("dry-run must not PUT: %v", putStatuses(s))
			}
			if !env.DryRun {
				got := putStatuses(s)
				if len(got) != len(tc.wantPUTs) {
					t.Fatalf("PUTs=%v want %v", got, tc.wantPUTs)
				}
				for i := range got {
					if got[i] != tc.wantPUTs[i] {
						t.Fatalf("PUTs=%v want %v", got, tc.wantPUTs)
					}
				}
				applied, _ := payload["applied"].([]any)
				if len(applied) != len(tc.wantPUTs) {
					t.Fatalf("applied=%v want %v", payload["applied"], tc.wantPUTs)
				}
				if payload["refreshed_status"] != tc.wantPUTs[len(tc.wantPUTs)-1] {
					t.Fatalf("refreshed_status=%v want %v", payload["refreshed_status"], tc.wantPUTs[len(tc.wantPUTs)-1])
				}
			}
		})
	}
}

// The fallback error must render as type "api" with the platform status code while
// carrying the no-path context (#123 cause split); dry-run must never PUT.
func TestBugTransitionBFSFallbackPlatformRejectEnvelope(t *testing.T) {
	writeBugTransitionProfile123(t, setupBugTransitionEnv123(t), nil)
	s := newBugTransitionServer(t, "st-closed-fixed")
	s.putStatus = http.StatusBadRequest

	_, stderr, code := runBugTransition123(t, false, "--to", "processing", "--yes")
	if code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if eb.Type != "api" || eb.Code != http.StatusBadRequest {
		t.Fatalf("error body=%+v", eb)
	}
	for _, want := range []string{"no path in profile edges", "platform rejected"} {
		if !strings.Contains(eb.Message, want) {
			t.Fatalf("message=%q missing %q", eb.Message, want)
		}
	}
	if !strings.Contains(eb.Message, "HTTP 400") && !strings.Contains(eb.Message, "不能流转到目标状态") {
		t.Fatalf("message=%q missing platform detail", eb.Message)
	}
	if !strings.Contains(eb.Hint, "+explore-workflow") {
		t.Fatalf("hint=%q", eb.Hint)
	}
	if len(s.puts) != 1 {
		t.Fatalf("exactly one fallback PUT expected, got %v", putStatuses(s))
	}
}

// Help documents that edges are unverified template assumptions and where the
// fallback is visible (#123 item 3).
func TestBugTransitionHelpDocumentsFallback(t *testing.T) {
	long := workitemBugTransitionCmd.Long
	for _, want := range []string{"UNVERIFIED", "bfs_no_path_direct", "--direct", "no path in profile edges", "platform rejected", "+explore-workflow"} {
		if !strings.Contains(long, want) {
			t.Fatalf("Long help missing %q:\n%s", want, long)
		}
	}
	if f := workitemBugTransitionCmd.Flags().Lookup("direct"); f == nil {
		t.Fatal("--direct flag must exist")
	}
}
