package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

// #385: types list without --space-id falls back to profile.space_id.
func TestWorkitemTypesListFallsBackToProfileSpaceID(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-types-space-385-not-real")
	t.Setenv(config.EnvOrganizationID, "org-types-space-385")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	pf := &profile.Profile{Name: "typespace", OrganizationID: "org-types-space-385", SpaceID: "space-from-profile"}
	if _, err := pf.Save(); err != nil {
		t.Fatalf("save profile: %v", err)
	}
	t.Setenv(profile.EnvProfile, "typespace")
	prevProfile := globalProfile
	globalProfile = ""
	t.Cleanup(func() { globalProfile = prevProfile })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemTypesListCmd, "space-id", "category")
	rootCmd.SetArgs([]string{
		"workitem", "types", "list",
		"--category", "Bug",
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
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/projects/space-from-profile/workitemTypes") {
		t.Fatalf("expected profile space in url, got %q", url)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestWorkitemTypesListMissingSpaceIDErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-types-nospace-385-not-real")
	t.Setenv(config.EnvOrganizationID, "org-types-nospace-385")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	prevProfile := globalProfile
	globalProfile = ""
	t.Cleanup(func() { globalProfile = prevProfile })

	stdout := withCmdJSONCapture(t)
	var stderr strings.Builder
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	prevExit := processExit
	code := 0
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })

	resetStringFlags(t, workitemTypesListCmd, "space-id", "category")
	rootCmd.SetArgs([]string{"workitem", "types", "list", "--category", "Bug", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
				}
			}
		}()
		_ = rootCmd.Execute()
	}()
	if code != 1 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "space-id") {
		t.Fatalf("stderr should mention space-id: %s", stderr.String())
	}
}

func TestWorkitemTypesListFlagOverridesProfile(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-types-flag-385-not-real")
	t.Setenv(config.EnvOrganizationID, "org-types-flag-385")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	pf := &profile.Profile{Name: "typeflag", OrganizationID: "org-types-flag-385", SpaceID: "space-profile"}
	if _, err := pf.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	t.Setenv(profile.EnvProfile, "typeflag")
	prevProfile := globalProfile
	globalProfile = ""
	t.Cleanup(func() { globalProfile = prevProfile })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemTypesListCmd, "space-id", "category")
	rootCmd.SetArgs([]string{
		"workitem", "types", "list",
		"--space-id", "space-flag",
		"--category", "Bug",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v", err)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/projects/space-flag/workitemTypes") {
		t.Fatalf("flag should win: url=%q", url)
	}
	if strings.Contains(url, "space-profile") {
		t.Fatalf("profile should not win: url=%q", url)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}
