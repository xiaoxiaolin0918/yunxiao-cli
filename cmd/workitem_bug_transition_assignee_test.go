package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

func TestBugTransitionStepBodyOmitsAssignedTo(t *testing.T) {
	body := bugTransitionStepBody("st-test", map[string]any{
		"dev-fid":    []string{"u-dev"},
		"assignedTo": "u-should-not-appear",
	})
	if _, ok := body["assignedTo"]; ok {
		t.Fatalf("PUT must not implicit-write assignedTo: %v", body)
	}
	if body["status"] != "st-test" || body["dev-fid"] == nil {
		t.Fatalf("%v", body)
	}
}

type bugTransitionPeopleServer struct {
	mu      sync.Mutex
	puts    []map[string]any
	gets    int
	refresh map[string]any
}

func newBugTransitionPeopleServer(t *testing.T, pre, refresh map[string]any) (*httptest.Server, *bugTransitionPeopleServer) {
	t.Helper()
	s := &bugTransitionPeopleServer{refresh: refresh}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet:
			s.gets++
			w.Header().Set("Content-Type", "application/json")
			if s.gets > 1 {
				_ = json.NewEncoder(w).Encode(s.refresh)
				return
			}
			_ = json.NewEncoder(w).Encode(pre)
		case strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodPut:
			raw, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			s.puts = append(s.puts, body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, s
}

func setupBugTransition414Env(t *testing.T, srv *httptest.Server, name string) {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           name,
		OrganizationID: "org-414",
		SpaceID:        "space-1",
		BugTypeID:      "bug-type-1",
		BugStatuses:    map[string]string{"processing": "st-proc", "testing": "st-test"},
		BugEdges:       map[string][]string{"st-proc": {"st-test"}, "st-test": {}},
		BugFields:      map[string]string{"developer": "dev-fid", "responsible_person": "resp-fid"},
	})
	t.Setenv(config.EnvAccessToken, "test-token-414-not-real")
	t.Setenv(config.EnvOrganizationID, "org-414")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", name)
}

func pre414Item() map[string]any {
	return map[string]any{
		"id":           "wi-6598",
		"serialNumber": "ZYPT-6598",
		"subject":      "缺陷 6598",
		"description":  "<p>流转前描述</p>",
		"spaceId":      "space-1",
		"categoryId":   "Bug",
		"status":       map[string]any{"id": "st-proc", "displayName": "待处理"},
		"assignedTo":   map[string]any{"id": "674fb074259532263f38b332", "name": "符宁"},
		"verifier":     map[string]any{"id": "65cc38447d3e6485f086914e", "name": "高扬"},
		"customFieldValues": []any{
			map[string]any{"fieldId": "dev-fid", "values": []any{map[string]any{"identifier": "64083945196808e7b89a5545", "displayValue": "肖晓霖"}}},
		},
	}
}

func TestBugTransitionDryRunPlannedPutsOmitAssignedTo(t *testing.T) {
	pre := pre414Item()
	srv, _ := newBugTransitionPeopleServer(t, pre, pre)
	setupBugTransition414Env(t, srv, "p414d")

	resetStringFlags(t, workitemBugTransitionCmd, "id", "to", "plan-due-date", "developer", "responsible-person", "bug-reason", "bug-impact-scope", "full", "brief", "direct")
	code, stdout, stderr := execWorkitemCmdCapture(t, []string{
		"workitem", "+bug-transition",
		"--profile", "p414d",
		"--id", "ZYPT-6598",
		"--to", "testing",
		"--developer", "64083945196808e7b89a5545",
		"--dry-run",
	}, true, false)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	puts, _ := req["planned_puts"].([]any)
	if len(puts) == 0 {
		t.Fatalf("planned_puts missing: %s", raw)
	}
	for i, p := range puts {
		m, _ := p.(map[string]any)
		body, _ := m["body"].(map[string]any)
		if _, ok := body["assignedTo"]; ok {
			t.Fatalf("planned_puts[%d] must not include assignedTo: %v", i, body)
		}
	}
}

