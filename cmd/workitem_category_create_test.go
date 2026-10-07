package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

// #128 fixtures. Fields config: 优先级 required with options; the created item
// carries serialNumber/status so no refresh GET is needed on the happy path.
const catCreateFieldsFixture = `[
 {"id":"subject","name":"标题","type":"NativeField","format":"string","required":true,"showWhenCreate":true},
 {"id":"assignedTo","name":"负责人","type":"NativeField","format":"user","required":true,"showWhenCreate":true},
 {"id":"priority","name":"优先级","type":"SystemCustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"prio-high","value":"高","displayValue":"高"},{"id":"prio-medium","value":"中","displayValue":"中"}]}
]`

const catCreateCreatedFixture = `{"id":"wi-9","serialNumber":"ZYPT-99","subject":"标题","categoryId":"Risk",
 "status":{"id":"1","displayName":"待处理"},"assignedTo":"user-1","spaceId":"space-1"}`

// Same as catCreateFieldsFixture plus one required custom field (所属模块).
const catCreateFieldsExtraFixture = `[
 {"id":"subject","name":"标题","type":"NativeField","format":"string","required":true,"showWhenCreate":true},
 {"id":"assignedTo","name":"负责人","type":"NativeField","format":"user","required":true,"showWhenCreate":true},
 {"id":"priority","name":"优先级","type":"SystemCustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"prio-high","value":"高","displayValue":"高"},{"id":"prio-medium","value":"中","displayValue":"中"}]},
 {"id":"mod-1","name":"所属模块","type":"CustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"m-a","value":"MES","displayValue":"MES"}]}
]`

const catCreateMembersFixture = `{"members":[{"id":"user-cui","name":"崔健"},{"id":"user-other","name":"其他人"}]}`

type catCreateServer struct {
	mu             sync.Mutex
	fields         string // type fields JSON (default catCreateFieldsFixture)
	fieldsStatus   int    // non-zero: fields GET answers with this status
	members        string
	memberStatus   int
	posts          []map[string]any // POST /workitems bodies
	fieldsGETs     int
	memberSearches int
	other          []string
}

func newCatCreateServer(t *testing.T) *catCreateServer {
	t.Helper()
	s := &catCreateServer{fields: catCreateFieldsFixture, members: catCreateMembersFixture}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/workitemTypes/") && strings.HasSuffix(r.URL.Path, "/fields"):
			s.fieldsGETs++
			if s.fieldsStatus != 0 {
				w.WriteHeader(s.fieldsStatus)
				_, _ = io.WriteString(w, `{"errorCode":"Forbidden","errorMessage":"no"}`)
				return
			}
			_, _ = io.WriteString(w, s.fields)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/members:search"):
			s.memberSearches++
			if s.memberStatus != 0 {
				w.WriteHeader(s.memberStatus)
				_, _ = io.WriteString(w, `{"errorCode":"SystemError","errorMessage":"boom"}`)
				return
			}
			_, _ = io.WriteString(w, s.members)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/workitems"):
			b, _ := io.ReadAll(r.Body)
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			s.posts = append(s.posts, m)
			_, _ = io.WriteString(w, catCreateCreatedFixture)
		default:
			s.other = append(s.other, r.Method+" "+r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-cat-create-not-real")
	t.Setenv(config.EnvOrganizationID, "org-cat-create")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	return s
}

// runCatCreate runs `workitem +<name>-create` with the given profile and returns
// stdout, stderr and processExit code (0 when the command did not exit).
func runCatCreate(t *testing.T, dryRun bool, name string, pf *profile.Profile, extra ...string) (string, string, int) {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if pf != nil {
		writeTransitionTestProfile(t, xdg, pf)
		t.Setenv("YUNXIAO_PROFILE", pf.Name)
	}
	stdout := withCmdJSONCapture(t)
	globalProfile = pfName(pf)
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

	var cc *cobra.Command
	switch name {
	case "+risk-create":
		cc = workitemRiskCreateCmd
	default:
		cc = workitemReqCreateCmd
	}
	resetStringFlags(t, cc, "title", "title-file", "description", "description-file", "priority",
		"assignee", "assigned-to", "participants", "sprint", "type-id")
	resetBoolFlags := func(names ...string) {
		for _, n := range names {
			_ = cc.Flags().Set(n, "false")
		}
	}
	resetBoolFlags("no-defaults", "no-precheck", "full")

	args := append([]string{"workitem", name, "--title", "标题", "--description", "说明"}, extra...)
	if dryRun {
		args = append(args, "--dry-run")
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
			t.Fatalf("execute: %v", err)
		}
	}()
	return stdout.String(), stderr.String(), code
}

