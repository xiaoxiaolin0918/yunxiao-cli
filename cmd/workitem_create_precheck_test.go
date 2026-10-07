package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

// #95 fixture (GetWorkitemTypeFieldConfig shape): 优先级 + 所属模块 are user-required;
// status / creator / hidden / server-defaulted / optional fields must not be reported.
const wiFieldsFixture = `[
 {"id":"subject","name":"标题","type":"NativeField","format":"string","required":true,"showWhenCreate":true},
 {"id":"assignedTo","name":"负责人","type":"NativeField","format":"user","required":true,"showWhenCreate":true},
 {"id":"status","name":"状态","type":"NativeField","format":"list","required":true,"showWhenCreate":true},
 {"id":"creator","name":"创建人","type":"NativeField","format":"user","required":true},
 {"id":"priority","name":"优先级","type":"SystemCustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"prio-high","value":"高","displayValue":"高"},{"id":"prio-low","value":"低","displayValue":"低"}]},
 {"id":"mod-1","name":"所属模块","type":"CustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"m-a","value":"MES","displayValue":"MES"}]},
 {"id":"note","name":"备注","type":"CustomField","format":"text","required":false,"showWhenCreate":true},
 {"id":"hidden-req","name":"内部","type":"CustomField","format":"string","required":true,"showWhenCreate":false},
 {"id":"src","name":"来源","type":"CustomField","format":"list","required":true,"showWhenCreate":true,"defaultValue":"src-1"}
]`

type wiCreateServer struct {
	mu           sync.Mutex
	fields       string
	fieldsStatus int    // non-zero: fields GET answers with this status
	retryAfter   string // Retry-After header on fields GET errors
	postStatus   int    // non-zero: POST answers with this status
	postBody     string
	fieldsGETs   int
	fieldsHang   atomic.Bool // fields GET waits until the client gives up
	posts        []map[string]any
	other        []string
}

