package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

func writeExploreProfile(t *testing.T, pf *profile.Profile) {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	profDir := filepath.Join(xdg, "yunxiao", "profiles")
	if err := os.MkdirAll(profDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(pf.Name) == "" {
		t.Fatal("profile name required")
	}
	if err := profile.SaveFile(filepath.Join(profDir, pf.Name+".json"), pf); err != nil {
		t.Fatal(err)
	}
}

func setupExploreAPIEnv(t *testing.T, pf *profile.Profile, handler http.HandlerFunc) *atomic.Int32 {
	t.Helper()
	writeExploreProfile(t, pf)
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if handler != nil {
			handler(w, r)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"unexpected request"}`))
	}))
	t.Cleanup(srv.Close)
	org := strings.TrimSpace(pf.OrganizationID)
	if org == "" {
		org = "org-412"
	}
	t.Setenv(config.EnvAccessToken, "test-token-explore-412")
	t.Setenv(config.EnvOrganizationID, org)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv(profile.EnvProfile, pf.Name)
	return &requests
}

func zhiyiExploreProfile() *profile.Profile {
	return &profile.Profile{
		Name:           "zhiyi",
		SpaceID:        "space-zypt",
		OrganizationID: "org-412",
		SerialPrefix:   "ZYPT",
		WorkitemDefaults: map[string]profile.WorkitemTypeDefaults{
			"req-type-412": {Category: "Req"},
		},
	}
}

func playExploreProfile() *profile.Profile {
	return &profile.Profile{
		Name:              "play",
		SpaceID:           "space-yxcli",
		OrganizationID:    "org-412",
		SerialPrefix:      "YXCLI",
		DefaultAssignedTo: "user-1",
		WorkitemDefaults: map[string]profile.WorkitemTypeDefaults{
			"req-type-412": {Category: "Req"},
		},
	}
}

func parseExploreErr(t *testing.T, stderr string) output.Envelope {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(stderr), &env); err != nil {
		t.Fatalf("stderr not JSON envelope: %v / %s", err, stderr)
	}
	if env.OK {
		t.Fatalf("expected ok=false envelope: %s", stderr)
	}
	if env.Error == nil {
		t.Fatalf("missing error body: %s", stderr)
	}
	return env
}

func assertNoAPIRequests(t *testing.T, n int32, stdout, stderr string) {
	t.Helper()
	if n != 0 {
		t.Fatalf("safety gate sent %d API request(s), want 0\nstdout=%s\nstderr=%s", n, stdout, stderr)
	}
}

// TestExploreWorkflowRejectsNonSandboxCreateEvenWithYes locks #412: a business
// space (zhiyi / serial_prefix=ZYPT) must not create probe items even with --yes.
func TestExploreWorkflowRejectsNonSandboxCreateEvenWithYes(t *testing.T) {
	requests := setupExploreAPIEnv(t, zhiyiExploreProfile(), nil)
	stdout, stderr, code := runExploreWorkflowCmd(t, false,
		"workitem", "+explore-workflow",
		"--type-id", "req-type-412",
		"--cleanup",
		"--yes",
	)
	if code != 1 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	assertNoAPIRequests(t, requests.Load(), stdout, stderr)
	env := parseExploreErr(t, stderr)
	if env.Error.Subtype != "explore_workflow_safety" {
		t.Fatalf("subtype=%q stderr=%s", env.Error.Subtype, stderr)
	}
	blob := stderr
	for _, want := range []string{"allow-non-sandbox", "YXCLI", "ZYPT"} {
		if !strings.Contains(blob, want) {
			t.Fatalf("stderr missing %q: %s", want, stderr)
		}
	}
}

// TestExploreWorkflowRejectsRealItemIDByDefault locks #412: --id must not
// mutate a real work item unless the explicit confirmation flag is set, even
// in a sandbox and even with --yes.
func TestExploreWorkflowRejectsRealItemIDByDefault(t *testing.T) {
	requests := setupExploreAPIEnv(t, playExploreProfile(), nil)
	stdout, stderr, code := runExploreWorkflowCmd(t, false,
		"workitem", "+explore-workflow",
		"--type-id", "req-type-412",
		"--id", "ZYPT-6489",
		"--yes",
	)
	if code != 1 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	assertNoAPIRequests(t, requests.Load(), stdout, stderr)
	env := parseExploreErr(t, stderr)
	if env.Error.Subtype != "explore_workflow_safety" {
		t.Fatalf("subtype=%q stderr=%s", env.Error.Subtype, stderr)
	}
	blob := env.Error.Message + " " + env.Error.Hint + " " + stderr
	for _, want := range []string{
		"--i-know-this-mutates-real-item",
		"cannot be reverted",
	} {
		if !strings.Contains(blob, want) {
			t.Fatalf("error missing %q: %s", want, stderr)
		}
	}
}

// TestExploreWorkflowRejectsCreateWithoutCleanup locks #412: creating probes
// without --cleanup is refused even in the YXCLI sandbox.
func TestExploreWorkflowRejectsCreateWithoutCleanup(t *testing.T) {
	requests := setupExploreAPIEnv(t, playExploreProfile(), nil)
	stdout, stderr, code := runExploreWorkflowCmd(t, false,
		"workitem", "+explore-workflow",
		"--type-id", "req-type-412",
		"--yes",
	)
	if code != 1 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	assertNoAPIRequests(t, requests.Load(), stdout, stderr)
	env := parseExploreErr(t, stderr)
	if env.Error.Subtype != "explore_workflow_safety" {
		t.Fatalf("subtype=%q stderr=%s", env.Error.Subtype, stderr)
	}
	if !strings.Contains(stderr, "--cleanup") {
		t.Fatalf("stderr missing --cleanup: %s", stderr)
	}
}

// TestExploreWorkflowRejectsWriteProfileOnNonSandbox locks #412: --write-profile
// against a business space is refused without --allow-non-sandbox.
func TestExploreWorkflowRejectsWriteProfileOnNonSandbox(t *testing.T) {
	requests := setupExploreAPIEnv(t, zhiyiExploreProfile(), nil)
	stdout, stderr, code := runExploreWorkflowCmd(t, false,
		"workitem", "+explore-workflow",
		"--type-id", "req-type-412",
		"--cleanup",
		"--write-profile",
		"--yes",
	)
	if code != 1 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	assertNoAPIRequests(t, requests.Load(), stdout, stderr)
	env := parseExploreErr(t, stderr)
	if env.Error.Subtype != "explore_workflow_safety" {
		t.Fatalf("subtype=%q stderr=%s", env.Error.Subtype, stderr)
	}
	if !strings.Contains(stderr, "--allow-non-sandbox") && !strings.Contains(stderr, "write-profile") {
		t.Fatalf("stderr should mention the non-sandbox / write-profile gate: %s", stderr)
	}
}

func newExploreWorkflowStub() (*exploreWorkflowStub, http.HandlerFunc) {
	st := &exploreWorkflowStub{status: "s1"}
	return st, st.ServeHTTP
}

type exploreWorkflowStub struct {
	mu      sync.Mutex
	status  string
	created bool
	deleted bool
	posts   int
	puts    int
	deletes int
}

func (s *exploreWorkflowStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && strings.Contains(path, "/workflows"):
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":              "wf-412",
			"name":            "req wf",
			"defaultStatusId": "s1",
			"statuses": []map[string]any{
				{"id": "s1", "name": "todo", "displayName": "待处理"},
				{"id": "s2", "name": "done", "displayName": "已完成"},
			},
		})
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/workitems"):
		s.posts++
		s.created = true
		s.status = "s1"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":             "probe-412",
			"serialNumber":   "YXCLI-412",
			"workitemTypeId": "req-type-412",
			"status":         map[string]any{"id": s.status},
		})
	case strings.Contains(path, "/workitems/"):
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "probe-412",
				"serialNumber":   "YXCLI-412",
				"workitemTypeId": "req-type-412",
				"status":         map[string]any{"id": s.status},
			})
		case http.MethodPut:
			s.puts++
			var body map[string]any
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &body)
			if st, _ := body["status"].(string); st != "" {
				s.status = st
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     "probe-412",
				"status": map[string]any{"id": s.status},
			})
		case http.MethodDelete:
			s.deletes++
			s.deleted = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"unhandled `+r.Method+` `+path+`"}`)
	}
}

