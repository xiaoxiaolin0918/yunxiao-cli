package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// #98 fixture: a long Chinese description plus the fields the brief view keeps.
var workitemGetFixture = `{
 "id":"wi-5916","serialNumber":"ZYPT-5916","subject":"产品类需求",
 "status":{"id":"100005","name":"处理中","displayName":"处理中","nameEn":"DOING"},
 "assignedTo":{"id":"u-1","name":"肖晓霖"},
 "sprint":{"id":"sp-1","name":"S39"},
 "gmtModified":"2026-09-29T10:00:00Z","gmtCreate":"2026-09-01T10:00:00Z",
 "description":"` + strings.Repeat("需求说明", 800) + `",
 "formatType":"MARKDOWN",
 "space":{"id":"space-1","name":"智衣平台"},
 "workitemType":{"id":"type-req","name":"产品类需求"},
 "customFieldValues":[
  {"fieldId":"module","fieldName":"所属模块","values":[{"identifier":"m-1","displayValue":"订单"}]},
  {"fieldId":"priority","fieldName":"优先级","values":[{"identifier":"p-high","displayValue":"高"}]}
 ]
}`

type workitemGetServer struct {
	mu   sync.Mutex
	gets int
}

func newWorkitemGetServer(t *testing.T) *workitemGetServer {
	t.Helper()
	s := &workitemGetServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.gets++
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/workitems/ZYPT-5916"):
			_, _ = io.WriteString(w, workitemGetFixture)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"errorCode":"NotFound","errorMessage":"workitem not found"}`)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-workitem-get-not-real")
	t.Setenv(config.EnvOrganizationID, "org-workitem-get-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

// runWorkitemGet runs `workitem get <id> extra...` and returns stdout, stderr and
// the processExit code (0 when the command did not exit).
func runWorkitemGet(t *testing.T, dryRun bool, id string, extra ...string) (string, string, int) {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	prevDry := globalDryRun
	globalDryRun = dryRun
	t.Cleanup(func() { globalDryRun = prevDry })
	prevJQ := globalJQ
	t.Cleanup(func() { globalJQ = prevJQ })
	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	prevExit := processExit
	code := 0
	processExit = func(c int) {
		code = c
		panic(exitPanic{code: c})
	}
	t.Cleanup(func() { processExit = prevExit })
	resetStringFlags(t, workitemGetCmd, "id", "full", "brief", "fields")
	t.Cleanup(func() { resetStringFlags(t, workitemGetCmd, "id", "full", "brief", "fields") })
	rootCmd.SetArgs(append([]string{"workitem", "get", id}, extra...))
	t.Cleanup(func() { rootCmd.SetArgs(nil); rootCmd.PersistentFlags().Set("jq", "") })
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

func decodeEnvelopeStrict(t *testing.T, raw string) (output.Envelope, map[string]json.RawMessage) {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("JSON: %v / %s", err, raw)
	}
	var top map[string]json.RawMessage
	_ = json.Unmarshal([]byte(raw), &top)
	return env, top
}

func topKeys(top map[string]json.RawMessage) []string {
	var ks []string
	for k := range top {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func fixtureMap(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(workitemGetFixture), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// Default = brief: key fields, description summarized, meta.url kept, envelope intact.
func TestWorkitemGetDefaultIsBrief(t *testing.T) {
	s := newWorkitemGetServer(t)
	stdout, stderr, code := runWorkitemGet(t, false, "ZYPT-5916")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env, top := decodeEnvelopeStrict(t, stdout)
	if !env.OK || env.Error != nil || !reflect.DeepEqual(topKeys(top), []string{"data", "meta", "ok"}) {
		t.Fatalf("envelope keys=%v: %s", topKeys(top), stdout)
	}
	data := env.Data.(map[string]any)
	for _, k := range []string{"id", "serialNumber", "subject", "status", "assignedTo", "sprint", "priority", "gmtModified", "description_summary"} {
		if _, ok := data[k]; !ok {
			t.Fatalf("brief missing %s: %s", k, stdout)
		}
	}
	for _, k := range []string{"description", "customFieldValues", "formatType", "space"} {
		if _, ok := data[k]; ok {
			t.Fatalf("brief must not contain %s", k)
		}
	}
	if strings.Contains(stdout, "需求说明需求说明") {
		t.Fatal("brief output leaks description text")
	}
	if data["description_summary"] != "(description: 3200 chars, use --full or --fields description)" {
		t.Fatalf("summary=%v", data["description_summary"])
	}
	if env.Meta["url"] == nil || env.Meta["serial_number"] != "ZYPT-5916" || env.Meta["resolved_id"] != "wi-5916" {
		t.Fatalf("meta=%#v", env.Meta)
	}
	if env.Meta["projection"] != "brief" {
		t.Fatalf("meta.projection=%v", env.Meta["projection"])
	}
	if s.gets != 1 {
		t.Fatalf("gets=%d", s.gets)
	}
}

func TestWorkitemGetExplicitBriefEqualsDefault(t *testing.T) {
	newWorkitemGetServer(t)
	def, _, _ := runWorkitemGet(t, false, "ZYPT-5916")
	brief, _, code := runWorkitemGet(t, false, "ZYPT-5916", "--brief")
	if code != 0 || def != brief {
		t.Fatalf("--brief differs from default:\n%s\n%s", def, brief)
	}
}

// --full is the pre-0.16.34 output: raw API object, meta without projection.
func TestWorkitemGetFullMatchesLegacyOutput(t *testing.T) {
	newWorkitemGetServer(t)
	stdout, stderr, code := runWorkitemGet(t, false, "ZYPT-5916", "--full")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env, top := decodeEnvelopeStrict(t, stdout)
	if !reflect.DeepEqual(topKeys(top), []string{"data", "meta", "ok"}) {
		t.Fatalf("envelope keys=%v", topKeys(top))
	}
	if !reflect.DeepEqual(env.Data, any(fixtureMap(t))) {
		t.Fatalf("--full data differs from the raw API object")
	}
	legacyMeta := zhiyi.EnrichWorkItemMeta(map[string]any{"risk": risk.Read}, fixtureMap(t), "", "")
	wantMeta, _ := json.Marshal(legacyMeta)
	gotMeta, _ := json.Marshal(env.Meta)
	if string(wantMeta) != string(gotMeta) {
		t.Fatalf("--full meta=%s want legacy %s", gotMeta, wantMeta)
	}
}

func TestWorkitemGetFieldsProjection(t *testing.T) {
	newWorkitemGetServer(t)
	stdout, stderr, code := runWorkitemGet(t, false, "ZYPT-5916", "--fields", "subject,description,priority,customFieldValues")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env, _ := decodeEnvelopeStrict(t, stdout)
	data := env.Data.(map[string]any)
	if len(data) != 4 || data["description"] != fixtureMap(t)["description"] {
		t.Fatalf("data keys=%d", len(data))
	}
	if !reflect.DeepEqual(data["priority"], map[string]any{"id": "p-high", "displayValue": "高"}) {
		t.Fatalf("priority=%#v", data["priority"])
	}
	if env.Meta["projection"] != "fields" || env.Meta["url"] == nil {
		t.Fatalf("meta=%#v", env.Meta)
	}
}

// Unknown names fail after the read with the available keys and case suggestions.
func TestWorkitemGetFieldsUnknownName(t *testing.T) {
	s := newWorkitemGetServer(t)
	stdout, stderr, code := runWorkitemGet(t, false, "ZYPT-5916", "--fields", "subject,nope,SerialNumber")
	if code != 1 || stdout != "" {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	env, _ := decodeEnvelopeStrict(t, stderr)
	e := env.Error
	if env.OK || e == nil || e.Type != "cli" || e.Subtype != "unknown_fields" {
		t.Fatalf("stderr=%s", stderr)
	}
	if !strings.Contains(e.Message, "nope") || !strings.Contains(e.Message, "SerialNumber") || !strings.Contains(e.Hint, "--full") {
		t.Fatalf("message=%q hint=%q", e.Message, e.Hint)
	}
	raw, _ := json.Marshal(e.Details)
	var d struct {
		Unknown     []string          `json:"unknown"`
		Available   []string          `json:"available"`
		Suggestions map[string]string `json:"suggestions"`
	}
	_ = json.Unmarshal(raw, &d)
	if !reflect.DeepEqual(d.Unknown, []string{"nope", "SerialNumber"}) || d.Suggestions["SerialNumber"] != "serialNumber" || len(d.Available) == 0 {
		t.Fatalf("details=%s", raw)
	}
	if s.gets != 1 {
		t.Fatalf("gets=%d", s.gets)
	}
}

// Malformed --fields and conflicting view flags fail before any request (also under --dry-run).
func TestWorkitemGetFlagErrorsBeforeRequest(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"empty fields", []string{"--fields", ""}, "at least one"},
		{"empty entry", []string{"--fields", "subject,,id"}, "empty field name"},
		{"nested path", []string{"--fields", "status.id"}, "--jq"},
		{"full+brief", []string{"--full", "--brief"}, "mutually exclusive"},
		{"full+fields", []string{"--full", "--fields", "subject"}, "mutually exclusive"},
		{"brief+fields", []string{"--brief", "--fields", "subject"}, "mutually exclusive"},
	}
	for _, dry := range []bool{false, true} {
		for _, tc := range cases {
			s := newWorkitemGetServer(t)
			stdout, stderr, code := runWorkitemGet(t, dry, "ZYPT-5916", tc.args...)
			env, _ := decodeEnvelopeStrict(t, stderr)
			if code != 1 || stdout != "" || env.OK || env.Error == nil || env.Error.Type != "cli" || !strings.Contains(env.Error.Message, tc.want) {
				t.Fatalf("%s (dry=%v): exit=%d stdout=%s stderr=%s", tc.name, dry, code, stdout, stderr)
			}
			if s.gets != 0 {
				t.Fatalf("%s (dry=%v): no request expected, got %d", tc.name, dry, s.gets)
			}
		}
	}
}

// --dry-run sends nothing and shows which view the real call would print.
func TestWorkitemGetDryRunShowsProjection(t *testing.T) {
	cases := []struct {
		args []string
		want any
	}{
		{nil, map[string]any{"mode": "brief"}},
		{[]string{"--fields", "subject,status"}, map[string]any{"mode": "fields", "fields": []any{"subject", "status"}}},
		{[]string{"--full"}, nil},
	}
	for _, tc := range cases {
		s := newWorkitemGetServer(t)
		stdout, stderr, code := runWorkitemGet(t, true, "ZYPT-5916", tc.args...)
		if code != 0 {
			t.Fatalf("%v: exit=%d stderr=%s", tc.args, code, stderr)
		}
		env, _ := decodeEnvelopeStrict(t, stdout)
		req, _ := env.Request.(map[string]any)
		if !env.OK || !env.DryRun || env.Risk != "read" || req["method"] != "GET" || !strings.HasSuffix(req["url"].(string), "/workitems/ZYPT-5916") {
			t.Fatalf("%v: %s", tc.args, stdout)
		}
		if !reflect.DeepEqual(req["projection"], tc.want) {
			t.Fatalf("%v: projection=%#v", tc.args, req["projection"])
		}
		if s.gets != 0 {
			t.Fatalf("dry-run sent %d requests", s.gets)
		}
	}
}

// --jq filters the projected envelope; --full keeps old jq paths working.
func TestWorkitemGetJQComposes(t *testing.T) {
	newWorkitemGetServer(t)
	stdout, _, code := runWorkitemGet(t, false, "ZYPT-5916", "--jq", ".data.status.displayName")
	if code != 0 || strings.TrimSpace(stdout) != `"处理中"` {
		t.Fatalf("brief jq: exit=%d %s", code, stdout)
	}
	stdout, _, code = runWorkitemGet(t, false, "ZYPT-5916", "--full", "--jq", ".data.customFieldValues | length")
	if code != 0 || strings.TrimSpace(stdout) != "2" {
		t.Fatalf("full jq: exit=%d %s", code, stdout)
	}
}

func TestWorkitemGetAPIErrorPassesThrough(t *testing.T) {
	for _, extra := range [][]string{nil, {"--full"}, {"--fields", "subject"}} {
		newWorkitemGetServer(t)
		stdout, stderr, code := runWorkitemGet(t, false, "ZYPT-404", extra...)
		env, _ := decodeEnvelopeStrict(t, stderr)
		if code != 1 || stdout != "" || env.Error == nil || env.Error.Type != "api" || env.Error.Code != 404 {
			t.Fatalf("%v: exit=%d stderr=%s", extra, code, stderr)
		}
	}
}

func TestWorkitemGetHelpDocumentsViews(t *testing.T) {
	h := workitemGetCmd.Long
	for _, want := range []string{"--full", "--brief", "--fields", "description_summary", "0.16.34", "meta.url", "--jq"} {
		if !strings.Contains(h, want) {
			t.Fatalf("help missing %q:\n%s", want, h)
		}
	}
}
// --full=false is the same as not passing --full (no conflict with --fields).
func TestWorkitemGetFalseBoolIsNotAConflict(t *testing.T) {
	newWorkitemGetServer(t)
	stdout, stderr, code := runWorkitemGet(t, false, "ZYPT-5916", "--full=false", "--fields", "subject")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env, _ := decodeEnvelopeStrict(t, stdout)
	if d := env.Data.(map[string]any); len(d) != 1 || d["subject"] != "产品类需求" {
		t.Fatalf("data=%#v", env.Data)
	}
}