func newWiCreateServer(t *testing.T, fields string) *wiCreateServer {
	t.Helper()
	s := &wiCreateServer{fields: fields}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.fieldsHang.Load() && r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/fields") {
			select {
			case <-r.Context().Done():
			case <-time.After(10 * time.Second):
			}
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/space-1/workitemTypes/type-req/fields"):
			s.fieldsGETs++
			if s.fieldsStatus != 0 {
				if s.retryAfter != "" {
					w.Header().Set("Retry-After", s.retryAfter)
				}
				w.WriteHeader(s.fieldsStatus)
				_, _ = io.WriteString(w, `{"errorCode":"Forbidden","errorMessage":"no permission"}`)
				return
			}
			_, _ = io.WriteString(w, s.fields)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/workitems"):
			b, _ := io.ReadAll(r.Body)
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			s.posts = append(s.posts, m)
			if s.postStatus != 0 {
				w.WriteHeader(s.postStatus)
				_, _ = io.WriteString(w, s.postBody)
				return
			}
			_, _ = io.WriteString(w, `{"id":"wi-1","serialNumber":"ZYPT-1","subject":"需求","status":{"displayName":"待处理"}}`)
		default:
			s.other = append(s.other, r.Method+" "+r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-wi-precheck-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-precheck")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return s
}

type wiCreateRun struct {
	stdout, stderr string
	code           int
}

// runWiCreate runs `workitem create` for space-1 / type-req with subject + assignee
// plus extra args (profile defaults disabled unless useDefaults).
func runWiCreate(t *testing.T, dryRun, useDefaults bool, extra ...string) wiCreateRun {
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

	resetStringFlags(t, workitemCreateCmd,
		"space-id", "type-id", "subject", "subject-file", "assigned-to",
		"description", "description-file", "custom-fields", "custom-fields-file", "priority",
		"format-type", "parent-id", "sprint", "labels", "participants", "trackers", "verifier", "versions",
		"no-defaults", "no-precheck", "full")
	args := []string{"workitem", "create", "--space-id", "space-1", "--type-id", "type-req", "--subject", "需求", "--assigned-to", "u1"}
	if !useDefaults {
		args = append(args, "--no-defaults")
	}
	if dryRun {
		args = append(args, "--dry-run")
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
	return wiCreateRun{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// wiErrorBody decodes the stderr JSON error envelope, skipping "warning: ..." lines
// (degraded precheck).
func wiErrorBody(t *testing.T, stderr string) output.ErrorBody {
	t.Helper()
	var keep []string
	for _, l := range strings.SplitAfter(stderr, "\n") {
		if !strings.HasPrefix(l, "warning: ") {
			keep = append(keep, l)
		}
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(strings.Join(keep, "")), &env); err != nil || env.Error == nil {
		t.Fatalf("stderr JSON: %v / %s", err, stderr)
	}
	return *env.Error
}

func wiEnvelope(t *testing.T, stdout string) output.Envelope {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout)
	}
	return env
}

func wiDryRunRequest(t *testing.T, stdout string) map[string]any {
	t.Helper()
	env := wiEnvelope(t, stdout)
	if !env.OK || !env.DryRun {
		t.Fatalf("envelope: %s", stdout)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	return req
}

func assertMissingPrecheckError(t *testing.T, r wiCreateRun) {
	t.Helper()
	if r.code != 1 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", r.code, r.stdout, r.stderr)
	}
	eb := wiErrorBody(t, r.stderr)
	if eb.Type != "cli" || eb.Subtype != "missing_required_fields" {
		t.Fatalf("error body=%+v", eb)
	}
	// Context: what failed, for which type, and every missing field (name + id) in one message.
	for _, want := range []string{"workitem create precheck", "type-req", "2 required field", "优先级 (priority)", "所属模块 (mod-1)"} {
		if !strings.Contains(eb.Message, want) {
			t.Fatalf("message missing %q: %s", want, eb.Message)
		}
	}
	for _, want := range []string{"--custom-fields", "--custom-fields-file", "yunxiao workitem fields --space-id space-1 --type-id type-req", "--no-precheck"} {
		if !strings.Contains(eb.Hint, want) {
			t.Fatalf("hint missing %q: %s", want, eb.Hint)
		}
	}
	missing, _ := eb.Details["missing"].([]any)
	if len(missing) != 2 {
		t.Fatalf("details.missing=%#v", eb.Details)
	}
	first := missing[0].(map[string]any)
	opts, _ := first["options"].([]any)
	if first["field_id"] != "priority" || first["name"] != "优先级" || first["pass_via"] != "customFieldValues" || len(opts) != 2 {
		t.Fatalf("first missing=%#v", first)
	}
	if o := opts[0].(map[string]any); o["id"] != "prio-high" || o["display_value"] != "高" {
		t.Fatalf("option=%#v", o)
	}
	if missing[1].(map[string]any)["field_id"] != "mod-1" {
		t.Fatalf("second missing=%#v", missing[1])
	}
	if eb.Details["space_id"] != "space-1" || eb.Details["type_id"] != "type-req" {
		t.Fatalf("details=%#v", eb.Details)
	}
}

// #95 acceptance: missing 所属模块 + 优先级 → one error listing both; nothing POSTed.
func TestWorkitemCreatePrecheckReportsAllMissing(t *testing.T) {
	s := newWiCreateServer(t, wiFieldsFixture)
	r := runWiCreate(t, false, false)
	assertMissingPrecheckError(t, r)
	if len(s.posts) != 0 || s.fieldsGETs != 1 {
		t.Fatalf("posts=%d fieldsGETs=%d other=%v", len(s.posts), s.fieldsGETs, s.other)
	}
	if r.stdout != "" {
		t.Fatalf("stdout must be empty on failure: %s", r.stdout)
	}
}

func TestWorkitemCreatePrecheckDryRunReportsAllMissing(t *testing.T) {
	s := newWiCreateServer(t, wiFieldsFixture)
	r := runWiCreate(t, true, false)
	assertMissingPrecheckError(t, r)
	if len(s.posts) != 0 || s.fieldsGETs != 1 {
		t.Fatalf("posts=%d fieldsGETs=%d", len(s.posts), s.fieldsGETs)
	}
}

// A blank value for a required field is still missing; the provided one is not reported.
func TestWorkitemCreatePrecheckBlankValueIsMissing(t *testing.T) {
	s := newWiCreateServer(t, wiFieldsFixture)
	r := runWiCreate(t, false, false, "--custom-fields", `{"priority":" ","mod-1":"m-a"}`)
	if r.code != 1 {
		t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
	}
	eb := wiErrorBody(t, r.stderr)
	missing, _ := eb.Details["missing"].([]any)
	if len(missing) != 1 || missing[0].(map[string]any)["field_id"] != "priority" || !strings.Contains(eb.Message, "1 required field") {
		t.Fatalf("error=%+v", eb)
	}
	if len(s.posts) != 0 {
		t.Fatalf("posts=%d", len(s.posts))
	}
}

// Complete input: no false positive; explicit values are sent unchanged; meta.precheck=ok.
func TestWorkitemCreatePrecheckPassesWhenComplete(t *testing.T) {
	s := newWiCreateServer(t, wiFieldsFixture)
	r := runWiCreate(t, false, false, "--custom-fields", `{"priority":"prio-low","mod-1":"m-a","note":"x"}`)
	if r.code != 0 {
		t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
	}
	if len(s.posts) != 1 || s.fieldsGETs != 1 {
		t.Fatalf("posts=%d fieldsGETs=%d", len(s.posts), s.fieldsGETs)
	}
	cf, _ := s.posts[0]["customFieldValues"].(map[string]any)
	if len(cf) != 3 || cf["priority"] != "prio-low" || cf["mod-1"] != "m-a" || cf["note"] != "x" || s.posts[0]["subject"] != "需求" {
		t.Fatalf("post=%#v", s.posts[0])
	}
	env := wiEnvelope(t, r.stdout)
	pc, _ := env.Meta["precheck"].(map[string]any)
	if !env.OK || pc["status"] != "ok" || pc["source"] != "fields" || pc["required_checked"] != float64(4) {
		t.Fatalf("envelope=%s", r.stdout)
	}
	// 来源 (src) has a server defaultValue: skipped, but listed.
	if sd, _ := pc["skipped_default"].([]any); len(sd) != 1 || sd[0] != "src" {
		t.Fatalf("skipped_default=%#v", pc["skipped_default"])
	}
	if _, ok := pc["warning"]; ok || r.stderr != "" {
		t.Fatalf("no warning expected: meta=%#v stderr=%q", pc, r.stderr)
	}
}

// --custom-fields-file / --description-file values count as provided (dry-run: one GET, no POST).
func TestWorkitemCreatePrecheckFileInputsSatisfy(t *testing.T) {
	fixture := strings.Replace(wiFieldsFixture, `{"id":"note","name":"备注","type":"CustomField","format":"text","required":false`,
		`{"id":"description","name":"描述","type":"NativeField","format":"text","required":true,"showWhenCreate":true},
 {"id":"note","name":"备注","type":"CustomField","format":"text","required":false`, 1)
	s := newWiCreateServer(t, fixture)
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	bom := []byte{0xEF, 0xBB, 0xBF}
	_ = os.WriteFile("cf.json", append(bom, []byte(`{"priority":"prio-high","mod-1":"m-a"}`)...), 0o600)
	_ = os.WriteFile("desc.md", append(bom, []byte("详细说明")...), 0o600)
	r := runWiCreate(t, true, false, "--custom-fields-file", "cf.json", "--description-file", "desc.md")
	if r.code != 0 {
		t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
	}
	req := wiDryRunRequest(t, r.stdout)
	pc, _ := req["precheck"].(map[string]any)
	if pc["status"] != "ok" || pc["required_checked"] != float64(5) {
		t.Fatalf("request.precheck=%#v", req["precheck"])
	}
	if s.fieldsGETs != 1 || len(s.posts) != 0 {
		t.Fatalf("fieldsGETs=%d posts=%d", s.fieldsGETs, len(s.posts))
	}
	// Same file inputs without the description → description reported with its flags.
	resetStringFlags(t, workitemCreateCmd, "description-file")
	r = runWiCreate(t, true, false, "--custom-fields-file", "cf.json")
	eb := wiErrorBody(t, r.stderr)
	missing, _ := eb.Details["missing"].([]any)
	if r.code != 1 || len(missing) != 1 || missing[0].(map[string]any)["pass_via"] != "--description / --description-file" {
		t.Fatalf("code=%d error=%+v", r.code, eb)
	}
}

// Profile workitem_defaults are applied before the precheck, so a defaulted priority is not missing.
func TestWorkitemCreatePrecheckAfterProfileDefaults(t *testing.T) {
	s := newWiCreateServer(t, wiFieldsFixture)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name: "p95", OrganizationID: "org-wi-precheck", SpaceID: "space-1",
		WorkitemDefaults: map[string]profile.WorkitemTypeDefaults{
			"type-req": {Name: "产品需求", Fields: map[string]profile.WorkitemDefaultField{"priority": {Value: "prio-high"}}},
		},
	})
	t.Setenv("YUNXIAO_PROFILE", "p95")
	globalProfile = "p95"
	r := runWiCreate(t, false, true, "--custom-fields", `{"mod-1":"m-a"}`)
	if r.code != 0 {
		t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
	}
	cf, _ := s.posts[0]["customFieldValues"].(map[string]any)
	if cf["priority"] != "prio-high" || cf["mod-1"] != "m-a" {
		t.Fatalf("post=%#v", s.posts[0])
	}
}

