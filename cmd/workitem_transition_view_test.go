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

// TestTransitionItemViewFromFlags is the unit table for the #114 view resolver:
// flag > YUNXIAO_WORKITEM_GET_VIEW > default brief, mutual exclusion, invalid env.
func TestTransitionItemViewFromFlags(t *testing.T) {
	cmd := workitemTransitionCmd
	resetStringFlags(t, cmd, "full", "brief")

	setFlags := func(t *testing.T, flags ...string) {
		t.Helper()
		resetStringFlags(t, cmd, "full", "brief")
		for _, f := range flags {
			if err := cmd.Flags().Set(f, "true"); err != nil {
				t.Fatal(err)
			}
		}
	}

	cases := []struct {
		name    string
		flags   []string
		env     string // YUNXIAO_WORKITEM_GET_VIEW
		wantMode string
		wantSrc  string
		wantErr  string // "" | "mutual" | "invalid_env"
	}{
		{name: "default brief", wantMode: "brief", wantSrc: "default"},
		{name: "full flag", flags: []string{"full"}, wantMode: "full", wantSrc: "flag"},
		{name: "brief flag", flags: []string{"brief"}, wantMode: "brief", wantSrc: "flag"},
		{name: "full flag overrides env full", flags: []string{"full"}, env: "brief", wantMode: "full", wantSrc: "flag"},
		{name: "env full", env: "full", wantMode: "full", wantSrc: "env"},
		{name: "env brief", env: "brief", wantMode: "brief", wantSrc: "env"},
		{name: "env case-insensitive", env: "FULL", wantMode: "full", wantSrc: "env"},
		{name: "env empty", env: " ", wantMode: "brief", wantSrc: "default"},
		{name: "mutually exclusive flags", flags: []string{"full", "brief"}, wantErr: "mutual"},
		{name: "invalid env", env: "pretty", wantErr: "invalid_env"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("YUNXIAO_WORKITEM_GET_VIEW", tc.env)
			setFlags(t, tc.flags...)
			view, err := transitionItemViewFromFlags(cmd)
			switch tc.wantErr {
			case "mutual":
				if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
					t.Fatalf("err = %v", err)
				}
			case "invalid_env":
				de, ok := err.(*detailedError)
				if !ok || de.Subtype != "invalid_env" {
					t.Fatalf("err = %v (%T)", err, err)
				}
			default:
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				if view.mode != tc.wantMode || view.source != tc.wantSrc {
					t.Fatalf("view = %+v want mode=%s source=%s", view, tc.wantMode, tc.wantSrc)
				}
			}
		})
	}
}

// t114View is the workflow used by the #114 command-level tests.
func t114View() profile.WorkitemWorkflow {
	return profile.WorkitemWorkflow{
		Name:     "产品类需求",
		Category: "Req",
		Statuses: map[string]string{"待处理": "st-pending", "待测试": "st-test"},
		Edges:    map[string][]string{"st-pending": {"st-test"}, "st-test": {}},
	}
}

