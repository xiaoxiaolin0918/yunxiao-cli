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

// runMrsMerge is the mrs merge execution path (#124 + #130): same gate/dry-run
// contract as runJSONMutating (high-risk-write → --yes, --dry-run previews the POST),
// with a read-only precheck before the POST (#130: status/conflicts/merge-type vs
// repo merge settings; failure refuses the merge without POSTing, success attaches
// meta.precheck). On an API error from the merge POST it GETs the MR once and
// enriches the error with error.details.mr (current status/wip/ahead/behind/
// mergeable/todo) plus an actionable hint. Success output keeps the raw API object
// (meta gains url/status/precheck).
func runMrsMerge(ctx context.Context, c *client.Client, repoArg, repositoryID, localID, path string, body map[string]any, mergeType string, rp client.RequestPreview) error {
	return runMutating("codeup mrs merge", risk.HighRiskWrite, globalDryRun, globalYes, rp, func() error {
		pre, perr := precheckMrsMerge(ctx, c, client.EncodeRepoID(repositoryID), localID, mergeType)
		if perr != nil {
			return perr
		}
		var out any
		if _, err := c.Do(ctx, "POST", path, nil, body, &out); err != nil {
			return enrichMrsMergeError(ctx, c, repoArg, repositoryID, localID, mergeType, err)
		}
		meta := map[string]any{"risk": risk.HighRiskWrite, "precheck": pre}
		mr := zhiyi.StabilizeMergeRequest(zhiyi.UnwrapMergeRequestPayload(asStringMap(out)))
		zhiyi.EnrichMergeRequestMeta(meta, mr)
		return output.Success(out, meta)
	})
}

// enrichMrsMergeError (#124 + #127): when the merge POST fails with an API error,
// fetch the MR once and attach structured diagnostics. The original merge error is
// never masked: a failing or non-API refresh returns it untouched.
//
// Envelope (#127): subtype "merge_rejected" plus error.details.current_status /
// state_gap / suggested_actions / diagnose.source, and details.mr (#124) with
// status/wip/ahead/behind/mergeable/todo/url. Hint prefers the #127 gap+next
// wording and still mentions failed todo types / title-prefix WIP when relevant.
//
// Push-review WIP context: while an MR is UNDER_DEV (开发中) the server rejects
// merges (405 SYSTEM_FORBIDDEN_ERROR「该状态下的评审不允许合并」) even when the review
// itself has PASSED. There is NO confirmed OpenAPI to transition the server-side
// status (UpdateChangeRequest only accepts title/description; the official MCP
// server has no WIP tool), so suggested_actions point at the web UI's 取消 WIP.
// This is the server-side status, deliberately distinct from the "WIP: " title
// prefix (PR #135 adds --wip/--unwip title toggles on mrs update; a title-prefix
// WIP is cleared by renaming, which does not help an UNDER_DEV push-review MR).
func enrichMrsMergeError(ctx context.Context, c *client.Client, repoArg, repositoryID, localID, mergeType string, mergeErr error) error {
	var ae *client.APIError
	if !errors.As(mergeErr, &ae) {
		return mergeErr // only API errors carry merge-block context
	}
	mr, err := fetchMrsDetail(ctx, c, repositoryID, localID)
	if err != nil {
		return mergeErr // never mask the merge error with a refresh failure
	}
	status := zhiyi.MRStatus(mr)
	detail := mrsMergeErrorDetail(mr)
	actions := mergeSuggestedActions(repoArg, localID, mergeType, status)
	details := map[string]any{
		"action":            "merge",
		"state_gap":         mergeStateGap(status),
		"suggested_actions": actions,
		"diagnose":          map[string]any{"source": "GET changeRequests/" + localID},
		"mr":                detail,
	}
	if status != "" {
		details["current_status"] = status
	}
	hint := mergeRejectionHint(status, actions)
	// Keep #124 extras that the structured table may not cover.
	if failed := mrsMergeFailedTodoTypes(mr); len(failed) > 0 {
		hint += ". merge requirements not met: " + strings.Join(failed, ", ") +
			" (see error.details.mr.todo; resolve conflicts / pending comments via `mrs comments resolve` / CI / reviewer approval)"
	}
	title, _ := zhiyi.StabilizeMergeRequest(mr)["title"].(string)
	if mergeStatusKey(status) != "UNDER_DEV" && strings.HasPrefix(strings.TrimSpace(title), "WIP:") {
		hint += ". MR title starts with \"WIP:\", which also blocks merges; rename without the prefix: " +
			fmt.Sprintf("yunxiao codeup mrs update --repo %s --local-id %s --title %q",
				shellArg(repoArg), shellArg(localID), strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(title), "WIP:")))
	}
	return &contextError{
		Context: "merge MR " + localID + " blocked",
		Subtype: "merge_rejected",
		Hint:    hint,
		Err:     mergeErr,
		Details: details,
	}
}


// mrsMergeErrorDetail builds error.details.mr from the fetched MR object (#124/#127).
func mrsMergeErrorDetail(mr map[string]any) map[string]any {
	mr = zhiyi.StabilizeMergeRequest(mr)
	detail := map[string]any{}
	for _, k := range []string{"localId", "title", "status", "state", "wip", "url", "detailUrl"} {
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

