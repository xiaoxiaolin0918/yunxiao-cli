package cmd

import (
	"fmt"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

func resetStringFlags(t *testing.T, cmd *cobra.Command, names ...string) {
	t.Helper()
	for _, name := range names {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			continue
		}
		_ = cmd.Flags().Set(name, f.DefValue)
		f.Changed = false
	}
}

func withCmdJSONCapture(t *testing.T) *bytes.Buffer {
	t.Helper()
	var stdout bytes.Buffer
	prevOut := output.Stdout
	prevErr := output.Stderr
	prevJQ := output.JQ
	prevFmt := output.Format
	output.Stdout = &stdout
	output.Stderr = &bytes.Buffer{}
	output.JQ = ""
	output.Format = "json"
	t.Cleanup(func() {
		output.Stdout = prevOut
		output.Stderr = prevErr
		output.JQ = prevJQ
		output.Format = prevFmt
	})
	prevYes := globalYes
	prevDry := globalDryRun
	prevProfile := globalProfile
	prevOrg := globalOrg
	globalYes = false
	globalDryRun = true
	globalProfile = ""
	globalOrg = ""
	t.Cleanup(func() {
		globalYes = prevYes
		globalDryRun = prevDry
		globalProfile = prevProfile
		globalOrg = prevOrg
	})
	return &stdout
}

func TestMrsLinkRegisteredAndHelp(t *testing.T) {
	var buf bytes.Buffer
	codeupMrsLinkCmd.SetOut(&buf)
	codeupMrsLinkCmd.SetErr(&buf)
	_ = codeupMrsLinkCmd.Help()
	s := buf.String()
	if !strings.Contains(s, "extRelationRecords") {
		t.Fatalf("help missing extRelationRecords: %s", s)
	}
	found := false
	for _, c := range codeupMrsCmd.Commands() {
		if c.Name() == "link" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("link not registered on mrs")
	}
}

func TestMrsUnlinkRegisteredAndHelp(t *testing.T) {
	var buf bytes.Buffer
	codeupMrsUnlinkCmd.SetOut(&buf)
	codeupMrsUnlinkCmd.SetErr(&buf)
	_ = codeupMrsUnlinkCmd.Help()
	s := buf.String()
	if !strings.Contains(s, "relationRecordId") {
		t.Fatalf("help missing relationRecordId: %s", s)
	}
}


func TestCollectWorkItemRefs(t *testing.T) {
	got := collectWorkItemRefs("a, b", []string{"b", "c"})
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("%#v", got)
	}
	if collectWorkItemRefs("", nil) != nil && len(collectWorkItemRefs("", nil)) != 0 {
		t.Fatal(collectWorkItemRefs("", nil))
	}
}

