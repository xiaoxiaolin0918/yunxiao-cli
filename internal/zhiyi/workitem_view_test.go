package zhiyi

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const viewFixture = `{
 "id":"wi-1","serialNumber":"ZYPT-5916","subject":"产品类需求",
 "status":{"id":"100005","name":"处理中","displayName":"处理中","nameEn":"DOING"},
 "assignedTo":{"id":"u-1","name":"肖晓霖"},
 "sprint":{"id":"sp-1","name":"S39"},
 "gmtModified":"2026-09-29T10:00:00Z",
 "description":"需求说明需求说明",
 "formatType":"MARKDOWN",
 "space":{"id":"space-1","name":"智衣平台"},
 "customFieldValues":[
  {"fieldId":"module","fieldName":"所属模块","values":[{"identifier":"m-1","displayValue":"订单"}]},
  {"fieldId":"priority","fieldName":"优先级","values":[{"identifier":"p-high","displayValue":"高"}]}
 ]
}`

func viewItem(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestWorkItemGetBriefKeepsKeyFieldsAndSummarizesDescription(t *testing.T) {
	got := WorkItemGetBrief(viewItem(t, viewFixture))
	want := map[string]any{
		"id": "wi-1", "serialNumber": "ZYPT-5916", "subject": "产品类需求",
		"status":              map[string]any{"id": "100005", "displayName": "处理中"},
		"assignedTo":          map[string]any{"id": "u-1", "name": "肖晓霖"},
		"sprint":              map[string]any{"id": "sp-1", "name": "S39"},
		"priority":            map[string]any{"id": "p-high", "displayValue": "高"},
		"gmtModified":         "2026-09-29T10:00:00Z",
		"description_summary": "(description: 8 chars, use --full or --fields description)",
	}
	if !reflect.DeepEqual(got, want) {
		gb, _ := json.Marshal(got)
		t.Fatalf("brief=%s", gb)
	}
}

func TestWorkItemGetBriefOmitsAbsentFields(t *testing.T) {
	got := WorkItemGetBrief(viewItem(t, `{"id":"wi-2","subject":"s","description":"","sprint":null,"customFieldValues":[{"fieldId":"priority","values":[]}]}`))
	want := map[string]any{"id": "wi-2", "subject": "s"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("brief=%#v", got)
	}
}

func TestWorkItemGetBriefTopLevelPriorityWins(t *testing.T) {
	got := WorkItemGetBrief(viewItem(t, `{"id":"wi-3","priority":{"id":"p1","name":"紧急"},"customFieldValues":[{"fieldId":"priority","values":[{"identifier":"x","displayValue":"低"}]}]}`))
	if !reflect.DeepEqual(got["priority"], map[string]any{"id": "p1", "name": "紧急"}) {
		t.Fatalf("priority=%#v", got["priority"])
	}
}

func TestParseWorkItemFieldList(t *testing.T) {
	got, err := ParseWorkItemFieldList(" subject , status,subject,customFieldValues ")
	if err != nil || !reflect.DeepEqual(got, []string{"subject", "status", "customFieldValues"}) {
		t.Fatalf("got %v err=%v", got, err)
	}
	for in, wantErr := range map[string]string{
		"":             "at least one",
		" , ":          "empty field name",
		"subject,,id":  "empty field name",
		"status.id":    "--jq",
		"sub ject":     "invalid field name",
	} {
		if _, err := ParseWorkItemFieldList(in); err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("%q: err=%v, want %q", in, err, wantErr)
		}
	}
}

func TestProjectWorkItemFields(t *testing.T) {
	item := viewItem(t, viewFixture)
	got, unknown := ProjectWorkItem(item, []string{"subject", "description", "priority", "customFieldValues"})
	if len(unknown) != 0 {
		t.Fatalf("unknown=%v", unknown)
	}
	if got["description"] != "需求说明需求说明" || got["subject"] != "产品类需求" || len(got) != 4 {
		t.Fatalf("projection=%#v", got)
	}
	if !reflect.DeepEqual(got["priority"], map[string]any{"id": "p-high", "displayValue": "高"}) {
		t.Fatalf("derived priority=%#v", got["priority"])
	}
	if !reflect.DeepEqual(got["customFieldValues"], item["customFieldValues"]) {
		t.Fatal("raw values must be passed through unchanged")
	}
}

// A key present with null counts as known (not an unknown field).
func TestProjectWorkItemUnknownAndNull(t *testing.T) {
	item := viewItem(t, `{"id":"wi-4","sprint":null}`)
	got, unknown := ProjectWorkItem(item, []string{"sprint", "nope", "ID", "priority"})
	if !reflect.DeepEqual(unknown, []string{"nope", "ID", "priority"}) {
		t.Fatalf("unknown=%v", unknown)
	}
	if v, ok := got["sprint"]; !ok || v != nil {
		t.Fatalf("null field must be kept as null: %#v", got)
	}
}

func TestWorkItemFieldSuggestions(t *testing.T) {
	item := viewItem(t, viewFixture)
	avail := WorkItemAvailableFields(item)
	if !contains(avail, "serialNumber") || !contains(avail, "priority") || !contains(avail, "description") {
		t.Fatalf("available=%v", avail)
	}
	sug := WorkItemFieldSuggestions([]string{"SerialNumber", "nope"}, avail)
	if !reflect.DeepEqual(sug, map[string]string{"SerialNumber": "serialNumber"}) {
		t.Fatalf("suggestions=%v", sug)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}