package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

// Bug-create field fixture: priority is satisfied by BuildCreateBugArgs; mod-1 and
// extra-req are required create-visible fields that --minimal does not send.
const bugCreateFieldsFixture = `[
 {"id":"subject","name":"标题","type":"NativeField","format":"string","required":true,"showWhenCreate":true},
 {"id":"assignedTo","name":"负责人","type":"NativeField","format":"user","required":true,"showWhenCreate":true},
 {"id":"sprint","name":"迭代","type":"NativeField","format":"sprint","required":true,"showWhenCreate":true},
 {"id":"priority","name":"优先级","type":"SystemCustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"prio-high","value":"高","displayValue":"高"},{"id":"prio-low","value":"低","displayValue":"低"}]},
 {"id":"seriousLevel","name":"严重程度","type":"SystemCustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"sev-normal","value":"一般","displayValue":"一般"}]},
 {"id":"mod-1","name":"所属模块","type":"CustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"m-a","value":"MES","displayValue":"MES"}]},
 {"id":"extra-req","name":"额外必填","type":"CustomField","format":"string","required":true,"showWhenCreate":true},
 {"id":"src","name":"来源","type":"CustomField","format":"list","required":true,"showWhenCreate":true,"defaultValue":"src-1"}
]`

type bugCreateServer struct {
	mu         sync.Mutex
	fields     string
	fieldsStat int
	postStat   int
	postBody   string
	fieldsGETs int
	posts      []map[string]any
}

