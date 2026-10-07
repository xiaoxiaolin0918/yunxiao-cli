package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

func TestCategoryFromProfile(t *testing.T) {
	pf := &profile.Profile{
		BugTypeID: "bug-type-1",
		WorkitemDefaults: map[string]profile.WorkitemTypeDefaults{
			"req-type-1": {Category: "Req", Name: "产品类需求"},
		},
		Workflows: map[string]profile.WorkitemWorkflow{
			"task-type-1": {Category: "Task"},
		},
	}
	if got := categoryFromProfile(pf, "req-type-1"); got != "Req" {
		t.Fatalf("defaults: %q", got)
	}
	if got := categoryFromProfile(pf, "task-type-1"); got != "Task" {
		t.Fatalf("workflows: %q", got)
	}
	if got := categoryFromProfile(pf, "bug-type-1"); got != "Bug" {
		t.Fatalf("bug_type_id: %q", got)
	}
	if got := categoryFromProfile(pf, "unknown"); got != "" {
		t.Fatalf("unknown: %q", got)
	}
	if got := categoryFromProfile(nil, "req-type-1"); got != "" {
		t.Fatalf("nil: %q", got)
	}
}

func TestResolveExploreCategory_AutoOverrideDefault(t *testing.T) {
	cat, note, err := resolveExploreCategory("Bug", false, "Req", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cat != "Req" {
		t.Fatalf("cat=%q", cat)
	}
	if note == "" || !strings.Contains(note, "Req") {
		t.Fatalf("note=%q", note)
	}
}

func TestResolveExploreCategory_ExplicitMismatchErrors(t *testing.T) {
	_, _, err := resolveExploreCategory("Bug", true, "Req", nil)
	if err == nil || !strings.Contains(err.Error(), "Req") || !strings.Contains(err.Error(), "Bug") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveExploreCategory_UnresolvedDefaultErrors(t *testing.T) {
	_, _, err := resolveExploreCategory("Bug", false, "", fmt.Errorf("not found"))
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "categor") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveExploreCategory_UnresolvedExplicitKeepsFlag(t *testing.T) {
	cat, note, err := resolveExploreCategory("Req", true, "", fmt.Errorf("not found"))
	if err != nil {
		t.Fatal(err)
	}
	if cat != "Req" || note == "" {
		t.Fatalf("cat=%q note=%q", cat, note)
	}
}

func TestResolveExploreCategory_MatchNoNote(t *testing.T) {
	cat, note, err := resolveExploreCategory("Req", false, "Req", nil)
	if err != nil || cat != "Req" || note != "" {
		t.Fatalf("cat=%q note=%q err=%v", cat, note, err)
	}
}

func TestTypeIDInWorkitemTypesList(t *testing.T) {
	raw := []any{
		map[string]any{"id": "abc", "name": "x"},
		map[string]any{"id": "9uy29901re573f561d69jn40", "name": "产品类需求"},
	}
	if !typeIDInWorkitemTypesList(raw, "9uy29901re573f561d69jn40") {
		t.Fatal("expected match")
	}
	if typeIDInWorkitemTypesList(raw, "nope") {
		t.Fatal("unexpected match")
	}
	wrapped := map[string]any{"items": raw}
	if !typeIDInWorkitemTypesList(wrapped, "abc") {
		t.Fatal("wrapped items")
	}
}

func TestLookupTypeCategoryAPI(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/workitemTypes") {
			w.WriteHeader(404)
			return
		}
		cat := r.URL.Query().Get("category")
		seen = append(seen, cat)
		switch cat {
		case "Req":
			_ = json.NewEncoder(w).Encode([]any{
				map[string]any{"id": "req-1", "name": "产品类需求"},
			})
		case "Bug":
			_ = json.NewEncoder(w).Encode([]any{
				map[string]any{"id": "bug-1", "name": "缺陷"},
			})
		default:
			_ = json.NewEncoder(w).Encode([]any{})
		}
	}))
	t.Cleanup(srv.Close)

	c := &client.Client{BaseURL: srv.URL, Token: "t", OrgID: "org", UserAgent: "test", HTTP: srv.Client()}
	got, err := lookupTypeCategoryAPI(context.Background(), c, "space-1", "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Req" {
		t.Fatalf("got=%q seen=%v", got, seen)
	}
}

