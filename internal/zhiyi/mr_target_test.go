package zhiyi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeBranchRef(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{" master ", "master"},
		{"refs/heads/master", "master"},
		{"refs/heads/feat/x", "feat/x"},
		{"", ""},
		{"  ", ""},
	}
	for _, tc := range cases {
		if got := NormalizeBranchRef(tc.in); got != tc.want {
			t.Fatalf("NormalizeBranchRef(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestLooksLikeCommitSHA(t *testing.T) {
	if !LooksLikeCommitSHA("8b08d11") {
		t.Fatal("short sha")
	}
	if !LooksLikeCommitSHA("8b08d11abcdef0123456789abcdef012345678") { // 40
		t.Fatal("full sha")
	}
	if LooksLikeCommitSHA("feat/x") || LooksLikeCommitSHA("8b08d1") || LooksLikeCommitSHA("master") {
		t.Fatal("non-sha must be false")
	}
}

func TestMRTargetSelfOrNotMaster(t *testing.T) {
	cases := []struct {
		name, source, target string
		equal, blocked       bool
		reason               string
	}{
		{name: "normal feature to master", source: "feat/x", target: "master"},
		{name: "equal branches", source: "hifox-agent/pao-tui-xiao-di/ZYPT-6522X", target: "hifox-agent/pao-tui-xiao-di/ZYPT-6522X", equal: true, blocked: true, reason: "target_equals_source"},
		{name: "equal after refs prefix", source: "refs/heads/feat/x", target: "feat/x", equal: true, blocked: true, reason: "target_equals_source"},
		{name: "sha source feature target (push-review anomaly #407)", source: "8b08d11c0ffee", target: "hifox-agent/pao-tui-xiao-di/ZYPT-6522X", blocked: true, reason: "source_is_revision_target_not_integration"},
		{name: "sha source targeting master is fine", source: "8b08d11c0ffee", target: "master"},
		{name: "sha source targeting main is fine", source: "8b08d11c0ffee", target: "main"},
		{name: "empty branches", source: "", target: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eq, blocked, reason := ClassifyMRTarget(tc.source, tc.target)
			if eq != tc.equal || blocked != tc.blocked || reason != tc.reason {
				t.Fatalf("ClassifyMRTarget(%q,%q)=(eq=%v blocked=%v reason=%q) want (%v %v %q)",
					tc.source, tc.target, eq, blocked, reason, tc.equal, tc.blocked, tc.reason)
			}
		})
	}
}

func TestBuildMRSummaryViewFlagsSelfTarget(t *testing.T) {
	detail := map[string]any{
		"localId":             float64(194),
		"title":               "auto review",
		"status":              "TO_BE_MERGED",
		"sourceBranch":        "hifox-agent/pao-tui-xiao-di/ZYPT-6522X",
		"targetBranch":        "hifox-agent/pao-tui-xiao-di/ZYPT-6522X",
		"allRequirementsPass": true,
		"detailUrl":           "https://codeup.aliyun.com/org/repo/change/194",
	}
	view := BuildMRSummaryView(nil, detail)
	if view.TargetEqualsSource == nil || !*view.TargetEqualsSource {
		t.Fatalf("target_equals_source: %+v", view)
	}
	if view.CannotMergeToMaster == nil || !*view.CannotMergeToMaster {
		t.Fatalf("cannot_merge_to_master: %+v", view)
	}
	if view.Mergeable == nil || !*view.Mergeable {
		t.Fatalf("mergeable should stay API truth: %+v", view)
	}
	if !strings.Contains(view.NextStep, "不可直接合入 master") {
		t.Fatalf("next_step missing Chinese label: %q", view.NextStep)
	}
	if !strings.Contains(view.NextStep, "mrs update") || !strings.Contains(view.NextStep, "--target-branch master") {
		t.Fatalf("next_step should point at mrs update --target-branch: %q", view.NextStep)
	}
	if !strings.Contains(view.NextStep, "194") {
		t.Fatalf("next_step should include local id: %q", view.NextStep)
	}
	b, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(b)
	for _, k := range []string{`"cannot_merge_to_master":true`, `"target_equals_source":true`, `"next_step"`} {
		if !strings.Contains(raw, k) {
			t.Fatalf("json missing %s: %s", k, raw)
		}
	}
}

func TestBuildMRSummaryViewFlagsShaSourceNotMaster(t *testing.T) {
	// Real push-review anomaly (#407 localId 194): sourceBranch is a commit SHA,
	// targetBranch is the pushed feature branch, not master.
	detail := map[string]any{
		"localId":      float64(194),
		"status":       "UNDER_REVIEW",
		"sourceBranch": "8b08d11abcdef",
		"targetBranch": "hifox-agent/pao-tui-xiao-di/ZYPT-6522X",
	}
	view := BuildMRSummaryView(nil, detail)
	if view.TargetEqualsSource != nil && *view.TargetEqualsSource {
		t.Fatalf("SHA != branch name, target_equals_source must not be true: %+v", view)
	}
	if view.CannotMergeToMaster == nil || !*view.CannotMergeToMaster {
		t.Fatalf("cannot_merge_to_master: %+v", view)
	}
	if !strings.Contains(view.NextStep, "不可直接合入 master") || !strings.Contains(view.NextStep, "--target-branch master") {
		t.Fatalf("next_step: %q", view.NextStep)
	}
}

func TestBuildMRSummaryViewOmitsTargetFlagsWhenNormal(t *testing.T) {
	view := BuildMRSummaryView(nil, map[string]any{
		"localId": 139, "sourceBranch": "feat/x", "targetBranch": "master", "status": "UNDER_DEV",
	})
	if view.CannotMergeToMaster != nil || view.TargetEqualsSource != nil || view.NextStep != "" {
		t.Fatalf("normal MR must not carry target warnings: %+v", view)
	}
	b, _ := json.Marshal(view)
	raw := string(b)
	for _, k := range []string{"cannot_merge_to_master", "target_equals_source", "next_step"} {
		if strings.Contains(raw, k) {
			t.Fatalf("json should omit %s: %s", k, raw)
		}
	}
}
