package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)



func TestTesthubCasesCreateMissingAssignedToPrecheck(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-assigned-precheck-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-assigned-precheck")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
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

	resetStringFlags(t, testhubCasesCreateCmd, "repo-id", "subject", "directory-id", "assigned-to", "data", "data-file")
	rootCmd.SetArgs([]string{
		"testhub", "cases", "create",
		"--repo-id", "repo-x",
		"--subject", "no-owner",
		"--dry-run",
	})
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
	if hits != 0 {
		t.Fatalf("must not hit API, hits=%d", hits)
	}
	if code != 1 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	msg := stderr.String() + stdout.String()
	if !strings.Contains(msg, "assignedTo") {
		t.Fatalf("expected assignedTo precheck, got stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

func TestTesthubCasesCreateRefreshCustomCode(t *testing.T) {
	var posts, gets int
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/testcases"):
			posts++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"case-new-1"}`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/testcases/case-new-1"):
			gets++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"case-new-1","customCode":"HEGL-9999","subject":"refreshed"}`))
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":"unexpected ` + r.Method + ` ` + r.URL.Path + `"}`))
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-create-refresh-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-create-refresh")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	globalDryRun = false
	globalYes = true
	resetStringFlags(t, testhubCasesCreateCmd, "repo-id", "subject", "directory-id", "assigned-to", "data", "data-file")
	rootCmd.SetArgs([]string{
		"testhub", "cases", "create",
		"--repo-id", "repo-r",
		"--subject", "title",
		"--assigned-to", "user-1",
		"--yes",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Data)
	var data map[string]any
	_ = json.Unmarshal(raw, &data)
	if data["customCode"] != "HEGL-9999" || data["subject"] != "refreshed" {
		t.Fatalf("data=%v env=%+v raw=%s posts=%d gets=%d seen=%v", data, env, stdout.String(), posts, gets, seen)
	}
	if env.Meta["refresh_ok"] != true {
		t.Fatalf("meta=%v", env.Meta)
	}
	if posts != 1 || gets != 1 {
		t.Fatalf("posts=%d gets=%d", posts, gets)
	}
}

func TestTesthubCasesGetMetaURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","customCode":"HEGL-1","subject":"s"}`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-get-url-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-get-url")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	// no profile space → webPathHint
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	globalDryRun = false
	resetStringFlags(t, testhubCasesGetCmd, "repo-id", "id")
	rootCmd.SetArgs([]string{"testhub", "cases", "get", "--repo-id", "r1", "--id", "c1"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if env.Meta["webPathHint"] == nil && env.Meta["url"] == nil {
		t.Fatalf("meta should have url or webPathHint: meta=%v env=%+v raw=%s", env.Meta, env, stdout.String())
	}
}

func TestTesthubReposListIDAlias(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"testRepoIdentifier":"repo-abc","name":"lib"}]`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-repos-id-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-repos-id")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	globalDryRun = false
	resetStringFlags(t, testhubReposListCmd, "name")
	rootCmd.SetArgs([]string{"testhub", "repos", "list"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	raw, _ := json.Marshal(env.Data)
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("data=%s", raw)
	}
	if len(items) != 1 || items[0]["id"] != "repo-abc" {
		t.Fatalf("items=%v env=%+v raw=%s", items, env, stdout.String())
	}
}
