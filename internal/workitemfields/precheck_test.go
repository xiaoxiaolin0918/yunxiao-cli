package workitemfields

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func decode(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// Shape from GetWorkitemTypeFieldConfig (GET .../workitemTypes/{id}/fields).
const config = `[
 {"id":"subject","name":"标题","type":"NativeField","format":"string","required":true,"showWhenCreate":true},
 {"id":"assignedTo","name":"负责人","type":"NativeField","format":"user","required":true,"showWhenCreate":true},
 {"id":"status","name":"状态","type":"NativeField","format":"list","required":true,"showWhenCreate":true},
 {"id":"creator","name":"创建人","type":"NativeField","format":"user","required":true},
 {"id":"sprint","name":"迭代","type":"NativeField","format":"sprint","required":true,"showWhenCreate":true},
 {"id":"priority","name":"优先级","type":"SystemCustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"prio-high","value":"高","displayValue":"高"},{"id":"prio-low","value":"低","displayValue":"低"}]},
 {"id":"mod-1","name":"所属模块","type":"CustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"m-a","value":"MES","displayValue":"MES"}]},
 {"id":"note","name":"备注","type":"CustomField","format":"text","required":false,"showWhenCreate":true},
 {"id":"hidden-req","name":"内部","type":"CustomField","format":"string","required":true,"showWhenCreate":false},
 {"id":"src","name":"来源","type":"CustomField","format":"list","required":true,"showWhenCreate":true,"defaultValue":"src-1"}
]`

func parse(t *testing.T, s string) []Field {
	t.Helper()
	fields, err := Parse(decode(t, s))
	if err != nil {
		t.Fatal(err)
	}
	return fields
}

func ids(ms []Missing) string {
	var out []string
	for _, m := range ms {
		out = append(out, m.FieldID)
	}
	return strings.Join(out, ",")
}

func TestParseFieldConfig(t *testing.T) {
	fields := parse(t, config)
	if len(fields) != 10 {
		t.Fatalf("got %d fields", len(fields))
	}
	p := fields[5]
	if p.ID != "priority" || p.Name != "优先级" || !p.Required || p.Format != "list" || p.Type != "SystemCustomField" || len(p.Options) != 2 || p.Options[0].ID != "prio-high" || p.Options[0].DisplayValue != "高" {
		t.Fatalf("priority=%+v", p)
	}
	if fields[8].ShowWhenCreate == nil || *fields[8].ShowWhenCreate {
		t.Fatalf("hidden-req showWhenCreate=%v", fields[8].ShowWhenCreate)
	}
	if fields[3].ShowWhenCreate != nil {
		t.Fatalf("creator showWhenCreate should be unknown (nil)")
	}
}

func TestParseWrappedAndLegacyKeys(t *testing.T) {
	// {result:[...]} wrapper; ListWorkItemAllFields-style identifier / isRequired / isShowWhenCreate.
	fields := parse(t, `{"result":[{"identifier":"mod-1","name":"所属模块","isRequired":true,"isShowWhenCreate":true}]}`)
	if len(fields) != 1 || fields[0].ID != "mod-1" || !fields[0].Required || fields[0].ShowWhenCreate == nil || !*fields[0].ShowWhenCreate {
		t.Fatalf("fields=%+v", fields)
	}
}

func TestParseRejectsUnexpectedPayload(t *testing.T) {
	for _, s := range []string{`{"foo":1}`, `"x"`, `null`} {
		if _, err := Parse(decode(t, s)); err == nil {
			t.Fatalf("%s: expected error", s)
		}
	}
}

// #95 acceptance: missing 所属模块 + 优先级 are reported together, in config order, with options.
func TestMissingRequiredReportsAllAtOnce(t *testing.T) {
	body := map[string]any{"subject": "需求", "assignedTo": "u1", "sprint": "sp-1"}
	missing, checked := MissingRequired(parse(t, config), body)
	if ids(missing) != "priority,mod-1" {
		t.Fatalf("missing=%s", ids(missing))
	}
	if checked != 5 { // subject, assignedTo, sprint, priority, mod-1
		t.Fatalf("checked=%d", checked)
	}
	p := missing[0]
	if p.Name != "优先级" || p.PassVia != "customFieldValues" || p.OptionsTotal != 2 || len(p.Options) != 2 || p.Options[0].ID != "prio-high" || p.Options[0].DisplayValue != "高" {
		t.Fatalf("priority missing=%+v", p)
	}
}

func TestMissingRequiredNoFalsePositiveWhenComplete(t *testing.T) {
	body := map[string]any{
		"subject": "需求", "assignedTo": "u1", "sprint": "sp-1",
		"customFieldValues": map[string]any{"priority": "prio-high", "mod-1": "m-a"},
	}
	if missing, _ := MissingRequired(parse(t, config), body); len(missing) != 0 {
		t.Fatalf("unexpected missing: %+v", missing)
	}
}

// Skipped: not required, showWhenCreate=false, server-managed (status/creator), server default.
func TestMissingRequiredSkipsNonUserFields(t *testing.T) {
	body := map[string]any{"subject": "s", "assignedTo": "u", "sprint": "sp",
		"customFieldValues": map[string]any{"priority": "p", "mod-1": "m"}}
	missing, _ := MissingRequired(parse(t, config), body)
	for _, m := range missing {
		switch m.FieldID {
		case "note", "hidden-req", "status", "creator", "src":
			t.Fatalf("%s must be skipped", m.FieldID)
		}
	}
}

// Root (flag) fields: a required sprint is satisfied by body.sprint; missing one names the flag.
func TestMissingRequiredRootFieldsNameTheFlag(t *testing.T) {
	body := map[string]any{"subject": "s", "assignedTo": "u",
		"customFieldValues": map[string]any{"priority": "p", "mod-1": "m"}}
	missing, _ := MissingRequired(parse(t, config), body)
	if ids(missing) != "sprint" || missing[0].PassVia != "--sprint" {
		t.Fatalf("missing=%+v", missing)
	}
	// labels / description map to their flags.
	cfg := `[{"id":"tag","name":"标签","required":true},{"id":"description","name":"描述","required":true}]`
	missing, _ = MissingRequired(parse(t, cfg), map[string]any{})
	if ids(missing) != "tag,description" || missing[0].PassVia != "--labels" || missing[1].PassVia != "--description / --description-file" {
		t.Fatalf("missing=%+v", missing)
	}
	missing, _ = MissingRequired(parse(t, cfg), map[string]any{"labels": []string{"l1"}, "description": "d"})
	if len(missing) != 0 {
		t.Fatalf("labels/description provided: %+v", missing)
	}
}

// Blank strings / empty lists / nil count as missing; numbers and bools count as provided.
func TestMissingRequiredValuePresence(t *testing.T) {
	cfg := `[{"id":"a","name":"A","required":true},{"id":"b","name":"B","required":true},{"id":"c","name":"C","required":true},
	 {"id":"d","name":"D","required":true},{"id":"e","name":"E","required":true},{"id":"f","name":"F","required":true}]`
	body := map[string]any{"customFieldValues": map[string]any{"a": "  ", "b": []any{}, "c": nil, "d": float64(0), "e": false, "f": []any{"x"}}}
	missing, _ := MissingRequired(parse(t, cfg), body)
	if ids(missing) != "a,b,c" {
		t.Fatalf("missing=%s", ids(missing))
	}
}

// Large option lists are capped; options_total keeps the real count.
func TestMissingRequiredCapsOptions(t *testing.T) {
	var opts []string
	for i := 0; i < 30; i++ {
		opts = append(opts, `{"id":"o`+strconv.Itoa(i)+`","displayValue":"v"}`)
	}
	cfg := `[{"id":"big","name":"大","required":true,"options":[` + strings.Join(opts, ",") + `]}]`
	missing, _ := MissingRequired(parse(t, cfg), map[string]any{})
	if len(missing) != 1 || len(missing[0].Options) != MaxOptions || missing[0].OptionsTotal != 30 {
		t.Fatalf("missing=%+v", missing)
	}
}

// Root-level fields count only when set on the body root (as the CLI flag sends them),
// not when smuggled into customFieldValues.
func TestMissingRequiredRootFieldOnlyAtRoot(t *testing.T) {
	cfg := `[{"id":"sprint","name":"迭代","required":true},{"id":"mod-1","name":"所属模块","required":true}]`
	missing, _ := MissingRequired(parse(t, cfg), map[string]any{"customFieldValues": map[string]any{"sprint": "sp-1", "mod-1": "m"}})
	if ids(missing) != "sprint" || missing[0].PassVia != "--sprint" {
		t.Fatalf("missing=%+v", missing)
	}
	if missing, _ := MissingRequired(parse(t, cfg), map[string]any{"sprint": "sp-1", "customFieldValues": map[string]any{"mod-1": "m"}}); len(missing) != 0 {
		t.Fatalf("root sprint must count: %+v", missing)
	}
}

// Custom fields count only inside customFieldValues (a root key is not sent as a field value).
func TestMissingRequiredCustomFieldOnlyInCustomFieldValues(t *testing.T) {
	cfg := `[{"id":"mod-1","name":"所属模块","required":true}]`
	if missing, _ := MissingRequired(parse(t, cfg), map[string]any{"mod-1": "m"}); ids(missing) != "mod-1" {
		t.Fatalf("missing=%+v", missing)
	}
}

// DefaultSkipped lists the required, create-visible, user fields skipped for a server defaultValue.
func TestDefaultSkipped(t *testing.T) {
	cfg := `[{"id":"src","name":"来源","required":true,"defaultValue":"s1"},
	 {"id":"opt","name":"可选","required":false,"defaultValue":"x"},
	 {"id":"hid","name":"隐藏","required":true,"showWhenCreate":false,"defaultValue":"x"},
	 {"id":"status","name":"状态","required":true,"defaultValue":"100005"},
	 {"id":"prio","name":"优先级","required":true}]`
	if got := DefaultSkipped(parse(t, cfg)); len(got) != 1 || got[0] != "src" {
		t.Fatalf("got %v", got)
	}
}