// Degrade: fields unavailable (403 / 5xx / odd payload) never blocks create; meta + warning say why.
func TestWorkitemCreatePrecheckDegradesWhenFieldsUnavailable(t *testing.T) {
	t.Cleanup(client.SetRetrySleepForTest(func(ctx context.Context, d time.Duration) error { return nil }))
	cases := []struct {
		name, fields, reason string
		status               int
	}{
		{"forbidden", wiFieldsFixture, "HTTP 403", http.StatusForbidden},
		{"server-error", wiFieldsFixture, "HTTP 500", http.StatusInternalServerError},
		{"unexpected-payload", `{"foo":1}`, "unexpected fields payload", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newWiCreateServer(t, tc.fields)
			s.fieldsStatus = tc.status
			r := runWiCreate(t, false, false)
			if r.code != 0 {
				t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
			}
			if len(s.posts) != 1 {
				t.Fatalf("create must still POST: posts=%d", len(s.posts))
			}
			env := wiEnvelope(t, r.stdout)
			pc, _ := env.Meta["precheck"].(map[string]any)
			reason, _ := pc["reason"].(string)
			hint, _ := pc["hint"].(string)
			if pc["status"] != "skipped" || pc["source"] != "none" || !strings.Contains(reason, tc.reason) || !strings.Contains(hint, "yunxiao workitem fields --space-id space-1 --type-id type-req") {
				t.Fatalf("meta.precheck=%#v", pc)
			}
			// The warning is in meta.precheck and printed once as "warning: ..." on stderr.
			warning, _ := pc["warning"].(string)
			if !strings.Contains(warning, "precheck skipped") || r.stderr != "warning: "+warning+"\n" {
				t.Fatalf("warning=%q stderr=%q", warning, r.stderr)
			}
		})
	}
}