func pfName(pf *profile.Profile) string {
	if pf == nil {
		return ""
	}
	return pf.Name
}

func catEnvelope(t *testing.T, stdout string) output.Envelope {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout)
	}
	return env
}

func catDryRunBody(t *testing.T, stdout string) map[string]any {
	t.Helper()
	env := catEnvelope(t, stdout)
	if !env.OK || !env.DryRun {
		t.Fatalf("envelope: %s", stdout)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	body, _ := req["body"].(map[string]any)
	if body == nil {
		t.Fatalf("request=%s", raw)
	}
	return body
}

func catErrorBody(t *testing.T, stderr string) output.ErrorBody {
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

// Base profile for most cases: explicit risk/req type ids + shared priority alias map.
func catBaseProfile(name string) *profile.Profile {
	return &profile.Profile{
		Name:              name,
		OrganizationID:    "org-cat-create",
		SpaceID:           "space-1",
		RiskTypeID:        "type-risk",
		ReqTypeID:         "type-req",
		DefaultAssignedTo: "user-1",
		BugCreateFields: profile.BugCreateFields{
			Priority: map[string]string{"high": "prio-high", "medium": "prio-medium"},
		},
	}
}

// Dry-run body contract: type id from profile, priority mapped, MARKDOWN, no sprint.
func TestCategoryCreateDryRunBody(t *testing.T) {
	cases := []struct {
		name         string
		cmdName      string
		pf           *profile.Profile
		extra        []string
		wantType     string
		wantPrio     any
		skipPrecheck bool
	}{
		{
			name: "risk alias priority", cmdName: "+risk-create", pf: catBaseProfile("cat128a"),
			wantType: "type-risk", wantPrio: "prio-medium",
		},
		{
			name: "risk display priority high", cmdName: "+risk-create", pf: catBaseProfile("cat128b"),
			extra: []string{"--priority", "prio-high"}, wantType: "type-risk", wantPrio: "prio-high",
		},
		{
			name: "req explicit type flag", cmdName: "+req-create", pf: catBaseProfile("cat128c"),
			extra: []string{"--type-id", "type-req-2"}, wantType: "type-req-2", wantPrio: "prio-medium",
		},
		{
			// --priority "" omits the key entirely (asserted with --no-precheck so the
			// required-priority precheck does not block the preview).
			name: "priority empty omits customFieldValues", cmdName: "+risk-create", pf: catBaseProfile("cat128d"),
			extra: []string{"--priority", "", "--no-precheck"}, wantType: "type-risk", wantPrio: nil,
			skipPrecheck: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newCatCreateServer(t)
			stdout, stderr, code := runCatCreate(t, true, tc.cmdName, tc.pf, tc.extra...)
			if code != 0 {
				t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
			}
			body := catDryRunBody(t, stdout)
			if body["workitemTypeId"] != tc.wantType {
				t.Fatalf("workitemTypeId=%v", body["workitemTypeId"])
			}
			if body["formatType"] != "MARKDOWN" || body["subject"] != "标题" || body["description"] != "说明" {
				t.Fatalf("body=%#v", body)
			}
			if body["assignedTo"] != "user-1" {
				t.Fatalf("assignedTo=%v", body["assignedTo"])
			}
			if _, hasSprint := body["sprint"]; hasSprint {
				t.Fatalf("sprint must not be sent by default: %#v", body)
			}
			if tc.wantPrio == nil {
				if _, ok := body["customFieldValues"]; ok {
					t.Fatalf("customFieldValues must be omitted: %#v", body)
				}
			} else {
				cf, _ := body["customFieldValues"].(map[string]any)
				if cf["priority"] != tc.wantPrio {
					t.Fatalf("customFieldValues=%#v", cf)
				}
			}
			// #95 precheck ran (one read GET) and is attached to the dry-run preview,
			// unless the case skips it.
			env := catEnvelope(t, stdout)
			raw, _ := json.Marshal(env.Request)
			var req map[string]any
			_ = json.Unmarshal(raw, &req)
			pc, _ := req["precheck"].(map[string]any)
			if tc.skipPrecheck {
				if pc != nil {
					t.Fatalf("--no-precheck must not attach precheck: %#v", pc)
				}
				if s.fieldsGETs != 0 {
					t.Fatalf("fieldsGETs=%d", s.fieldsGETs)
				}
			} else {
				if pc["status"] != "ok" {
					t.Fatalf("request.precheck=%#v", pc)
				}
				if s.fieldsGETs != 1 {
					t.Fatalf("fieldsGETs=%d other=%v", s.fieldsGETs, s.other)
				}
			}
			if len(s.posts) != 0 {
				t.Fatalf("dry-run must not POST: %#v", s.posts)
			}
		})
	}
}

// --sprint is honored when explicitly passed.
func TestCategoryCreateSprintExplicit(t *testing.T) {
	s := newCatCreateServer(t)
	stdout, stderr, code := runCatCreate(t, true, "+risk-create", catBaseProfile("cat128sp"), "--sprint", "sprint-7")
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
	if body := catDryRunBody(t, stdout); body["sprint"] != "sprint-7" {
		t.Fatalf("sprint=%v", body["sprint"])
	}
	if len(s.posts) != 0 {
		t.Fatalf("dry-run must not POST: %#v", s.posts)
	}
}

// Type resolution matrix: explicit key > single discovered candidate; ambiguity and
// missing type error with candidates / hints (never guess).
func TestCategoryCreateTypeResolution(t *testing.T) {
	ambiguous := &profile.Profile{
		Name: "cat128amb", OrganizationID: "org-cat-create", SpaceID: "space-1", DefaultAssignedTo: "user-1",
		BugCreateFields: profile.BugCreateFields{Priority: map[string]string{"medium": "prio-medium"}},
		WorkitemDefaults: map[string]profile.WorkitemTypeDefaults{
			"type-req-a": {Name: "产品类需求", Category: "Req"},
			"type-req-b": {Name: "技术类需求", Category: "Req"},
		},
	}
	single := &profile.Profile{
		Name: "cat128single", OrganizationID: "org-cat-create", SpaceID: "space-1", DefaultAssignedTo: "user-1",
		BugCreateFields: profile.BugCreateFields{Priority: map[string]string{"medium": "prio-medium"}},
		Workflows: map[string]profile.WorkitemWorkflow{
			"type-risk": {Category: "Risk", Name: "风险"},
		},
	}
	cases := []struct {
		name        string
		cmdName     string
		pf          *profile.Profile
		extra       []string
		wantType    string
		wantErrPart string
	}{
		{
			name: "single workflows candidate resolves", cmdName: "+risk-create", pf: single,
			wantType: "type-risk",
		},
		{
			name: "ambiguous req errors with candidates", cmdName: "+req-create", pf: ambiguous,
			wantErrPart: "multiple Req workitem types: type-req-a (产品类需求, workitem_defaults), type-req-b (技术类需求, workitem_defaults)",
		},
		{
			name: "--type-id override wins", cmdName: "+req-create", pf: ambiguous,
			extra: []string{"--type-id", "type-req-b"}, wantType: "type-req-b",
		},
		{
			name: "no candidate errors with types-list hint", cmdName: "+risk-create", pf: single,
			wantErrPart: "yunxiao workitem types list --space-id space-1 --category Risk",
		},
	}
	// The "no candidate" case needs a profile without the Risk workflow entry.
	noRisk := &profile.Profile{
		Name: "cat128norisk", OrganizationID: "org-cat-create", SpaceID: "space-1", DefaultAssignedTo: "user-1",
		BugCreateFields: profile.BugCreateFields{Priority: map[string]string{"medium": "prio-medium"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pf := tc.pf
			if strings.Contains(tc.name, "no candidate") {
				pf = noRisk
			}
			s := newCatCreateServer(t)
			stdout, stderr, code := runCatCreate(t, true, tc.cmdName, pf, tc.extra...)
			if tc.wantErrPart != "" {
				if code != 1 {
					t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
				}
				eb := catErrorBody(t, stderr)
				if !strings.Contains(eb.Message, tc.wantErrPart) {
					t.Fatalf("message=%q want %q", eb.Message, tc.wantErrPart)
				}
				if len(s.posts) != 0 {
					t.Fatalf("must not POST: %#v", s.posts)
				}
				return
			}
			if code != 0 {
				t.Fatalf("exit %d stderr=%s", code, stderr)
			}
			if body := catDryRunBody(t, stdout); body["workitemTypeId"] != tc.wantType {
				t.Fatalf("workitemTypeId=%v", body["workitemTypeId"])
			}
		})
	}
}

// --assignee display-name resolution via members:search.
func TestCategoryCreateAssignee(t *testing.T) {
	twoCuis := `{"members":[{"id":"user-cui-1","name":"崔健"},{"id":"user-cui-2","name":"崔健"}]}`
	cases := []struct {
		name        string
		members     string
		extra       []string
		want        string
		wantErrPart string
		wantSearch  bool
	}{
		{name: "unique exact name", members: catCreateMembersFixture, extra: []string{"--assignee", "崔健"}, want: "user-cui", wantSearch: true},
		{name: "case and spaces tolerated", members: catCreateMembersFixture, extra: []string{"--assignee", " 崔健 "}, want: "user-cui", wantSearch: true},
		{
			name: "ambiguous lists candidates", members: twoCuis, extra: []string{"--assignee", "崔健"},
			wantErrPart: "--assignee \"崔健\" is ambiguous: user-cui-1 (崔健), user-cui-2 (崔健)", wantSearch: true,
		},
		{
			name: "no exact match", members: catCreateMembersFixture, extra: []string{"--assignee", "不存在"},
			wantErrPart: "no organization member named \"不存在\"", wantSearch: true,
		},
		{
			name: "both assignee and assigned-to rejected", members: catCreateMembersFixture,
			extra: []string{"--assignee", "崔健", "--assigned-to", "user-1"},
			wantErrPart: "use only one of --assignee", wantSearch: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newCatCreateServer(t)
			s.members = tc.members
			stdout, stderr, code := runCatCreate(t, true, "+risk-create", catBaseProfile("cat128asg"), tc.extra...)
			if tc.wantErrPart != "" {
				if code != 1 {
					t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
				}
				eb := catErrorBody(t, stderr)
				if !strings.Contains(eb.Message, tc.wantErrPart) {
					t.Fatalf("message=%q want %q", eb.Message, tc.wantErrPart)
				}
				if len(s.posts) != 0 {
					t.Fatalf("must not POST: %#v", s.posts)
				}
				return
			}
			if code != 0 {
				t.Fatalf("exit %d stderr=%s", code, stderr)
			}
			if body := catDryRunBody(t, stdout); body["assignedTo"] != tc.want {
				t.Fatalf("assignedTo=%v", body["assignedTo"])
			}
		})
	}
}

