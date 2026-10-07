package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// mergeSrv doubles the Codeup MR endpoints: GET .../changeRequests/9 returns a
// per-case detail payload (detailStatus), POST .../changeRequests/9/merge
// records the body and answers {"result":"merged"}.
type mergeSrv struct {
	mu         sync.Mutex
	detailJSON string
	detailCode int // non-0 → respond with this status instead
	gets       int
	posts      []map[string]any
}

func (s *mergeSrv) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/changeRequests/9"):
		s.gets++
		if s.detailCode != 0 {
			w.WriteHeader(s.detailCode)
			_, _ = io.WriteString(w, `{"errorMessage":"mr detail gone"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, s.detailJSON)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/changeRequests/9/merge"):
		var m map[string]any
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &m)
		s.posts = append(s.posts, m)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"result":true}`)
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"errorMessage":"unexpected `+r.Method+" "+r.URL.Path+`"}`)
	}
}

func (s *mergeSrv) snapshot() (int, []map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]map[string]any, len(s.posts))
	copy(out, s.posts)
	return s.gets, out
}

// mergeDetail builds an MR detail payload with the given overrides.
func mergeDetail(overrides map[string]any) string {
	m := map[string]any{
		"localId":             float64(9),
		"title":               "feat: x",
		"status":              "TO_BE_MERGED",
		"mergeable":           true,
		"conflictCheckStatus": "NO_CONFLICT",
	}
	for k, v := range overrides {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// runMrsMerge executes `codeup mrs merge --repo 123 --local-id 9 --merge-type <t>`
// with dryRun/yes and returns the envelopes plus server counters.
func runMrsMerge(t *testing.T, s *mergeSrv, dryRun, yes bool, mergeType string, extra ...string) (output.Envelope, output.Envelope, int) {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-mrs-merge-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-merge")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer
	prevOut, prevErr, prevJQ, prevFmt := output.Stdout, output.Stderr, output.JQ, output.Format
	output.Stdout = &stdout
	output.Stderr = &stderr
	output.JQ = ""
	output.Format = "json"
	prevGlobals := []any{globalYes, globalDryRun, globalProfile, globalOrg}
	globalYes = yes
	globalDryRun = dryRun
	globalProfile = ""
	globalOrg = ""
	t.Cleanup(func() {
		output.Stdout = prevOut
		output.Stderr = prevErr
		output.JQ = prevJQ
		output.Format = prevFmt
		globalYes, globalDryRun, globalProfile, globalOrg = prevGlobals[0].(bool), prevGlobals[1].(bool), prevGlobals[2].(string), prevGlobals[3].(string)
	})
	prevExit := processExit
	code := 0
	processExit = func(c int) { code = c; panic(exitPanic{code}) }
	t.Cleanup(func() { processExit = prevExit })

	resetStringFlags(t, codeupMrsMergeCmd, "repo", "local-id", "merge-type", "message", "remove-source-branch")
	args := append([]string{"codeup", "mrs", "merge", "--repo", "123", "--local-id", "9", "--merge-type", mergeType}, extra...)
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
	var env, eenv output.Envelope
	_ = json.Unmarshal([]byte(stdout.String()), &env)
	_ = json.Unmarshal([]byte(stderr.String()), &eenv)
	return env, eenv, code
}

// TestMrsMergePrecheck (#130): merge-method / status precheck before the POST.
func TestMrsMergePrecheck(t *testing.T) {
	cases := []struct {
		name        string
		detail      string
		detailCode  int
		mergeType   string
		dryRun      bool
		yes         bool
		wantOK      bool
		wantCode    int
		wantSubtype string // error.subtype; "" skips the check
		wantMsgHas  []string
		wantGets    int
		wantPosts   int
	}{
		{
			name:       "pass with --yes: precheck then merge",
			detail:     mergeDetail(map[string]any{"mergeTypes": []any{"ff-only", "no-fast-forward"}}),
			mergeType:  "no-fast-forward",
			yes:        true,
			wantOK:     true,
			wantCode:   0,
			wantMsgHas: []string{`"mergeType":"no-fast-forward"`},
			wantGets:   1,
			wantPosts:  1,
		},
		{
			name:      "no availability info still merges",
			detail:    mergeDetail(nil),
			mergeType: "rebase",
			yes:       true,
			wantOK:    true,
			wantCode:  0,
			wantGets:  1,
			wantPosts: 1,
		},
		{
			name:        "unsupported rebase reported locally, no POST",
			detail:      mergeDetail(map[string]any{"mergeTypes": []any{"Fast-forward-only", "创建合并节点"}}),
			mergeType:   "rebase",
			yes:         true,
			wantOK:      false,
			wantCode:    1,
			wantSubtype: "merge_type_not_supported",
			wantMsgHas:  []string{"does not support merge-type rebase", "available: ff-only, no-fast-forward"},
			wantGets:    1,
			wantPosts:   0,
		},
		{
			name:        "ff-only blocked via supportMergeFastForwardOnly=false",
			detail:      mergeDetail(map[string]any{"supportMergeFastForwardOnly": false}),
			mergeType:   "ff-only",
			yes:         true,
			wantOK:      false,
			wantCode:    1,
			wantSubtype: "merge_type_not_supported",
			wantMsgHas:  []string{"no-fast-forward, squash, rebase"},
			wantGets:    1,
			wantPosts:   0,
		},
		{
			name:        "already merged",
			detail:      mergeDetail(map[string]any{"status": "MERGED"}),
			mergeType:   "squash",
			yes:         true,
			wantOK:      false,
			wantCode:    1,
			wantSubtype: "mr_already_merged",
			wantGets:    1,
			wantPosts:   0,
		},
		{
			name:        "closed MR points at reopen",
			detail:      mergeDetail(map[string]any{"status": "CLOSED"}),
			mergeType:   "squash",
			yes:         true,
			wantOK:      false,
			wantCode:    1,
			wantSubtype: "mr_closed",
			wantMsgHas:  []string{"mrs reopen"},
			wantGets:    1,
			wantPosts:   0,
		},
		{
			name:        "conflict blocks merge",
			detail:      mergeDetail(map[string]any{"conflictCheckStatus": "HAS_CONFLICT", "mergeable": false}),
			mergeType:   "squash",
			yes:         true,
			wantOK:      false,
			wantCode:    1,
			wantSubtype: "mr_conflict",
			wantGets:    1,
			wantPosts:   0,
		},
		{
			name:        "conflict check still running",
			detail:      mergeDetail(map[string]any{"conflictCheckStatus": "CHECKING"}),
			mergeType:   "squash",
			yes:         true,
			wantOK:      false,
			wantCode:    1,
			wantSubtype: "mr_conflict_checking",
			wantGets:    1,
			wantPosts:   0,
		},
		{
			name:        "mergeable=false blocks merge",
			detail:      mergeDetail(map[string]any{"mergeable": false}),
			mergeType:   "squash",
			yes:         true,
			wantOK:      false,
			wantCode:    1,
			wantSubtype: "mr_not_mergeable",
			wantGets:    1,
			wantPosts:   0,
		},
		{
			name:      "dry-run shows passing precheck, sends nothing",
			detail:    mergeDetail(map[string]any{"mergeTypes": []any{"squash"}}),
			mergeType: "squash",
			dryRun:    true,
			wantOK:    true,
			wantCode:  0,
			wantGets:  1,
			wantPosts: 0,
		},
		{
			name:        "dry-run surfaces failed precheck too",
			detail:      mergeDetail(map[string]any{"mergeTypes": []any{"ff-only"}}),
			mergeType:   "rebase",
			dryRun:      true,
			wantOK:      false,
			wantCode:    1,
			wantSubtype: "merge_type_not_supported",
			wantGets:    1,
			wantPosts:   0,
		},
		{
			name:      "gate without --yes still exits 10 before any request",
			detail:    mergeDetail(nil),
			mergeType: "squash",
			wantOK:    false,
			wantCode:  10,
			wantGets:  0,
			wantPosts: 0,
		},
		{
			name:        "unreadable detail fails closed",
			detailCode:  404,
			mergeType:   "squash",
			yes:         true,
			wantOK:      false,
			wantCode:    1,
			wantSubtype: "",
			wantMsgHas:  []string{"merge precheck"},
			wantGets:    1,
			wantPosts:   0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &mergeSrv{detailJSON: tc.detail, detailCode: tc.detailCode}
			env, eenv, code := runMrsMerge(t, s, tc.dryRun, tc.yes, tc.mergeType)
			if code != tc.wantCode {
				t.Fatalf("code=%d want %d (stdout ok=%v)", code, tc.wantCode, env.OK)
			}
			if tc.wantOK {
				if !env.OK {
					t.Fatalf("want ok envelope, got error envelope")
				}
				pre := mergePrecheckMap(t, env)
				if pre["status"] != "ok" {
					t.Fatalf("precheck=%#v", pre)
				}
				if s.snapshotPostsLen() != tc.wantPosts {
					t.Fatalf("posts=%d want %d", s.snapshotPostsLen(), tc.wantPosts)
				}
				return
			}
			// Failure path: parse the error envelope from stderr.
			eb := eenv.Error
			if eb == nil {
				t.Fatalf("no error envelope (eenv=%#v)", eenv)
			}
			if tc.wantSubtype != "" && eb.Subtype != tc.wantSubtype {
				t.Fatalf("subtype=%q want %q: %#v", eb.Subtype, tc.wantSubtype, eb)
			}
			for _, want := range tc.wantMsgHas {
				if !strings.Contains(eb.Message, want) && !strings.Contains(eb.Hint, want) {
					t.Fatalf("message/hint missing %q: msg=%q hint=%q", want, eb.Message, eb.Hint)
				}
			}
			if s.snapshotPostsLen() != tc.wantPosts {
				t.Fatalf("posts=%d want %d (must not POST on failed precheck)", s.snapshotPostsLen(), tc.wantPosts)
			}
		})
	}
}

func (s *mergeSrv) snapshotPostsLen() int {
	_, posts := s.snapshot()
	return len(posts)
}

// mergePrecheckMap extracts meta.precheck (success) or request.precheck (dry-run).
func mergePrecheckMap(t *testing.T, env output.Envelope) map[string]any {
	t.Helper()
	var holder map[string]any
	if env.DryRun {
		raw, _ := json.Marshal(env.Request)
		var req map[string]any
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Fatalf("request: %v", err)
		}
		holder, _ = req["precheck"].(map[string]any)
	} else if env.Meta != nil {
		holder, _ = env.Meta["precheck"].(map[string]any)
	}
	if holder == nil {
		t.Fatalf("precheck map missing: %#v", env)
	}
	return holder
}

// TestMrsMergePrecheckDryRunRequestShape: dry-run preview carries both the
// request and the precheck outcome in one envelope.
func TestMrsMergePrecheckDryRunRequestShape(t *testing.T) {
	s := &mergeSrv{detailJSON: mergeDetail(map[string]any{"mergeTypes": []any{"squash", "rebase"}})}
	env, _, code := runMrsMerge(t, s, true, false, "rebase")
	if code != 0 || !env.OK || !env.DryRun {
		t.Fatalf("code=%d env=%#v", code, env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	if req["method"] != "POST" || !strings.Contains(fmt.Sprint(req["url"]), "/merge") {
		t.Fatalf("request=%#v", req)
	}
	pre := mergePrecheckMap(t, env)
	if pre["mr_status"] != "TO_BE_MERGED" || pre["merge_type"] != "rebase" {
		t.Fatalf("precheck=%#v", pre)
	}
	if gets, _ := s.snapshot(); gets != 1 {
		t.Fatalf("detail GETs=%d want 1", gets)
	}
}
