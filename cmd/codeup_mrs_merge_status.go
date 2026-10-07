package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// runMrsMerge is the mrs merge execution path (#124): same gate/dry-run contract as
// runJSONMutating (high-risk-write → --yes, --dry-run previews the POST), but on an
// API error from the merge POST it GETs the MR once and enriches the error with
// error.details.mr (current status/wip/ahead/behind/mergeable/todo) plus an
// actionable hint. Success output keeps the raw API object (meta gains url/status).
func runMrsMerge(ctx context.Context, c *client.Client, repoArg, repositoryID, localID, path string, body map[string]any) error {
	return runMutating("codeup mrs merge", risk.HighRiskWrite, globalDryRun, globalYes, c.Preview("POST", path, nil, body), func() error {
		var out any
		if _, err := c.Do(ctx, "POST", path, nil, body, &out); err != nil {
			return enrichMrsMergeError(ctx, c, repoArg, repositoryID, localID, err)
		}
		meta := map[string]any{"risk": risk.HighRiskWrite}
		mr := zhiyi.StabilizeMergeRequest(zhiyi.UnwrapMergeRequestPayload(asStringMap(out)))
		zhiyi.EnrichMergeRequestMeta(meta, mr)
		return output.Success(out, meta)
	})
}

// enrichMrsMergeError (#124): when the merge POST fails with an API error, fetch the
// MR once and attach its current state. The original merge error is never masked: a
// failing or non-API refresh returns it untouched.
//
// Push-review WIP context: while an MR is UNDER_DEV (开发中) the server rejects
// merges (405 SYSTEM_FORBIDDEN_ERROR「该状态下的评审不允许合并」) even when the review
// itself has PASSED. There is NO confirmed OpenAPI to transition the server-side
// status (UpdateChangeRequest only accepts title/description; the official MCP
// server has no WIP tool), so the hint points at the web UI's 取消 WIP action.
// This is the server-side status, deliberately distinct from the "WIP: " title
// prefix (PR #135 adds --wip/--unwip title toggles on mrs update; a title-prefix
// WIP is cleared by renaming, which does not help an UNDER_DEV push-review MR).
func enrichMrsMergeError(ctx context.Context, c *client.Client, repoArg, repositoryID, localID string, mergeErr error) error {
	var ae *client.APIError
	if !errors.As(mergeErr, &ae) {
		return mergeErr // only API errors carry merge-block context
	}
	mr, err := fetchMrsDetail(ctx, c, repositoryID, localID)
	if err != nil {
		return mergeErr // never mask the merge error with a refresh failure
	}
	detail := mrsMergeErrorDetail(mr)
	return &contextError{
		Context: "merge MR " + localID + " blocked",
		Hint:    mrsMergeBlockedHint(mr, repoArg, localID),
		Err:     mergeErr,
		Details: map[string]any{"mr": detail},
	}
}

// mrsMergeErrorDetail builds error.details.mr from the fetched MR object.
func mrsMergeErrorDetail(mr map[string]any) map[string]any {
	mr = zhiyi.StabilizeMergeRequest(mr)
	detail := map[string]any{}
	for _, k := range []string{"localId", "title", "status", "state", "wip"} {
		if v, ok := mr[k]; ok && v != nil {
			detail[k] = v
		}
	}
	for _, k := range []string{"ahead", "behind", "allRequirementsPass"} {
		if v, ok := mr[k]; ok && v != nil {
			detail[k] = v
		}
	}
	if todoList, ok := mr["todoList"].(map[string]any); ok {
		if items, ok := todoList["requirementCheckItems"].([]any); ok && len(items) > 0 {
			detail["todo"] = items
		}
	}
	return detail
}

// mrsMergeFailedTodoTypes lists the requirement check types that currently fail
// (e.g. MERGE_CONFLICT_CHECK, COMMENTS_CHECK, CI_CHECK, REVIEWER_APPROVED_CHECK).
func mrsMergeFailedTodoTypes(mr map[string]any) []string {
	todoList, _ := mr["todoList"].(map[string]any)
	if todoList == nil {
		return nil
	}
	items, _ := todoList["requirementCheckItems"].([]any)
	var failed []string
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if pass, ok := m["pass"].(bool); ok && !pass {
			if t, ok := m["itemType"].(string); ok && t != "" {
				failed = append(failed, t)
			}
		}
	}
	return failed
}

// mrsMergeBlockedHint composes the actionable hint for a failed merge (#124).
func mrsMergeBlockedHint(mr map[string]any, repoArg, localID string) string {
	mr = zhiyi.StabilizeMergeRequest(mr)
	status, _ := mr["status"].(string)
	title, _ := mr["title"].(string)
	var parts []string
	if strings.EqualFold(status, zhiyi.MRStatusUnderDev) {
		parts = append(parts, fmt.Sprintf(
			"MR status is UNDER_DEV (开发中, push-review WIP): the server rejects merges in this state even when the review PASSED. "+
				"There is no OpenAPI to cancel WIP (UpdateChangeRequest only edits title/description) — open the Codeup web UI, MR page → 更多(…) → 取消 WIP (status flips to 待合并/TO_BE_MERGED), then retry: "+
				"yunxiao codeup mrs merge --repo %s --local-id %s --merge-type <type> --yes",
			shellArg(repoArg), shellArg(localID)))
	} else if strings.HasPrefix(strings.TrimSpace(title), "WIP:") {
		// Title-prefix WIP is a separate signal from the push-review UNDER_DEV status;
		// clearing it is a rename (PR #135 adds --wip/--unwip toggles for exactly this).
		parts = append(parts, "MR title starts with \"WIP:\", which also blocks merges; rename without the prefix: "+
			fmt.Sprintf("yunxiao codeup mrs update --repo %s --local-id %s --title %q", shellArg(repoArg), shellArg(localID), strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(title), "WIP:"))))
	}
	if failed := mrsMergeFailedTodoTypes(mr); len(failed) > 0 {
		parts = append(parts, "merge requirements not met: "+strings.Join(failed, ", ")+
			" (see error.details.mr.todo; resolve conflicts / pending comments via `mrs comments resolve` / CI / reviewer approval)")
	}
	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("check the MR state: yunxiao codeup mrs get --repo %s --local-id %s --brief",
			shellArg(repoArg), shellArg(localID)))
	}
	parts = append(parts, fmt.Sprintf("track push-review MRs: yunxiao codeup mrs +push-review-status --repo %s", shellArg(repoArg)))
	return strings.Join(parts, ". ")
}
