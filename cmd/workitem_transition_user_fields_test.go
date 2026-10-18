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
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

func TestRequireLooksLikeUserID(t *testing.T) {
	if err := requireLooksLikeUserID("--developer", "64083945196808e7b89a5545"); err != nil {
		t.Fatalf("valid id: %v", err)
	}
	err := requireLooksLikeUserID("--developer", "\u8096\u6653\u9716")
	if err == nil {
		t.Fatal("expected error for display name")
	}
	msg := err.Error()
	if !strings.Contains(msg, "\u7528\u6237\u503c\u4e0d\u5408\u6cd5") {
		t.Fatalf("message missing \u7528\u6237\u503c\u4e0d\u5408\u6cd5: %q", msg)
	}
	if !strings.Contains(msg, "+member-resolve") && !strings.Contains(msg, "organization") {
		t.Fatalf("message should hint organization/+member-resolve: %q", msg)
	}
	if !strings.Contains(msg, "\u8096\u6653\u9716") {
		t.Fatalf("message should echo value: %q", msg)
	}
}

func TestValidateUserTypedFieldValues(t *testing.T) {
	known := map[string]string{
		"171ea64c4a9b51542c960a6d7c": "--developer",
		"assignedTo":                 "assignedTo",
	}
	if err := validateUserTypedFieldValues(known, map[string]any{
		"171ea64c4a9b51542c960a6d7c": []string{"64083945196808e7b89a5545"},
		"80":                         "2026-10-31T00:00:00+08:00",
	}); err != nil {
		t.Fatalf("valid: %v", err)
	}
	err := validateUserTypedFieldValues(known, map[string]any{
		"171ea64c4a9b51542c960a6d7c": []any{"\u8096\u6653\u9716"},
	})
	if err == nil || !strings.Contains(err.Error(), "--developer") {
		t.Fatalf("want developer error, got %v", err)
	}
	err = validateUserTypedFieldValues(known, map[string]any{
		"assignedTo": "\u5d14\u5065",
	})
	if err == nil || !strings.Contains(err.Error(), "\u5d14\u5065") {
		t.Fatalf("want assignedTo error, got %v", err)
	}
}

func TestKnownTransitionUserFieldIDs(t *testing.T) {
	pf := &profile.Profile{BugFields: map[string]string{
		"developer":          "fid-dev",
		"responsible_person": "fid-resp",
	}}
	known := knownTransitionUserFieldIDs(pf)
	if known["fid-dev"] == "" || known["fid-resp"] == "" {
		t.Fatalf("missing profile user fields: %+v", known)
	}
	if known["assignedTo"] == "" {
		t.Fatalf("missing native assignedTo: %+v", known)
	}
}

func bugTransitionUserFieldFixture(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "wi-6058",
				"serialNumber":   "ZYPT-6058",
				"spaceId":        "space-1",
				"categoryId":     "Bug",
				"workitemTypeId": "bug-type-1",
				"status":         map[string]any{"id": "st-new", "displayName": "\u5f85\u5904\u7406"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           "p378",
		OrganizationID: "org-378",
		SpaceID:        "space-1",
		BugTypeID:      "bug-type-1",
		BugFields: map[string]string{
			"plan_due_date":      "80",
			"developer":          "171ea64c4a9b51542c960a6d7c",
			"responsible_person": "4086ed55f8852e203dcf3d0042",
		},
		BugStatuses: map[string]string{
			"pending":    "st-new",
			"processing": "st-proc",
		},
		BugEdges: map[string][]string{
			"st-new":  {"st-proc"},
			"st-proc": {},
		},
		BugTransitionRequired: map[string][]string{
			"st-proc": {"80", "171ea64c4a9b51542c960a6d7c"},
		},
	})
	t.Setenv(config.EnvAccessToken, "test-token-378-not-real")
	t.Setenv(config.EnvOrganizationID, "org-378")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "p378")
	return srv, xdg
}

func runBugTransitionExpectFail(t *testing.T, args ...string) (code int, stderr string) {
	t.Helper()
	var errBuf bytes.Buffer
	prevErr := output.Stderr
	prevOut := output.Stdout
	prevJQ := output.JQ
	prevFmt := output.Format
	output.Stderr = &errBuf
	output.Stdout = &bytes.Buffer{}
	output.JQ = ""
	output.Format = "json"
	t.Cleanup(func() {
		output.Stderr = prevErr
		output.Stdout = prevOut
		output.JQ = prevJQ
		output.Format = prevFmt
	})

	prevYes, prevDry, prevProfile, prevOrg := globalYes, globalDryRun, globalProfile, globalOrg
	globalYes, globalDryRun, globalProfile, globalOrg = false, true, "p378", ""
	t.Cleanup(func() {
		globalYes, globalDryRun, globalProfile, globalOrg = prevYes, prevDry, prevProfile, prevOrg
	})

	prevExit := processExit
	var gotCode int
	processExit = func(c int) {
		gotCode = c
		panic(exitPanic{code: c})
	}
	t.Cleanup(func() { processExit = prevExit })

	resetStringFlags(t, workitemBugTransitionCmd, "id", "to", "plan-due-date", "developer", "responsible-person", "bug-reason", "bug-impact-scope", "full", "brief", "direct")
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
		_ = rootCmd.Execute()
	}()
	return gotCode, errBuf.String()
}

