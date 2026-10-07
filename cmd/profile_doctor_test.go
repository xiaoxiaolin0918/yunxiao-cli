package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

// #120 fixtures: live field config + workflow for type t-bug in space sp-1.
// 100010 (处理中 / In Progress) and 100030 (已关闭 / Closed) are live but absent
// from the default test profile; f-mod / f-env / f-exp carry fieldNames.
const doctorFieldsFixture = `[
 {"id":"subject","name":"标题","format":"text","required":true},
 {"id":"f-mod","name":"所属模块","format":"list"},
 {"id":"f-env","name":"测试环境","format":"list"},
 {"id":"f-exp","name":"期望完成时间","format":"dateTime"}
]`

const doctorWorkflowFixture = `{
 "id":"wf-1","name":"缺陷流程","defaultStatusId":"100001",
 "statuses":[
  {"id":"100001","name":"confirm","displayName":"待确认","nameEn":"New"},
  {"id":"100010","name":"processing","displayName":"处理中","nameEn":"In Progress"},
  {"id":"100030","name":"closed","displayName":"已关闭","nameEn":"Closed"}
 ]
}`

type doctorServer struct {
	mu         sync.Mutex
	gets       []string
	fieldsJS   string
	workflowJS string
}

func newDoctorServer(t *testing.T, fieldsJS, workflowJS string) *doctorServer {
	t.Helper()
	s := &doctorServer{fieldsJS: fieldsJS, workflowJS: workflowJS}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/sp-1/workitemTypes/t-bug/fields"):
			s.gets = append(s.gets, "fields")
			_, _ = w.Write([]byte(s.fieldsJS))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/sp-1/workitemTypes/t-bug/workflows"):
			s.gets = append(s.gets, "workflows")
			_, _ = w.Write([]byte(s.workflowJS))
		default:
			s.gets = append(s.gets, "OTHER "+r.Method+" "+r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-profile-doctor-not-real")
	t.Setenv(config.EnvOrganizationID, "org-profile-doctor-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

// writeDoctorProfile installs profile "pdoc" under a temp XDG_CONFIG_HOME and
// returns its path. Default shape: bug_fields module=dead-module (missing live),
// environment=f-env (live), bug_statuses confirm=100001 (live).
func writeDoctorProfile(t *testing.T, mutate func(p *profile.Profile)) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p := &profile.Profile{
		Name:      "pdoc",
		SpaceID:   "sp-1",
		BugTypeID: "t-bug",
		BugFields: map[string]string{
			"module":      "dead-module",
			"environment": "f-env",
		},
		BugStatuses: map[string]string{
			"confirm": "100001",
		},
		BugEdges: map[string][]string{},
	}
	if mutate != nil {
		mutate(p)
	}
	path := filepath.Join(dir, "yunxiao", "profiles", "pdoc.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := profile.SaveFile(path, p); err != nil {
		t.Fatal(err)
	}
	return path
}

// runProfileDoctor executes `profile doctor pdoc` and returns stdout, stderr and
// the processExit code (0 when the command did not exit).
func runProfileDoctor(t *testing.T, dryRun bool, extra ...string) (string, string, int) {
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
	processExit = func(c int) {
		code = c
		panic(exitPanic{code: c})
	}
	t.Cleanup(func() { processExit = prevExit })

	resetStringFlags(t, profileDoctorCmd, "all-workflows", "fix-suggest", "write")
	args := append([]string{"profile", "doctor", "pdoc"}, extra...)
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

func decodeDoctorData(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout)
	}
	if !env.OK {
		t.Fatalf("envelope not ok: %+v / %s", env, stdout)
	}
	raw, _ := json.Marshal(env.Data)
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("data: %v", err)
	}
	return data
}

// doctorFindings returns type[0] findings with the given status.
func doctorFindings(t *testing.T, data map[string]any, status string) []map[string]any {
	t.Helper()
	types, _ := data["types"].([]any)
	if len(types) == 0 {
		t.Fatalf("no types in report: %#v", data)
	}
	t0, _ := types[0].(map[string]any)
	raw, _ := t0["findings"].([]any)
	var out []map[string]any
	for _, f := range raw {
		if m, ok := f.(map[string]any); ok && m["status"] == status {
			out = append(out, m)
		}
	}
	return out
}

