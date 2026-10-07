package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

// #126 fixture: the #95 fields plus a multiList field (分类) and a list field whose
// display value 高 is ambiguous (影响程度, two ids share the display value).
const wiFieldsResolveFixture = `[
 {"id":"subject","name":"标题","type":"NativeField","format":"string","required":true,"showWhenCreate":true},
 {"id":"assignedTo","name":"负责人","type":"NativeField","format":"user","required":true,"showWhenCreate":true},
 {"id":"priority","name":"优先级","type":"SystemCustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"prio-high","value":"高","displayValue":"高"},{"id":"prio-low","value":"低","displayValue":"低"}]},
 {"id":"mod-1","name":"所属模块","type":"CustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"m-a","value":"MES","displayValue":"MES"}]},
 {"id":"note","name":"备注","type":"CustomField","format":"text","required":false,"showWhenCreate":true},
 {"id":"cats","name":"分类","type":"CustomField","format":"multiList","required":false,"showWhenCreate":true,
  "options":[{"id":"c-1","value":"甲","displayValue":"甲"},{"id":"c-2","value":"乙","displayValue":"乙"}]},
 {"id":"impact","name":"影响程度","type":"CustomField","format":"list","required":false,"showWhenCreate":true,
  "options":[{"id":"imp-a","value":"高","displayValue":"高"},{"id":"imp-b","value":"大","displayValue":"高"}]}
]`

// wiResolveResolution decodes meta.option_resolution (success) / request.option_resolution (dry-run).
func wiResolveResolution(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	res, _ := m["option_resolution"].(map[string]any)
	if res == nil {
		t.Fatalf("option_resolution missing from %v", m)
	}
	return res
}

func wiResolvedEntries(t *testing.T, res map[string]any) []map[string]any {
	t.Helper()
	raw, _ := res["resolved"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		m, _ := r.(map[string]any)
		out = append(out, m)
	}
	return out
}

// wiCF returns the customFieldValues map of a POSTed body (server side).
func wiCF(t *testing.T, posts []map[string]any) map[string]any {
	t.Helper()
	if len(posts) != 1 {
		t.Fatalf("posts=%d", len(posts))
	}
	cf, _ := posts[0]["customFieldValues"].(map[string]any)
	if cf == nil {
		t.Fatalf("customFieldValues missing: %#v", posts[0])
	}
	return cf
}