func TestBugTransitionWarnsWhenAssignedToChanges(t *testing.T) {
	pre := pre414Item()
	refresh := pre414Item()
	refresh["status"] = map[string]any{"id": "st-test", "displayName": "待测试"}
	refresh["assignedTo"] = map[string]any{"id": "65cc38447d3e6485f086914e", "name": "高扬"}
	refresh["description"] = "<p>流转后描述</p>"

	srv, rec := newBugTransitionPeopleServer(t, pre, refresh)
	setupBugTransition414Env(t, srv, "p414w")

	resetStringFlags(t, workitemBugTransitionCmd, "id", "to", "plan-due-date", "developer", "responsible-person", "bug-reason", "bug-impact-scope", "full", "brief", "direct")
	code, stdout, stderr := execWorkitemCmdCapture(t, []string{
		"workitem", "+bug-transition",
		"--profile", "p414w",
		"--id", "ZYPT-6598",
		"--to", "testing",
		"--developer", "64083945196808e7b89a5545",
		"--yes",
	}, false, true)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	for i, body := range rec.puts {
		if _, ok := body["assignedTo"]; ok {
			t.Fatalf("PUT[%d] must not include assignedTo: %v", i, body)
		}
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	data, _ := env.Data.(map[string]any)
	changes, _ := data["personnel_changes"].([]any)
	if len(changes) == 0 {
		t.Fatalf("expected personnel_changes, data=%v", data)
	}
	raw, _ := json.Marshal(changes)
	if !strings.Contains(string(raw), "assignedTo") || !strings.Contains(string(raw), "674fb074259532263f38b332") {
		t.Fatalf("changes=%s", raw)
	}
	if !strings.Contains(stderr.String(), "assignedTo 已由 符宁 (674fb074259532263f38b332) 变为 高扬 (65cc38447d3e6485f086914e)（平台或字段联动）") {
		t.Fatalf("stderr missing warning: %s", stderr)
	}
	if !strings.Contains(stderr.String(), "yunxiao workitem update --id ZYPT-6598 --assigned-to 674fb074259532263f38b332") {
		t.Fatalf("stderr missing restore: %s", stderr)
	}
	item, _ := data["item"].(map[string]any)
	if item["verifier"] == nil {
		t.Fatalf("brief item should keep verifier when present: %v", item)
	}
	if _, has := item["description"]; has {
		t.Fatalf("brief still must not carry description: %v", item)
	}
	if env.Meta["personnel_changed"] != true {
		t.Fatalf("meta=%v", env.Meta)
	}
}

func TestBugTransitionNoPersonnelWarningWhenUnchanged(t *testing.T) {
	pre := pre414Item()
	refresh := pre414Item()
	refresh["status"] = map[string]any{"id": "st-test", "displayName": "待测试"}
	// same people; assignedTo given as a bare id to confirm no false positive
	refresh["assignedTo"] = "674fb074259532263f38b332"

	srv, _ := newBugTransitionPeopleServer(t, pre, refresh)
	setupBugTransition414Env(t, srv, "p414n")

	resetStringFlags(t, workitemBugTransitionCmd, "id", "to", "plan-due-date", "developer", "responsible-person", "bug-reason", "bug-impact-scope", "full", "brief", "direct")
	code, stdout, stderr := execWorkitemCmdCapture(t, []string{
		"workitem", "+bug-transition",
		"--profile", "p414n",
		"--id", "ZYPT-6598",
		"--to", "testing",
		"--developer", "64083945196808e7b89a5545",
		"--yes",
	}, false, true)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var env output.Envelope
	_ = json.Unmarshal(stdout.Bytes(), &env)
	data, _ := env.Data.(map[string]any)
	if _, ok := data["personnel_changes"]; ok {
		t.Fatalf("unexpected personnel_changes: %v", data["personnel_changes"])
	}
	if strings.Contains(stderr.String(), "已由") {
		t.Fatalf("unexpected warning: %s", stderr)
	}
	if env.Meta["personnel_changed"] != nil {
		t.Fatalf("meta=%v", env.Meta)
	}
}

func TestBugTransitionHelpDocumentsAssigneeWarning(t *testing.T) {
	for _, want := range []string{"assignedTo", "平台或字段联动", "workitem update --assigned-to", "#414"} {
		if !strings.Contains(workitemBugTransitionCmd.Long, want) {
			t.Fatalf("help missing %q", want)
		}
	}
}

func TestWorkItemGetBriefKeepsVerifierWhenPresent(t *testing.T) {
	got := zhiyi.WorkItemGetBrief(map[string]any{
		"id":         "wi-1",
		"subject":    "x",
		"assignedTo": map[string]any{"id": "u-owner", "name": "符宁"},
		"verifier":   map[string]any{"id": "u-qa", "name": "高扬"},
	})
	v, _ := got["verifier"].(map[string]any)
	if v["id"] != "u-qa" {
		t.Fatalf("verifier=%v", got["verifier"])
	}
	got2 := zhiyi.WorkItemGetBrief(map[string]any{"id": "wi-1", "subject": "x", "verifier": nil})
	if _, ok := got2["verifier"]; ok {
		t.Fatalf("nil verifier must be omitted: %v", got2)
	}
}