// Degrade also in dry-run: preview still printed, request.precheck.status=skipped.
func TestWorkitemCreatePrecheckDegradesDryRun(t *testing.T) {
	s := newWiCreateServer(t, wiFieldsFixture)
	s.fieldsStatus = http.StatusForbidden
	r := runWiCreate(t, true, false)
	if r.code != 0 {
		t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
	}
	pc, _ := wiDryRunRequest(t, r.stdout)["precheck"].(map[string]any)
	if pc["status"] != "skipped" || len(s.posts) != 0 {
		t.Fatalf("precheck=%#v posts=%d", pc, len(s.posts))
	}
	if w, _ := pc["warning"].(string); w == "" || r.stderr != "warning: "+w+"\n" {
		t.Fatalf("dry-run warning: request.precheck.warning=%q stderr=%q", w, r.stderr)
	}
}

// --no-precheck: no fields GET, create exactly as before (no meta.precheck).
func TestWorkitemCreateNoPrecheckSkipsLookup(t *testing.T) {
	s := newWiCreateServer(t, wiFieldsFixture)
	r := runWiCreate(t, false, false, "--no-precheck")
	if r.code != 0 {
		t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
	}
	if s.fieldsGETs != 0 || len(s.posts) != 1 {
		t.Fatalf("fieldsGETs=%d posts=%d", s.fieldsGETs, len(s.posts))
	}
	env := wiEnvelope(t, r.stdout)
	if _, ok := env.Meta["precheck"]; ok {
		t.Fatalf("--no-precheck must not add meta.precheck: %#v", env.Meta)
	}
}