// #378: display-name --developer must fail dry-run (same check as real PUT).
func TestBugTransitionDryRunRejectsDeveloperDisplayName(t *testing.T) {
	bugTransitionUserFieldFixture(t)

	code, stderr := runBugTransitionExpectFail(t,
		"workitem", "+bug-transition",
		"--id", "ZYPT-6058",
		"--to", "processing",
		"--plan-due-date", "2026-10-31",
		"--developer", "\u8096\u6653\u9716",
		"--dry-run",
	)
	if code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stderr), &env); err != nil {
		t.Fatalf("stderr JSON: %v / %s", err, stderr)
	}
	if env.OK || env.Error == nil {
		t.Fatalf("want ok:false, got %+v", env)
	}
	msg := env.Error.Message
	if !strings.Contains(msg, "\u7528\u6237\u503c\u4e0d\u5408\u6cd5") || !strings.Contains(msg, "\u8096\u6653\u9716") {
		t.Fatalf("message=%q", msg)
	}
	if !strings.Contains(msg, "member-resolve") && !strings.Contains(msg, "organization") {
		t.Fatalf("hint missing: %q", msg)
	}
}

func TestBugTransitionDryRunAcceptsDeveloperUserID(t *testing.T) {
	bugTransitionUserFieldFixture(t)

	stdout := withCmdJSONCapture(t)
	globalProfile = "p378"
	resetStringFlags(t, workitemBugTransitionCmd, "id", "to", "plan-due-date", "developer", "responsible-person", "bug-reason", "bug-impact-scope", "full", "brief", "direct")
	rootCmd.SetArgs([]string{
		"workitem", "+bug-transition",
		"--id", "ZYPT-6058",
		"--to", "processing",
		"--plan-due-date", "2026-10-31",
		"--developer", "64083945196808e7b89a5545",
		"--dry-run",
	})
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
	provided, _ := req["provided_fields"].(map[string]any)
	v := provided["171ea64c4a9b51542c960a6d7c"]
	arr, ok := v.([]any)
	if !ok || len(arr) != 1 || arr[0] != "64083945196808e7b89a5545" {
		t.Fatalf("provided developer=%#v req=%s", v, raw)
	}
}

func TestTransitionDryRunRejectsUserFieldDisplayNameInFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "wi-1", "serialNumber": "ZYPT-1", "spaceId": "space-1",
				"categoryId": "Bug", "workitemTypeId": "bug-type-1",
				"status": map[string]any{"id": "st-new", "displayName": "\u5f85\u5904\u7406"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           "p378t",
		OrganizationID: "org-378",
		SpaceID:        "space-1",
		BugTypeID:      "bug-type-1",
		BugFields: map[string]string{
			"developer": "171ea64c4a9b51542c960a6d7c",
		},
		Workflows: map[string]profile.WorkitemWorkflow{
			"bug-type-1": {
				Name: "Bug", Category: "Bug",
				Statuses: map[string]string{"\u5f85\u5904\u7406": "st-new", "\u5904\u7406\u4e2d": "st-proc"},
				Edges:    map[string][]string{"st-new": {"st-proc"}, "st-proc": {}},
			},
		},
	})
	t.Setenv(config.EnvAccessToken, "test-token-378-not-real")
	t.Setenv(config.EnvOrganizationID, "org-378")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "p378t")

	var errBuf bytes.Buffer
	prevErr, prevOut, prevJQ, prevFmt := output.Stderr, output.Stdout, output.JQ, output.Format
	output.Stderr, output.Stdout, output.JQ, output.Format = &errBuf, &bytes.Buffer{}, "", "json"
	t.Cleanup(func() {
		output.Stderr, output.Stdout, output.JQ, output.Format = prevErr, prevOut, prevJQ, prevFmt
	})
	prevYes, prevDry, prevProfile, prevOrg := globalYes, globalDryRun, globalProfile, globalOrg
	globalYes, globalDryRun, globalProfile, globalOrg = false, true, "p378t", ""
	t.Cleanup(func() {
		globalYes, globalDryRun, globalProfile, globalOrg = prevYes, prevDry, prevProfile, prevOrg
	})
	prevExit := processExit
	var gotCode int
	processExit = func(c int) { gotCode = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })

	resetStringFlags(t, workitemTransitionCmd, "id", "to", "type-id", "fields")
	rootCmd.SetArgs([]string{
		"workitem", "+transition",
		"--id", "ZYPT-1",
		"--to", "\u5904\u7406\u4e2d",
		"--fields", `{"171ea64c4a9b51542c960a6d7c":["\u8096\u6653\u9716"]}`,
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
	if gotCode != 1 {
		t.Fatalf("exit=%d stderr=%s", gotCode, errBuf.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(errBuf.Bytes(), &env); err != nil || env.OK || env.Error == nil {
		t.Fatalf("env=%+v err=%v stderr=%s", env, err, errBuf.String())
	}
	if !strings.Contains(env.Error.Message, "\u7528\u6237\u503c\u4e0d\u5408\u6cd5") {
		t.Fatalf("message=%q", env.Error.Message)
	}
}
