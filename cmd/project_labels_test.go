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

type labelsServer struct {
	mu         sync.Mutex
	listPath   string
	createBody map[string]any
	creates    int
}

func newLabelsServer(t *testing.T) *labelsServer {
	t.Helper()
	s := &labelsServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/labels"):
			s.listPath = r.URL.Path
			_, _ = io.WriteString(w, `[{"id":"lab-1","name":"fast-lane","color":"#A773E0"}]`)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/labels"):
			s.creates++
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &s.createBody)
			_, _ = io.WriteString(w, `{"id":"lab-new","name":"fast-lane","color":"#A773E0"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"errorCode":"NotFound"}`)
		}
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           "p141",
		OrganizationID: "org-141",
		SpaceID:        "space-1",
	})
	t.Setenv(config.EnvAccessToken, "test-token-141-labels-not-real")
	t.Setenv(config.EnvOrganizationID, "org-141")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "p141")
	return s
}

func runProjectLabels(t *testing.T, dryRun bool, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	out := withCmdJSONCapture(t)
	var errBuf bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &errBuf
	t.Cleanup(func() { output.Stderr = prevErr })

	prevDry := globalDryRun
	prevYes := globalYes
	prevProfile := globalProfile
	globalDryRun = dryRun
	if globalProfile == "" {
		globalProfile = "p141"
	}
	if !dryRun {
		globalYes = true
	}
	t.Cleanup(func() {
		globalDryRun = prevDry
		globalYes = prevYes
		globalProfile = prevProfile
	})

	resetStringFlags(t, projectLabelsListCmd, "space-id")
	resetStringFlags(t, projectLabelsCreateCmd, "space-id", "name", "color")

	full := append([]string{}, args...)
	if dryRun {
		full = append(full, "--dry-run")
	} else if containsYesNeeded(args) {
		full = append(full, "--yes")
	}
	rootCmd.SetArgs(full)
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	prevExit := processExit
	code = 0
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })
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
	return out.String(), errBuf.String(), code
}

func containsYesNeeded(args []string) bool {
	for _, a := range args {
		if a == "create" {
			return true
		}
	}
	return false
}

func TestProjectLabelsList(t *testing.T) {
	s := newLabelsServer(t)
	stdout, stderr, code := runProjectLabels(t, false, "project", "labels", "list", "--space-id", "space-1")
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(s.listPath, "/projects/space-1/labels") {
		t.Fatalf("path=%q", s.listPath)
	}
	if !strings.Contains(stdout, "lab-1") || !strings.Contains(stdout, "fast-lane") {
		t.Fatalf("stdout=%s", stdout)
	}
}

func TestProjectLabelsCreateDryRunAndWrite(t *testing.T) {
	s := newLabelsServer(t)
	stdout, stderr, code := runProjectLabels(t, true, "project", "labels", "create", "--name", "fast-lane")
	if code != 0 {
		t.Fatalf("dry-run exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if s.creates != 0 {
		t.Fatalf("dry-run must not POST, creates=%d", s.creates)
	}
	if !strings.Contains(stdout, "/labels") && !strings.Contains(stdout, "fast-lane") {
		t.Fatalf("dry-run stdout=%s", stdout)
	}

	stdout, stderr, code = runProjectLabels(t, false, "project", "labels", "create", "--name", "fast-lane")
	if code != 0 {
		t.Fatalf("create exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if s.creates != 1 {
		t.Fatalf("creates=%d", s.creates)
	}
	if s.createBody["name"] != "fast-lane" || s.createBody["color"] != "#A773E0" {
		t.Fatalf("body=%v", s.createBody)
	}
	if !strings.Contains(stdout, "lab-new") {
		t.Fatalf("stdout=%s", stdout)
	}
}