// Server-side validation errors after a passing precheck are passed through unchanged.
func TestWorkitemCreatePrecheckServerErrorPassesThrough(t *testing.T) {
	s := newWiCreateServer(t, wiFieldsFixture)
	s.postStatus = http.StatusBadRequest
	s.postBody = `{"errorCode":"InvalidParam","errorMessage":"【截止日期】必填"}`
	r := runWiCreate(t, false, false, "--custom-fields", `{"priority":"prio-high","mod-1":"m-a"}`)
	if r.code != 1 {
		t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
	}
	eb := wiErrorBody(t, r.stderr)
	if eb.Type != "api" || eb.Code != http.StatusBadRequest || !strings.Contains(eb.Message, "截止日期") {
		t.Fatalf("error=%+v", eb)
	}
	if len(s.posts) != 1 {
		t.Fatalf("posts=%d", len(s.posts))
	}
}

func TestWorkitemCreateHelpDocumentsPrecheck(t *testing.T) {
	if f := workitemCreateCmd.Flags().Lookup("no-precheck"); f == nil {
		t.Fatal("--no-precheck flag missing")
	}
	for _, want := range []string{"precheck", "--no-precheck", "workitem fields", "details.missing"} {
		if !strings.Contains(workitemCreateCmd.Long, want) {
			t.Fatalf("help missing %q", want)
		}
	}
}

// A required root field (sprint) only counts when passed via its flag, not in customFieldValues.
func TestWorkitemCreatePrecheckRootFieldNotInCustomFields(t *testing.T) {
	fixture := strings.Replace(wiFieldsFixture, `{"id":"note",`, `{"id":"sprint","name":"迭代","type":"NativeField","format":"sprint","required":true,"showWhenCreate":true},
 {"id":"note",`, 1)
	s := newWiCreateServer(t, fixture)
	r := runWiCreate(t, true, false, "--custom-fields", `{"priority":"prio-high","mod-1":"m-a","sprint":"sp-1"}`)
	eb := wiErrorBody(t, r.stderr)
	missing, _ := eb.Details["missing"].([]any)
	if r.code != 1 || len(missing) != 1 || missing[0].(map[string]any)["field_id"] != "sprint" || missing[0].(map[string]any)["pass_via"] != "--sprint" {
		t.Fatalf("code=%d error=%+v", r.code, eb)
	}
	r = runWiCreate(t, true, false, "--custom-fields", `{"priority":"prio-high","mod-1":"m-a"}`, "--sprint", "sp-1")
	if r.code != 0 || s.fieldsGETs != 2 {
		t.Fatalf("code=%d stderr=%s gets=%d", r.code, r.stderr, s.fieldsGETs)
	}
}