func TestBuildExploreCreateBody_BugFieldsOnlyForBug(t *testing.T) {
	pf := &profile.Profile{
		DefaultAssignedTo: "u1",
		BugCreateFields: profile.BugCreateFields{
			Priority:     map[string]string{"high": "p-high"},
			SeriousLevel: map[string]string{"normal": "s-normal"},
		},
	}
	bugBody := buildExploreCreateBody(context.Background(), pf, nil, "space", "bug-t", "Bug", nil)
	cf, _ := bugBody["customFieldValues"].(map[string]any)
	if cf["priority"] != "p-high" || cf["seriousLevel"] != "s-normal" {
		t.Fatalf("bug body cf=%v body=%v", cf, bugBody)
	}
	reqBody := buildExploreCreateBody(context.Background(), pf, nil, "space", "req-t", "Req", nil)
	if _, ok := reqBody["customFieldValues"]; ok {
		t.Fatalf("Req must not inject Bug fields: %v", reqBody)
	}
}

// runExploreWorkflowCmd executes +explore-workflow with the given args and
// global toggles; returns stdout, stderr and the captured processExit code
// (0 when the command completed without exiting).
func runExploreWorkflowCmd(t *testing.T, dryRun bool, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	prevOut, prevErr := output.Stdout, output.Stderr
	prevJQ, prevFmt := output.JQ, output.Format
	output.Stdout = &stdout
	output.Stderr = &stderr
	output.JQ = ""
	output.Format = "json"
	t.Cleanup(func() {
		output.Stdout = prevOut
		output.Stderr = prevErr
		output.JQ = prevJQ
		output.Format = prevFmt
	})

	prevYes, prevDry, prevProfile, prevOrg := globalYes, globalDryRun, globalProfile, globalOrg
	globalYes = false
	globalDryRun = dryRun
	globalProfile = ""
	globalOrg = ""
	t.Cleanup(func() {
		globalYes = prevYes
		globalDryRun = prevDry
		globalProfile = prevProfile
		globalOrg = prevOrg
	})

	prevExit := processExit
	code := 0
	processExit = func(c int) {
		code = c
		panic(exitPanic{code: c})
	}
	t.Cleanup(func() { processExit = prevExit })

	resetStringFlags(t, workitemExploreWorkflowCmd, "type-id", "space-id", "id", "category", "type-name", "custom-fields", "fields", "from")
	for _, b := range []string{"cleanup", "write-profile"} {
		if f := workitemExploreWorkflowCmd.Flags().Lookup(b); f != nil {
			_ = workitemExploreWorkflowCmd.Flags().Set(b, f.DefValue)
			f.Changed = false
		}
	}
	rootCmd.SetArgs(args)
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
				}
			}
		}()
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("execute: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
		}
	}()
	return stdout.String(), stderr.String(), code
}