// Real run: gate (--yes), POST body, defaults, brief output + meta.
func TestCategoryCreateRun(t *testing.T) {
	pf := catBaseProfile("cat128run")
	pf.WorkitemDefaults = map[string]profile.WorkitemTypeDefaults{
		"type-risk": {
			Name: "风险", Category: "Risk",
			Fields: map[string]profile.WorkitemDefaultField{
				"participants": {Value: []any{"user-cc"}, FieldName: "参与者"},
			},
		},
	}
	s := newCatCreateServer(t)
	stdout, stderr, code := runCatCreate(t, false, "+risk-create", pf, "--participants", "user-extra", "--yes")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if len(s.posts) != 1 {
		t.Fatalf("posts=%#v", s.posts)
	}
	p := s.posts[0]
	if p["workitemTypeId"] != "type-risk" || p["formatType"] != "MARKDOWN" || p["assignedTo"] != "user-1" {
		t.Fatalf("post=%#v", p)
	}
	// Explicit --participants wins over the defaults value (never overridden).
	parts, _ := p["participants"].([]any)
	if len(parts) != 1 || parts[0] != "user-extra" {
		t.Fatalf("participants=%#v", p["participants"])
	}
	env := catEnvelope(t, stdout)
	if !env.OK {
		t.Fatalf("envelope=%s", stdout)
	}
	meta := env.Meta
	if meta["category"] != "Risk" || meta["type_id"] != "type-risk" || meta["type_source"] != "profile" {
		t.Fatalf("meta=%#v", meta)
	}
	pc, _ := meta["precheck"].(map[string]any)
	if pc["status"] != "ok" {
		t.Fatalf("meta.precheck=%#v", pc)
	}
	if u, _ := meta["url"].(string); !strings.HasPrefix(u, "https://devops.aliyun.com/projex/project/space-1/") {
		t.Fatalf("meta.url=%q", u)
	}
	data, _ := env.Data.(map[string]any)
	if data["serialNumber"] != "ZYPT-99" {
		t.Fatalf("data=%#v (brief view expected)", data)
	}
	if _, hasDesc := data["description"]; hasDesc {
		t.Fatalf("brief view must not carry description: %#v", data)
	}
}

