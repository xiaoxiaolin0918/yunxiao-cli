package orguid

import "testing"

func TestMembersFromAPI_Array(t *testing.T) {
	in := []any{map[string]any{"userId": "a"}}
	got := MembersFromAPI(in)
	if len(got) != 1 {
		t.Fatalf("%v", got)
	}
}

func TestMembersFromAPI_Wrapped(t *testing.T) {
	in := map[string]any{"data": []any{map[string]any{"userId": "a"}}}
	got := MembersFromAPI(in)
	if len(got) != 1 {
		t.Fatal(got)
	}
}

func TestReplaceMembersInAPI_PreservesDataKey(t *testing.T) {
	in := map[string]any{
		"data":  []any{map[string]any{"userId": "a"}},
		"total": 1,
	}
	out := ReplaceMembersInAPI(in, []any{map[string]any{"userId": "a", "aliyunUid": "9"}})
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("%T", out)
	}
	if _, ok := m["data"].([]any); !ok {
		t.Fatalf("data missing: %#v", m)
	}
	if m["total"] != 1 {
		t.Fatalf("total=%v", m["total"])
	}
	items := m["data"].([]any)
	row := items[0].(map[string]any)
	if row["aliyunUid"] != "9" {
		t.Fatalf("%v", row)
	}
}

func TestReplaceMembersInAPI_BareSlice(t *testing.T) {
	in := []any{map[string]any{"userId": "a"}}
	out := ReplaceMembersInAPI(in, []any{map[string]any{"userId": "a", "aliyunUid": "1"}})
	if _, ok := out.([]any); !ok {
		t.Fatalf("%T", out)
	}
}
