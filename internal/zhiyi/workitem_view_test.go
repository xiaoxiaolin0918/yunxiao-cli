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
 "categoryId":"Req",
 "workitemType":{"id":"type-req","name":"产品类需求","nameEn":"Req"},
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
		"workitemType":        map[string]any{"id": "type-req", "name": "产品类需求"},
		"categoryId":          "Req",
		"gmtModified":         "2026-09-29T10:00:00Z",
		"description_summary": "(description: 8 chars, use --full or --fields description)",
	}
	if !reflect.DeepEqual(got, want) {
		gb, _ := json.Marshal(got)
		t.Fatalf("brief=%s", gb)
	}
	for k := range got {
		if !contains(WorkItemGetBriefFields, k) {
			t.Fatalf("WorkItemGetBriefFields misses %q", k)
		}
	}
}

func TestWorkItemGetBriefOmitsAbsentFields(t *testing.T) {
	got := WorkItemGetBrief(viewItem(t, `{"id":"wi-2","subject":"s","description":"","sprint":null,"categoryId":"",
		"priority":"","workitemType":{"id":"","name":null},"customFieldValues":[{"fieldId":"priority","values":[]}]}`))
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

func TestWorkItemPriorityDerivation(t *testing.T) {
	low := map[string]any{"id": "p-low", "displayValue": "低"}
	cases := []struct {
		name string
		item string
		want any // nil = not derivable
	}{
		{"fieldId priority", `{"customFieldValues":[{"fieldId":"priority","values":[{"identifier":"p-low","displayValue":"低"}]}]}`, low},
		{"hash fieldId, fieldName 优先级", `{"customFieldValues":[{"fieldId":"8f3a0c2e9b","fieldName":"优先级","values":[{"identifier":"p-low","displayValue":"低"}]}]}`, low},
		{"hash fieldId, fieldName Priority", `{"customFieldValues":[{"fieldId":"8f3a0c2e9b","fieldName":"Priority","values":[{"identifier":"p-low","displayValue":"低"}]}]}`, low},
		{"fieldName case-insensitive", `{"customFieldValues":[{"fieldId":"x","fieldName":" priority ","values":[{"identifier":"p-low","displayValue":"低"}]}]}`, low},
		{"duplicate entries: empty ones skipped", `{"customFieldValues":[
			{"fieldId":"priority","values":[]},
			{"fieldId":"8f3a","fieldName":"优先级","values":[{"identifier":"","displayValue":""},null]},
			{"fieldId":"9b2c","fieldName":"优先级","values":[{"identifier":"p-low","displayValue":"低"}]}]}`, low},
		{"value uses id instead of identifier", `{"customFieldValues":[{"fieldId":"priority","values":[{"id":"p-low","displayValue":"低"}]}]}`, low},
		{"no id keys: displayValue only, no id:null", `{"customFieldValues":[{"fieldId":"priority","values":[{"displayValue":"低"}]}]}`, map[string]any{"displayValue": "低"}},
		{"root null falls back", `{"priority":null,"customFieldValues":[{"fieldId":"priority","values":[{"identifier":"p-low","displayValue":"低"}]}]}`, low},
		{"root empty string falls back", `{"priority":"","customFieldValues":[{"fieldId":"priority","values":[{"identifier":"p-low","displayValue":"低"}]}]}`, low},
		{"root empty string, nothing else", `{"priority":""}`, nil},
		{"other fields only", `{"customFieldValues":[{"fieldId":"module","fieldName":"所属模块","values":[{"identifier":"m","displayValue":"订单"}]}]}`, nil},
		{"all priority entries empty", `{"customFieldValues":[{"fieldId":"priority","values":[{"identifier":null}]},{"fieldName":"优先级"}]}`, nil},
	}
	for _, tc := range cases {
		got, ok := WorkItemPriority(viewItem(t, tc.item))
		if tc.want == nil {
			if ok || got != nil {
				t.Fatalf("%s: got %#v, want not derivable", tc.name, got)
			}
			continue
		}
		if !ok || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: got %#v ok=%v, want %#v", tc.name, got, ok, tc.want)
		}
	}
}

