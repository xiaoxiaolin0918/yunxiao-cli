package zhiyi

import "testing"

func TestNormalizeWorkitemCommentListShape(t *testing.T) {
	c := NormalizeWorkitemComment(map[string]any{
		"id":          "23638657",
		"content":     "hi",
		"gmtModified": int64(1792051197000),
		"user":        map[string]any{"id": "u1", "name": "肖晓霖的AI"},
	})
	user, _ := c["user"].(map[string]any)
	if user["displayName"] != "肖晓霖的AI" || user["name"] != "肖晓霖的AI" {
		t.Fatalf("user=%v", user)
	}
	creator, _ := c["creator"].(map[string]any)
	if creator["name"] != "肖晓霖的AI" || creator["displayName"] != "肖晓霖的AI" {
		t.Fatalf("creator=%v", creator)
	}
	if c["modifiedTime"] != int64(1792051197000) {
		t.Fatalf("modifiedTime=%v", c["modifiedTime"])
	}
}

func TestNormalizeWorkitemCommentUpdateShape(t *testing.T) {
	c := NormalizeWorkitemComment(map[string]any{
		"id":           "1",
		"content":      "x",
		"modifiedTime": int64(42),
		"user": map[string]any{
			"identifier":  "u1",
			"displayName": "肖晓霖的AI",
		},
	})
	user, _ := c["user"].(map[string]any)
	if user["name"] != "肖晓霖的AI" || user["displayName"] != "肖晓霖的AI" {
		t.Fatalf("user=%v", user)
	}
	if user["id"] != "u1" {
		t.Fatalf("id alias=%v", user["id"])
	}
	if c["gmtModified"] != int64(42) {
		t.Fatalf("gmtModified=%v", c["gmtModified"])
	}
}

func TestNormalizeWorkitemCommentsSlice(t *testing.T) {
	out := NormalizeWorkitemComments([]any{
		map[string]any{"id": "1", "user": map[string]any{"name": "A"}},
		map[string]any{"id": "2", "creator": map[string]any{"name": "B"}},
	})
	s, ok := out.([]any)
	if !ok || len(s) != 2 {
		t.Fatalf("%T %#v", out, out)
	}
	u0 := s[0].(map[string]any)["user"].(map[string]any)
	if u0["displayName"] != "A" {
		t.Fatalf("%v", u0)
	}
	u1 := s[1].(map[string]any)["user"].(map[string]any)
	if u1["displayName"] != "B" {
		t.Fatalf("%v", u1)
	}
}

func TestNormalizeWorkitemCommentNilCreatorFilled(t *testing.T) {
	c := NormalizeWorkitemComment(map[string]any{
		"id":      "1",
		"creator": nil,
		"user":    map[string]any{"id": "u", "name": "N"},
	})
	cr, _ := c["creator"].(map[string]any)
	if cr["name"] != "N" {
		t.Fatalf("creator=%v", cr)
	}
}
