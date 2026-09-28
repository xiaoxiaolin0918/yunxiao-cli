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

func TestBugCreateFileFlagsHelp(t *testing.T) {
	for _, name := range []string{"title-file", "description-file"} {
		if workitemBugCreateCmd.Flags().Lookup(name) == nil {
			t.Fatalf("missing --%s flag", name)
		}
	}
	long := workitemBugCreateCmd.Long
	for _, needle := range []string{"title-file", "description-file", "Windows"} {
		if !strings.Contains(long, needle) {
			t.Fatalf("+bug-create Long should mention %q: %s", needle, long)
		}
	}
	tf := workitemBugCreateCmd.Flags().Lookup("title")
	if tf == nil || !strings.Contains(tf.Usage, "title-file") {
		t.Fatalf("--title usage should point at --title-file: %q", tf.Usage)
	}
	tff := workitemBugCreateCmd.Flags().Lookup("title-file")
	if tff == nil || !strings.Contains(strings.ToLower(tff.Usage), "windows") {
		t.Fatalf("--title-file usage should mention Windows: %q", tff.Usage)
	}
	df := workitemBugCreateCmd.Flags().Lookup("description")
	if df == nil || !strings.Contains(df.Usage, "description-file") {
		t.Fatalf("--description usage should point at --description-file: %q", df.Usage)
	}
}

func TestBugCreateFileInputsDryRun(t *testing.T) {
	var gotMethods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethods = append(gotMethods, r.Method)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:              "t89",
		OrganizationID:    "org-89",
		SpaceID:           "space-1",
		BugTypeID:         "bug-type-1",
		DefaultAssignedTo: "user-assignee",
		BugCreateFields: profile.BugCreateFields{
			Priority:     map[string]string{"high": "prio-high"},
			SeriousLevel: map[string]string{"normal": "sev-normal"},
		},
	})

	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	title := append([]byte{0xEF, 0xBB, 0xBF}, []byte("缺陷标题中文")...)
	desc := append([]byte{0xEF, 0xBB, 0xBF}, []byte("详细说明：已复现")...)
	if err := os.WriteFile("title.txt", title, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("desc.md", desc, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(config.EnvAccessToken, "test-token-89-bug-create-file-not-real")
	t.Setenv(config.EnvOrganizationID, "org-89")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "t89")

	stdout := withCmdJSONCapture(t)
	globalProfile = "t89"
	resetStringFlags(t, workitemBugCreateCmd,
		"title", "title-file", "description", "description-file",
		"environment", "module", "priority", "serious-level", "expected-completion", "sprint", "assigned-to", "verifier")
	_ = workitemBugCreateCmd.Flags().Set("minimal", "true")
	t.Cleanup(func() { _ = workitemBugCreateCmd.Flags().Set("minimal", "false") })

	rootCmd.SetArgs([]string{
		"workitem", "+bug-create",
		"--title-file", "title.txt",
		"--description-file", "desc.md",
		"--sprint", "sprint-1",
		"--minimal",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("envelope: %+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	body, _ := req["body"].(map[string]any)
	if body["subject"] != "缺陷标题中文" {
		t.Fatalf("subject=%v body=%s", body["subject"], raw)
	}
	if body["description"] != "详细说明：已复现" {
		t.Fatalf("description=%v", body["description"])
	}
	for _, m := range gotMethods {
		if m == http.MethodPost || m == http.MethodPut || m == http.MethodDelete {
			t.Fatalf("mutating during dry-run: %v", gotMethods)
		}
	}
}

func TestBugCreateTitleFileMutexCLI(t *testing.T) {
	prevExit := processExit
	var code int
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:              "t89m",
		OrganizationID:    "org-89",
		SpaceID:           "space-1",
		BugTypeID:         "bug-type-1",
		DefaultAssignedTo: "user-assignee",
		BugCreateFields: profile.BugCreateFields{
			Priority:     map[string]string{"high": "prio-high"},
			SeriousLevel: map[string]string{"normal": "sev-normal"},
		},
	})
	t.Setenv(config.EnvAccessToken, "test-token-89-mutex-not-real")
	t.Setenv(config.EnvOrganizationID, "org-89")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, "http://127.0.0.1:9")
	t.Setenv("YUNXIAO_PROFILE", "t89m")

	stdout := withCmdJSONCapture(t)
	globalProfile = "t89m"
	resetStringFlags(t, workitemBugCreateCmd,
		"title", "title-file", "description", "description-file",
		"environment", "module", "priority", "serious-level", "expected-completion", "sprint", "assigned-to", "verifier")
	_ = workitemBugCreateCmd.Flags().Set("minimal", "true")
	t.Cleanup(func() { _ = workitemBugCreateCmd.Flags().Set("minimal", "false") })

	rootCmd.SetArgs([]string{
		"workitem", "+bug-create",
		"--title", "inline",
		"--title-file", "x.txt",
		"--description", "d",
		"--sprint", "sprint-1",
		"--minimal",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected processExit panic")
		}
		if _, ok := r.(exitPanic); !ok {
			panic(r)
		}
		if code != 1 {
			t.Fatalf("exit code=%d", code)
		}
		var env output.Envelope
		if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
			t.Fatalf("stderr: %v / %s", err, stderr.Bytes())
		}
		if env.OK || env.Error == nil {
			t.Fatalf("want ok=false error: %+v", env)
		}
		if !strings.Contains(env.Error.Message, "title-file") {
			t.Fatalf("message=%q", env.Error.Message)
		}
		_ = stdout
	}()
	_ = rootCmd.Execute()
}

func TestBugCreateEmptyTitleFileFails(t *testing.T) {
	prevExit := processExit
	var code int
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:              "t89e",
		OrganizationID:    "org-89",
		SpaceID:           "space-1",
		BugTypeID:         "bug-type-1",
		DefaultAssignedTo: "user-assignee",
		BugCreateFields: profile.BugCreateFields{
			Priority:     map[string]string{"high": "prio-high"},
			SeriousLevel: map[string]string{"normal": "sev-normal"},
		},
	})

	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("empty-title.txt", []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(config.EnvAccessToken, "test-token-89-empty-title-not-real")
	t.Setenv(config.EnvOrganizationID, "org-89")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, "http://127.0.0.1:9")
	t.Setenv("YUNXIAO_PROFILE", "t89e")

	stdout := withCmdJSONCapture(t)
	globalProfile = "t89e"
	resetStringFlags(t, workitemBugCreateCmd,
		"title", "title-file", "description", "description-file",
		"environment", "module", "priority", "serious-level", "expected-completion", "sprint", "assigned-to", "verifier")
	_ = workitemBugCreateCmd.Flags().Set("minimal", "true")
	t.Cleanup(func() { _ = workitemBugCreateCmd.Flags().Set("minimal", "false") })

	rootCmd.SetArgs([]string{
		"workitem", "+bug-create",
		"--title-file", "empty-title.txt",
		"--description", "d",
		"--sprint", "sprint-1",
		"--minimal",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected processExit panic")
		}
		if _, ok := r.(exitPanic); !ok {
			panic(r)
		}
		if code != 1 {
			t.Fatalf("exit code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
		}
		var env output.Envelope
		if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
			t.Fatalf("stderr: %v / %s", err, stderr.Bytes())
		}
		if env.OK || env.Error == nil {
			t.Fatalf("want ok=false error: %+v", env)
		}
		if !strings.Contains(env.Error.Message, "title-file") {
			t.Fatalf("message=%q", env.Error.Message)
		}
	}()
	_ = rootCmd.Execute()
}