// An empty field config is reported as status "empty" (not ok with required_checked 0).
func TestWorkitemCreatePrecheckEmptyConfig(t *testing.T) {
	s := newWiCreateServer(t, `[]`)
	r := runWiCreate(t, false, false)
	if r.code != 0 || len(s.posts) != 1 {
		t.Fatalf("code=%d posts=%d stderr=%s", r.code, len(s.posts), r.stderr)
	}
	pc, _ := wiEnvelope(t, r.stdout).Meta["precheck"].(map[string]any)
	warning, _ := pc["warning"].(string)
	if pc["status"] != "empty" || pc["source"] != "none" || !strings.Contains(warning, "no fields") {
		t.Fatalf("meta.precheck=%#v", pc)
	}
	if _, ok := pc["required_checked"]; ok {
		t.Fatalf("empty config must not claim required_checked: %#v", pc)
	}
}

// 401 on the field-config GET is an auth problem: fail (no degrade, no POST).
func TestWorkitemCreatePrecheckUnauthorizedFails(t *testing.T) {
	for _, dry := range []bool{false, true} {
		s := newWiCreateServer(t, wiFieldsFixture)
		s.fieldsStatus = http.StatusUnauthorized
		r := runWiCreate(t, dry, false)
		eb := wiErrorBody(t, r.stderr)
		if r.code != 1 || eb.Type != "api" || eb.Code != http.StatusUnauthorized || len(s.posts) != 0 || r.stdout != "" {
			t.Fatalf("dry=%v code=%d error=%+v posts=%d", dry, r.code, eb, len(s.posts))
		}
	}
}

// The fields GET uses a short retry bound: at most 1 retry, backoff capped at 1s even
// with Retry-After: 30, so degrading cannot stall ~90s.
func TestWorkitemCreatePrecheckShortRetry(t *testing.T) {
	var sleeps []time.Duration
	t.Cleanup(client.SetRetrySleepForTest(func(ctx context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		return nil
	}))
	s := newWiCreateServer(t, wiFieldsFixture)
	s.fieldsStatus = http.StatusServiceUnavailable
	s.retryAfter = "30"
	r := runWiCreate(t, false, false)
	if r.code != 0 || len(s.posts) != 1 {
		t.Fatalf("code=%d posts=%d stderr=%s", r.code, len(s.posts), r.stderr)
	}
	if s.fieldsGETs != 2 || len(sleeps) != 1 || sleeps[0] > time.Second {
		t.Fatalf("fieldsGETs=%d sleeps=%v", s.fieldsGETs, sleeps)
	}
	pc, _ := wiEnvelope(t, r.stdout).Meta["precheck"].(map[string]any)
	if pc["status"] != "skipped" {
		t.Fatalf("meta.precheck=%#v", pc)
	}
}

// Field config unavailable/empty + profile create_required → warn-only fallback, never blocks.
func TestWorkitemCreatePrecheckProfileFallback(t *testing.T) {
	cases := []struct {
		name, fields, status string
		fieldsStatus         int
	}{
		{"forbidden", wiFieldsFixture, "skipped", http.StatusForbidden},
		{"empty", `[]`, "empty", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newWiCreateServer(t, tc.fields)
			s.fieldsStatus = tc.fieldsStatus
			xdg := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", xdg)
			writeTransitionTestProfile(t, xdg, &profile.Profile{
				Name: "p95fb", OrganizationID: "org-wi-precheck", SpaceID: "space-1",
				WorkitemDefaults: map[string]profile.WorkitemTypeDefaults{
					"type-req": {Name: "产品需求", CreateRequired: []string{"subject", "priority", "mod-1", "sprint"}},
				},
			})
			t.Setenv("YUNXIAO_PROFILE", "p95fb")
			prevProfile := globalProfile
			globalProfile = "p95fb"
			t.Cleanup(func() { globalProfile = prevProfile })
			// --no-defaults: the fallback still reads create_required from the profile.
			r := runWiCreate(t, false, false, "--custom-fields", `{"mod-1":"m-a"}`)
			pc, _ := wiEnvelope(t, r.stdout).Meta["precheck"].(map[string]any)
			warning, _ := pc["warning"].(string)
			if r.code != 0 || len(s.posts) != 1 || r.stderr != "warning: "+warning+"\n" {
				t.Fatalf("code=%d posts=%d stderr=%s", r.code, len(s.posts), r.stderr)
			}
			mp, _ := pc["profile_missing"].([]any)
			if pc["status"] != tc.status || pc["source"] != "profile_fallback" || len(mp) != 2 || mp[0] != "priority" || mp[1] != "sprint" {
				t.Fatalf("meta.precheck=%#v", pc)
			}
			if !strings.Contains(warning, "priority") || !strings.Contains(warning, "sprint") || !strings.Contains(warning, "create_required") {
				t.Fatalf("warning=%q", warning)
			}
		})
	}
}

