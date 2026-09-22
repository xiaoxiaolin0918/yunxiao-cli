package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

func writeOwnershipTestProfile(t *testing.T, dir string, pf *profile.Profile) {
	t.Helper()
	profDir := filepath.Join(dir, "yunxiao", "profiles")
	if err := os.MkdirAll(profDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(profDir, pf.Name+".json")
	if err := profile.SaveFile(path, pf); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureNumericRepoOwnershipUnit(t *testing.T) {
	if err := ensureNumericRepoOwnership(t.Context(), nil, "alias", "1"); err != nil {
		t.Fatalf("alias skip: %v", err)
	}
	err := ensureNumericRepoOwnership(t.Context(), nil, "999", "999")
	if err == nil || !strings.Contains(err.Error(), "reachable") {
		t.Fatalf("nil client + numeric miss: %v", err)
	}
}

func TestMrsUpdateNumericRepoRejectedWhenNotReachable(t *testing.T) {
	// #63: wrong numeric id must not proceed when not in profile/org inventory.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repositories/") && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorCode":"Repository.NotFound"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeOwnershipTestProfile(t, xdg, &profile.Profile{
		Name:           "t63",
		OrganizationID: "org-63",
		Repositories:   map[string]int64{"sandbox": 7472289},
	})

	t.Setenv(config.EnvAccessToken, "test-token-63-ownership-not-real")
	t.Setenv(config.EnvOrganizationID, "org-63")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "t63")

	stdout := withCmdJSONCapture(t)
	var stderrBuf bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderrBuf
	t.Cleanup(func() { output.Stderr = prevErr })

	globalProfile = "t63"
	t.Cleanup(func() { globalProfile = "" })

	prevExit := processExit
	var gotCode int
	processExit = func(code int) {
		gotCode = code
		panic(exitPanic{code: code})
	}
	t.Cleanup(func() { processExit = prevExit })

	resetStringFlags(t, codeupMrsUpdateCmd, "repo", "local-id", "title", "description", "work-item", "full")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "update",
		"--repo", "9999999",
		"--local-id", "125",
		"--title", "should-not-run",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	var execErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
				}
			}
		}()
		execErr = rootCmd.Execute()
	}()

	combined := stdout.String() + stderrBuf.String()
	if execErr != nil {
		combined += execErr.Error()
	}
	if gotCode != 1 {
		t.Fatalf("expected exit 1, gotCode=%d err=%v out=%s", gotCode, execErr, combined)
	}
	if !strings.Contains(combined, "9999999") || !strings.Contains(strings.ToLower(combined), "reachable") {
		t.Fatalf("expected ownership error about 9999999 reachable, gotCode=%d err=%v out=%s", gotCode, execErr, combined)
	}
}

func TestMrsUpdateNumericRepoAllowedWhenInProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repositories/") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "7472289", "name": "sandbox"})
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeOwnershipTestProfile(t, xdg, &profile.Profile{
		Name:           "t63ok",
		OrganizationID: "org-63",
		Repositories:   map[string]int64{"sandbox": 7472289},
	})

	t.Setenv(config.EnvAccessToken, "test-token-63-ok-not-real")
	t.Setenv(config.EnvOrganizationID, "org-63")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "t63ok")

	stdout := withCmdJSONCapture(t)
	globalProfile = "t63ok"
	t.Cleanup(func() { globalProfile = "" })
	resetStringFlags(t, codeupMrsUpdateCmd, "repo", "local-id", "title", "description", "work-item", "full")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "update",
		"--repo", "7472289",
		"--local-id", "125",
		"--title", "WIP: docs",
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
		t.Fatalf("envelope: %+v / %s", env, stdout.String())
	}
}

func TestMrsUpdateNumericRepoAllowedViaOrgGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repositories/") && !strings.Contains(r.URL.Path, "/changeRequests") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "4951320", "name": "demo"})
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-63-org-not-real")
	t.Setenv(config.EnvOrganizationID, "org-63-org")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	globalProfile = ""
	resetStringFlags(t, codeupMrsUpdateCmd, "repo", "local-id", "title", "description", "work-item", "full")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "update",
		"--repo", "4951320",
		"--local-id", "125",
		"--title", "WIP: docs",
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
		t.Fatalf("envelope: %+v / %s", env, stdout.String())
	}
}

func TestMrsUpdateAliasSkipsNumericOwnership(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Alias path skips ownership GET; dry-run PUT preview needs no network beyond client init.
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeOwnershipTestProfile(t, xdg, &profile.Profile{
		Name:           "t63alias",
		OrganizationID: "org-63",
		Repositories:   map[string]int64{"sandbox": 7472289},
	})

	t.Setenv(config.EnvAccessToken, "test-token-63-alias-not-real")
	t.Setenv(config.EnvOrganizationID, "org-63")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "t63alias")

	stdout := withCmdJSONCapture(t)
	globalProfile = "t63alias"
	t.Cleanup(func() { globalProfile = "" })
	resetStringFlags(t, codeupMrsUpdateCmd, "repo", "local-id", "title", "description", "work-item", "full")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "update",
		"--repo", "sandbox",
		"--local-id", "125",
		"--title", "WIP: docs",
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
		t.Fatalf("envelope: %+v / %s", env, stdout.String())
	}
}