func doctorFindingByID(findings []map[string]any, id string) map[string]any {
	for _, f := range findings {
		if f["id"] == id {
			return f
		}
	}
	return nil
}

func doctorStr(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// #120 part 1 + 2: findings carry live displayName/nameEn/fieldName/similar_fields,
// and unknown_in_profile gets alias suggestions unless --fix-suggest=false.
func TestProfileDoctorFindingsLiveMetadataAndSuggestions(t *testing.T) {
	cases := []struct {
		name           string
		extra          []string
		wantSuggestion bool
	}{
		{name: "default-fix-suggest-on", wantSuggestion: true},
		{name: "fix-suggest-off", extra: []string{"--fix-suggest=false"}, wantSuggestion: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newDoctorServer(t, doctorFieldsFixture, doctorWorkflowFixture)
			writeDoctorProfile(t, nil)
			stdout, stderr, code := runProfileDoctor(t, false, tc.extra...)

			// dead-module is missing_on_type → mismatches found → exit 1 after the report.
			if code != 1 {
				t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
			}
			data := decodeDoctorData(t, stdout)

			// unknown_in_profile: live displayName + nameEn attached.
			unknown := doctorFindings(t, data, "unknown_in_profile")
			f := doctorFindingByID(unknown, "100010")
			if f == nil {
				t.Fatalf("no unknown_in_profile finding for 100010: %#v", unknown)
			}
			if got := doctorStr(f, "display_name"); got != "处理中" {
				t.Fatalf("display_name=%q want 处理中: %#v", got, f)
			}
			if got := doctorStr(f, "name_en"); got != "In Progress" {
				t.Fatalf("name_en=%q want \"In Progress\": %#v", got, f)
			}
			if tc.wantSuggestion {
				if got := doctorStr(f, "suggest_alias"); got != "processing" {
					t.Fatalf("suggest_alias=%q want processing: %#v", got, f)
				}
				sug := doctorStr(f, "suggestion")
				for _, want := range []string{`bug_statuses["processing"] = "100010"`, `workflows["t-bug"].statuses["处理中"] = "100010"`} {
					if !strings.Contains(sug, want) {
						t.Fatalf("suggestion %q missing %q", sug, want)
					}
				}
			} else {
				if _, has := f["suggestion"]; has {
					t.Fatalf("suggestion must be omitted with --fix-suggest=false: %#v", f)
				}
				if _, has := f["suggest_alias"]; has {
					t.Fatalf("suggest_alias must be omitted with --fix-suggest=false: %#v", f)
				}
			}

			// ok status finding also carries live metadata.
			okStatus := doctorFindings(t, data, "ok")
			if f := doctorFindingByID(okStatus, "100001"); f == nil || doctorStr(f, "display_name") != "待确认" {
				t.Fatalf("ok finding 100001 missing display_name 待确认: %#v", okStatus)
			}

			// ok field finding carries field_name.
			missing := doctorFindings(t, data, "missing_on_type")
			if f := doctorFindingByID(missing, "dead-module"); f == nil {
				t.Fatalf("no missing_on_type finding for dead-module: %#v", missing)
			}
			var modOK map[string]any
			for _, m := range doctorFindings(t, data, "ok") {
				if m["id"] == "f-env" && strings.HasPrefix(doctorStr(m, "source"), "bug_fields.") {
					modOK = m
				}
			}
			if modOK == nil || doctorStr(modOK, "field_name") != "测试环境" {
				t.Fatalf("field f-env ok finding missing field_name 测试环境: %#v", modOK)
			}

			// missing_on_type hints same-name live fields (所属模块 matches module).
			f = doctorFindingByID(missing, "dead-module")
			sim, _ := f["similar_fields"].([]any)
			found := false
			for _, s := range sim {
				sm, _ := s.(map[string]any)
				if doctorStr(sm, "id") == "f-mod" && doctorStr(sm, "field_name") == "所属模块" {
					found = true
				}
			}
			if !found {
				t.Fatalf("similar_fields missing f-mod/所属模块: %#v", f)
			}
		})
	}
}