func t114Server(t *testing.T, refreshStatus int) (*httptest.Server, *int) {
	t.Helper()
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch {
		case strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet:
			// First GET = pre-PUT item (待处理); later GETs = refresh (待测试 when OK).
			if refreshStatus == http.StatusOK && requests > 2 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":           "wi-114",
					"serialNumber": "ZYPT-114",
					"subject":      "需求 114",
					"description":  "<p>刷后的超长描述</p>",
					"customFieldValues": []any{
						map[string]any{"fieldId": "f80", "values": []any{"x"}},
					},
					"status": map[string]any{"id": "st-test", "displayName": "待测试"},
				})
				return
			}
			if refreshStatus != http.StatusOK && requests > 2 {
				w.WriteHeader(refreshStatus)
				_, _ = w.Write([]byte(`{"errorMsg":"refresh boom"}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "wi-114",
				"serialNumber":   "ZYPT-114",
				"spaceId":        map[string]any{"id": "space-1"},
				"categoryId":     "Req",
				"workitemTypeId": "type-req-1",
				"subject":        "需求 114",
				"description":    "<p>流转前的超长描述</p>",
				"status":         map[string]any{"id": "st-pending", "displayName": "待处理"},
			})
		case strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

func setupTransition114Env(t *testing.T, srv *httptest.Server, name string) {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           name,
		OrganizationID: "org-114",
		SpaceID:        "space-1",
		Workflows:      map[string]profile.WorkitemWorkflow{"type-req-1": t114View()},
	})
	t.Setenv(config.EnvAccessToken, "test-token-114-not-real")
	t.Setenv(config.EnvOrganizationID, "org-114")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
}

// TestTransitionSuccessBriefViewByDefault locks the #114 success output: brief item
// projection + from/to status displayNames by default, raw object with --full / env,
// env validated before any request.
func TestTransitionSuccessBriefViewByDefault(t *testing.T) {
	cases := []struct {
		name        string
		extraArgs   []string
		env         string // YUNXIAO_WORKITEM_GET_VIEW
		wantBrief   bool
		wantInvalid bool
	}{
		{name: "brief default", wantBrief: true},
		{name: "explicit --brief", extraArgs: []string{"--brief"}, wantBrief: true},
		{name: "--full raw object", extraArgs: []string{"--full"}},
		{name: "env full raw object", env: "full"},
		{name: "invalid env fails before requests", env: "pretty", wantInvalid: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			noRetrySleep(t)
			srv, requests := t114Server(t, http.StatusOK)
			setupTransition114Env(t, srv, "t114")
			t.Setenv("YUNXIAO_WORKITEM_GET_VIEW", tc.env)

			resetStringFlags(t, workitemTransitionCmd, "id", "to", "type-id", "fields", "full", "brief")
			args := append([]string{
				"workitem", "+transition",
				"--profile", "t114",
				"--id", "ZYPT-114",
				"--to", "待测试",
				"--yes",
			}, tc.extraArgs...)
			code, stdout, stderr := execWorkitemCmdCapture(t, args, false, true)

			if tc.wantInvalid {
				if code != 1 {
					t.Fatalf("exit = %d stdout = %s", code, stdout.String())
				}
				var env output.Envelope
				if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
					t.Fatal(err)
				}
				if env.OK || env.Error == nil || env.Error.Subtype != "invalid_env" {
					t.Fatalf("envelope = %+v stderr = %s", env, stderr.String())
				}
				if *requests != 0 {
					t.Fatalf("invalid env must fail before any request, got %d", *requests)
				}
				return
			}

			if code != 0 {
				t.Fatalf("exit = %d stdout = %s stderr = %s", code, stdout.String(), stderr.String())
			}
			var env output.Envelope
			if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
				t.Fatal(err)
			}
			if !env.OK {
				t.Fatalf("envelope = %+v stdout = %s", env, stdout.String())
			}
			data, _ := env.Data.(map[string]any)
			if data == nil {
				t.Fatalf("data = %v", env.Data)
			}
			item, _ := data["item"].(map[string]any)
			if item == nil {
				t.Fatalf("item = %v", data["item"])
			}
			_, hasDescription := item["description"]
			if hasDescription == tc.wantBrief {
				t.Fatalf("description presence = %v want %v (item = %v)", hasDescription, tc.wantBrief, item)
			}
			if _, has := data["refresh_ok"]; !has || data["refresh_ok"] != true {
				t.Fatalf("refresh_ok = %v (must survive, #114)", data["refresh_ok"])
			}
			if data["serial_number"] != "ZYPT-114" || item["serialNumber"] != "ZYPT-114" {
				t.Fatalf("serial numbers = %v / %v", data["serial_number"], item["serialNumber"])
			}
			if u, _ := data["url"].(string); u == "" {
				t.Fatalf("data.url missing")
			}
			if u, _ := env.Meta["url"].(string); u == "" {
				t.Fatalf("meta.url missing")
			}
			// from → to display names only in brief mode; --full keeps pre-#114 keys.
			fs, hasFrom := data["from_status"].(map[string]any)
			if tc.wantBrief {
				if !hasFrom || fs["id"] != "st-pending" || fs["displayName"] != "待处理" {
					t.Fatalf("from_status = %v", data["from_status"])
				}
				ts, _ := data["to_status"].(map[string]any)
				if ts == nil || ts["id"] != "st-test" || ts["displayName"] != "待测试" {
					t.Fatalf("to_status = %v", data["to_status"])
				}
				if env.Meta["projection"] != "brief" {
					t.Fatalf("meta.projection = %v", env.Meta["projection"])
				}
				st, _ := item["status"].(map[string]any)
				if st == nil || st["displayName"] != "待测试" {
					t.Fatalf("brief item status = %v", item["status"])
				}
			} else {
				if hasFrom {
					t.Fatalf("--full must not add from_status: %v", fs)
				}
				if _, has := env.Meta["projection"]; has {
					t.Fatalf("--full meta must stay untouched: %v", env.Meta)
				}
				if item["subject"] != "需求 114" {
					t.Fatalf("full item passthrough = %v", item)
				}
			}
		})
	}
}

// TestTransitionBriefViewRefreshFailure: refresh_ok stays false with the stderr
// warning; the brief item falls back to the pre-PUT payload and to_status resolves
// the displayName from the profile statuses map (#114).
func TestTransitionBriefViewRefreshFailure(t *testing.T) {
	noRetrySleep(t)
	srv, _ := t114Server(t, http.StatusInternalServerError)
	setupTransition114Env(t, srv, "t114r")

	resetStringFlags(t, workitemTransitionCmd, "id", "to", "type-id", "fields", "full", "brief")
	code, stdout, stderr := execWorkitemCmdCapture(t, []string{
		"workitem", "+transition",
		"--profile", "t114r",
		"--id", "ZYPT-114",
		"--to", "待测试",
		"--yes",
	}, false, true)
	if code != 0 {
		t.Fatalf("exit = %d stdout = %s", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning: transition succeeded but refresh failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("envelope = %+v", env)
	}
	data, _ := env.Data.(map[string]any)
	if data["refresh_ok"] != false {
		t.Fatalf("refresh_ok = %v", data["refresh_ok"])
	}
	item, _ := data["item"].(map[string]any)
	if item == nil || item["serialNumber"] != "ZYPT-114" || item["description"] != nil {
		t.Fatalf("brief fallback item = %v", item)
	}
	ts, _ := data["to_status"].(map[string]any)
	if ts == nil || ts["id"] != "st-test" || ts["displayName"] != "待测试" {
		t.Fatalf("to_status = %v (reverse-resolved from profile statuses)", data["to_status"])
	}
	if env.Meta["projection"] != "brief" {
		t.Fatalf("meta.projection = %v", env.Meta["projection"])
	}
}

// TestTransitionDryRunProjectionAndNote: dry-run stays read-only and reports the
// chosen output view (#114) plus the entry-required note (#113).
func TestTransitionDryRunProjectionAndNote(t *testing.T) {
	srv, _ := t114Server(t, http.StatusOK)
	setupTransition114Env(t, srv, "t114d")

	resetStringFlags(t, workitemTransitionCmd, "id", "to", "type-id", "fields", "full", "brief")
	code, stdout, _ := execWorkitemCmdCapture(t, []string{
		"workitem", "+transition",
		"--profile", "t114d",
		"--id", "ZYPT-114",
		"--to", "待测试",
		"--dry-run",
	}, true, false)
	if code != 0 {
		t.Fatalf("exit = %d stdout = %s", code, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("envelope = %+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	pj, _ := req["projection"].(map[string]any)
	if pj == nil || pj["mode"] != "brief" || pj["source"] != "default" {
		t.Fatalf("projection = %v req = %s", pj, raw)
	}
	if _, has := req["required_fields_note"]; !has {
		t.Fatalf("required_fields_note missing: %s", raw)
	}
	// --full via flag keeps the pre-#114 dry-run shape (no projection key).
	resetStringFlags(t, workitemTransitionCmd, "id", "to", "type-id", "fields", "full", "brief")
	code, stdout, _ = execWorkitemCmdCapture(t, []string{
		"workitem", "+transition",
		"--profile", "t114d",
		"--id", "ZYPT-114",
		"--to", "待测试",
		"--dry-run",
		"--full",
	}, true, false)
	if code != 0 {
		t.Fatalf("full exit = %d stdout = %s", code, stdout.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(env.Request)
	var reqFull map[string]any
	if err := json.Unmarshal(raw, &reqFull); err != nil {
		t.Fatal(err)
	}
	if _, has := reqFull["projection"]; has {
		t.Fatalf("--full (flag) dry-run must not gain projection: %s", raw)
	}
}

// TestBugTransitionBriefView: +bug-transition gets the same #114 brief treatment.
func TestBugTransitionBriefView(t *testing.T) {
	noRetrySleep(t)
	srv, requests := t114Server(t, http.StatusOK)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           "t114b",
		OrganizationID: "org-114",
		SpaceID:        "space-1",
		BugStatuses:    map[string]string{"待处理": "st-pending", "待测试": "st-test"},
	})
	t.Setenv(config.EnvAccessToken, "test-token-114-not-real")
	t.Setenv(config.EnvOrganizationID, "org-114")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	_ = requests

	resetStringFlags(t, workitemBugTransitionCmd, "id", "to", "plan-due-date", "developer", "responsible-person", "bug-reason", "bug-impact-scope", "full", "brief")
	code, stdout, stderr := execWorkitemCmdCapture(t, []string{
		"workitem", "+bug-transition",
		"--profile", "t114b",
		"--id", "ZYPT-114",
		"--to", "待测试",
		"--yes",
	}, false, true)
	if code != 0 {
		t.Fatalf("exit = %d stdout = %s stderr = %s", code, stdout.String(), stderr.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("envelope = %+v", env)
	}
	data, _ := env.Data.(map[string]any)
	item, _ := data["item"].(map[string]any)
	if item == nil {
		t.Fatalf("item = %v", data["item"])
	}
	if _, has := item["description"]; has {
		t.Fatalf("brief item must not carry description: %v", item)
	}
	fs, _ := data["from_status"].(map[string]any)
	ts, _ := data["to_status"].(map[string]any)
	if fs == nil || fs["displayName"] != "待处理" || ts == nil || ts["displayName"] != "待测试" {
		t.Fatalf("from/to = %v %v", fs, ts)
	}
	if data["refresh_ok"] != true {
		t.Fatalf("refresh_ok = %v", data["refresh_ok"])
	}
	if env.Meta["projection"] != "brief" {
		t.Fatalf("meta.projection = %v", env.Meta["projection"])
	}
}
