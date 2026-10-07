package zhiyi

import (
	"reflect"
	"testing"
)

func TestStatusDisplayNameByID(t *testing.T) {
	statuses := map[string]string{
		"待测试":  "st-test",
		"testing": "st-test",
		"已完成":   "st-done",
	}
	cases := []struct {
		id   string
		want string
	}{
		{"st-test", "testing"}, // deterministic: alphabetically first alias
		{"st-done", "已完成"},
		{"st-unknown", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := StatusDisplayNameByID(statuses, tc.id); got != tc.want {
			t.Fatalf("StatusDisplayNameByID(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
	// self-keyed entry wins
	self := map[string]string{"141230": "141230", "已取消": "141230"}
	if got := StatusDisplayNameByID(self, "141230"); got != "141230" {
		t.Fatalf("self-key = %q", got)
	}
	if got := StatusDisplayNameByID(nil, "x"); got != "" {
		t.Fatalf("nil map = %q", got)
	}
}

func TestTransitionStatusBrief(t *testing.T) {
	statuses := map[string]string{"待测试": "st-test", "已完成": "st-done"}

	// refreshed status matches target → its object wins (displayName from the item)
	refreshed := map[string]any{"status": map[string]any{"id": "st-test", "displayName": "待测试", "name": "待测试"}}
	got := TransitionStatusBrief(refreshed, true, "st-test", statuses)
	if !reflect.DeepEqual(got, map[string]any{"id": "st-test", "displayName": "待测试"}) {
		t.Fatalf("refreshed = %v", got)
	}

	// refresh failed → reverse-resolve displayName from the alias→id map
	got = TransitionStatusBrief(nil, false, "st-test", statuses)
	if !reflect.DeepEqual(got, map[string]any{"id": "st-test", "displayName": "待测试"}) {
		t.Fatalf("fallback = %v", got)
	}

	// refreshed status does not match target (refresh raced) → fallback map
	stale := map[string]any{"status": map[string]any{"id": "st-pending", "displayName": "待处理"}}
	got = TransitionStatusBrief(stale, true, "st-done", statuses)
	if !reflect.DeepEqual(got, map[string]any{"id": "st-done", "displayName": "已完成"}) {
		t.Fatalf("stale refresh = %v", got)
	}

	// unknown id, no alias → id only
	got = TransitionStatusBrief(nil, false, "st-mystery", statuses)
	if !reflect.DeepEqual(got, map[string]any{"id": "st-mystery"}) {
		t.Fatalf("unknown = %v", got)
	}

	if got := TransitionStatusBrief(refreshed, true, "", statuses); got != nil {
		t.Fatalf("empty target = %v", got)
	}
}