// #120 part 3: --write backfills suggested status ids (bug_statuses alias +
// workflows[type].statuses), keeps edges untouched, skips conflicts, and
// --dry-run previews without touching the file.
func TestProfileDoctorWrite(t *testing.T) {
	cases := []struct {
		name       string
		mutate     func(p *profile.Profile)
		extra      []string
		wantCode   int
		wantDryRun bool
		verify     func(t *testing.T, data map[string]any, env output.Envelope, stdout string, profilePath string)
	}{
		{
			name:     "write-applies-and-keeps-edges",
			wantCode: 1, // dead-module still missing_on_type after the write
			mutate: func(p *profile.Profile) {
				// 100030 is referenced via bug_edges, so only 100010 is unknown_in_profile.
				p.BugEdges = map[string][]string{"100001": {"100030"}}
			},
			extra: []string{"--write"},
			verify: func(t *testing.T, data map[string]any, env output.Envelope, stdout string, profilePath string) {
				write, _ := data["write"].(map[string]any)
				if write == nil || write["saved"] != true {
					t.Fatalf("write.saved missing/false: %#v", write)
				}
				applied, _ := write["applied"].([]any)
				wantApplied := map[string]bool{
					`bug_statuses["processing"]`: false,
					`workflows.statuses["处理中"]`:  false,
				}
				for _, a := range applied {
					m, _ := a.(map[string]any)
					key := wantAppliedKey(m)
					if _, ok := wantApplied[key]; ok {
						wantApplied[key] = true
					}
				}
				for k, seen := range wantApplied {
					if !seen {
						t.Fatalf("applied missing %s: %#v", k, applied)
					}
				}
				if env.Meta["risk"] != "write" {
					t.Fatalf("meta.risk=%v want write", env.Meta["risk"])
				}
				// File on disk: alias backfilled, displayName key under
				// workflows[t-bug].statuses, edges untouched.
				p, err := profile.Load("pdoc")
				if err != nil {
					t.Fatal(err)
				}
				if p.BugStatuses["processing"] != "100010" {
					t.Fatalf("bug_statuses not backfilled: %#v", p.BugStatuses)
				}
				if _, ok := p.BugStatuses["closed-fixed"]; ok {
					t.Fatalf("100030 is referenced via edges; closed-fixed must stay unset: %#v", p.BugStatuses)
				}
				wf, ok := p.Workflows["t-bug"]
				if !ok || wf.Statuses["处理中"] != "100010" {
					t.Fatalf("workflows[t-bug].statuses not backfilled: %#v", wf)
				}
				if len(p.BugEdges["100001"]) != 1 || p.BugEdges["100001"][0] != "100030" {
					t.Fatalf("bug_edges must be untouched: %#v", p.BugEdges)
				}
				if len(wf.Edges) != 0 {
					t.Fatalf("workflows edges must stay empty: %#v", wf.Edges)
				}
			},
		},
		{
			name:     "write-skips-conflict",
			wantCode: 1,
			mutate: func(p *profile.Profile) {
				// processing already maps to a different (non-live) id: conflict.
				p.BugStatuses["processing"] = "dead-old"
			},
			extra: []string{"--write"},
			verify: func(t *testing.T, data map[string]any, env output.Envelope, stdout string, profilePath string) {
				write, _ := data["write"].(map[string]any)
				if write == nil {
					t.Fatalf("data.write missing: %#v", data)
				}
				skipped, _ := write["skipped"].([]any)
				found := false
				for _, s := range skipped {
					m, _ := s.(map[string]any)
					if m["map"] == "bug_statuses" && m["key"] == "processing" && strings.Contains(doctorStr(m, "reason"), "dead-old") {
						found = true
					}
				}
				if !found {
					t.Fatalf("skipped missing bug_statuses/processing conflict: %#v", skipped)
				}
				p, err := profile.Load("pdoc")
				if err != nil {
					t.Fatal(err)
				}
				if p.BugStatuses["processing"] != "dead-old" {
					t.Fatalf("conflicting key must keep its value: %#v", p.BugStatuses)
				}
				// Non-conflicting displayName target is still written.
				if wf, ok := p.Workflows["t-bug"]; !ok || wf.Statuses["处理中"] != "100010" {
					t.Fatalf("workflows[t-bug].statuses[处理中] should be backfilled: %#v", p.Workflows)
				}
				// The conflict also appears on the finding itself.
				data2 := decodeDoctorData(t, stdout)
				f := doctorFindingByID(doctorFindings(t, data2, "unknown_in_profile"), "100010")
				if !strings.Contains(doctorStr(f, "suggest_conflict"), "dead-old") {
					t.Fatalf("finding 100010 missing suggest_conflict: %#v", f)
				}
			},
		},
		{
			name:       "write-dry-run-previews-only",
			wantCode:   1,
			wantDryRun: true,
			extra:      []string{"--write", "--dry-run"},
			verify: func(t *testing.T, data map[string]any, env output.Envelope, stdout string, profilePath string) {
				if !env.DryRun {
					t.Fatalf("envelope must be dry_run: %+v / %s", env, stdout)
				}
				req, _ := env.Request.(map[string]any)
				if req == nil || req["action"] != "profile doctor --write" {
					t.Fatalf("request preview missing: %#v", req)
				}
				applied, _ := req["applied"].([]any)
				if len(applied) == 0 {
					t.Fatalf("preview applied empty: %#v", req)
				}
				p, err := profile.Load("pdoc")
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := p.BugStatuses["processing"]; ok {
					t.Fatalf("dry-run must not write the profile: %#v", p.BugStatuses)
				}
				if len(p.Workflows) != 0 {
					t.Fatalf("dry-run must not write workflows: %#v", p.Workflows)
				}
				// The full doctor report is embedded in the preview.
				rep, _ := req["report"].(map[string]any)
				if rep == nil || len(doctorFindings(t, rep, "unknown_in_profile")) != 2 {
					t.Fatalf("preview report missing findings: %#v", rep)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newDoctorServer(t, doctorFieldsFixture, doctorWorkflowFixture)
			path := writeDoctorProfile(t, tc.mutate)
			stdout, stderr, code := runProfileDoctor(t, tc.wantDryRun, tc.extra...)
			if code != tc.wantCode {
				t.Fatalf("exit %d want %d stdout=%s stderr=%s", code, tc.wantCode, stdout, stderr)
			}
			var env output.Envelope
			if err := json.Unmarshal([]byte(stdout), &env); err != nil {
				t.Fatalf("stdout JSON: %v / %s", err, stdout)
			}
			if !env.OK {
				t.Fatalf("envelope not ok: %+v / %s", env, stdout)
			}
			var data map[string]any
			if raw, err := json.Marshal(env.Data); err == nil {
				_ = json.Unmarshal(raw, &data)
			}
			tc.verify(t, data, env, stdout, path)
		})
	}
}

func wantAppliedKey(m map[string]any) string {
	mp, _ := m["map"].(string)
	return mp + `["` + doctorStr(m, "key") + `"]`
}

// Healthy profile: no findings to fix — --write saves nothing and the run exits 0.
func TestProfileDoctorWriteHealthyNoop(t *testing.T) {
	newDoctorServer(t, doctorFieldsFixture, doctorWorkflowFixture)
	writeDoctorProfile(t, func(p *profile.Profile) {
		p.BugFields = map[string]string{"environment": "f-env"}
		p.BugStatuses = map[string]string{
			"confirm":      "100001",
			"processing":   "100010",
			"closed-fixed": "100030",
		}
	})
	stdout, stderr, code := runProfileDoctor(t, false, "--write")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	data := decodeDoctorData(t, stdout)
	write, _ := data["write"].(map[string]any)
	if write == nil || write["saved"] != false {
		t.Fatalf("healthy --write must not save: %#v", write)
	}
	applied, _ := write["applied"].([]any)
	if len(applied) != 0 {
		t.Fatalf("healthy --write must apply nothing: %#v", applied)
	}
	if data["ok"] != true {
		t.Fatalf("report should be healthy: %#v", data)
	}
}

// ---- merged from #121 (PR #154) ----

// #121 fixtures: bug type fields with 所属模块 / 所属环境 options + a one-status workflow.
const doctorEnumFieldsFixture = `[
 {"id":"subject","name":"标题","type":"NativeField","format":"string"},
 {"id":"mod-1","name":"所属模块","type":"CustomField","format":"list",
  "options":[{"id":"m-a","value":"MES","displayValue":"MES"},{"id":"m-b","value":"OMS","displayValue":"OMS"},{"id":"m-c","value":"PDM","displayValue":"PDM"},{"id":"m-d","value":"系统服务","displayValue":"系统服务"}]},
 {"id":"env-1","name":"所属环境","type":"CustomField","format":"list",
  "options":[{"id":"e-1","value":"生产环境","displayValue":"生产环境"},{"id":"e-2","value":"测试环境","displayValue":"测试环境"}]}
]`

const doctorEnumWorkflowFixture = `{"id":"wf-1","name":"bug","statuses":[{"id":"st-1","name":"待处理","displayName":"待处理"}]}`

type doctorEnumServer struct {
	mu       sync.Mutex
	fields   string
	gets     int
	otherReq []string
}

func newDoctorEnumServer(t *testing.T, fields string) *doctorEnumServer {
	t.Helper()
	s := &doctorEnumServer{fields: fields}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/space-enum/workitemTypes/type-bug/fields"):
			s.gets++
			_, _ = io.WriteString(w, s.fields)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/space-enum/workitemTypes/type-bug/workflows"):
			_, _ = io.WriteString(w, doctorEnumWorkflowFixture)
		default:
			s.otherReq = append(s.otherReq, r.Method+" "+r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-profile-doctor-enum-not-real")
	t.Setenv(config.EnvOrganizationID, "org-profile-doctor-enum")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	return s
}

// runProfileDoctor executes `profile doctor <name>` and returns stdout, stderr and
// the processExit code (0 when the command did not exit).
func runProfileDoctorEnum(t *testing.T, name string) (string, string, int) {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	prevDry := globalDryRun
	globalDryRun = false
	t.Cleanup(func() { globalDryRun = prevDry })
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

	rootCmd.SetArgs([]string{"profile", "doctor", name})
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

type doctorEnumFinding struct {
	ID          string   `json:"id"`
	Source      string   `json:"source"`
	FieldID     string   `json:"field_id,omitempty"`
	Status      string   `json:"status"`
	LiveOptions []string `json:"live_options,omitempty"`
	Allowed     []string `json:"allowed,omitempty"`
}

type doctorEnumReport struct {
	OK    bool `json:"ok"`
	Types []struct {
		TypeID   string              `json:"type_id"`
		OK       bool                `json:"ok"`
		Findings []doctorEnumFinding `json:"findings"`
		Counts   map[string]int      `json:"counts"`
	} `json:"types"`
}

func decodeDoctorEnumReport(t *testing.T, stdout string) doctorEnumReport {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil || !env.OK {
		t.Fatalf("stdout envelope: %v ok=%v / %s", err, env.OK, stdout)
	}
	raw, _ := json.Marshal(env.Data)
	var rep doctorEnumReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatalf("decode report: %v / %s", err, stdout)
	}
	if len(rep.Types) != 1 {
		t.Fatalf("want exactly the bug type report: %+v", rep)
	}
	return rep
}

func findStatus(rep doctorEnumReport, status string) []doctorEnumFinding {
	var out []doctorEnumFinding
	for _, f := range rep.Types[0].Findings {
		if f.Status == status {
			out = append(out, f)
		}
	}
	return out
}

// #121 acceptance: the allowed_modules snapshot drifts from the live 所属模块 options
// (公共组件 removed, 系统服务 added) → doctor reports both directions as findings and
// fails; the in-sync allowed_environments produces no enum findings.
func TestProfileDoctorAllowedEnumDriftReported(t *testing.T) {
	cases := []struct {
		name        string
		fields      string
		pf          *profile.Profile
		wantCode    int
		wantOK      bool
		wantFindIns []doctorEnumFinding // findings that must be present (id+status)
		wantNoEnum  bool
	}{
		{
			name:   "module_drift_both_directions",
			fields: doctorEnumFieldsFixture,
			pf: &profile.Profile{
				Name:      "docenum",
				SpaceID:   "space-enum",
				BugTypeID: "type-bug",
				BugCreateFields: profile.BugCreateFields{
					Module:      "mod-1",
					Environment: "env-1",
				},
				AllowedModules:      []string{"MES", "OMS", "PDM", "公共组件"},
				AllowedEnvironments: []string{"生产环境", "测试环境"},
			},
			wantCode: 1,
			wantOK:   false,
			wantFindIns: []doctorEnumFinding{
				{ID: "公共组件", Source: "allowed_modules", FieldID: "mod-1", Status: "enum_stale_in_profile"},
				{ID: "系统服务", Source: "allowed_modules", FieldID: "mod-1", Status: "enum_missing_in_profile"},
			},
		},
		{
			name:   "in_sync_no_enum_findings",
			fields: doctorEnumFieldsFixture,
			pf: &profile.Profile{
				Name:      "docenum",
				SpaceID:   "space-enum",
				BugTypeID: "type-bug",
				BugCreateFields: profile.BugCreateFields{
					Module:      "mod-1",
					Environment: "env-1",
				},
				AllowedModules:      []string{"MES", "OMS", "PDM", "系统服务"},
				AllowedEnvironments: []string{"生产环境", "测试环境"},
			},
			wantCode:   0,
			wantOK:     true,
			wantNoEnum: true,
		},
		{
			// Field exists but exposes no options (text format): informational
			// enum_unverified, does not fail the check.
			name: "field_without_options_is_unverified",
			fields: `[
			 {"id":"mod-1","name":"所属模块","type":"CustomField","format":"text"},
			 {"id":"env-1","name":"所属环境","type":"CustomField","format":"list",
			  "options":[{"id":"e-1","value":"生产环境","displayValue":"生产环境"}]}
			]`,
			pf: &profile.Profile{
				Name:      "docenum",
				SpaceID:   "space-enum",
				BugTypeID: "type-bug",
				BugCreateFields: profile.BugCreateFields{
					Module:      "mod-1",
					Environment: "env-1",
				},
				AllowedModules:      []string{"MES"},
				AllowedEnvironments: []string{"生产环境"},
			},
			wantCode: 0,
			wantOK:   true,
			wantFindIns: []doctorEnumFinding{
				{ID: "mod-1", Source: "allowed_modules", Status: "enum_unverified"},
			},
		},
		{
			// Field id gone from the live type: existing missing_on_type covers it;
			// the enum diff must stay silent instead of double-reporting.
			name:   "missing_field_id_no_enum_findings",
			fields: doctorEnumFieldsFixture,
			pf: &profile.Profile{
				Name:      "docenum",
				SpaceID:   "space-enum",
				BugTypeID: "type-bug",
				BugCreateFields: profile.BugCreateFields{
					Module:      "mod-gone",
					Environment: "env-1",
				},
				AllowedModules:      []string{"MES"},
				AllowedEnvironments: []string{"生产环境", "测试环境"},
			},
			wantCode:   1,
			wantOK:     false,
			wantNoEnum: true,
		},
		{
			// play-like profile: no module/environment field ids (and no snapshots)
			// → the enum check is skipped entirely.
			name:   "no_gate_skips_enum_check",
			fields: doctorEnumFieldsFixture,
			pf: &profile.Profile{
				Name:                "docenum",
				SpaceID:             "space-enum",
				BugTypeID:           "type-bug",
				AllowedEnvironments: []string{},
			},
			wantCode:   0,
			wantOK:     true,
			wantNoEnum: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newDoctorEnumServer(t, tc.fields)
			xdg := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", xdg)
			writeTransitionTestProfile(t, xdg, tc.pf)

			stdout, stderr, code := runProfileDoctorEnum(t, "docenum")
			rep := decodeDoctorEnumReport(t, stdout)

			if rep.OK != tc.wantOK || rep.Types[0].OK != tc.wantOK {
				t.Fatalf("ok=%v type ok=%v want %v stdout=%s stderr=%s", rep.OK, rep.Types[0].OK, tc.wantOK, stdout, stderr)
			}
			if code != tc.wantCode {
				t.Fatalf("exit=%d want %d stdout=%s stderr=%s", code, tc.wantCode, stdout, stderr)
			}
			for _, want := range tc.wantFindIns {
				found := false
				for _, f := range findStatus(rep, want.Status) {
					if f.ID == want.ID && f.Source == want.Source && f.FieldID == want.FieldID {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("missing finding %+v in %#v", want, rep.Types[0].Findings)
				}
			}
			if tc.wantNoEnum {
				for _, f := range rep.Types[0].Findings {
					if strings.HasPrefix(f.Status, "enum_") {
						t.Fatalf("unexpected enum finding %+v", f)
					}
				}
			}
			// Enum drift must also show up in counts (and never when wantNoEnum).
			enumCount := 0
			for st, n := range rep.Types[0].Counts {
				if strings.HasPrefix(st, "enum_") {
					enumCount += n
				}
			}
			if tc.wantNoEnum && enumCount != 0 {
				t.Fatalf("counts must not contain enum_* : %+v", rep.Types[0].Counts)
			}
			if !tc.wantNoEnum && enumCount < len(tc.wantFindIns) {
				t.Fatalf("counts=%+v want at least %d enum findings", rep.Types[0].Counts, len(tc.wantFindIns))
			}
			if s.gets != 1 || len(s.otherReq) != 0 {
				t.Fatalf("fieldsGETs=%d other=%v", s.gets, s.otherReq)
			}
		})
	}
}

// Drift findings carry the live options / allowed snapshot so the profile edit is
// actionable without another API call.
func TestProfileDoctorEnumFindingsCarryLists(t *testing.T) {
	newDoctorEnumServer(t, doctorEnumFieldsFixture)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:      "docenum",
		SpaceID:   "space-enum",
		BugTypeID: "type-bug",
		BugCreateFields: profile.BugCreateFields{
			Module: "mod-1",
		},
		AllowedModules: []string{"公共组件"},
	})

	stdout, _, code := runProfileDoctorEnum(t, "docenum")
	if code != 1 {
		t.Fatalf("exit=%d stdout=%s", code, stdout)
	}
	rep := decodeDoctorEnumReport(t, stdout)
	stale := findStatus(rep, "enum_stale_in_profile")
	if len(stale) != 1 || stale[0].ID != "公共组件" {
		t.Fatalf("stale=%+v", stale)
	}
	if strings.Join(stale[0].LiveOptions, "/") != "MES/OMS/PDM/系统服务" {
		t.Fatalf("live_options=%+v", stale[0].LiveOptions)
	}
	missing := findStatus(rep, "enum_missing_in_profile")
	if len(missing) != 4 {
		t.Fatalf("missing=%+v", missing)
	}
	if strings.Join(missing[0].Allowed, "/") != "公共组件" {
		t.Fatalf("allowed=%+v", missing[0].Allowed)
	}
}

func TestProfileDoctorHelpDocumentsEnumFindings(t *testing.T) {
	for _, want := range []string{"allowed_environments", "allowed_modules", "enum_stale_in_profile", "enum_missing_in_profile", "enum_unverified"} {
		if !strings.Contains(profileDoctorCmd.Long, want) {
			t.Fatalf("doctor help missing %q: %s", want, profileDoctorCmd.Long)
		}
	}
}
