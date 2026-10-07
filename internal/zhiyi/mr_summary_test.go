package zhiyi

import (
	"encoding/json"
	"strings"
	"testing"
)

// mrSummaryFixture is a realistic GetChangeRequest-shaped payload (documented
// fields + the mergeable/checkList extras the issue author observed).
func mrSummaryFixture() map[string]any {
	return map[string]any{
		"localId":                        float64(9),
		"title":                          "feat: x",
		"status":                         "TO_BE_MERGED",
		"mergeable":                      true,
		"conflictCheckStatus":            "NO_CONFLICT",
		"supportMergeFastForwardOnly":    true,
		"allRequirementsPass":            true,
		"targetProjectPathWithNamespace": "org/repo",
		"reviewers": []any{
			map[string]any{"name": "alice", "reviewOpinionStatus": "PASS", "state": "active"},
			map[string]any{"name": "bob", "hasReviewed": false},
			map[string]any{"email": "nobody@example.com"}, // no name/opinion -> dropped
		},
		"checkList": map[string]any{
			"requirementRuleItems": []any{
				map[string]any{"name": "code review", "pass": true},
			},
		},
		"description": "very long description …",
		"subscribers": []any{map[string]any{"name": "carol"}},
	}
}

func TestSummaryMergeRequest(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(mr map[string]any)
		wantHas []string // keys that must be present with these values (JSON-encoded)
		wantNot []string // keys that must be absent
	}{
		{
			name: "full fixture keeps issue field set",
			wantHas: []string{
				`"mergeable":true`,
				`"conflictCheckStatus":"NO_CONFLICT"`,
				`"checkList":{"requirementRuleItems":[`,
				`"supportMergeFastForwardOnly":true`,
				`"allRequirementsPass":true`,
			},
			wantNot: []string{"description", "subscribers"},
		},
		{
			name:    "missing merge fields are omitted",
			mutate:  func(mr map[string]any) { delete(mr, "mergeable"); delete(mr, "checkList"); delete(mr, "reviewers") },
			wantNot: []string{"mergeable", "checkList", "reviewers"},
		},
		{
			name: "snake_case aliases tolerated",
			mutate: func(mr map[string]any) {
				mr["conflict_check_status"] = "HAS_CONFLICT"
				delete(mr, "conflictCheckStatus")
			},
			wantHas: []string{`"conflictCheckStatus":"HAS_CONFLICT"`},
		},
		{
			name: "reviewer without opinion kept with name only",
			mutate: func(mr map[string]any) {
				mr["reviewers"] = []any{map[string]any{"name": "bob"}}
			},
			wantHas: []string{`"reviewers":[{"name":"bob"}]`},
		},
		{
			name: "nil mergeable treated as absent",
			mutate: func(mr map[string]any) {
				mr["mergeable"] = nil
			},
			wantNot: []string{`"mergeable"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mr := mrSummaryFixture()
			if tc.mutate != nil {
				tc.mutate(mr)
			}
			got := SummaryMergeRequest(mr)
			b, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			s := string(b)
			for _, want := range tc.wantHas {
				if !strings.Contains(s, want) {
					t.Fatalf("summary missing %s: %s", want, s)
				}
			}
			for _, no := range tc.wantNot {
				if strings.Contains(s, no) {
					t.Fatalf("summary must not contain %s: %s", no, s)
				}
			}
			// Brief base survives in every case.
			if got["localId"] != "9" || got["status"] != "TO_BE_MERGED" || got["state"] != "TO_BE_MERGED" {
				t.Fatalf("brief base lost: %#v", got)
			}
			if u, _ := got["url"].(string); u == "" || !strings.Contains(u, "/change/9") {
				t.Fatalf("url not constructed: %#v", got["url"])
			}
		})
	}
}

func TestReviewersSummary(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string // JSON of reviewers summary or "" for nil
	}{
		{name: "nil", in: nil, want: ""},
		{name: "opinion alias", in: []any{map[string]any{"name": "a", "opinion": "PASS"}}, want: `[{"name":"a","opinion":"PASS"}]`},
		{name: "review_opinion alias", in: []any{map[string]any{"name": "a", "review_opinion": "NOT_PASS"}}, want: `[{"name":"a","opinion":"NOT_PASS"}]`},
		{name: "displayName fallback", in: []any{map[string]any{"displayName": "d", "reviewOpinionStatus": "PASS"}}, want: `[{"name":"d","opinion":"PASS"}]`},
		{name: "opinion only kept", in: []any{map[string]any{"reviewOpinion": "PASS"}}, want: `[{"opinion":"PASS"}]`},
		{name: "garbage entries dropped", in: []any{"str", 42, map[string]any{}}, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ReviewersSummary(map[string]any{"reviewers": tc.in})
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("want empty, got %#v", got)
				}
				return
			}
			b, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tc.want {
				t.Fatalf("got %s want %s", string(b), tc.want)
			}
		})
	}
}