// Defaults fill participants when the flag is absent; --no-defaults skips them.
func TestCategoryCreateDefaultsAppliedAndSkipped(t *testing.T) {
	withDefaults := catBaseProfile("cat128def")
	withDefaults.WorkitemDefaults = map[string]profile.WorkitemTypeDefaults{
		"type-risk": {Fields: map[string]profile.WorkitemDefaultField{
			"participants": {Value: []any{"user-cc"}},
		}},
	}
	s := newCatCreateServer(t)
	stdout, stderr, code := runCatCreate(t, true, "+risk-create", withDefaults)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
	body := catDryRunBody(t, stdout)
	parts, _ := body["participants"].([]any)
	if len(parts) != 1 || parts[0] != "user-cc" {
		t.Fatalf("participants=%#v", body["participants"])
	}
	if s.fieldsGETs != 1 {
		t.Fatalf("fieldsGETs=%d", s.fieldsGETs)
	}

	stdout, stderr, code = runCatCreate(t, true, "+risk-create", withDefaults, "--no-defaults")
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
	body = catDryRunBody(t, stdout)
	if _, ok := body["participants"]; ok {
		t.Fatalf("--no-defaults must skip participants: %#v", body)
	}
}

// Gate: real run without --yes exits 10 with confirmation_required.
func TestCategoryCreateRequiresYes(t *testing.T) {
	for _, name := range []string{"+risk-create", "+req-create"} {
		t.Run(name, func(t *testing.T) {
			s := newCatCreateServer(t)
			stdout, stderr, code := runCatCreate(t, false, name, catBaseProfile("cat128gate"))
			if code != 10 {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			if !strings.Contains(stderr, "confirmation_required") {
				t.Fatalf("stderr=%s", stderr)
			}
			if len(s.posts) != 0 {
				t.Fatalf("must not POST without --yes: %#v", s.posts)
			}
		})
	}
}

// #95 precheck: a missing required custom field blocks the POST with
// missing_required_fields (also under --dry-run); --no-precheck skips the GET.
func TestCategoryCreatePrecheck(t *testing.T) {
	for _, dry := range []bool{false, true} {
		t.Run(fmtDry(dry), func(t *testing.T) {
			s := newCatCreateServer(t)
			s.fields = catCreateFieldsExtraFixture
			stdout, stderr, code := runCatCreate(t, dry, "+risk-create", catBaseProfile("cat128pc"))
			if code != 1 || stdout != "" {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			eb := catErrorBody(t, stderr)
			if eb.Subtype != "missing_required_fields" {
				t.Fatalf("error=%+v", eb)
			}
			missing, _ := eb.Details["missing"].([]any)
			if len(missing) != 1 || missing[0].(map[string]any)["field_id"] != "mod-1" {
				t.Fatalf("details=%#v", eb.Details)
			}
			if len(s.posts) != 0 || s.fieldsGETs != 1 {
				t.Fatalf("posts=%d fieldsGETs=%d", len(s.posts), s.fieldsGETs)
			}
		})
	}
	s := newCatCreateServer(t)
	stdout, stderr, code := runCatCreate(t, true, "+risk-create", catBaseProfile("cat128npc"), "--no-precheck")
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
	if s.fieldsGETs != 0 {
		t.Fatalf("--no-precheck must skip the fields GET (got %d)", s.fieldsGETs)
	}
	env := catEnvelope(t, stdout)
	raw, _ := json.Marshal(env.Request)
	if strings.Contains(string(raw), "precheck") {
		t.Fatalf("--no-precheck must not attach precheck: %s", raw)
	}
}

func fmtDry(dry bool) string {
	if dry {
		return "dry-run"
	}
	return "real"
}

// Help contract: type key, priority chain, assignee and precheck documented.
func TestCategoryCreateHelp(t *testing.T) {
	for _, tc := range []struct {
		cmd  *cobra.Command
		key  string
		name string
	}{
		{workitemRiskCreateCmd, "risk_type_id", "+risk-create"},
		{workitemReqCreateCmd, "req_type_id", "+req-create"},
	} {
		long := tc.cmd.Long
		for _, want := range []string{"Risk: write", tc.key, "priority", "显示值", "--assignee", "precheck", "MARKDOWN", "未启用此字段【迭代】"} {
			if !strings.Contains(long, want) {
				t.Fatalf("%s Long missing %q: %s", tc.name, want, long)
			}
		}
		for _, flag := range []string{"title", "title-file", "description", "description-file", "priority",
			"assignee", "assigned-to", "participants", "sprint", "type-id", "no-defaults", "no-precheck", "full"} {
			if tc.cmd.Flags().Lookup(flag) == nil {
				t.Fatalf("%s missing --%s", tc.name, flag)
			}
		}
	}
}
