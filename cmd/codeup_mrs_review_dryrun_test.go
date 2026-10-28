package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

// reviewDryRunServer serves MR detail + platform/user so #395 author precheck
// can run under --dry-run (POST must stay at 0 hits).
type reviewDryRunServer struct {
	mu   sync.Mutex
	hits int
	post int
}

func newReviewDryRunServer(t *testing.T, authorID, currentID string) *reviewDryRunServer {
	t.Helper()
	s := &reviewDryRunServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.hits++
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/platform/user"):
			_, _ = io.WriteString(w, `{"id":"`+currentID+`","name":"tester"}`)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/changeRequests/"):
			_, _ = io.WriteString(w, `{"localId":139,"status":"UNDER_REVIEW","author":{"userId":"`+authorID+`","name":"author"}}`)
		case r.Method == http.MethodPost:
			s.post++
			w.WriteHeader(500)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-codeup-mrs-review-not-real")
	t.Setenv(config.EnvOrganizationID, "org-codeup-mrs-review")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

func TestCodeupMrsReviewDryRun(t *testing.T) {
	s := newReviewDryRunServer(t, "author-1", "reviewer-1")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsReviewCmd, "repo", "local-id", "opinion", "comment")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "review",
		"--repo", "4951320",
		"--local-id", "139",
		"--opinion", "PASS",
		"--comment", "lgtm dry-run",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	if env.Risk != string(risk.HighRiskWrite) && env.Risk != "high-risk-write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/repositories/4951320/changeRequests/139/review") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["reviewOpinion"] != "PASS" || body["reviewComment"] != "lgtm dry-run" {
		t.Fatalf("body=%v", body)
	}
	pre, _ := req["precheck"].(map[string]any)
	if pre == nil || pre["status"] != "ok" || pre["is_author"] != false {
		t.Fatalf("precheck=%v", pre)
	}
	if s.post != 0 {
		t.Fatalf("post=%d", s.post)
	}
}

func TestCodeupMrsReviewNotPassDryRun(t *testing.T) {
	s := newReviewDryRunServer(t, "author-2", "reviewer-2")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsReviewCmd, "repo", "local-id", "opinion", "comment")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "review",
		"--repo", "4951321",
		"--local-id", "140",
		"--opinion", "NOT_PASS",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	if env.Risk != string(risk.HighRiskWrite) && env.Risk != "high-risk-write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/repositories/4951321/changeRequests/140/review") {
		t.Fatalf("url=%q", url)
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["reviewOpinion"] != "NOT_PASS" {
		t.Fatalf("body=%v", body)
	}
	if _, has := body["reviewComment"]; has {
		t.Fatalf("unexpected reviewComment=%v", body["reviewComment"])
	}
	if s.post != 0 {
		t.Fatalf("post=%d", s.post)
	}
}
