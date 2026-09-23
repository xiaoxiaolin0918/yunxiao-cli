package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

func TestBugCreateDryRunIncludesVerifierFromFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:              "t77",
		OrganizationID:    "org-77",
		SpaceID:           "space-1",
		BugTypeID:         "bug-type-1",
		DefaultAssignedTo: "user-assignee",
		DefaultVerifier:   "user-default-verifier",
		BugCreateFields: profile.BugCreateFields{
			Priority:     map[string]string{"high": "prio-high"},
			SeriousLevel: map[string]string{"normal": "sev-normal", "severe": "sev-severe"},
		},
	})

	t.Setenv(config.EnvAccessToken, "test-token-77-not-real")
	t.Setenv(config.EnvOrganizationID, "org-77")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "t77")

	stdout := withCmdJSONCapture(t)
	globalProfile = "t77"
	resetStringFlags(t, workitemBugCreateCmd, "title", "description", "environment", "module", "priority", "serious-level", "expected-completion", "sprint", "assigned-to", "verifier")
	_ = workitemBugCreateCmd.Flags().Set("minimal", "true")
	t.Cleanup(func() { _ = workitemBugCreateCmd.Flags().Set("minimal", "false") })

	rootCmd.SetArgs([]string{
		"workitem", "+bug-create",
		"--title", "t",
		"--description", "d",
		"--sprint", "sprint-1",
		"--verifier", "user-flag-verifier",
		"--minimal",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	if !strings.Contains(string(raw), "user-flag-verifier") {
		t.Fatalf("expected verifier in dry-run body: %s", raw)
	}
}

func TestBugCreateWarnsWhenVerifierUnset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:              "t77w",
		OrganizationID:    "org-77",
		SpaceID:           "space-1",
		BugTypeID:         "bug-type-1",
		DefaultAssignedTo: "user-assignee",
		BugCreateFields: profile.BugCreateFields{
			Priority:     map[string]string{"high": "prio-high"},
			SeriousLevel: map[string]string{"normal": "sev-normal"},
		},
	})

	t.Setenv(config.EnvAccessToken, "test-token-77-not-real")
	t.Setenv(config.EnvOrganizationID, "org-77")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "t77w")

	var stderr bytes.Buffer
	prevStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = prevStderr })

	stdout := withCmdJSONCapture(t)
	globalProfile = "t77w"
	resetStringFlags(t, workitemBugCreateCmd, "title", "description", "environment", "module", "priority", "serious-level", "expected-completion", "sprint", "assigned-to", "verifier")
	_ = workitemBugCreateCmd.Flags().Set("minimal", "true")
	t.Cleanup(func() { _ = workitemBugCreateCmd.Flags().Set("minimal", "false") })

	rootCmd.SetArgs([]string{
		"workitem", "+bug-create",
		"--title", "t",
		"--description", "d",
		"--sprint", "sprint-1",
		"--minimal",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	err := rootCmd.Execute()
	_ = w.Close()
	_, _ = stderr.ReadFrom(r)
	if err != nil {
		t.Fatalf("execute: %v\nstdout=%s stderr=%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "verifier unset") {
		t.Fatalf("expected verifier warning; stderr=%q", stderr.String())
	}
}

func TestBugCreateSeriousLevelHelpMentionsSevereNotSeriousPipe(t *testing.T) {
	f := workitemBugCreateCmd.Flags().Lookup("serious-level")
	if f == nil {
		t.Fatal("missing flag")
	}
	u := f.Usage
	if strings.Contains(u, "serious|severe") {
		t.Fatalf("help still has serious|severe pipe form: %q", u)
	}
	if !strings.Contains(u, "severe") || !strings.Contains(u, "serious→severe") {
		t.Fatalf("help should document severe + serious synonym: %q", u)
	}
}
