package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// #96 fixtures: three MRs across two source branches and two target branches.
var mrsListBranchFixture = []any{
	map[string]any{"localId": 1, "title": "feat: a", "sourceBranch": "feat/x", "targetBranch": "master", "status": "OPEN"},
	map[string]any{"localId": 2, "title": "feat: b", "sourceBranch": "feat/y", "targetBranch": "master", "status": "OPEN"},
	map[string]any{"localId": 3, "title": "fix: c", "sourceBranch": "feat/x", "targetBranch": "develop", "status": "MERGED"},
}

type mrsListServer struct {
	mu       sync.Mutex
	queries  []url.Values
	perPage  int // >0: serve ceil(len/perPage) pages driven by the page param (--all)
	fixtures []any
}

// newMrsListServer serves GET .../changeRequests with mrsListBranchFixture and
// records every query string it saw.
func newMrsListServer(t *testing.T) *mrsListServer {
	t.Helper()
	s := &mrsListServer{fixtures: mrsListBranchFixture}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/changeRequests") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		s.mu.Lock()
		s.queries = append(s.queries, r.URL.Query())
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		items := s.fixtures
		if s.perPage > 0 {
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page < 1 {
				page = 1
			}
			start := (page - 1) * s.perPage
			if start > len(items) {
				start = len(items)
			}
			end := start + s.perPage
			if end > len(items) {
				end = len(items)
			}
			items = items[start:end]
			// x-page/x-per-page/x-total drive inferHasMore → ListAll keeps paging.
			w.Header().Set("x-page", strconv.Itoa(page))
			w.Header().Set("x-per-page", strconv.Itoa(s.perPage))
			w.Header().Set("x-total", strconv.Itoa(len(s.fixtures)))
		}
		_ = json.NewEncoder(w).Encode(items)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-mrs-list-filter-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-list-filter-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

// runMrsList runs `codeup mrs list` and returns the stdout envelope plus exit code.
func runMrsList(t *testing.T, extra ...string) (output.Envelope, string, int) {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	prevDry := globalDryRun
	globalDryRun = false
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

	resetStringFlags(t, codeupMrsListCmd, "repo", "search", "state", "source", "target", "sort", "all", "page", "per-page")
	// Later tests reuse this shared command; never leak --source/--target/--all into them.
	t.Cleanup(func() {
		resetStringFlags(t, codeupMrsListCmd, "search", "state", "source", "target", "all", "page", "per-page")
	})
	args := append([]string{"codeup", "mrs", "list"}, extra...)
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
	raw := stdout.String()
	if code != 0 {
		raw = stderr.String()
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("output JSON (exit %d): %v / stdout=%s stderr=%s", code, err, stdout.String(), stderr.String())
	}
	return env, raw, code
}

func mrsItemLocalID(m map[string]any) int {
	switch v := m["localId"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

func mrsListLocalIDs(t *testing.T, env output.Envelope) []int {
	t.Helper()
	items, ok := env.Data.([]any)
	if !ok {
		t.Fatalf("data is not a list: %#v", env.Data)
	}
	ids := make([]int, 0, len(items))
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			t.Fatalf("item not an object: %#v", it)
		}
		ids = append(ids, mrsItemLocalID(m))
	}
	return ids
}

// TestMrsListBranchFilters covers #96: --source/--target filter client-side, the
// branch names never reach the query string, state still passes through, no
// match is an ok empty list, and --all filters across collected pages.
func TestMrsListBranchFilters(t *testing.T) {
	cases := []struct {
		name          string
		args          []string
		wantIDs       []int
		wantFiltered  bool
		wantStateQ    string // expected state query value ("" = not asserted)
		wantPageCount int    // expected GET count; 0 = not asserted
	}{
		{name: "source only", args: []string{"--source", "feat/x"}, wantIDs: []int{1, 3}, wantFiltered: true},
		{name: "target only", args: []string{"--target", "master"}, wantIDs: []int{1, 2}, wantFiltered: true},
		{name: "source and target", args: []string{"--source", "feat/x", "--target", "master"}, wantIDs: []int{1}, wantFiltered: true},
		{name: "with state", args: []string{"--source", "feat/x", "--state", "opened"}, wantIDs: []int{1, 3}, wantFiltered: true, wantStateQ: "opened"},
		{name: "no match is ok empty", args: []string{"--source", "nope/branch"}, wantIDs: []int{}, wantFiltered: true},
		{name: "no filters untouched", args: nil, wantIDs: []int{1, 2, 3}, wantFiltered: false},
		{name: "all pages combined", args: []string{"--source", "feat/x", "--all", "--per-page", "2"}, wantIDs: []int{1, 3}, wantFiltered: true, wantPageCount: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newMrsListServer(t)
			if tc.wantPageCount > 1 {
				s.perPage = 2
			}
			env, raw, code := runMrsList(t, tc.args...)
			if code != 0 || !env.OK {
				t.Fatalf("exit=%d envelope: %s", code, raw)
			}
			got := mrsListLocalIDs(t, env)
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("ids=%v want %v", got, tc.wantIDs)
			}
			for i := range got {
				if got[i] != tc.wantIDs[i] {
					t.Fatalf("ids=%v want %v", got, tc.wantIDs)
				}
			}
			if tc.wantFiltered && env.Meta["filtered_by"] != "client" {
				t.Fatalf("meta.filtered_by=%#v want client: %#v", env.Meta["filtered_by"], env.Meta)
			}
			if !tc.wantFiltered {
				if _, ok := env.Meta["filtered_by"]; ok {
					t.Fatalf("filtered_by must be absent without --source/--target: %#v", env.Meta)
				}
			}
			if tc.wantPageCount > 0 && len(s.queries) != tc.wantPageCount {
				t.Fatalf("GET count=%d want %d (queries=%v)", len(s.queries), tc.wantPageCount, s.queries)
			}
			for _, q := range s.queries {
				for _, forbidden := range []string{"sourceBranch", "targetBranch", "source", "target"} {
					if _, ok := q[forbidden]; ok {
						t.Fatalf("branch filter leaked into query as %q: %v", forbidden, q)
					}
				}
				if tc.wantStateQ != "" && q.Get("state") != tc.wantStateQ {
					t.Fatalf("state query=%q want %q: %v", q.Get("state"), tc.wantStateQ, q)
				}
			}
		})
	}
}

