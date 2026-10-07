package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

// writeTestProfile writes <xdg>/yunxiao/profiles/<name>.json (test fixture helper).
func writeTestProfile(t *testing.T, xdg, name, content string) string {
	t.Helper()
	dir := filepath.Join(xdg, "yunxiao", "profiles")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name+".json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// repoAddServer answers the read-only GETs repo-add resolution needs (#125):
// repos search (bare names) and repositories/{repo} (path form).
type repoAddServer struct {
	mu         sync.Mutex
	searchGETs int
	repoGETs   []string // escaped paths
}

func newRepoAddServer(t *testing.T, searchResult string) *repoAddServer {
	t.Helper()
	s := &repoAddServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		escaped := r.URL.EscapedPath()
		switch {
		case r.Method == http.MethodGet && escaped == "/oapi/v1/codeup/organizations/org-repo-add/repositories":
			s.searchGETs++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(searchResult))
		case r.Method == http.MethodGet && strings.HasPrefix(escaped, "/oapi/v1/codeup/organizations/org-repo-add/repositories/"):
			s.repoGETs = append(s.repoGETs, escaped)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":4951320,"name":"zhiyi_doc","pathWithNamespace":"sanzhi/zhiyi/zhiyi_doc"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-repo-add-not-real")
	t.Setenv(config.EnvOrganizationID, "org-repo-add")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	return s
}

