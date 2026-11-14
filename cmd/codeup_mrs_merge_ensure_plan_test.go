package cmd

import (
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/client"
)

func TestBuildEnsureApprovedPlanAddsThenPassThenMerge(t *testing.T) {
	mr := map[string]any{
		"localId": 21,
		"status":  "UNDER_REVIEW",
		"author":  map[string]any{"userId": "author-1"},
		"reviewers": []any{
			map[string]any{"userId": "other", "reviewOpinionStatus": "PASS"},
		},
	}
	plan, err := buildEnsureApprovedPlan(mr, "user-me", "", "demo-repo", "21")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if plan.ReviewerUserID != "user-me" || plan.CurrentUserID != "user-me" {
		t.Fatalf("ids: %+v", plan)
	}
	if got := stepStatuses(plan); strings.Join(got, ",") != "reviewers_add:will_run,review_pass:will_run,merge:will_run" {
		t.Fatalf("steps=%v", got)
	}
}

func TestBuildEnsureApprovedPlanSkipsWhenAlreadyReviewerAndPass(t *testing.T) {
	mr := map[string]any{
		"localId": 21,
		"status":  "TO_BE_MERGED",
		"author":  map[string]any{"userId": "author-1"},
		"reviewers": []any{
			map[string]any{"userId": "user-me", "reviewOpinionStatus": "PASS"},
		},
	}
	plan, err := buildEnsureApprovedPlan(mr, "user-me", "", "demo-repo", "21")
	if err != nil {
		t.Fatal(err)
	}
	if got := stepStatuses(plan); strings.Join(got, ",") != "reviewers_add:skip_already_reviewer,review_pass:skip_already_pass,merge:will_run" {
		t.Fatalf("steps=%v", got)
	}
}

func TestBuildEnsureApprovedPlanSkipsPassOnToBeMerged(t *testing.T) {
	mr := map[string]any{
		"localId": 21,
		"status":  "TO_BE_MERGED",
		"author":  map[string]any{"userId": "author-1"},
		"reviewers": []any{
			map[string]any{"userId": "user-me"},
		},
	}
	plan, err := buildEnsureApprovedPlan(mr, "user-me", "", "demo-repo", "21")
	if err != nil {
		t.Fatal(err)
	}
	if got := stepStatuses(plan); strings.Join(got, ",") != "reviewers_add:skip_already_reviewer,review_pass:skip_already_pass,merge:will_run" {
		t.Fatalf("steps=%v", got)
	}
}

func TestBuildEnsureApprovedPlanAuthorBlocksPass(t *testing.T) {
	mr := map[string]any{
		"localId":   21,
		"status":    "UNDER_REVIEW",
		"author":    map[string]any{"userId": "user-me"},
		"reviewers": []any{},
	}
	_, err := buildEnsureApprovedPlan(mr, "user-me", "", "demo-repo", "21")
	if err == nil {
		t.Fatal("expected author self-review error")
	}
	de, ok := err.(*detailedError)
	if !ok || de.Subtype != "review_as_author" {
		t.Fatalf("err=%T %+v", err, err)
	}
}

func TestBuildEnsureApprovedPlanAuthorAlreadyPassCanMerge(t *testing.T) {
	mr := map[string]any{
		"localId": 21,
		"status":  "TO_BE_MERGED",
		"author":  map[string]any{"userId": "user-me"},
		"reviewers": []any{
			map[string]any{"userId": "other", "reviewOpinionStatus": "PASS"},
		},
	}
	plan, err := buildEnsureApprovedPlan(mr, "user-me", "", "demo-repo", "21")
	if err != nil {
		t.Fatal(err)
	}
	// Author may merge an already-approved MR; PASS is skipped so no self-review.
	if got := stepStatuses(plan); !strings.Contains(strings.Join(got, ","), "review_pass:skip_already_pass") || !strings.Contains(strings.Join(got, ","), "merge:will_run") {
		t.Fatalf("steps=%v", got)
	}
}

func TestBuildEnsureApprovedPlanRequestedReviewer(t *testing.T) {
	mr := map[string]any{"localId": 21, "status": "UNDER_REVIEW", "author": map[string]any{"userId": "author-1"}}
	plan, err := buildEnsureApprovedPlan(mr, "user-me", "reviewer-cfg", "demo-repo", "21")
	if err != nil {
		t.Fatal(err)
	}
	if plan.ReviewerUserID != "reviewer-cfg" {
		t.Fatalf("reviewer=%q", plan.ReviewerUserID)
	}
	// Configured reviewer is added; current user must also be a reviewer to PASS.
	got := stepStatuses(plan)
	joined := strings.Join(got, ",")
	if !strings.Contains(joined, "reviewers_add:will_run") || !strings.Contains(joined, "review_pass:will_run") {
		t.Fatalf("steps=%v", got)
	}
}

func TestBuildEnsureApprovedPlanMissingIdentity(t *testing.T) {
	_, err := buildEnsureApprovedPlan(map[string]any{"status": "UNDER_REVIEW"}, "", "", "demo-repo", "21")
	if err == nil {
		t.Fatal("expected error")
	}
	de, ok := err.(*detailedError)
	if !ok || de.Subtype != "ensure_approved_identity_unknown" {
		t.Fatalf("err=%T %+v", err, err)
	}
}

func TestClassifyMergeRejection(t *testing.T) {
	pushBody := "SYSTEM_FORBIDDEN_ERROR \u65e0\u6743\u9650\u63a8\u9001\u5230\u4fdd\u62a4\u5206\u652f"
	approvalBody := "\u8be5\u72b6\u6001\u4e0b\u7684\u8bc4\u5ba1\u4e0d\u5141\u8bb8\u5408\u5e76"
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "protected branch 403", status: 403, body: pushBody, want: mergeRejectionPushPermission},
		{name: "english protected branch", status: 403, body: `{"errorMessage":"No permission to push to protected branch"}`, want: mergeRejectionPushPermission},
		{name: "approval 405", status: 405, body: approvalBody, want: mergeRejectionApprovalGate},
		{name: "author 403 is not push permission", status: 403, body: "can not submit review opinion as the author", want: mergeRejectionApprovalGate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := &client.APIError{Status: tc.status, Body: tc.body, Method: "POST", URL: "/merge"}
			if got := classifyMergeRejection(err); got != tc.want {
				t.Fatalf("class=%q want %q", got, tc.want)
			}
		})
	}
}

func stepStatuses(p *ensureApprovedPlan) []string {
	out := make([]string, 0, len(p.Steps))
	for _, s := range p.Steps {
		out = append(out, s.Action+":"+s.Status)
	}
	return out
}
