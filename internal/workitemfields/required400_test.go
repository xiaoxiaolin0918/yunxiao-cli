package workitemfields

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRequiredFieldMessage(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"errorMessage", `{"errorCode":"InvalidParam","errorMessage":"迭代必填"}`, "迭代必填"},
		{"errorMsg", `{"errorMsg":"【验收说明】必填"}`, "【验收说明】必填"},
		{"message", `{"message":"计划提测时间必填"}`, "计划提测时间必填"},
		{"json without message key", `{"errorCode":"InvalidParam"}`, `{"errorCode":"InvalidParam"}`},
		{"empty message falls back", `{"errorMessage":"  "}`, `{"errorMessage":"  "}`},
		{"non-json raw", "迭代必填;计划提测时间必填", "迭代必填;计划提测时间必填"},
		{"json array passthrough", `["迭代必填"]`, `["迭代必填"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RequiredFieldMessage(tc.body); got != tc.want {
				t.Fatalf("RequiredFieldMessage(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

func TestRequiredFieldNames(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want []string
	}{
		{
			name: "#113 observed list with a bare entry",
			msg:  "迭代必填;计划提测时间必填;前端开发必填;后端开发必填;需求大小必填;测试负责人必填;验收负责人;VP分配必填",
			want: []string{"迭代", "计划提测时间", "前端开发", "后端开发", "需求大小", "测试负责人", "验收负责人", "VP分配"},
		},
		{"bracketed", "【截止日期】必填", []string{"截止日期"}},
		{"chinese comma list", "迭代必填，计划提测时间必填", []string{"迭代", "计划提测时间"}},
		{"prose prefix before name", "状态流转失败：迭代必填", []string{"迭代"}},
		{"ascii field name", "ExpCompletionTime必填", []string{"ExpCompletionTime"}},
		{"dedup", "迭代必填;迭代必填;【迭代】必填", []string{"迭代"}},
		{"no marker", "一切正常", nil},
		{"url junk only", "yunxiao API PUT https://x/y -> HTTP 400: bad request", nil},
		{"url junk plus name", "yunxiao API PUT https://x/y -> HTTP 400: 迭代必填", []string{"迭代"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RequiredFieldNames(tc.msg)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("RequiredFieldNames(%q) = %v, want %v", tc.msg, got, tc.want)
			}
		})
	}
}

func TestRequiredFieldNamesJSONJunkBracketSweep(t *testing.T) {
	// Caller passed a raw JSON body: segments are rejected as junk, but the 【…】
	// sweep over the whole message must not lose the field name.
	got := RequiredFieldNames(`{"errorCode":"InvalidParam","errorMessage":"【验收说明】必填"}`)
	if !reflect.DeepEqual(got, []string{"验收说明"}) {
		t.Fatalf("got %v", got)
	}
}

func required400TestFields() []Field {
	return []Field{
		{ID: "subject", Name: "标题", Type: "NativeField", Format: "text"},
		{ID: "sprint", Name: "迭代", Type: "NativeField", Format: "sprint"},
		{ID: "f80", Name: "计划提测时间", Type: "SystemCustomField", Format: "dateTime"},
		{ID: "f9x", Name: "", DisplayName: "需求大小", Type: "CustomField", Format: "multiList",
			Options: []Option{{ID: "L", DisplayValue: "大"}, {ID: "M", DisplayValue: "中"}, {ID: "S", DisplayValue: "小"}}},
	}
}

func TestMapRequiredFields(t *testing.T) {
	fields := required400TestFields()
	names := []string{"迭代", "计划提测时间", "需求大小", "神秘字段"}
	hits, unmapped := MapRequiredFields(fields, names)

	if len(hits) != 3 {
		t.Fatalf("hits = %+v", hits)
	}
	if !reflect.DeepEqual(unmapped, []string{"神秘字段"}) {
		t.Fatalf("unmapped = %v", unmapped)
	}
	// sprint maps to the named root key and its flag, not the custom-fields channel.
	if hits[0].Field.ID != "sprint" || hits[0].RootKey != "sprint" || hits[0].Via != "--sprint" {
		t.Fatalf("sprint hit = %+v", hits[0])
	}
	// custom fields keep RootKey empty: body key is the fieldId, via --custom-fields.
	if hits[1].Field.ID != "f80" || hits[1].RootKey != "" || hits[1].Via != "--custom-fields" {
		t.Fatalf("f80 hit = %+v", hits[1])
	}
	if hits[1].BodyKey() != "f80" || hits[0].BodyKey() != "sprint" {
		t.Fatalf("BodyKey: %q / %q", hits[0].BodyKey(), hits[1].BodyKey())
	}
	// displayName-only config entries still match.
	if hits[2].Field.ID != "f9x" {
		t.Fatalf("displayName hit = %+v", hits[2])
	}

	// empty match sets
	hits, unmapped = MapRequiredFields(nil, names)
	if len(hits) != 0 || !reflect.DeepEqual(unmapped, names) {
		t.Fatalf("no config: %+v %v", hits, unmapped)
	}
}

func TestMapRequiredFieldsRootFlagTrimmed(t *testing.T) {
	fields := []Field{{ID: "subject", Name: "标题"}, {ID: "assignedTo", Name: "处理人"}}
	hits, _ := MapRequiredFields(fields, []string{"标题", "处理人"})
	if hits[0].Via != "--subject" { // create table says "--subject / --subject-file"
		t.Fatalf("subject via = %q", hits[0].Via)
	}
	if hits[1].Via != "--assigned-to" {
		t.Fatalf("assignedTo via = %q", hits[1].Via)
	}
}

func TestRequiredHitCurrentValue(t *testing.T) {
	item := map[string]any{
		"sprint": map[string]any{"id": "sp-9", "name": "迭代 9"},
		"customFieldValues": []any{
			map[string]any{"fieldId": "f80", "values": []any{"2026-09-20T00:00:00+08:00"}},
			map[string]any{"fieldId": "other"},
		},
	}
	fields := required400TestFields()
	hits, _ := MapRequiredFields(fields, []string{"迭代", "计划提测时间", "需求大小"})

	if got := hits[0].CurrentValue(item); got == nil {
		t.Fatalf("sprint current value = %v", got)
	}
	if got, ok := hits[1].CurrentValue(item).([]any); !ok || len(got) != 1 {
		t.Fatalf("f80 current value = %v", hits[1].CurrentValue(item))
	}
	if got := hits[2].CurrentValue(item); got != nil {
		t.Fatalf("absent custom field should be nil, got %v", got)
	}
	if got := hits[0].CurrentValue(nil); got != nil {
		t.Fatalf("nil item should be nil, got %v", got)
	}
}

func TestRequiredDetailsAndDrafts(t *testing.T) {
	item := map[string]any{}
	fields := required400TestFields()
	hits, unmapped := MapRequiredFields(fields, []string{"迭代", "计划提测时间", "需求大小", "神秘字段"})
	if len(unmapped) != 1 {
		t.Fatalf("unmapped = %v", unmapped)
	}

	details := RequiredDetails(hits, item)
	if len(details) != 3 {
		t.Fatalf("details = %+v", details)
	}
	// JSON tags match the #95 details precedent (snake_case).
	b, err := json.Marshal(details[0])
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"field_id", "name", "pass_via", "current_value", "draft"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("details[0] missing %q: %s", k, b)
		}
	}
	if m["field_id"] != "sprint" || m["pass_via"] != "--sprint" {
		t.Fatalf("sprint detail = %s", b)
	}
	if m["draft"] != "--sprint <sprintId>" {
		t.Fatalf("sprint draft = %v", m["draft"])
	}
	// enum options capped with options_total
	b2, _ := json.Marshal(details[2])
	var m2 map[string]any
	_ = json.Unmarshal(b2, &m2)
	if m2["options_total"] != float64(3) {
		t.Fatalf("options_total = %v (%s)", m2["options_total"], b2)
	}
	opts, _ := m2["options"].([]any)
	if len(opts) != 3 {
		t.Fatalf("options = %v", m2["options"])
	}

	// fields draft: root key for sprint, fieldId otherwise (channel split, #113)
	draft := FieldsDraft(hits)
	if draft["sprint"] != "<sprintId>" {
		t.Fatalf("draft[sprint] = %v", draft)
	}
	if draft["f80"] != "<date>" {
		t.Fatalf("draft[f80] = %v (dateTime placeholder)", draft["f80"])
	}
	if draft["f9x"] != "<option id>" {
		t.Fatalf("draft[f9x] = %v (enum placeholder)", draft["f9x"])
	}
}

func TestParseReadsDisplayName(t *testing.T) {
	raw := []any{
		map[string]any{"id": "f9x", "displayName": "需求大小", "format": "multiList", "required": false},
	}
	fields, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0].DisplayName != "需求大小" {
		t.Fatalf("fields = %+v", fields)
	}
	hits, unmapped := MapRequiredFields(fields, []string{"需求大小"})
	if len(hits) != 1 || len(unmapped) != 0 || hits[0].Field.ID != "f9x" {
		t.Fatalf("hits=%+v unmapped=%v", hits, unmapped)
	}
}
