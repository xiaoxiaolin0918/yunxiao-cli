package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

func TestBugTransitionDirectFallbackDryRun(t *testing.T) {
	// Both statuses on graph keys, but no BFS path — should plan single-step direct_fallback.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "wi-1",
				"serialNumber":   "ZYPT-1",
				"spaceId":        "space-1",
				"categoryId":     "Bug",
				"workitemTypeId": "bug-type-1",
				"status":         map[string]any{"id": "st-closed", "displayName": "closed"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           "p123",
		OrganizationID: "org-123",
		SpaceID:        "space-1",
		BugTypeID:      "bug-type-1",
		BugStatuses: map[string]string{
			"closed-fixed": "st-closed",
			"confirm":      "st-confirm",
			"processing":   "st-proc",
		},
		BugEdges: map[string][]string{
			"st-closed":  {},
			"st-confirm": {"st-proc"},
			"st-proc":    {"st-confirm"},
		},
	})
	t.Setenv(config.EnvAccessToken, "test-token-123-not-real")
	t.Setenv(config.EnvOrganizationID, "org-123")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "p123")

	stdout := withCmdJSONCapture(t)
	prevProfile, prevDry := globalProfile, globalDryRun
	globalProfile, globalDryRun = "p123", true
	t.Cleanup(func() { globalProfile, globalDryRun = prevProfile, prevDry })

	resetStringFlags(t, workitemBugTransitionCmd, "id", "to", "plan-due-date", "developer", "responsible-person", "bug-reason", "bug-impact-scope", "full", "brief", "direct")
	rootCmd.SetArgs([]string{"workitem", "+bug-transition", "--id", "ZYPT-1", "--to", "confirm", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil || !env.OK || !env.DryRun {
		t.Fatalf("env=%+v err=%v out=%s", env, err, stdout.String())
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	if req["transition_mode"] != "direct_fallback" {
		t.Fatalf("transition_mode=%v req=%s", req["transition_mode"], raw)
	}
	if req["profile_edges"] != "no_path" {
		t.Fatalf("profile_edges=%v", req["profile_edges"])
	}
	steps, _ := req["steps"].([]any)
	if len(steps) != 1 || steps[0] != "st-confirm" {
		t.Fatalf("steps=%v", steps)
	}
	warn, _ := req["warning"].(string)
	if !strings.Contains(warn, "no BFS path") {
		t.Fatalf("warning=%q", warn)
	}
}

func TestBugTransitionDirectFlagSkipsBFS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "wi-1", "serialNumber": "ZYPT-1", "spaceId": "space-1",
				"status": map[string]any{"id": "st-proc", "displayName": "processing"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name: "p123d", OrganizationID: "org-123", SpaceID: "space-1", BugTypeID: "bug-type-1",
		BugStatuses: map[string]string{"processing": "st-proc", "confirm": "st-confirm"},
		BugEdges:    map[string][]string{"st-proc": {"st-confirm"}, "st-confirm": {}},
	})
	t.Setenv(config.EnvAccessToken, "test-token-123d-not-real")
	t.Setenv(config.EnvOrganizationID, "org-123")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "p123d")

	stdout := withCmdJSONCapture(t)
	prevProfile, prevDry := globalProfile, globalDryRun
	globalProfile, globalDryRun = "p123d", true
	t.Cleanup(func() { globalProfile, globalDryRun = prevProfile, prevDry })

	resetStringFlags(t, workitemBugTransitionCmd, "id", "to", "plan-due-date", "developer", "responsible-person", "bug-reason", "bug-impact-scope", "full", "brief", "direct")
	rootCmd.SetArgs([]string{"workitem", "+bug-transition", "--id", "ZYPT-1", "--to", "confirm", "--direct", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var env output.Envelope
	_ = json.Unmarshal(stdout.Bytes(), &env)
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	if req["transition_mode"] != "direct" {
		t.Fatalf("transition_mode=%v req=%s", req["transition_mode"], raw)
	}
}

func TestBugTransitionHelpDocumentsDirectFallback(t *testing.T) {
	if f := workitemBugTransitionCmd.Flags().Lookup("direct"); f == nil {
		t.Fatal("--direct missing")
	}
	for _, want := range []string{"bug_edges", "direct_fallback", "+explore-workflow", "#123"} {
		if !strings.Contains(workitemBugTransitionCmd.Long, want) {
			t.Fatalf("help missing %q", want)
		}
	}
}

func TestAnnotateBugTransitionPutErr(t *testing.T) {
	err := annotateBugTransitionPutErr(fmt.Errorf("http 400"), "direct_fallback", "no path note", "a", "b")
	de, ok := err.(*detailedError)
	if !ok || de.Subtype != "platform_rejected_transition" {
		t.Fatalf("%T %+v", err, err)
	}
	if !strings.Contains(de.Message, "platform rejected") || !strings.Contains(de.Message, "no path note") {
		t.Fatalf("message=%q", de.Message)
	}
}
