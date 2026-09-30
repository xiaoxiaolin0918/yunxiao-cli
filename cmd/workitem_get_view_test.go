package cmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
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
 "categoryId":"Req",
 "workitemType":{"id":"type-req","name":"产品类需求","nameEn":"Req"},
 "customFieldValues":[
  {"fieldId":"module","fieldName":"所属模块","values":[{"identifier":"m-1","displayValue":"订单"}]},
  {"fieldId":"priority","fieldName":"优先级","values":[{"identifier":"p-high","displayValue":"高"}]}
 ]
}`

// Extra fixtures: no derivable priority / non-object payloads.
const workitemGetNoPriorityFixture = `{"id":"wi-7","serialNumber":"ZYPT-7","subject":"无优先级","priority":null,
 "customFieldValues":[{"fieldId":"module","fieldName":"所属模块","values":[{"identifier":"m-1","displayValue":"订单"}]}]}`

// Pagination headers on the single-item GET: the legacy path copied them into
// meta (pagination_headers, pagination, …); --full must keep doing so.
var workitemGetPagingHeaders = map[string]string{"x-page": "1", "x-per-page": "20", "x-total": "1", "x-total-pages": "1"}

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
		body := map[string]string{
			"/workitems/ZYPT-5916": workitemGetFixture,
			"/workitems/ZYPT-7":    workitemGetNoPriorityFixture,
			"/workitems/ZYPT-ARR":  `[{"id":"wi-a"}]`,
			"/workitems/ZYPT-NULL": `null`,
		}
		for suffix, b := range body {
			if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, suffix) {
				for k, v := range workitemGetPagingHeaders {
					w.Header().Set(k, v)
				}
				_, _ = io.WriteString(w, b)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"errorCode":"NotFound","errorMessage":"workitem not found"}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-workitem-get-not-real")
	t.Setenv(config.EnvOrganizationID, "org-workitem-get-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv(envWorkitemGetView, "")
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
	for _, k := range zhiyi.WorkItemGetBriefFields {
		if _, ok := data[k]; !ok {
			t.Fatalf("brief missing %s: %s", k, stdout)
		}
	}
	if len(data) != len(zhiyi.WorkItemGetBriefFields) {
		t.Fatalf("brief has %d keys, want %d: %s", len(data), len(zhiyi.WorkItemGetBriefFields), stdout)
	}
	if !reflect.DeepEqual(data["workitemType"], map[string]any{"id": "type-req", "name": "产品类需求"}) || data["categoryId"] != "Req" {
		t.Fatalf("workitemType=%#v categoryId=%#v", data["workitemType"], data["categoryId"])
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

// legacyWorkitemGetRun is workitemGetCmd.Run as of 0.16.33 (main 55cee26), verbatim:
// the real old code path through runRead, including MetaWithPagination.
func legacyWorkitemGetRun(cmd *cobra.Command, args []string) {
	flagOrg(globalOrg)
	if _, err := applyActiveProfileOrg(); err != nil {
		handleErr(err)
		return
	}
	id, err := workitemIDFromFlagOrArg(cmd, args)
	if err != nil {
		handleErr(err)
		return
	}
	c, _, err := mustClient()
	if err != nil {
		handleErr(err)
		return
	}
	path, err := c.ProjexPath(cmd.Context(), "/workitems/"+id)
	if err != nil {
		handleErr(err)
		return
	}
	handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, func(out any, meta map[string]any) (any, map[string]any) {
		zhiyi.EnrichWorkItemMeta(meta, asStringMap(out), profileSpaceID(), "")
		return out, meta
	}))
}

// runLegacyWorkitemGet runs `workitem get <id>` with the 0.16.33 Run function.
func runLegacyWorkitemGet(t *testing.T, id string) string {
	t.Helper()
	prevRun := workitemGetCmd.Run
	workitemGetCmd.Run = legacyWorkitemGetRun
	t.Cleanup(func() { workitemGetCmd.Run = prevRun })
	stdout, stderr, code := runWorkitemGet(t, false, id)
	workitemGetCmd.Run = prevRun
	if code != 0 {
		t.Fatalf("legacy exit=%d stderr=%s", code, stderr)
	}
	return stdout
}

// --full (and YUNXIAO_WORKITEM_GET_VIEW=full) print byte-for-byte what 0.16.33 printed,
// driven through the old runRead path against the same server, paging headers included.
func TestWorkitemGetFullMatchesLegacyOutput(t *testing.T) {
	newWorkitemGetServer(t)
	legacy := runLegacyWorkitemGet(t, "ZYPT-5916")
	if !strings.Contains(legacy, `"pagination_headers"`) || !strings.Contains(legacy, `"x-total":"1"`) {
		t.Fatalf("fixture must exercise paging meta: %s", legacy)
	}
	full, stderr, code := runWorkitemGet(t, false, "ZYPT-5916", "--full")
	if code != 0 || full != legacy {
		t.Fatalf("--full differs from legacy (exit=%d stderr=%s):\nnew:    %s\nlegacy: %s", code, stderr, full, legacy)
	}
	t.Setenv(envWorkitemGetView, "full")
	viaEnv, _, code := runWorkitemGet(t, false, "ZYPT-5916")
	if code != 0 || viaEnv != legacy {
		t.Fatalf("%s=full differs from legacy:\n%s", envWorkitemGetView, viaEnv)
	}
	env, _ := decodeEnvelopeStrict(t, full)
	if !reflect.DeepEqual(env.Data, any(fixtureMap(t))) {
		t.Fatalf("--full data differs from the raw API object")
	}
	if _, ok := env.Meta["projection"]; ok {
		t.Fatalf("--full must not add meta.projection: %#v", env.Meta)
	}
}

var updateGolden = flag.Bool("update-workitem-get-golden", false, "rewrite cmd/testdata/workitem_get/*.golden.json")

// Golden stdout for each view (legacy.golden.json is the 0.16.33 path). Regenerate with
// go test ./cmd -run TestWorkitemGetGolden -update-workitem-get-golden
func TestWorkitemGetGolden(t *testing.T) {
	cases := []struct {
		name, id string
		args     []string
		legacy   bool
	}{
		{"legacy", "ZYPT-5916", nil, true},
		{"full", "ZYPT-5916", []string{"--full"}, false},
		{"brief", "ZYPT-5916", nil, false},
		{"fields", "ZYPT-5916", []string{"--fields", "subject,priority,verifier,sprint"}, false},
		{"fields_no_priority", "ZYPT-7", []string{"--fields", "subject,priority"}, false},
	}
	for _, tc := range cases {
		newWorkitemGetServer(t)
		var got string
		if tc.legacy {
			got = runLegacyWorkitemGet(t, tc.id)
		} else {
			var stderr string
			var code int
			got, stderr, code = runWorkitemGet(t, false, tc.id, tc.args...)
			if code != 0 {
				t.Fatalf("%s: exit=%d stderr=%s", tc.name, code, stderr)
			}
		}
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, []byte(got), "", "  "); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		path := filepath.Join("testdata", "workitem_get", tc.name+".golden.json")
		if *updateGolden {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v (run with -update-workitem-get-golden)", tc.name, err)
		}
		if strings.ReplaceAll(string(want), "\r\n", "\n") != pretty.String() {
			t.Fatalf("%s: stdout differs from %s:\n%s", tc.name, path, pretty.String())
		}
	}
	legacy, _ := os.ReadFile(filepath.Join("testdata", "workitem_get", "legacy.golden.json"))
	full, _ := os.ReadFile(filepath.Join("testdata", "workitem_get", "full.golden.json"))
	if !*updateGolden && !bytes.Equal(legacy, full) {
		t.Fatal("full.golden.json must equal legacy.golden.json")
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
		{nil, map[string]any{"mode": "brief", "source": "default"}},
		{[]string{"--brief"}, map[string]any{"mode": "brief", "source": "flag"}},
		{[]string{"--fields", "subject,status"}, map[string]any{"mode": "fields", "fields": []any{"subject", "status"}, "source": "flag"}},
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
	for _, want := range []string{"--full", "--brief", "--fields", "description_summary", "0.16.34", "meta.url", "--jq",
		"YUNXIAO_WORKITEM_GET_VIEW", "flag > env > default", "workitemType", "categoryId", "优先级", "absent_fields", "non_object_response"} {
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

// A GetWorkitem schema key this item lacks (fixture has no verifier) is null and listed
// in meta.absent_fields, not unknown_fields.
func TestWorkitemGetFieldsKnownButAbsentIsNull(t *testing.T) {
	newWorkitemGetServer(t)
	stdout, stderr, code := runWorkitemGet(t, false, "ZYPT-5916", "--fields", "subject,verifier")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env, top := decodeEnvelopeStrict(t, stdout)
	var data map[string]json.RawMessage
	_ = json.Unmarshal(top["data"], &data)
	if string(data["verifier"]) != "null" || len(data) != 2 {
		t.Fatalf("data=%s", top["data"])
	}
	if !reflect.DeepEqual(env.Meta["absent_fields"], []any{"verifier"}) {
		t.Fatalf("meta=%#v", env.Meta)
	}
}

// Underivable priority: --fields priority is null with meta.hint (exit 0); brief omits it.
func TestWorkitemGetFieldsPriorityUnresolved(t *testing.T) {
	newWorkitemGetServer(t)
	stdout, stderr, code := runWorkitemGet(t, false, "ZYPT-7", "--fields", "subject,priority")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	env, top := decodeEnvelopeStrict(t, stdout)
	var data map[string]json.RawMessage
	_ = json.Unmarshal(top["data"], &data)
	if string(data["priority"]) != "null" {
		t.Fatalf("data=%s", top["data"])
	}
	hint, _ := env.Meta["hint"].(string)
	if !strings.Contains(hint, "优先级") || !strings.Contains(hint, "workitem fields") {
		t.Fatalf("meta.hint=%q", hint)
	}
	if _, ok := env.Meta["absent_fields"]; ok {
		t.Fatalf("priority is reported via hint, not absent_fields: %#v", env.Meta)
	}
	stdout, _, code = runWorkitemGet(t, false, "ZYPT-7")
	env, _ = decodeEnvelopeStrict(t, stdout)
	if _, ok := env.Data.(map[string]any)["priority"]; code != 0 || ok {
		t.Fatalf("brief must omit underivable priority: %s", stdout)
	}
}

// --fields on an array / null payload fails clearly (nothing on stdout); brief and
// --full pass such payloads through unchanged as before.
func TestWorkitemGetFieldsNonObjectResponse(t *testing.T) {
	for id, kind := range map[string]string{"ZYPT-ARR": "array", "ZYPT-NULL": "null"} {
		newWorkitemGetServer(t)
		stdout, stderr, code := runWorkitemGet(t, false, id, "--fields", "subject")
		env, _ := decodeEnvelopeStrict(t, stderr)
		if code != 1 || stdout != "" || env.Error == nil || env.Error.Type != "cli" || env.Error.Subtype != "non_object_response" ||
			!strings.Contains(env.Error.Message, kind) || !strings.Contains(env.Error.Hint, "--full") {
			t.Fatalf("%s: exit=%d stdout=%s stderr=%s", id, code, stdout, stderr)
		}
		for _, extra := range [][]string{nil, {"--full"}} {
			stdout, stderr, code = runWorkitemGet(t, false, id, extra...)
			if code != 0 {
				t.Fatalf("%s %v: exit=%d stderr=%s", id, extra, code, stderr)
			}
			env, _ = decodeEnvelopeStrict(t, stdout)
			if _, ok := env.Meta["projection"]; ok || (kind == "null") != (env.Data == nil) {
				t.Fatalf("%s %v: %s", id, extra, stdout)
			}
		}
	}
}

// YUNXIAO_WORKITEM_GET_VIEW: flag > env > default; values are trimmed, case-insensitive.
func TestWorkitemGetViewEnvPrecedence(t *testing.T) {
	cases := []struct {
		env  string
		args []string
		want string // meta.projection; "" = full (absent)
	}{
		{"", nil, "brief"},
		{"full", nil, ""},
		{" FULL ", nil, ""},
		{"brief", nil, "brief"},
		{"full", []string{"--brief"}, "brief"},
		{"brief", []string{"--full"}, ""},
		{"full", []string{"--fields", "subject"}, "fields"},
		{"full", []string{"--full=false"}, ""},
	}
	for _, tc := range cases {
		newWorkitemGetServer(t)
		t.Setenv(envWorkitemGetView, tc.env)
		stdout, stderr, code := runWorkitemGet(t, false, "ZYPT-5916", tc.args...)
		if code != 0 {
			t.Fatalf("env=%q %v: exit=%d stderr=%s", tc.env, tc.args, code, stderr)
		}
		env, _ := decodeEnvelopeStrict(t, stdout)
		got, _ := env.Meta["projection"].(string)
		if got != tc.want {
			t.Fatalf("env=%q %v: projection=%q want %q", tc.env, tc.args, got, tc.want)
		}
		if tc.want == "" && !reflect.DeepEqual(env.Data, any(fixtureMap(t))) {
			t.Fatalf("env=%q %v: full data expected", tc.env, tc.args)
		}
	}
}

// Invalid env values fail before any request unless a view flag overrides them.
func TestWorkitemGetViewEnvInvalid(t *testing.T) {
	for _, dry := range []bool{false, true} {
		s := newWorkitemGetServer(t)
		t.Setenv(envWorkitemGetView, "fields")
		stdout, stderr, code := runWorkitemGet(t, dry, "ZYPT-5916")
		env, _ := decodeEnvelopeStrict(t, stderr)
		if code != 1 || stdout != "" || env.Error == nil || env.Error.Subtype != "invalid_env" || !strings.Contains(env.Error.Message, envWorkitemGetView) || s.gets != 0 {
			t.Fatalf("dry=%v: exit=%d gets=%d stderr=%s", dry, code, s.gets, stderr)
		}
	}
	newWorkitemGetServer(t)
	t.Setenv(envWorkitemGetView, "bogus")
	if _, stderr, code := runWorkitemGet(t, false, "ZYPT-5916", "--brief"); code != 0 {
		t.Fatalf("flag must override an invalid env value: exit=%d %s", code, stderr)
	}
}

func TestWorkitemGetDryRunShowsEnvSource(t *testing.T) {
	for envVal, want := range map[string]any{
		"full":  map[string]any{"mode": "full", "source": "env"},
		"brief": map[string]any{"mode": "brief", "source": "env"},
	} {
		newWorkitemGetServer(t)
		t.Setenv(envWorkitemGetView, envVal)
		stdout, _, code := runWorkitemGet(t, true, "ZYPT-5916")
		env, _ := decodeEnvelopeStrict(t, stdout)
		req, _ := env.Request.(map[string]any)
		if code != 0 || !reflect.DeepEqual(req["projection"], want) {
			t.Fatalf("env=%s: %s", envVal, stdout)
		}
	}
}