func TestWorkItemDescriptionSummary(t *testing.T) {
	cases := []struct {
		name, item, want string // want "" = omitted
	}{
		{"markdown counts raw runes", `{"formatType":"MARKDOWN","description":"a<b 和 c>d"}`, "(description: 9 chars, use --full or --fields description)"},
		{"richtext strips tags and entities", `{"formatType":"RICHTEXT","description":"<p>需求<b>说明</b>&amp;</p><br/>"}`, "(description: 5 chars of text excluding HTML tags, use --full or --fields description)"},
		{"no formatType, HTML", `{"description":"<div>abc</div>"}`, "(description: 3 chars of text excluding HTML tags, use --full or --fields description)"},
		{"richtext without tags counts raw", `{"formatType":"RICHTEXT","description":"纯文本"}`, "(description: 3 chars, use --full or --fields description)"},
		{"object", `{"description":{"html":"<p>x</p>"}}`, "(description: non-string object value, use --full or --fields description)"},
		{"number", `{"description":42}`, "(description: non-string number value, use --full or --fields description)"},
		{"array", `{"description":["a"]}`, "(description: non-string array value, use --full or --fields description)"},
		{"null", `{"description":null}`, ""},
		{"empty", `{"description":""}`, ""},
		{"missing", `{}`, ""},
	}
	for _, tc := range cases {
		got, ok := WorkItemDescriptionSummary(viewItem(t, tc.item))
		if tc.want == "" {
			if ok {
				t.Fatalf("%s: got %q, want omitted", tc.name, got)
			}
			continue
		}
		if !ok || got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestParseWorkItemFieldList(t *testing.T) {
	got, err := ParseWorkItemFieldList(" subject , status,subject,customFieldValues ")
	if err != nil || !reflect.DeepEqual(got, []string{"subject", "status", "customFieldValues"}) {
		t.Fatalf("got %v err=%v", got, err)
	}
	for in, wantErr := range map[string]string{
		"":            "at least one",
		" , ":         "empty field name",
		"subject,,id": "empty field name",
		"status.id":   "--jq",
		"sub ject":    "invalid field name",
	} {
		if _, err := ParseWorkItemFieldList(in); err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("%q: err=%v, want %q", in, err, wantErr)
		}
	}
}

func TestProjectWorkItemFields(t *testing.T) {
	item := viewItem(t, viewFixture)
	res := ProjectWorkItem(item, []string{"subject", "description", "priority", "customFieldValues"})
	got := res.Data
	if len(res.Unknown) != 0 || len(res.Absent) != 0 || res.PriorityUnresolved {
		t.Fatalf("res=%+v", res)
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

// Present-with-null stays null; known-but-absent is null (Absent); priority that
// can't be derived is null (PriorityUnresolved); only truly unknown names are Unknown.
func TestProjectWorkItemNullAbsentUnknown(t *testing.T) {
	item := viewItem(t, `{"id":"wi-4","sprint":null,"priority":null}`)
	res := ProjectWorkItem(item, []string{"sprint", "nope", "verifier", "ID", "priority", "workitemType"})
	if !reflect.DeepEqual(res.Unknown, []string{"nope", "ID"}) {
		t.Fatalf("unknown=%v", res.Unknown)
	}
	if !reflect.DeepEqual(res.Absent, []string{"verifier", "workitemType"}) || !res.PriorityUnresolved {
		t.Fatalf("absent=%v priorityUnresolved=%v", res.Absent, res.PriorityUnresolved)
	}
	for _, k := range []string{"sprint", "verifier", "priority", "workitemType"} {
		if v, ok := res.Data[k]; !ok || v != nil {
			t.Fatalf("%s must be null: %#v", k, res.Data)
		}
	}
	if len(res.Data) != 4 {
		t.Fatalf("data=%#v", res.Data)
	}
}

// Brief and --fields agree on priority, including a null / "" root priority.
func TestWorkItemPrioritySameInBriefAndFields(t *testing.T) {
	for _, s := range []string{
		`{"priority":null,"customFieldValues":[{"fieldId":"h1","fieldName":"优先级","values":[{"id":"p","displayValue":"中"}]}]}`,
		`{"priority":"","customFieldValues":[{"fieldId":"h1","fieldName":"优先级","values":[{"id":"p","displayValue":"中"}]}]}`,
		`{"priority":{"id":"p1"}}`,
	} {
		item := viewItem(t, s)
		brief := WorkItemGetBrief(item)["priority"]
		fields := ProjectWorkItem(item, []string{"priority"}).Data["priority"]
		if brief == nil || !reflect.DeepEqual(brief, fields) {
			t.Fatalf("%s: brief=%#v fields=%#v", s, brief, fields)
		}
	}
}

func TestWorkItemFieldSuggestions(t *testing.T) {
	item := viewItem(t, viewFixture)
	avail := WorkItemAvailableFields(item)
	for _, k := range []string{"serialNumber", "priority", "description", "verifier", "labels"} {
		if !contains(avail, k) {
			t.Fatalf("available misses %s: %v", k, avail)
		}
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