func TestMrsLinkDryRunBody(t *testing.T) {
	var gotPaths []string
	var gotMethods []string
	var lastBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		gotMethods = append(gotMethods, r.Method)
		switch {
		case strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet && !strings.Contains(r.URL.Path, "extRelationRecords"):
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "wi-internal-1", "serialNumber": "ZYPT-1"})
		case strings.Contains(r.URL.Path, "/changeRequests/") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"localId":      125,
				"title":        "feat: x",
				"sourceBranch": "feat/x",
				"targetBranch": "master",
				"detailUrl":    "https://example/mr/125",
				"projectId":    "4951320",
			})
		case strings.Contains(r.URL.Path, "extRelationRecords") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"unexpected ` + r.Method + ` ` + r.URL.Path + `"}`))
		}
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&lastBody)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-link-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-link-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsLinkCmd, "repo", "local-id", "work-item")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "link",
		"--repo", "4951320",
		"--local-id", "125",
		"--work-item", "ZYPT-1",
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
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/extRelationRecords") {
		t.Fatalf("url=%q", url)
	}
	body, _ := req["body"].(map[string]any)
	if body["category"] != "codeupMergeRequest" {
		t.Fatalf("body=%#v", body)
	}
	if body["mergeRequestId"] != "125" {
		t.Fatalf("mergeRequestId=%#v", body["mergeRequestId"])
	}
	if body["projectId"] != "4951320" {
		t.Fatalf("projectId=%#v", body["projectId"])
	}
	// Ensure we did not POST for real during dry-run
	for _, m := range gotMethods {
		if m == http.MethodPost || m == http.MethodDelete {
			t.Fatalf("mutating method during dry-run: %v paths=%v", gotMethods, gotPaths)
		}
	}
}

func TestMrsUnlinkDryRunBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet && !strings.Contains(r.URL.Path, "extRelationRecords"):
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "wi-internal-1", "serialNumber": "ZYPT-1"})
		case strings.Contains(r.URL.Path, "extRelationRecords") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]any{
				map[string]any{
					"businessId":       "125",
					"projectId":        "4951320",
					"category":         "codeupMergeRequest",
					"relationRecordId": "rr-abc",
				},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-unlink-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-unlink-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsUnlinkCmd, "repo", "local-id", "work-item")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "unlink",
		"--repo", "4951320",
		"--local-id", "125",
		"--work-item", "ZYPT-1",
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
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	if req["method"] != "DELETE" {
		t.Fatalf("method=%v", req["method"])
	}
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/extRelationRecords/rr-abc") {
		t.Fatalf("url=%q", url)
	}
}

func TestMrsUpdateWorkItemOnlyDryRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet && !strings.Contains(r.URL.Path, "extRelationRecords"):
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "wi-internal-1", "serialNumber": "ZYPT-1"})
		case strings.Contains(r.URL.Path, "/changeRequests/") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"localId": 125, "title": "t", "projectId": "4951320"})
		case strings.Contains(r.URL.Path, "extRelationRecords") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]any{})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-update-wi-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-update-wi-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsUpdateCmd, "repo", "local-id", "title", "description", "work-item", "full")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "update",
		"--repo", "4951320",
		"--local-id", "125",
		"--work-item", "ZYPT-1",
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
	if req["method"] != "POST" {
		t.Fatalf("work-item-only update dry-run should preview POST link, got %#v", req)
	}
}


func TestMrsUpdateCombinedDryRunIncludesLink(t *testing.T) {
	var gotMethods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethods = append(gotMethods, r.Method)
		switch {
		case strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet && !strings.Contains(r.URL.Path, "extRelationRecords"):
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "wi-internal-1", "serialNumber": "ZYPT-1"})
		case strings.Contains(r.URL.Path, "/changeRequests/") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"localId": 125, "title": "old", "projectId": "4951320"})
		case strings.Contains(r.URL.Path, "extRelationRecords") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]any{})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-update-combined-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-update-combined-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsUpdateCmd, "repo", "local-id", "title", "description", "work-item", "full")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "update",
		"--repo", "4951320",
		"--local-id", "125",
		"--title", "WIP: docs",
		"--work-item", "ZYPT-1",
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
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	if _, ok := req["put"]; !ok {
		t.Fatalf("combined dry-run missing put: %#v", req)
	}
	if _, ok := req["link"]; !ok {
		t.Fatalf("combined dry-run missing link: %#v", req)
	}
	for _, m := range gotMethods {
		if m == http.MethodPut || m == http.MethodPost || m == http.MethodDelete {
			t.Fatalf("mutating method during combined dry-run: %v", gotMethods)
		}
	}
}

func TestFailWorkItemLinkNoStdoutSuccess(t *testing.T) {
	prevOut := output.Stdout
	prevErr := output.Stderr
	var stdout, stderr bytes.Buffer
	output.Stdout = &stdout
	output.Stderr = &stderr
	output.JQ = ""
	output.Format = "json"
	t.Cleanup(func() {
		output.Stdout = prevOut
		output.Stderr = prevErr
	})

	err := failWorkItemLink(fmt.Errorf("work item link(s) missing"), map[string]any{"missing": []string{"wi-1"}}, map[string]any{"to_link": 1})
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout must stay empty on fail, got %s", stdout.String())
	}
	var env output.Envelope
	if e := json.Unmarshal(stderr.Bytes(), &env); e != nil {
		t.Fatalf("stderr: %v / %s", e, stderr.String())
	}
	if env.OK || env.Error == nil || env.Error.Subtype != "work_item_link" {
		t.Fatalf("expected ok=false work_item_link, got %#v", env)
	}
}
