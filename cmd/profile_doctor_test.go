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

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

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
func runProfileDoctor(t *testing.T, name string) (string, string, int) {
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

			stdout, stderr, code := runProfileDoctor(t, "docenum")
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

	stdout, _, code := runProfileDoctor(t, "docenum")
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
