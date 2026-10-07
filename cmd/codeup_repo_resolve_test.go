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

// #125 fixtures for GET .../repositories?search=<name>. Uses the {"result": [...]}
// wrapper some Yunxiao list endpoints emit; discovery also tolerates a bare array.
const repoSearchUniqueFixture = `{"result":[
 {"id":4951320,"name":"zhiyi_doc","pathWithNamespace":"sanzhi/zhiyi/zhiyi_doc"},
 {"id":7287010,"name":"other","pathWithNamespace":"sanzhi/zhiyi/other"}
]}`

const repoSearchAmbiguousFixture = `{"result":[
 {"id":4951320,"name":"zhiyi_doc","pathWithNamespace":"sanzhi/g1/zhiyi_doc"},
 {"id":7287010,"name":"zhiyi_doc","pathWithNamespace":"sanzhi/g2/zhiyi_doc"}
]}`

const repoSearchNoMatchFixture = `{"result":[
 {"id":7287010,"name":"other","pathWithNamespace":"sanzhi/zhiyi/other"}
]}`

type repoResolveServer struct {
	mu         sync.Mutex
	searchGETs int
	searchQ    string
	repoGETs   []string // escaped paths of GET .../repositories/<repo>/...
	other      []string
}

func newRepoResolveServer(t *testing.T, searchResult string) *repoResolveServer {
	t.Helper()
	s := &repoResolveServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		escaped := r.URL.EscapedPath()
		switch {
		case r.Method == http.MethodGet && escaped == "/oapi/v1/codeup/organizations/org-repo-resolve/repositories":
			s.searchGETs++
			s.searchQ = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, searchResult)
		case r.Method == http.MethodGet && strings.HasPrefix(escaped, "/oapi/v1/codeup/organizations/org-repo-resolve/repositories/"):
			s.repoGETs = append(s.repoGETs, escaped)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[{"name":"master","commit":{"id":"abc"}}]`)
		default:
			s.other = append(s.other, r.Method+" "+escaped)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-repo-resolve-not-real")
	t.Setenv(config.EnvOrganizationID, "org-repo-resolve")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

// runBranchesList runs `codeup branches list --repo <repo>` and returns stdout,
// stderr and processExit code (0 when the command did not exit).
func runBranchesList(t *testing.T, repo string) (string, string, int) {
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

	resetStringFlags(t, codeupBranchesListCmd, "repo", "search")
	rootCmd.SetArgs([]string{"codeup", "branches", "list", "--repo", repo})
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

// #125: bare unregistered names are auto-discovered via one read-only repos search.
func TestBranchesListRepoBareNameOutcomes(t *testing.T) {
	cases := []struct {
		name         string
		fixture      string
		wantCode     int
		wantStderr   []string
		wantRepoGET  string // expected suffix of the repositories GET (escaped); "" → none
		wantSearches int
	}{
		{
			name:         "unique-match-resolves-to-numeric-id",
			fixture:      repoSearchUniqueFixture,
			wantCode:     0,
			wantRepoGET:  "/repositories/4951320/branches",
			wantSearches: 1,
		},
		{
			name:     "ambiguous-lists-candidates",
			fixture:  repoSearchAmbiguousFixture,
			wantCode: 1,
			wantStderr: []string{
				`ambiguous repository name "zhiyi_doc"`,
				"2 matches",
				"id=4951320", "sanzhi/g1/zhiyi_doc",
				"id=7287010", "sanzhi/g2/zhiyi_doc",
				"yunxiao profile repo-add zhiyi_doc",
			},
			wantSearches: 1,
		},
		{
			name:     "no-match-keeps-alias-error-with-hint",
			fixture:  repoSearchNoMatchFixture,
			wantCode: 1,
			wantStderr: []string{
				`unknown repository alias "zhiyi_doc"`,
				`no repository named "zhiyi_doc"`,
				"yunxiao profile repo-add zhiyi_doc <repo-id-or-org/repo-path>",
			},
			wantSearches: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newRepoResolveServer(t, tc.fixture)
			stdout, stderr, code := runBranchesList(t, "zhiyi_doc")
			if code != tc.wantCode {
				t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
			}
			if s.searchGETs != tc.wantSearches {
				t.Fatalf("searchGETs=%d want %d (query %q)", s.searchGETs, tc.wantSearches, s.searchQ)
			}
			if tc.wantSearches == 1 && !strings.Contains(s.searchQ, "search=zhiyi_doc") {
				t.Fatalf("search query missing name: %q", s.searchQ)
			}
			if tc.wantRepoGET == "" {
				if len(s.repoGETs) != 0 {
					t.Fatalf("unexpected repo GETs: %v", s.repoGETs)
				}
			} else {
				if len(s.repoGETs) != 1 || !strings.HasSuffix(s.repoGETs[0], tc.wantRepoGET) {
					t.Fatalf("repoGETs=%v want suffix %s", s.repoGETs, tc.wantRepoGET)
				}
			}
			for _, want := range tc.wantStderr {
				// stderr is a JSON envelope; assert on the decoded message so
				// quoted substrings survive JSON escaping.
				var msg string
				if tc.wantCode != 0 {
					msg = decodeErrorBody(t, stderr).Message
				} else {
					msg = stdout
				}
				if !strings.Contains(msg, want) {
					t.Fatalf("message missing %q: %s", want, msg)
				}
			}
			if tc.wantCode == 0 {
				var env output.Envelope
				if err := json.Unmarshal([]byte(stdout), &env); err != nil || !env.OK {
					t.Fatalf("envelope not ok: %v / %s", err, stdout)
				}
			}
			if len(s.other) != 0 {
				t.Fatalf("unexpected requests: %v", s.other)
			}
		})
	}
}

// #125: plain org[/group]/repo slash paths are URL-encoded by the CLI — the exact
// bash/PowerShell %2F pitfall from the issue. Pre-encoded input passes through.
func TestBranchesListRepoPathFormsEncodeSlashes(t *testing.T) {
	cases := map[string]string{
		"plain-slashes":   "sanzhi/zhiyi/zhiyi_doc",
		"pre-encoded":     "sanzhi%2Fzhiyi%2Fzhiyi_doc",
		"two-segment":     "sanzhi/zhiyi_doc",
	}
	for name, repo := range cases {
		t.Run(name, func(t *testing.T) {
			s := newRepoResolveServer(t, repoSearchUniqueFixture)
			stdout, stderr, code := runBranchesList(t, repo)
			if code != 0 {
				t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
			}
			// Path refs must not trigger bare-name discovery.
			if s.searchGETs != 0 {
				t.Fatalf("path form must not search: %d (q=%q)", s.searchGETs, s.searchQ)
			}
			if len(s.repoGETs) != 1 {
				t.Fatalf("repoGETs=%v", s.repoGETs)
			}
			want := "/oapi/v1/codeup/organizations/org-repo-resolve/repositories/sanzhi%2Fzhiyi%2Fzhiyi_doc/branches"
			if name == "two-segment" {
				want = "/oapi/v1/codeup/organizations/org-repo-resolve/repositories/sanzhi%2Fzhiyi_doc/branches"
			}
			if s.repoGETs[0] != want {
				t.Fatalf("escaped path=%q want %q", s.repoGETs[0], want)
			}
			var env output.Envelope
			if err := json.Unmarshal([]byte(stdout), &env); err != nil || !env.OK {
				t.Fatalf("envelope not ok: %v / %s", err, stdout)
			}
		})
	}
}

// #125: registered profile alias resolves without any discovery request.
func TestBranchesListRepoAliasFromProfile(t *testing.T) {
	s := newRepoResolveServer(t, repoSearchUniqueFixture)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTestProfile(t, xdg, "repotest", `{"name":"repotest","repositories":{"zhiyi_doc":4951320}}`)
	t.Setenv("YUNXIAO_PROFILE", "repotest")

	stdout, stderr, code := runBranchesList(t, "zhiyi_doc")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if s.searchGETs != 0 {
		t.Fatalf("registered alias must not trigger discovery: %d", s.searchGETs)
	}
	if len(s.repoGETs) != 1 || !strings.HasSuffix(s.repoGETs[0], "/repositories/4951320/branches") {
		t.Fatalf("repoGETs=%v", s.repoGETs)
	}
}