// TestExploreWorkflowSandboxCreateCleanupSucceeds locks #412: the supported
// path (YXCLI/play sandbox + self-created probe + --cleanup --yes) still
// completes against a fake client, with the probe deleted.
func TestExploreWorkflowSandboxCreateCleanupSucceeds(t *testing.T) {
	st, handler := newExploreWorkflowStub()
	_ = setupExploreAPIEnv(t, playExploreProfile(), handler)
	stdout, stderr, code := runExploreWorkflowCmd(t, false,
		"workitem", "+explore-workflow",
		"--type-id", "req-type-412",
		"--cleanup",
		"--yes",
	)
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout)
	}
	if !env.OK {
		t.Fatalf("expected ok envelope: %s", stdout)
	}
	raw, _ := json.Marshal(env.Data)
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data["probe_created"] != true {
		t.Fatalf("probe_created=%v data=%v", data["probe_created"], data)
	}
	if data["probe_cleaned"] != true {
		t.Fatalf("probe_cleaned=%v data=%v", data["probe_cleaned"], data)
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.created || !st.deleted || st.posts == 0 || st.deletes == 0 {
		t.Fatalf("stub state created=%v deleted=%v posts=%d deletes=%d puts=%d", st.created, st.deleted, st.posts, st.deletes, st.puts)
	}
}

// TestExploreWorkflowHelpDocumentsSafetyGates locks #412 help/error copy:
// irreversible terminal-status warning, sandbox recommendation, and the
// explicit escape flags must appear in command help.
func TestExploreWorkflowHelpDocumentsSafetyGates(t *testing.T) {
	h := workitemExploreWorkflowCmd.Long
	for _, want := range []string{
		"--cleanup",
		"--allow-non-sandbox",
		"--i-know-this-mutates-real-item",
		"cannot be reverted",
		"YXCLI",
		"serial_prefix",
	} {
		if !strings.Contains(h, want) {
			t.Fatalf("help missing %q: %s", want, h)
		}
	}
}

func TestIsExploreWorkflowSandbox(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		pf   *profile.Profile
		want bool
	}{
		{"nil", nil, false},
		{"zhiyi ZYPT", &profile.Profile{Name: "zhiyi", SerialPrefix: "ZYPT"}, false},
		{"play YXCLI", &profile.Profile{Name: "play", SerialPrefix: "YXCLI"}, true},
		{"custom YXCLI", &profile.Profile{Name: "mysandbox", SerialPrefix: "yxcli"}, true},
		{"play empty prefix", &profile.Profile{Name: "play"}, true},
		{"play with ZYPT", &profile.Profile{Name: "play", SerialPrefix: "ZYPT"}, false},
		{"unknown empty", &profile.Profile{Name: "other"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := isExploreWorkflowSandbox(tc.pf)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
