package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

// execWorkitemCmdCapture runs rootCmd with args under captured JSON output and the
// processExit override; returns the exit code (0 when Execute returned normally),
// stdout and stderr buffers. dry/yes control the global flags.
func execWorkitemCmdCapture(t *testing.T, args []string, dry, yes bool) (int, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	prevOut, prevErr, prevJQ, prevFmt := output.Stdout, output.Stderr, output.JQ, output.Format
	output.Stdout, output.Stderr, output.JQ, output.Format = stdout, stderr, "", "json"
	prevYes, prevDry, prevProfile, prevOrg := globalYes, globalDryRun, globalProfile, globalOrg
	globalYes, globalDryRun, globalProfile, globalOrg = yes, dry, "", ""
	prevWarn := refreshWarnOut
	refreshWarnOut = stderr
	t.Cleanup(func() {
		output.Stdout, output.Stderr, output.JQ, output.Format = prevOut, prevErr, prevJQ, prevFmt
		globalYes, globalDryRun, globalProfile, globalOrg = prevYes, prevDry, prevProfile, prevOrg
		refreshWarnOut = prevWarn
	})

	prevExit := processExit
	code := 0
	processExit = func(c int) {
		code = c
		panic(exitPanic{code: c})
	}
	t.Cleanup(func() { processExit = prevExit })

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
	return code, stdout, stderr
}

// noRetrySleep makes GET retry backoff instant for failure-path tests.
func noRetrySleep(t *testing.T) {
	t.Helper()
	restore := client.SetRetrySleepForTest(func(ctx context.Context, d time.Duration) error { return nil })
	t.Cleanup(restore)
}

// setupTransition113Env writes a profile with the given workflow and points the CLI at srv.
func setupTransition113Env(t *testing.T, srv *httptest.Server, name string, wf profile.WorkitemWorkflow) {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           name,
		OrganizationID: "org-113",
		SpaceID:        "space-1",
		Workflows:      map[string]profile.WorkitemWorkflow{"type-req-1": wf},
	})
	t.Setenv(config.EnvAccessToken, "test-token-113-not-real")
	t.Setenv(config.EnvOrganizationID, "org-113")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
}

// t113Item is the pre-PUT GET payload.
func t113Item() map[string]any {
	return map[string]any{
		"id":             "wi-113",
		"serialNumber":   "ZYPT-113",
		"spaceId":        map[string]any{"id": "space-1"},
		"categoryId":     "Req",
		"workitemTypeId": "type-req-1",
		"subject":        "需求 113",
		"description":    "<p>很长的描述</p>",
		"status":         map[string]any{"id": "st-pending", "displayName": "待处理"},
		"customFieldValues": []any{
			map[string]any{"fieldId": "f80", "values": []any{"2026-08-01T00:00:00+08:00"}},
		},
	}
}

var t113FieldsBody = map[string]any{
	"result": []any{
		map[string]any{"id": "sprint", "name": "迭代", "format": "sprint", "type": "NativeField", "required": false},
		map[string]any{"id": "f80", "name": "计划提测时间", "format": "dateTime", "type": "SystemCustomField", "required": false},
	},
}

func t113FieldsHandler(w http.ResponseWriter) {
	_ = json.NewEncoder(w).Encode(t113FieldsBody)
}