func runProfileRepoAdd(t *testing.T, dryRun bool, args ...string) (string, string, int) {
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

	resetStringFlags(t, profileRepoAddCmd, "force")
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

// loadTestProfile reads a profile fixture back via the package under test.
func loadTestProfile(t *testing.T, path string) *profile.Profile {
	t.Helper()
	pf, err := profile.LoadFile(path)
	if err != nil {
		t.Fatalf("reload profile: %v", err)
	}
	return pf
}

func TestProfileRepoAddPathFormResolvesNumericID(t *testing.T) {
	s := newRepoAddServer(t, `{"result":[]}`)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("YUNXIAO_PROFILE", "repotest")
	p := writeTestProfile(t, xdg, "repotest", `{"name":"repotest"}`)

	stdout, stderr, code := runProfileRepoAdd(t, false, "profile", "repo-add", "zhiyi_doc", "sanzhi/zhiyi/zhiyi_doc")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	// The path form must be URL-encoded on the resolving GET — plain slashes in, %2F out.
	if len(s.repoGETs) != 1 || s.repoGETs[0] != "/oapi/v1/codeup/organizations/org-repo-add/repositories/sanzhi%2Fzhiyi%2Fzhiyi_doc" {
		t.Fatalf("repoGETs=%v", s.repoGETs)
	}
	if s.searchGETs != 0 {
		t.Fatalf("path form must not search: %d", s.searchGETs)
	}
	pf := loadTestProfile(t, p)
	if pf.Repositories["zhiyi_doc"] != 4951320 {
		t.Fatalf("repositories=%v", pf.Repositories)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil || !env.OK {
		t.Fatalf("envelope not ok: %v / %s", err, stdout)
	}
	if env.Meta["resolved_from"] != "path" || env.Meta["resolved_repo"] != "sanzhi/zhiyi/zhiyi_doc" {
		t.Fatalf("meta=%v", env.Meta)
	}
	if env.Data == nil || !strings.Contains(stdout, `"repository_id":4951320`) {
		t.Fatalf("data missing repository_id: %s", stdout)
	}
}

func TestProfileRepoAddBareNameDiscovery(t *testing.T) {
	s := newRepoAddServer(t, `{"result":[
	 {"id":4951320,"name":"zhiyi_doc","pathWithNamespace":"sanzhi/zhiyi/zhiyi_doc"}
	]}`)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("YUNXIAO_PROFILE", "repotest")
	p := writeTestProfile(t, xdg, "repotest", `{"name":"repotest"}`)

	stdout, stderr, code := runProfileRepoAdd(t, false, "profile", "repo-add", "doc", "zhiyi_doc")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if s.searchGETs != 1 {
		t.Fatalf("searchGETs=%d", s.searchGETs)
	}
	if loadTestProfile(t, p).Repositories["doc"] != 4951320 {
		t.Fatalf("repositories not written")
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil || !env.OK {
		t.Fatalf("envelope not ok: %v / %s", err, stdout)
	}
	if env.Meta["resolved_from"] != "discovery" {
		t.Fatalf("meta=%v", env.Meta)
	}
}

func TestProfileRepoAddNumericStoresAsIs(t *testing.T) {
	s := newRepoAddServer(t, `{"result":[]}`)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("YUNXIAO_PROFILE", "repotest")
	p := writeTestProfile(t, xdg, "repotest", `{"name":"repotest"}`)

	stdout, stderr, code := runProfileRepoAdd(t, false, "profile", "repo-add", "doc", "4951320")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if s.searchGETs != 0 || len(s.repoGETs) != 0 {
		t.Fatalf("numeric must be offline: search=%d gets=%v", s.searchGETs, s.repoGETs)
	}
	if loadTestProfile(t, p).Repositories["doc"] != 4951320 {
		t.Fatalf("repositories not written")
	}
	if !strings.Contains(stdout, `"repository_id":4951320`) {
		t.Fatalf("stdout=%s", stdout)
	}
}

func TestProfileRepoAddAliasCopyAndForce(t *testing.T) {
	s := newRepoAddServer(t, `{"result":[]}`)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("YUNXIAO_PROFILE", "repotest")
	p := writeTestProfile(t, xdg, "repotest", `{"name":"repotest","repositories":{"old":111}}`)

	// Copy an existing alias without network.
	stdout, stderr, code := runProfileRepoAdd(t, false, "profile", "repo-add", "new", "old")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if s.searchGETs != 0 || len(s.repoGETs) != 0 {
		t.Fatalf("alias copy must be offline: %d/%v", s.searchGETs, s.repoGETs)
	}
	if loadTestProfile(t, p).Repositories["new"] != 111 {
		t.Fatalf("alias not copied: %v", loadTestProfile(t, p).Repositories)
	}

	// Replacing a different id needs --force.
	_, stderr, code = runProfileRepoAdd(t, false, "profile", "repo-add", "new", "4951320")
	if code != 1 || !strings.Contains(stderr, "already maps to repository 111") || !strings.Contains(stderr, "--force") {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if loadTestProfile(t, p).Repositories["new"] != 111 {
		t.Fatalf("must not overwrite without --force")
	}
	stdout, stderr, code = runProfileRepoAdd(t, false, "profile", "repo-add", "new", "4951320", "--force")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if loadTestProfile(t, p).Repositories["new"] != 4951320 {
		t.Fatalf("force should overwrite: %v", loadTestProfile(t, p).Repositories)
	}
}

func TestProfileRepoAddDryRunPreviewsWithoutWrite(t *testing.T) {
	s := newRepoAddServer(t, `{"result":[
	 {"id":4951320,"name":"zhiyi_doc","pathWithNamespace":"sanzhi/zhiyi/zhiyi_doc"}
	]}`)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("YUNXIAO_PROFILE", "repotest")
	p := writeTestProfile(t, xdg, "repotest", `{"name":"repotest"}`)

	stdout, stderr, code := runProfileRepoAdd(t, true, "profile", "repo-add", "doc", "zhiyi_doc")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil || !env.OK || !env.DryRun {
		t.Fatalf("envelope: %+v / %s", env, stdout)
	}
	if s.searchGETs != 1 {
		t.Fatalf("dry-run should still resolve (read-only GET): %d", s.searchGETs)
	}
	// The fixture file must stay untouched (no repositories key written).
	if repos := loadTestProfile(t, p).Repositories; len(repos) != 0 {
		t.Fatalf("dry-run must not write: %v", repos)
	}
}

func TestProfileRepoAddRequiresProfile(t *testing.T) {
	newRepoAddServer(t, `{"result":[]}`)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("YUNXIAO_PROFILE", "")
	_, stderr, code := runProfileRepoAdd(t, false, "profile", "repo-add", "doc", "4951320")
	if code != 1 || !strings.Contains(stderr, "no profile set") {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
}

func TestProfileRepoAddHelpDocumentsForms(t *testing.T) {
	for _, want := range []string{"Risk: write", "org/repo", "bare repo name", "--force"} {
		if !strings.Contains(profileRepoAddCmd.Long, want) {
			t.Fatalf("Long missing %q: %s", want, profileRepoAddCmd.Long)
		}
	}
}
