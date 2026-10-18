package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func TestWorkitemSearchSubjectAllANDConditionsDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-wi-search-subj-all-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-search-subj-all")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemSearchCmd,
		"category", "space-id", "assigned-to", "creator", "subject", "subject-all", "status",
		"status-stage", "type", "priority", "labels", "order-by", "sort",
		"created-after", "created-before", "updated-after", "updated-before",
		"finish-after", "finish-before",
	)
	_ = workitemSearchCmd.Flags().Set("page", "1")
	_ = workitemSearchCmd.Flags().Set("per-page", "20")
	_ = workitemSearchCmd.Flags().Set("all", "false")
	_ = workitemSearchCmd.Flags().Set("as-items", "false")
	_ = workitemSearchCmd.Flags().Set("recent-days", "0")
	rootCmd.SetArgs([]string{
		"workitem", "search",
		"--space-id", "space-subj-all-1",
		"--category", "Bug",
		"--subject-all", "清空,筛选",
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
	if env.Risk != string(risk.Read) && env.Risk != "read" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	body, _ := req["body"].(map[string]any)
	cond, _ := body["conditions"].(string)
	if strings.Count(cond, `"fieldIdentifier":"subject"`) != 2 {
		t.Fatalf("want 2 subject AND filters: %q", cond)
	}
	if !strings.Contains(cond, "清空") || !strings.Contains(cond, "筛选") || !strings.Contains(cond, "CONTAINS") {
		t.Fatalf("conditions missing terms: %q", cond)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestWorkitemSearchRecentDaysMapsUpdatedAfterDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-wi-search-recent-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-search-recent")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemSearchCmd,
		"category", "space-id", "assigned-to", "creator", "subject", "subject-all", "status",
		"status-stage", "type", "priority", "labels", "order-by", "sort",
		"created-after", "created-before", "updated-after", "updated-before",
		"finish-after", "finish-before",
	)
	_ = workitemSearchCmd.Flags().Set("page", "1")
	_ = workitemSearchCmd.Flags().Set("per-page", "20")
	_ = workitemSearchCmd.Flags().Set("all", "false")
	_ = workitemSearchCmd.Flags().Set("as-items", "false")
	_ = workitemSearchCmd.Flags().Set("recent-days", "0")
	rootCmd.SetArgs([]string{
		"workitem", "search",
		"--space-id", "space-recent-1",
		"--category", "Bug",
		"--recent-days", "30",
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
	body, _ := req["body"].(map[string]any)
	cond, _ := body["conditions"].(string)
	if !strings.Contains(cond, `"fieldIdentifier":"gmtModified"`) || !strings.Contains(cond, "BETWEEN") {
		t.Fatalf("conditions missing gmtModified BETWEEN: %q", cond)
	}
	want, err := recentDaysUpdatedAfter(30, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cond, want) {
		t.Fatalf("conditions missing recent bound %q: %q", want, cond)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestWorkitemSearchRecentDaysConflictsUpdatedAfter(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-wi-search-recent-conflict-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-search-recent-conflict")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	var exitCode int
	prevExit := processExit
	processExit = func(c int) { exitCode = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })

	resetStringFlags(t, workitemSearchCmd,
		"category", "space-id", "assigned-to", "creator", "subject", "subject-all", "status",
		"status-stage", "type", "priority", "labels", "order-by", "sort",
		"created-after", "created-before", "updated-after", "updated-before",
		"finish-after", "finish-before",
	)
	_ = workitemSearchCmd.Flags().Set("page", "1")
	_ = workitemSearchCmd.Flags().Set("per-page", "20")
	_ = workitemSearchCmd.Flags().Set("all", "false")
	_ = workitemSearchCmd.Flags().Set("as-items", "false")
	_ = workitemSearchCmd.Flags().Set("recent-days", "0")
	rootCmd.SetArgs([]string{
		"workitem", "search",
		"--space-id", "space-recent-2",
		"--category", "Bug",
		"--recent-days", "7",
		"--updated-after", "2026-01-01 00:00:00",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	var panicVal any
	func() {
		defer func() { panicVal = recover() }()
		_ = rootCmd.Execute()
	}()
	if panicVal == nil {
		t.Fatalf("want processExit panic, stdout=%s", stdout.String())
	}
	if ep, ok := panicVal.(exitPanic); !ok || ep.code != 1 {
		t.Fatalf("panic=%v exitCode=%d", panicVal, exitCode)
	}
	out := stderr.String()
	if !strings.Contains(out, "recent-days") || !strings.Contains(out, "updated-after") {
		t.Fatalf("want mutual exclusion error in stderr: %s (stdout=%s)", out, stdout.String())
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}