// TestTransitionPut400RequiredFieldsMapped exercises the #113 400→fieldId mapping end
// to end: 必填 lists mapped against the field config, degraded mapping when the fields
// endpoint is unusable, and passthrough for non-target errors.
func TestTransitionPut400RequiredFieldsMapped(t *testing.T) {
	cases := []struct {
		name string
		// server behaviour
		putStatus  int
		putBody    string
		put2Status int // second PUT in the multi-step case (0: single step only)
		fields     func(w http.ResponseWriter)
		// expectations
		wantOK         bool
		wantSubtype    string
		wantType       string
		wantCode       int
		check          func(t *testing.T, env output.Envelope)
		wantFieldsGets int64 // fields endpoint calls (-1: don't check)
	}{
		{
			name:        "400 mapped with draft and unmapped passthrough",
			putStatus:   http.StatusBadRequest,
			putBody:     `{"errorCode":"InvalidParam","errorMessage":"迭代必填;计划提测时间必填;神秘字段必填"}`,
			fields:      t113FieldsHandler,
			wantOK:      false,
			wantSubtype: "transition_required_fields",
			wantType:    "api",
			wantCode:    400,
			check: func(t *testing.T, env output.Envelope) {
				d := env.Error.Details
				if d["step"] != "1/1" {
					t.Fatalf("step = %v", d["step"])
				}
				if d["server_message"] != "迭代必填;计划提测时间必填;神秘字段必填" {
					t.Fatalf("server_message = %v", d["server_message"])
				}
				fields, _ := d["fields"].([]any)
				if len(fields) != 2 {
					t.Fatalf("fields = %v", d["fields"])
				}
				f0, _ := fields[0].(map[string]any)
				if f0["field_id"] != "sprint" || f0["name"] != "迭代" || f0["pass_via"] != "--sprint" {
					t.Fatalf("fields[0] = %v", f0)
				}
				if f0["draft"] != "--sprint <sprintId>" {
					t.Fatalf("fields[0].draft = %v", f0["draft"])
				}
				if _, has := f0["current_value"]; !has {
					t.Fatalf("fields[0] missing current_value: %v", f0)
				}
				f1, _ := fields[1].(map[string]any)
				if f1["field_id"] != "f80" {
					t.Fatalf("fields[1] = %v", f1)
				}
				if cv, _ := f1["current_value"].([]any); len(cv) != 1 {
					t.Fatalf("fields[1].current_value = %v (from customFieldValues)", f1["current_value"])
				}
				if draft, _ := f1["draft"].(string); !strings.Contains(draft, `--custom-fields '{"f80":"<date>"}`) {
					t.Fatalf("fields[1].draft = %v", f1["draft"])
				}
				if unm, _ := d["unmapped_names"].([]any); len(unm) != 1 || unm[0] != "神秘字段" {
					t.Fatalf("unmapped_names = %v", d["unmapped_names"])
				}
				fd, _ := d["fields_draft"].(map[string]any)
				if fd["sprint"] != "<sprintId>" || fd["f80"] != "<date>" {
					t.Fatalf("fields_draft = %v", fd)
				}
				mp, _ := d["mapping"].(map[string]any)
				if mp["source"] != "fields_endpoint" || mp["matched"] != float64(2) || mp["unmapped"] != float64(1) {
					t.Fatalf("mapping = %v", mp)
				}
				if !strings.Contains(env.Error.Message, "流转在第 1/1 步失败") || !strings.Contains(env.Error.Message, "迭代必填") {
					t.Fatalf("message must keep the original text: %q", env.Error.Message)
				}
				if !strings.Contains(env.Error.Hint, "workitem +transition --id ZYPT-113 --to 待测试 --fields") ||
					!strings.Contains(env.Error.Hint, "--yes") {
					t.Fatalf("hint = %q", env.Error.Hint)
				}
				if ins, _ := d["inspect"].(string); !strings.Contains(ins, "workitem fields --space-id space-1 --type-id type-req-1") {
					t.Fatalf("inspect = %v", ins)
				}
			},
			wantFieldsGets: 1,
		},
		{
			name:        "400 mapping degraded when fields endpoint errors",
			putStatus:   http.StatusBadRequest,
			putBody:     `迭代必填;计划提测时间必填`,
			fields:      func(w http.ResponseWriter) { w.WriteHeader(http.StatusInternalServerError) },
			wantOK:      false,
			wantSubtype: "transition_required_fields",
			wantType:    "api",
			wantCode:    400,
			check: func(t *testing.T, env output.Envelope) {
				d := env.Error.Details
				if _, has := d["fields"]; has {
					t.Fatalf("no fields on degraded mapping: %v", d)
				}
				mp, _ := d["mapping"].(map[string]any)
				if mp["source"] != "fields_endpoint_error" {
					t.Fatalf("mapping = %v", mp)
				}
				if unm, _ := d["unmapped_names"].([]any); len(unm) != 2 {
					t.Fatalf("unmapped_names = %v (verbatim passthrough)", unm)
				}
				if !strings.Contains(env.Error.Message, "迭代必填") {
					t.Fatalf("server text lost: %q", env.Error.Message)
				}
				if !strings.Contains(env.Error.Hint, "workitem fields") {
					t.Fatalf("hint = %q", env.Error.Hint)
				}
			},
			wantFieldsGets: 2, // bounded retry: 2 attempts
		},
		{
			name:        "400 mapping degraded when fields endpoint empty",
			putStatus:   http.StatusBadRequest,
			putBody:     `迭代必填`,
			fields:      func(w http.ResponseWriter) { _ = json.NewEncoder(w).Encode([]any{}) },
			wantOK:      false,
			wantSubtype: "transition_required_fields",
			check: func(t *testing.T, env output.Envelope) {
				mp, _ := env.Error.Details["mapping"].(map[string]any)
				if mp["source"] != "fields_endpoint_empty" {
					t.Fatalf("mapping = %v", mp)
				}
			},
			wantFieldsGets: 1,
		},
		{
			name:        "400 without required-field text passes through",
			putStatus:   http.StatusBadRequest,
			putBody:     `{"errorMsg":"boom"}`,
			fields:      t113FieldsHandler,
			wantOK:      false,
			wantSubtype: "",
			wantType:    "cli", // pre-#113 behaviour: wrapped error, no structured subtype
			check: func(t *testing.T, env output.Envelope) {
				if !strings.Contains(env.Error.Message, "流转在第 1/1 步失败") || !strings.Contains(env.Error.Message, "boom") {
					t.Fatalf("message = %q", env.Error.Message)
				}
			},
			wantFieldsGets: 0,
		},
		{
			name:        "non-400 error passes through",
			putStatus:   http.StatusForbidden,
			putBody:     `{"errorMsg":"denied"}`,
			fields:      t113FieldsHandler,
			wantOK:      false,
			wantSubtype: "",
			wantType:    "cli",
			check: func(t *testing.T, env output.Envelope) {
				if !strings.Contains(env.Error.Message, "denied") {
					t.Fatalf("message = %q", env.Error.Message)
				}
			},
			wantFieldsGets: 0,
		},
		{
			name:        "multi-step failure reports step and applied",
			putStatus:   http.StatusOK,
			putBody:     `{}`,
			put2Status:  http.StatusBadRequest,
			fields:      t113FieldsHandler,
			wantOK:      false,
			wantSubtype: "transition_required_fields",
			check: func(t *testing.T, env output.Envelope) {
				d := env.Error.Details
				if d["step"] != "2/2" {
					t.Fatalf("step = %v", d["step"])
				}
				if ap, _ := d["applied"].([]any); len(ap) != 1 || ap[0] != "st-mid" {
					t.Fatalf("applied = %v", d["applied"])
				}
				if !strings.Contains(env.Error.Message, "流转在第 2/2 步失败") {
					t.Fatalf("message = %q", env.Error.Message)
				}
			},
			wantFieldsGets: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			noRetrySleep(t)

			var fieldsGets, puts int64
			multiStep := tc.put2Status != 0
			wf := profile.WorkitemWorkflow{
				Name:     "产品类需求",
				Category: "Req",
				Statuses: map[string]string{"待处理": "st-pending", "待测试": "st-test"},
				Edges:    map[string][]string{"st-pending": {"st-test"}, "st-test": {}},
			}
			if multiStep {
				wf.Statuses["中转"] = "st-mid"
				wf.Edges = map[string][]string{"st-pending": {"st-mid"}, "st-mid": {"st-test"}, "st-test": {}}
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "/workitemTypes/type-req-1/fields") && r.Method == http.MethodGet:
					atomic.AddInt64(&fieldsGets, 1)
					tc.fields(w)
				case strings.HasSuffix(r.URL.Path, "/workitems/wi-113") && r.Method == http.MethodPut:
					n := atomic.AddInt64(&puts, 1)
					if multiStep && n == 1 {
						w.WriteHeader(tc.putStatus)
						_, _ = w.Write([]byte(tc.putBody))
						return
					}
					if multiStep {
						w.WriteHeader(tc.put2Status)
						_, _ = w.Write([]byte(`{"errorCode":"InvalidParam","errorMessage":"迭代必填"}`))
						return
					}
					w.WriteHeader(tc.putStatus)
					_, _ = w.Write([]byte(tc.putBody))
				case strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet:
					_ = json.NewEncoder(w).Encode(t113Item())
				default:
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"error":"unexpected ` + r.Method + ` ` + r.URL.Path + `"}`))
				}
			}))
			t.Cleanup(srv.Close)
			setupTransition113Env(t, srv, "t113", wf)

			code, _, stderr := execWorkitemCmdCapture(t, []string{
				"workitem", "+transition",
				"--profile", "t113",
				"--id", "ZYPT-113",
				"--to", "待测试",
				"--yes",
			}, false, true)
			if code != 1 {
				t.Fatalf("exit = %d stderr = %s", code, stderr.String())
			}
			var env output.Envelope
			if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
				t.Fatalf("stderr JSON: %v / %s", err, stderr.String())
			}
			if env.OK != tc.wantOK {
				t.Fatalf("ok = %v envelope = %+v", env.OK, env)
			}
			if env.Error == nil {
				t.Fatalf("missing error envelope: %s", stderr.String())
			}
			if env.Error.Subtype != tc.wantSubtype {
				t.Fatalf("subtype = %q want %q (%s)", env.Error.Subtype, tc.wantSubtype, stderr.String())
			}
			if tc.wantType != "" && env.Error.Type != tc.wantType {
				t.Fatalf("type = %q want %q", env.Error.Type, tc.wantType)
			}
			if tc.wantCode != 0 && env.Error.Code != tc.wantCode {
				t.Fatalf("code = %d want %d", env.Error.Code, tc.wantCode)
			}
			if tc.check != nil {
				tc.check(t, env)
			}
			if tc.wantFieldsGets >= 0 && atomic.LoadInt64(&fieldsGets) != tc.wantFieldsGets {
				t.Fatalf("fields endpoint calls = %d want %d", fieldsGets, tc.wantFieldsGets)
			}
		})
	}
}

// TestTransitionDryRunAnnotatesUnknownEntryRequired: dry-run must stay read-only
// (zero PUTs) and, when no required-field ids are known, tell the user that
// status-entry required fields are undiscoverable up front (#113).
func TestTransitionDryRunAnnotatesUnknownEntryRequired(t *testing.T) {
	var puts int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(t113Item())
		case r.Method == http.MethodPut:
			atomic.AddInt64(&puts, 1)
			w.WriteHeader(http.StatusTeapot)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	setupTransition113Env(t, srv, "t113d", profile.WorkitemWorkflow{
		Name:     "产品类需求",
		Category: "Req",
		Statuses: map[string]string{"待处理": "st-pending", "待测试": "st-test"},
		Edges:    map[string][]string{"st-pending": {"st-test"}, "st-test": {}},
	})

	code, stdout, _ := execWorkitemCmdCapture(t, []string{
		"workitem", "+transition",
		"--profile", "t113d",
		"--id", "ZYPT-113",
		"--to", "待测试",
		"--dry-run",
	}, true, false)
	if code != 0 {
		t.Fatalf("exit = %d stdout = %s", code, stdout.String())
	}
	if puts != 0 {
		t.Fatalf("dry-run must not PUT, got %d", puts)
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
	note, _ := req["required_fields_note"].(string)
	if note == "" || !strings.Contains(note, "状态入场必填") {
		t.Fatalf("required_fields_note = %q req = %s", note, raw)
	}
	if req["edge_validation"] != "validated" {
		t.Fatalf("edge_validation = %v (note must not change validation)", req["edge_validation"])
	}
}