// TestExploreWorkflowDryRunIsFullyOffline locks #110: the --dry-run plan is
// built with zero API requests — no types-list category lookup, no workflow
// GET, no /platform/user self resolution. Category comes from the active
// profile (workitem_defaults) and plan.network_reads is always 0.
func TestExploreWorkflowDryRunIsFullyOffline(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"dry-run must not call the API"}`))
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	profDir := filepath.Join(xdg, "yunxiao", "profiles")
	if err := os.MkdirAll(profDir, 0o700); err != nil {
		t.Fatal(err)
	}
	pf := &profile.Profile{
		Name:           "play110",
		SpaceID:        "space-110",
		OrganizationID: "org-110",
		WorkitemDefaults: map[string]profile.WorkitemTypeDefaults{
			"req-type-110": {Category: "Req"},
		},
	}
	if err := profile.SaveFile(filepath.Join(profDir, "play110.json"), pf); err != nil {
		t.Fatal(err)
	}

	t.Setenv(config.EnvAccessToken, "test-token-explore-110")
	t.Setenv(config.EnvOrganizationID, "org-110")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv(profile.EnvProfile, "play110")

	stdout, stderr, code := runExploreWorkflowCmd(t, true,
		"workitem", "+explore-workflow",
		"--type-id", "req-type-110",
		"--dry-run",
	)
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if n := requests.Load(); n != 0 {
		t.Fatalf("dry-run sent %d API request(s), want 0", n)
	}

	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout)
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("envelope: %+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var plan map[string]any
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if plan["category"] != "Req" {
		t.Fatalf("expected category=Req from profile workitem_defaults, plan=%v", plan)
	}
	if note, _ := plan["category_note"].(string); note == "" || !strings.Contains(note, "Req") {
		t.Fatalf("expected auto-override note, got %q", note)
	}
	if nr, _ := plan["network_reads"].(float64); nr != 0 {
		t.Fatalf("plan.network_reads=%v, want 0", plan["network_reads"])
	}
	cp, _ := plan["create_probe"].(map[string]any)
	if cp == nil {
		t.Fatalf("missing create_probe preview: %v", plan)
	}
	body, _ := cp["body"].(map[string]any)
	if cf, ok := body["customFieldValues"]; ok {
		t.Fatalf("Req probe must not have Bug customFieldValues: %v", cf)
	}
	if body["assignedTo"] != "self" {
		t.Fatalf("offline preview must keep assignedTo=self (resolved only in the real run): %v", body["assignedTo"])
	}
	if _, ok := plan["statuses"]; ok {
		t.Fatalf("offline plan must not carry fetched statuses: %v", plan["statuses"])
	}
}

// TestExploreWorkflowWriteProfileWithoutProfileExitsBeforeRequests locks #110:
// --write-profile (with or without --yes / --dry-run) and no active profile
// must exit 1 before any API call — the mock server sees zero requests of any
// method, so probes can never be created/transitioned/deleted before the error.
func TestExploreWorkflowWriteProfileWithoutProfileExitsBeforeRequests(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv(config.EnvAccessToken, "test-token-explore-110")
	t.Setenv(config.EnvOrganizationID, "org-110")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)

	for _, tc := range []struct {
		name   string
		dryRun bool
		extra  []string
	}{
		{"real_run", false, []string{"--yes"}},
		{"dry_run", true, []string{"--dry-run"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{
				"workitem", "+explore-workflow",
				"--type-id", "req-type-110",
				"--space-id", "space-110",
				"--category", "Req",
				"--write-profile",
			}, tc.extra...)
			stdout, stderr, code := runExploreWorkflowCmd(t, tc.dryRun, args...)
			if code != 1 {
				t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
			}
			if n := requests.Load(); n != 0 {
				t.Fatalf("sent %d API request(s) before failing on --write-profile, want 0", n)
			}
			if !strings.Contains(stderr, "--write-profile requires active --profile") {
				t.Fatalf("stderr missing profile requirement: %s", stderr)
			}
			var env output.Envelope
			if err := json.Unmarshal([]byte(stderr), &env); err != nil {
				t.Fatalf("stderr not JSON envelope: %v / %s", err, stderr)
			}
			if env.OK {
				t.Fatalf("expected ok=false envelope: %s", stderr)
			}
		})
	}
}

func TestBuildExploreCreateBodyOfflineKeepsSelf(t *testing.T) {
	pf := &profile.Profile{DefaultAssignedTo: "u1"}
	if got := buildExploreCreateBodyOffline(pf, "sp", "t", "Bug", nil)["assignedTo"]; got != "u1" {
		t.Fatalf("profile assignee must be kept: %v", got)
	}
	// No profile: "self" stays literal; no client is involved, so any network
	// attempt would panic here (nil client) — proving the offline path.
	if got := buildExploreCreateBodyOffline(nil, "sp", "t", "Req", nil)["assignedTo"]; got != "self" {
		t.Fatalf("offline preview must keep assignedTo=self: %v", got)
	}
}

func TestBuildExploreCreateBody_CustomFieldsAndDefaults(t *testing.T) {
	pf := &profile.Profile{
		DefaultAssignedTo: "u1",
		WorkitemDefaults: map[string]profile.WorkitemTypeDefaults{
			"req-t": {
				Category: "Req",
				Fields: map[string]profile.WorkitemDefaultField{
					"priority": {Value: "p-default"},
				},
			},
		},
	}
	body := buildExploreCreateBody(context.Background(), pf, nil, "space", "req-t", "Req", map[string]any{
		"sprint": "s-1",
	})
	cf, _ := body["customFieldValues"].(map[string]any)
	if cf["sprint"] != "s-1" {
		t.Fatalf("custom fields not applied: %v", body)
	}
	if cf["priority"] != "p-default" {
		t.Fatalf("workitem_defaults not applied after custom fields: %v", body)
	}
	// Flag should win over defaults when same key already set
	body2 := buildExploreCreateBody(context.Background(), pf, nil, "space", "req-t", "Req", map[string]any{
		"priority": "p-flag",
	})
	cf2, _ := body2["customFieldValues"].(map[string]any)
	if cf2["priority"] != "p-flag" {
		t.Fatalf("custom-fields should win over defaults: %v", body2)
	}
}