// A skipped / empty precheck is not lost when the POST then fails: error.hint carries
// "precheck skipped: <reason>" next to the API hint; a passing precheck adds nothing.
func TestWorkitemCreatePrecheckSkippedReasonInErrorHint(t *testing.T) {
	t.Cleanup(client.SetRetrySleepForTest(func(ctx context.Context, d time.Duration) error { return nil }))
	s := newWiCreateServer(t, wiFieldsFixture)
	s.fieldsStatus = http.StatusInternalServerError
	s.postStatus = http.StatusBadRequest
	s.postBody = `{"errorCode":"InvalidParam","errorMessage":"【所属模块】必填"}`
	r := runWiCreate(t, false, false)
	if r.code != 1 || r.stdout != "" || len(s.posts) != 1 {
		t.Fatalf("exit=%d posts=%d stdout=%s stderr=%s", r.code, len(s.posts), r.stdout, r.stderr)
	}
	eb := wiErrorBody(t, r.stderr)
	if eb.Type != "api" || eb.Code != http.StatusBadRequest || !strings.Contains(eb.Message, "所属模块") {
		t.Fatalf("error=%+v", eb)
	}
	if !strings.Contains(eb.Hint, "precheck skipped: GET fields -> HTTP 500") {
		t.Fatalf("hint=%q", eb.Hint)
	}

	s = newWiCreateServer(t, `[]`)
	s.postStatus = http.StatusBadRequest
	s.postBody = `{"errorCode":"InvalidParam","errorMessage":"bad"}`
	r = runWiCreate(t, false, false)
	if eb := wiErrorBody(t, r.stderr); r.code != 1 || !strings.Contains(eb.Hint, "precheck empty: field config returned no fields") {
		t.Fatalf("empty: exit=%d hint=%q", r.code, eb.Hint)
	}

	s = newWiCreateServer(t, wiFieldsFixture)
	s.postStatus = http.StatusBadRequest
	s.postBody = `{"errorCode":"InvalidParam","errorMessage":"bad"}`
	r = runWiCreate(t, false, false, "--custom-fields", `{"priority":"prio-high","mod-1":"m-a"}`)
	if eb := wiErrorBody(t, r.stderr); r.code != 1 || strings.Contains(eb.Hint, "precheck") {
		t.Fatalf("ok precheck must not add a hint: exit=%d hint=%q", r.code, eb.Hint)
	}
}

// A fields GET that never answers degrades after precheckTimeout (both attempts
// included) instead of stalling for the HTTP client timeout; the create still POSTs.
func TestWorkitemCreatePrecheckTimeout(t *testing.T) {
	prev := precheckTimeout
	precheckTimeout = 300 * time.Millisecond
	t.Cleanup(func() { precheckTimeout = prev })
	s := newWiCreateServer(t, wiFieldsFixture)
	s.fieldsHang.Store(true)
	start := time.Now()
	r := runWiCreate(t, false, false)
	elapsed := time.Since(start)
	if r.code != 0 || len(s.posts) != 1 {
		t.Fatalf("exit=%d posts=%d stderr=%s", r.code, len(s.posts), r.stderr)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("precheck took %s, want about %s", elapsed, precheckTimeout)
	}
	pc, _ := wiEnvelope(t, r.stdout).Meta["precheck"].(map[string]any)
	reason, _ := pc["reason"].(string)
	if pc["status"] != "skipped" || !strings.Contains(reason, "timed out after 300ms") || !strings.HasPrefix(r.stderr, "warning: required-field precheck skipped") {
		t.Fatalf("precheck=%#v stderr=%q", pc, r.stderr)
	}
}
