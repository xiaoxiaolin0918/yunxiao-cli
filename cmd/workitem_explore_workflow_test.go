package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	bugBody := buildExploreCreateBody(context.Background(), pf, nil, "space", "bug-t", "Bug")
	cf, _ := bugBody["customFieldValues"].(map[string]any)
	if cf["priority"] != "p-high" || cf["seriousLevel"] != "s-normal" {
		t.Fatalf("bug body cf=%v body=%v", cf, bugBody)
	}
	reqBody := buildExploreCreateBody(context.Background(), pf, nil, "space", "req-t", "Req")
	if _, ok := reqBody["customFieldValues"]; ok {
		t.Fatalf("Req must not inject Bug fields: %v", reqBody)
	}
}

func TestExploreWorkflowDryRunAutoOverridesCategory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/workitemTypes") && !strings.Contains(r.URL.Path, "/workflows") && r.Method == http.MethodGet:
			cat := r.URL.Query().Get("category")
			if cat == "Req" {
				_ = json.NewEncoder(w).Encode([]any{map[string]any{"id": "req-type-1", "name": "产品类需求"}})
				return
			}
			_ = json.NewEncoder(w).Encode([]any{})
		case strings.Contains(r.URL.Path, "/workflows") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "wf-1", "name": "req",
				"statuses": []any{
					map[string]any{"id": "s1", "name": "待处理", "displayName": "待处理"},
				},
			})
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":"unexpected"}`))
		}
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	profDir := filepath.Join(xdg, "yunxiao", "profiles")
	if err := os.MkdirAll(profDir, 0o700); err != nil {
		t.Fatal(err)
	}
	pf := &profile.Profile{Name: "play60", SpaceID: "space-1", OrganizationID: "org-60"}
	if err := profile.SaveFile(filepath.Join(profDir, "play60.json"), pf); err != nil {
		t.Fatal(err)
	}

	t.Setenv(config.EnvAccessToken, "test-token-explore-60")
	t.Setenv(config.EnvOrganizationID, "org-60")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv(profile.EnvProfile, "play60")

	stdout := withCmdJSONCapture(t)
	globalDryRun = true
	resetStringFlags(t, workitemExploreWorkflowCmd, "type-id", "space-id", "id", "category", "type-name")
	rootCmd.SetArgs([]string{
		"workitem", "+explore-workflow",
		"--profile", "play60",
		"--type-id", "req-type-1",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
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
		t.Fatalf("expected auto-override category=Req, plan=%v", plan)
	}
	cp, _ := plan["create_probe"].(map[string]any)
	body, _ := cp["body"].(map[string]any)
	if cf, ok := body["customFieldValues"]; ok {
		t.Fatalf("Req probe must not have Bug customFieldValues: %v", cf)
	}
}