func TestWorkitemCreateOptionDisplayValues(t *testing.T) {
	cases := []struct {
		name string
		// fixture: "" = wiFieldsFixture (#95), "resolve" = wiFieldsResolveFixture
		fixture     string
		fieldsSt    int // non-zero: fields GET fails with this status (degrade)
		dryRun      bool
		extra       []string
		wantCode    int
		wantSubtype string   // "" = success
		wantMsg     []string // message parts (failure cases)
		wantHint    []string // hint parts (failure cases)
		// POST expectations (real run, wantCode 0)
		wantPosts  int
		wantPostCF map[string]any
		// meta.option_resolution expectations
		wantResStatus string // "" = key absent
		wantResolved  []map[string]any
		wantNoPrecheckMeta bool
	}{
		{
			name:         "display values resolve to option ids before POST",
			extra:        []string{"--custom-fields", `{"priority":"高","mod-1":"MES","note":"文本"}`},
			wantCode:     0,
			wantPosts:    1,
			wantPostCF:   map[string]any{"priority": "prio-high", "mod-1": "m-a", "note": "文本"},
			wantResStatus: "ok",
			wantResolved: []map[string]any{
				{"field_id": "priority", "field_name": "优先级", "from": "高", "to": "prio-high"},
				{"field_id": "mod-1", "field_name": "所属模块", "from": "MES", "to": "m-a"},
			},
		},
		{
			name:         "exact ids pass through, text and unknown fields untouched",
			extra:        []string{"--custom-fields", `{"priority":"prio-low","mod-1":"m-a","note":"自由文本","unknown-fid":"x"}`},
			wantCode:     0,
			wantPosts:    1,
			wantPostCF:   map[string]any{"priority": "prio-low", "mod-1": "m-a", "note": "自由文本", "unknown-fid": "x"},
			wantResStatus: "ok",
			wantResolved:  nil, // no "resolved" key
		},
		{
			name:         "multiList display values resolve per element",
			fixture:      "resolve",
			extra:        []string{"--custom-fields", `{"priority":"高","mod-1":"MES","cats":["甲","c-2"]}`},
			wantCode:     0,
			wantPosts:    1,
			wantPostCF:   map[string]any{"priority": "prio-high", "mod-1": "m-a", "cats": []any{"c-1", "c-2"}},
			wantResStatus: "ok",
			wantResolved: []map[string]any{
				{"field_id": "priority", "field_name": "优先级", "from": "高", "to": "prio-high"},
				{"field_id": "mod-1", "field_name": "所属模块", "from": "MES", "to": "m-a"},
				{"field_id": "cats", "field_name": "分类", "from": "甲", "to": "c-1"},
			},
		},
		{
			name:      "dry-run previews resolved ids and request.option_resolution",
			dryRun:    true,
			extra:     []string{"--custom-fields", `{"priority":"高","mod-1":"MES"}`},
			wantCode:  0,
			wantPosts: 0,
			wantResStatus: "ok",
			wantResolved: []map[string]any{
				{"field_id": "priority", "field_name": "优先级", "from": "高", "to": "prio-high"},
				{"field_id": "mod-1", "field_name": "所属模块", "from": "MES", "to": "m-a"},
			},
		},
		{
			name:         "unknown value fails client-side with the valid options",
			extra:        []string{"--custom-fields", `{"priority":"最高","mod-1":"MES"}`},
			wantCode:     1,
			wantSubtype:  "invalid_option_values",
			wantMsg:      []string{"workitem create", "1 invalid option value", `优先级 (priority): "最高"`, "no option id or display value"},
			wantHint:     []string{"error.details.values[].options", "yunxiao workitem fields --space-id space-1 --type-id type-req", "--no-precheck"},
			wantPosts:    0,
			wantResStatus: "",
		},
		{
			name:         "resolution failure wins over missing required fields",
			extra:        []string{"--custom-fields", `{"priority":"最高"}`},
			wantCode:     1,
			wantSubtype:  "invalid_option_values",
			wantPosts:    0,
			wantResStatus: "",
		},
		{
			name:         "ambiguous display value reports the colliding options",
			fixture:      "resolve",
			extra:        []string{"--custom-fields", `{"priority":"高","mod-1":"MES","impact":"高"}`},
			wantCode:     1,
			wantSubtype:  "invalid_option_values",
			wantMsg:      []string{"影响程度 (impact)", "ambiguous", "imp-a, imp-b"},
			wantPosts:    0,
			wantResStatus: "",
		},
		{
			name:             "--no-precheck sends values as-is with no resolution meta",
			extra:            []string{"--no-precheck", "--custom-fields", `{"priority":"高","mod-1":"MES"}`},
			wantCode:         0,
			wantPosts:        1,
			wantPostCF:       map[string]any{"priority": "高", "mod-1": "MES"},
			wantResStatus:    "",
			wantNoPrecheckMeta: true,
		},
		{
			name:          "degraded config sends values as-is with skipped resolution",
			fieldsSt:      403,
			extra:         []string{"--custom-fields", `{"priority":"高","mod-1":"MES"}`},
			wantCode:      0,
			wantPosts:     1,
			wantPostCF:    map[string]any{"priority": "高", "mod-1": "MES"},
			wantResStatus: "skipped",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := wiFieldsFixture
			if tc.fixture == "resolve" {
				fixture = wiFieldsResolveFixture
			}
			s := newWiCreateServer(t, fixture)
			s.fieldsStatus = tc.fieldsSt
			r := runWiCreate(t, tc.dryRun, false, tc.extra...)
			if r.code != tc.wantCode {
				t.Fatalf("exit=%d want %d stdout=%s stderr=%s", r.code, tc.wantCode, r.stdout, r.stderr)
			}
			if tc.wantCode != 0 {
				eb := wiErrorBody(t, r.stderr)
				if eb.Type != "cli" || eb.Subtype != tc.wantSubtype {
					t.Fatalf("error body=%+v", eb)
				}
				for _, want := range tc.wantMsg {
					if !strings.Contains(eb.Message, want) {
						t.Fatalf("message missing %q: %s", want, eb.Message)
					}
				}
				for _, want := range tc.wantHint {
					if !strings.Contains(eb.Hint, want) {
						t.Fatalf("hint missing %q: %s", want, eb.Hint)
					}
				}
				if eb.Details["space_id"] != "space-1" || eb.Details["type_id"] != "type-req" {
					t.Fatalf("details=%#v", eb.Details)
				}
				values, _ := eb.Details["values"].([]any)
				if len(values) != 1 {
					t.Fatalf("details.values=%#v", eb.Details["values"])
				}
				v := values[0].(map[string]any)
				if v["field_id"] != "priority" && v["field_id"] != "impact" {
					t.Fatalf("value=%#v", v)
				}
				opts, _ := v["options"].([]any)
				if len(opts) != 2 || v["options_total"] != float64(2) {
					t.Fatalf("value options=%#v total=%v", opts, v["options_total"])
				}
				if o := opts[0].(map[string]any); o["id"] == "" || o["display_value"] == "" {
					t.Fatalf("option=%#v", o)
				}
				if len(s.posts) != tc.wantPosts {
					t.Fatalf("posts=%d want %d", len(s.posts), tc.wantPosts)
				}
				return
			}
			// Success / dry-run path.
			if len(s.posts) != tc.wantPosts {
				t.Fatalf("posts=%d want %d", len(s.posts), tc.wantPosts)
			}
			if !tc.dryRun && tc.wantPosts == 1 {
				cf := wiCF(t, s.posts)
				for k, want := range tc.wantPostCF {
					got := cf[k]
					if want == nil {
						if _, ok := cf[k]; ok {
							t.Fatalf("cf[%q]=%#v want absent", k, cf[k])
						}
						continue
					}
					if !jsonEqual(got, want) {
						t.Fatalf("cf[%q]=%#v want %#v (all=%#v)", k, got, want, cf)
					}
				}
				if len(cf) != len(tc.wantPostCF) {
					t.Fatalf("cf=%#v want %d keys", cf, len(tc.wantPostCF))
				}
			}
			var m map[string]any
			if tc.dryRun {
				req := wiDryRunRequest(t, r.stdout)
				m = req
				if body, _ := req["body"].(map[string]any); body != nil {
					cf, _ := body["customFieldValues"].(map[string]any)
					for k, want := range tc.wantPostCF {
						if !jsonEqual(cf[k], want) {
							t.Fatalf("dry-run body cf[%q]=%#v want %#v", k, cf[k], want)
						}
					}
				}
			} else {
				m = wiEnvelope(t, r.stdout).Meta
			}
			if tc.wantNoPrecheckMeta {
				if _, ok := m["precheck"]; ok {
					t.Fatalf("no meta.precheck expected: %#v", m)
				}
			}
			if tc.wantResStatus == "" {
				if _, ok := m["option_resolution"]; ok {
					t.Fatalf("no option_resolution expected: %#v", m)
				}
				return
			}
			res := wiResolveResolution(t, m)
			if res["status"] != tc.wantResStatus {
				t.Fatalf("option_resolution=%#v", res)
			}
			entries := wiResolvedEntries(t, res)
			if tc.wantResolved == nil {
				if len(entries) != 0 {
					t.Fatalf("resolved=%#v want none", entries)
				}
				return
			}
			if len(entries) != len(tc.wantResolved) {
				t.Fatalf("resolved=%#v want %#v", entries, tc.wantResolved)
			}
			for i, want := range tc.wantResolved {
				for k, w := range want {
					if entries[i][k] != w {
						t.Fatalf("resolved[%d][%q]=%v want %v", i, k, entries[i][k], w)
					}
				}
			}
			if tc.wantResStatus == "skipped" {
				if reason, _ := res["reason"].(string); !strings.Contains(reason, "HTTP 403") {
					t.Fatalf("skipped reason=%#v", res["reason"])
				}
			}
		})
	}
}