func newBugCreateServer(t *testing.T, fields string) *bugCreateServer {
	t.Helper()
	s := &bugCreateServer{fields: fields}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/workitemTypes/") && strings.HasSuffix(r.URL.Path, "/fields"):
			s.fieldsGETs++
			if s.fieldsStat != 0 {
				w.WriteHeader(s.fieldsStat)
				_, _ = io.WriteString(w, `{"errorCode":"Forbidden","errorMessage":"no permission"}`)
				return
			}
			_, _ = io.WriteString(w, s.fields)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/workitems"):
			b, _ := io.ReadAll(r.Body)
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			s.posts = append(s.posts, m)
			if s.postStat != 0 {
				w.WriteHeader(s.postStat)
				_, _ = io.WriteString(w, s.postBody)
				return
			}
			_, _ = io.WriteString(w, `{"id":"bug-1","serialNumber":"ZYPT-1","subject":"缺陷","status":{"displayName":"待处理"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:              "p107",
		OrganizationID:    "org-107",
		SpaceID:           "space-1",
		BugTypeID:         "bug-type-1",
		DefaultAssignedTo: "user-assignee",
		BugCreateFields: profile.BugCreateFields{
			Priority:     map[string]string{"high": "prio-high"},
			SeriousLevel: map[string]string{"normal": "sev-normal"},
		},
	})
	t.Setenv(config.EnvAccessToken, "test-token-107-bug-create-precheck-not-real")
	t.Setenv(config.EnvOrganizationID, "org-107")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "p107")
	return s
}

type bugCreateRun struct {
	stdout, stderr string
	code           int
}

func runBugCreate(t *testing.T, dryRun bool, extra ...string) bugCreateRun {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	prevDry := globalDryRun
	globalDryRun = dryRun
	t.Cleanup(func() { globalDryRun = prevDry })
	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	prevExit := processExit
	code := 0
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })

	prevProfile := globalProfile
	prevYes := globalYes
	if globalProfile == "" {
		globalProfile = "p107"
	}
	if !dryRun {
		globalYes = true
	}
	t.Cleanup(func() { globalProfile = prevProfile; globalYes = prevYes })

	resetStringFlags(t, workitemBugCreateCmd,
		"title", "title-file", "description", "description-file",
		"environment", "module", "priority", "serious-level", "expected-completion",
		"sprint", "assigned-to", "verifier", "minimal", "no-defaults", "no-precheck")

	args := []string{
		"workitem", "+bug-create",
		"--title", "缺陷标题",
		"--description", "描述",
		"--sprint", "sprint-1",
		"--minimal",
		"--no-defaults",
	}
	if dryRun {
		args = append(args, "--dry-run")
	} else {
		args = append(args, "--yes")
	}
	args = append(args, extra...)
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
			t.Fatalf("execute: %v", err)
		}
	}()
	return bugCreateRun{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func TestBugCreatePrecheckReportsAllMissing(t *testing.T) {
	s := newBugCreateServer(t, bugCreateFieldsFixture)
	r := runBugCreate(t, false)
	if r.code != 1 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", r.code, r.stdout, r.stderr)
	}
	eb := wiErrorBody(t, r.stderr)
	if eb.Type != "cli" || eb.Subtype != "missing_required_fields" {
		t.Fatalf("error=%+v", eb)
	}
	missing, _ := eb.Details["missing"].([]any)
	if len(missing) < 2 {
		t.Fatalf("want >=2 missing, got %#v message=%s", eb.Details, eb.Message)
	}
	ids := map[string]bool{}
	for _, m := range missing {
		ids[m.(map[string]any)["field_id"].(string)] = true
	}
	if !ids["mod-1"] || !ids["extra-req"] {
		t.Fatalf("missing ids=%v", ids)
	}
	if len(s.posts) != 0 || s.fieldsGETs != 1 {
		t.Fatalf("posts=%d fieldsGETs=%d", len(s.posts), s.fieldsGETs)
	}
}

func TestBugCreatePrecheckDryRunReportsAllMissing(t *testing.T) {
	s := newBugCreateServer(t, bugCreateFieldsFixture)
	r := runBugCreate(t, true)
	if r.code != 1 {
		t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
	}
	eb := wiErrorBody(t, r.stderr)
	if eb.Type != "cli" || eb.Subtype != "missing_required_fields" {
		t.Fatalf("error=%+v", eb)
	}
	if len(s.posts) != 0 || s.fieldsGETs != 1 {
		t.Fatalf("posts=%d fieldsGETs=%d", len(s.posts), s.fieldsGETs)
	}
}

func TestBugCreatePrecheckPassesWhenComplete(t *testing.T) {
	// Drop the two extra required fields so BuildCreateBugArgs body is complete.
	complete := `[
 {"id":"subject","name":"标题","type":"NativeField","format":"string","required":true,"showWhenCreate":true},
 {"id":"assignedTo","name":"负责人","type":"NativeField","format":"user","required":true,"showWhenCreate":true},
 {"id":"sprint","name":"迭代","type":"NativeField","format":"sprint","required":true,"showWhenCreate":true},
 {"id":"priority","name":"优先级","type":"SystemCustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"prio-high","value":"高","displayValue":"高"}]},
 {"id":"seriousLevel","name":"严重程度","type":"SystemCustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"sev-normal","value":"一般","displayValue":"一般"}]},
 {"id":"src","name":"来源","type":"CustomField","format":"list","required":true,"showWhenCreate":true,"defaultValue":"src-1"}
]`
	s := newBugCreateServer(t, complete)
	r := runBugCreate(t, false)
	if r.code != 0 || len(s.posts) != 1 {
		t.Fatalf("exit=%d posts=%d stderr=%s", r.code, len(s.posts), r.stderr)
	}
	pc, _ := wiEnvelope(t, r.stdout).Meta["precheck"].(map[string]any)
	if pc["status"] != "ok" || pc["source"] != "fields" {
		t.Fatalf("meta.precheck=%#v", pc)
	}
	skipped, _ := pc["skipped_default"].([]any)
	if len(skipped) != 1 || skipped[0] != "src" {
		t.Fatalf("skipped_default=%#v", skipped)
	}
}

func TestBugCreatePrecheckDryRunShowsOK(t *testing.T) {
	complete := `[
 {"id":"subject","name":"标题","type":"NativeField","format":"string","required":true,"showWhenCreate":true},
 {"id":"assignedTo","name":"负责人","type":"NativeField","format":"user","required":true,"showWhenCreate":true},
 {"id":"sprint","name":"迭代","type":"NativeField","format":"sprint","required":true,"showWhenCreate":true},
 {"id":"priority","name":"优先级","type":"SystemCustomField","format":"list","required":true,"showWhenCreate":true},
 {"id":"seriousLevel","name":"严重程度","type":"SystemCustomField","format":"list","required":true,"showWhenCreate":true}
]`
	s := newBugCreateServer(t, complete)
	r := runBugCreate(t, true)
	if r.code != 0 || len(s.posts) != 0 {
		t.Fatalf("exit=%d posts=%d stderr=%s", r.code, len(s.posts), r.stderr)
	}
	req := wiDryRunRequest(t, r.stdout)
	pc, _ := req["precheck"].(map[string]any)
	if pc["status"] != "ok" {
		t.Fatalf("request.precheck=%#v", pc)
	}
}

func TestBugCreateNoPrecheckSkipsLookup(t *testing.T) {
	s := newBugCreateServer(t, bugCreateFieldsFixture)
	r := runBugCreate(t, false, "--no-precheck")
	if r.code != 0 || len(s.posts) != 1 || s.fieldsGETs != 0 {
		t.Fatalf("exit=%d posts=%d fieldsGETs=%d stderr=%s", r.code, len(s.posts), s.fieldsGETs, r.stderr)
	}
	if _, ok := wiEnvelope(t, r.stdout).Meta["precheck"]; ok {
		t.Fatalf("--no-precheck must not add meta.precheck")
	}
}

func TestBugCreatePrecheckUnauthorizedFails(t *testing.T) {
	for _, dry := range []bool{false, true} {
		s := newBugCreateServer(t, bugCreateFieldsFixture)
		s.fieldsStat = http.StatusUnauthorized
		r := runBugCreate(t, dry)
		eb := wiErrorBody(t, r.stderr)
		if r.code != 1 || eb.Type != "api" || eb.Code != http.StatusUnauthorized || len(s.posts) != 0 {
			t.Fatalf("dry=%v code=%d error=%+v posts=%d", dry, r.code, eb, len(s.posts))
		}
	}
}

func TestBugCreatePrecheckDegradesWhenFieldsUnavailable(t *testing.T) {
	s := newBugCreateServer(t, bugCreateFieldsFixture)
	s.fieldsStat = http.StatusForbidden
	r := runBugCreate(t, false)
	if r.code != 0 || len(s.posts) != 1 {
		t.Fatalf("exit=%d posts=%d stderr=%s", r.code, len(s.posts), r.stderr)
	}
	pc, _ := wiEnvelope(t, r.stdout).Meta["precheck"].(map[string]any)
	if pc["status"] != "skipped" || pc["source"] != "none" {
		t.Fatalf("meta.precheck=%#v", pc)
	}
	if !strings.HasPrefix(r.stderr, "warning: ") {
		t.Fatalf("stderr=%q", r.stderr)
	}
}

func TestBugCreatePrecheckEmptyConfig(t *testing.T) {
	s := newBugCreateServer(t, `[]`)
	r := runBugCreate(t, false)
	if r.code != 0 || len(s.posts) != 1 {
		t.Fatalf("exit=%d posts=%d stderr=%s", r.code, len(s.posts), r.stderr)
	}
	pc, _ := wiEnvelope(t, r.stdout).Meta["precheck"].(map[string]any)
	if pc["status"] != "empty" || pc["source"] != "none" {
		t.Fatalf("meta.precheck=%#v", pc)
	}
}

func TestBugCreatePrecheckProfileFallback(t *testing.T) {
	s := newBugCreateServer(t, bugCreateFieldsFixture)
	s.fieldsStat = http.StatusForbidden
	xdg := os.Getenv("XDG_CONFIG_HOME")
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:              "p107",
		OrganizationID:    "org-107",
		SpaceID:           "space-1",
		BugTypeID:         "bug-type-1",
		DefaultAssignedTo: "user-assignee",
		BugCreateFields: profile.BugCreateFields{
			Priority:     map[string]string{"high": "prio-high"},
			SeriousLevel: map[string]string{"normal": "sev-normal"},
		},
		WorkitemDefaults: map[string]profile.WorkitemTypeDefaults{
			"bug-type-1": {Name: "缺陷", CreateRequired: []string{"subject", "mod-1", "extra-req"}},
		},
	})

	r := runBugCreate(t, false)
	if r.code != 0 || len(s.posts) != 1 {
		t.Fatalf("exit=%d posts=%d stdout=%s stderr=%s", r.code, len(s.posts), r.stdout, r.stderr)
	}
	pc, _ := wiEnvelope(t, r.stdout).Meta["precheck"].(map[string]any)
	mp, _ := pc["profile_missing"].([]any)
	if pc["status"] != "skipped" || pc["source"] != "profile_fallback" || len(mp) != 2 {
		t.Fatalf("meta.precheck=%#v", pc)
	}
}

func TestBugCreateHelpDocumentsPrecheck(t *testing.T) {
	if f := workitemBugCreateCmd.Flags().Lookup("no-precheck"); f == nil {
		t.Fatal("--no-precheck flag missing")
	}
	for _, want := range []string{"precheck", "--no-precheck", "missing_required_fields"} {
		if !strings.Contains(workitemBugCreateCmd.Long, want) {
			t.Fatalf("help missing %q", want)
		}
	}
}
