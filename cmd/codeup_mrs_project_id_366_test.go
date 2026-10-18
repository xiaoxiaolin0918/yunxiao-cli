package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// #366: JSON float64 ids must not become scientific notation via fmt %v.
func TestNormalizeRepositoryIDAvoidsScientificNotation(t *testing.T) {
	const id = float64(4951320)
	if got := fmt.Sprintf("%v", id); !strings.Contains(got, "e+") && !strings.Contains(got, "E+") {
		t.Logf("fmt %%v of float64(4951320)=%q (expected scientific on current Go)", got)
	}
	got := zhiyi.NormalizeRepositoryID(id)
	if got != "4951320" {
		t.Fatalf("NormalizeRepositoryID(float64(4951320))=%q want 4951320", got)
	}
	if strings.Contains(got, "e") || strings.Contains(got, "E") {
		t.Fatalf("scientific notation leaked: %q", got)
	}
}

type mrsPathRepoServer struct {
	mu       sync.Mutex
	repoGETs []string
	posts    int
}

func newMrsPathRepoServer(t *testing.T, org string, repoJSON string) *mrsPathRepoServer {
	t.Helper()
	s := &mrsPathRepoServer{}
	prefix := "/oapi/v1/codeup/organizations/" + org + "/repositories/"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		escaped := r.URL.EscapedPath()
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(escaped, prefix) && !strings.Contains(escaped[len(prefix):], "/"):
			s.repoGETs = append(s.repoGETs, escaped)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, repoJSON)
		case r.Method == http.MethodPost && strings.Contains(escaped, "/changeRequests"):
			s.posts++
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-mrs-path-repo-not-real")
	t.Setenv(config.EnvOrganizationID, org)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

func assertMrsCreateDryRunProjectIDs(t *testing.T, stdout string, wantID string) {
	t.Helper()
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
	body, _ := req["body"].(map[string]any)
	if body == nil {
		t.Fatalf("body nil: %v", req)
	}
	src, _ := body["sourceProjectId"].(string)
	tgt, _ := body["targetProjectId"].(string)
	if src != wantID || tgt != wantID {
		t.Fatalf("projectIds source=%q target=%q want %q; body=%v", src, tgt, wantID, body)
	}
	if strings.Contains(src, "e+") || strings.Contains(src, "E+") || src == "" {
		t.Fatalf("illegal sourceProjectId %q", src)
	}
}

// #366: path --repo on mrs create must GET repo and put decimal id in dry-run body
// (not path string, not float64 scientific notation).
func TestCodeupMrsCreatePathRepoResolvesDecimalProjectIDDryRun(t *testing.T) {
	const org = "org-mrs-create-path-366"
	const pathRepo = "sanzhi/zhiyi/frontend/mgy_wechat_app"
	s := newMrsPathRepoServer(t, org, `{"id":4951320,"name":"mgy_wechat_app","pathWithNamespace":"sanzhi/zhiyi/frontend/mgy_wechat_app"}`)

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsCreateCmd,
		"repo", "source", "target", "title", "description",
		"reviewer", "source-project-id", "target-project-id", "create-from", "work-item",
	)
	if f := codeupMrsCreateCmd.Flags().Lookup("create-from"); f != nil && f.DefValue == "" {
		_ = codeupMrsCreateCmd.Flags().Set("create-from", "WEB")
		f.Changed = false
	}
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "create",
		"--repo", pathRepo,
		"--source", "fix/xxx",
		"--target", "master",
		"--title", "probe",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	assertMrsCreateDryRunProjectIDs(t, stdout.String(), "4951320")
	wantEnc := client.EncodeRepoID(pathRepo)
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.repoGETs) != 1 || !strings.HasSuffix(s.repoGETs[0], "/repositories/"+wantEnc) {
		t.Fatalf("repoGETs=%v want .../repositories/%s", s.repoGETs, wantEnc)
	}
	if s.posts != 0 {
		t.Fatalf("dry-run must not POST changeRequests: posts=%d", s.posts)
	}
}

// #366: +create with path --repo must resolve numeric project ids before dry-run preview.
func TestCodeupMrsPlusCreatePathRepoResolvesDecimalProjectIDDryRun(t *testing.T) {
	const org = "org-mrs-plus-path-366"
	const pathRepo = "sanzhi/zhiyi/frontend/mgy_wechat_app"
	s := newMrsPathRepoServer(t, org, `{"id":4951320,"name":"mgy_wechat_app","pathWithNamespace":"sanzhi/zhiyi/frontend/mgy_wechat_app"}`)

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsPlusCreateCmd, "repo", "source", "target", "title", "description", "work-item", "reviewer")
	_ = codeupMrsPlusCreateCmd.Flags().Set("wip", "false")
	_ = codeupMrsPlusCreateCmd.Flags().Set("full", "false")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "+create",
		"--repo", pathRepo,
		"--source", "fix/xxx",
		"--target", "master",
		"--title", "WIP: fix",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	assertMrsCreateDryRunProjectIDs(t, stdout.String(), "4951320")
	wantEnc := client.EncodeRepoID(pathRepo)
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.repoGETs) != 1 || !strings.HasSuffix(s.repoGETs[0], "/repositories/"+wantEnc) {
		t.Fatalf("repoGETs=%v want .../repositories/%s", s.repoGETs, wantEnc)
	}
	if s.posts != 0 {
		t.Fatalf("dry-run must not POST: posts=%d", s.posts)
	}
}