// jsonEqual compares two values via their JSON encoding (float64 vs int in []any).
func jsonEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(ab) == string(bb)
}

// The fields GET is shared: one GET resolves values and prechecks requirements.
func TestWorkitemCreateOptionResolutionOneGET(t *testing.T) {
	s := newWiCreateServer(t, wiFieldsFixture)
	r := runWiCreate(t, false, false, "--custom-fields", `{"priority":"高","mod-1":"MES"}`)
	if r.code != 0 || s.fieldsGETs != 1 || len(s.posts) != 1 {
		t.Fatalf("code=%d fieldsGETs=%d posts=%d stderr=%s", r.code, s.fieldsGETs, len(s.posts), r.stderr)
	}
	pc, _ := wiEnvelope(t, r.stdout).Meta["precheck"].(map[string]any)
	if pc["status"] != "ok" {
		t.Fatalf("meta.precheck=%#v", pc)
	}
}

// Values that already are option ids get status ok without a resolved list; creates
// without customFieldValues get no option_resolution meta at all.
func TestWorkitemCreateOptionResolutionOmissions(t *testing.T) {
	s := newWiCreateServer(t, wiFieldsFixture)
	r := runWiCreate(t, true, false, "--custom-fields", `{"priority":"prio-high","mod-1":"m-a"}`)
	if r.code != 0 {
		t.Fatalf("code=%d stderr=%s", r.code, r.stderr)
	}
	res := wiResolveResolution(t, wiDryRunRequest(t, r.stdout))
	if res["status"] != "ok" {
		t.Fatalf("option_resolution=%#v", res)
	}
	if _, ok := res["resolved"]; ok {
		t.Fatalf("resolved should be omitted when nothing needed mapping: %#v", res)
	}
	if s.fieldsGETs != 1 || len(s.posts) != 0 {
		t.Fatalf("fieldsGETs=%d posts=%d (dry-run must not POST)", s.fieldsGETs, len(s.posts))
	}

	// No custom fields → no option_resolution key (a type with only root required fields).
	const noCFFixture = `[
	 {"id":"subject","name":"标题","type":"NativeField","format":"string","required":true,"showWhenCreate":true},
	 {"id":"assignedTo","name":"负责人","type":"NativeField","format":"user","required":true,"showWhenCreate":true}
	]`
	s2 := newWiCreateServer(t, noCFFixture)
	r2 := runWiCreate(t, false, false)
	if r2.code != 0 {
		t.Fatalf("code=%d stderr=%s", r2.code, r2.stderr)
	}
	if _, ok := wiEnvelope(t, r2.stdout).Meta["option_resolution"]; ok {
		t.Fatalf("no option_resolution expected without customFieldValues")
	}
	if s2.fieldsGETs != 1 || len(s2.posts) != 1 {
		t.Fatalf("fieldsGETs=%d posts=%d", s2.fieldsGETs, len(s2.posts))
	}
}