// The client-side filter must survive the non-array shapes AttachMergeRequestURLs
// handles (wrapper maps) and drop items missing the branch fields.
func TestFilterMrsByBranchShapes(t *testing.T) {
	base := []any{
		map[string]any{"localId": 1, "sourceBranch": "feat/x", "targetBranch": "master"},
		map[string]any{"localId": 2, "sourceBranch": "feat/y", "targetBranch": "master"},
		map[string]any{"localId": 3}, // no branch fields
	}
	wrapped := map[string]any{"data": append([]any{}, base...)}

	cases := []struct {
		name   string
		in     any
		source string
		target string
		want   []int
	}{
		{name: "bare slice source", in: append([]any{}, base...), source: "feat/x", want: []int{1}},
		{name: "bare slice target", in: append([]any{}, base...), target: "master", want: []int{1, 2}},
		{name: "wrapper map", in: wrapped, source: "feat/y", want: []int{2}},
		{name: "no constraint is no-op", in: append([]any{}, base...), want: []int{1, 2, 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := filterMrsByBranch(tc.in, tc.source, tc.target)
			var items []any
			switch v := out.(type) {
			case []any:
				items = v
			case map[string]any:
				inner, _ := v["data"].([]any)
				items = inner
			}
			got := make([]int, 0, len(items))
			for _, it := range items {
				got = append(got, mrsItemLocalID(it.(map[string]any)))
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ids=%v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("ids=%v want %v", got, tc.want)
				}
			}
		})
	}
}

// Help contract: --source/--target documented as client-side with the --all note.
func TestMrsListBranchFilterHelp(t *testing.T) {
	long := codeupMrsListCmd.Long
	for _, want := range []string{"--source/--target", "client-side", "meta.filtered_by = \"client\"", "combine with --all", "before filtering"} {
		if !strings.Contains(long, want) {
			t.Fatalf("mrs list help missing %q:\n%s", want, long)
		}
	}
	if f := codeupMrsListCmd.Flags().Lookup("source"); f == nil {
		t.Fatal("--source flag not registered")
	}
	if f := codeupMrsListCmd.Flags().Lookup("target"); f == nil {
		t.Fatal("--target flag not registered")
	}
}
