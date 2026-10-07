package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// enrichMrsMergeError attaches actionable diagnostics to a failed `codeup mrs merge` (#127).
//
// Server rejections (405 SYSTEM_FORBIDDEN_ERROR「该状态下的评审不允许合并」etc.) only echo
// the platform message with no current status, gap, or next step. The CLI best-effort
// GETs the MR (the same confirmed endpoint `mrs get` uses) and appends to the error
// envelope: subtype "merge_rejected" plus error.details (current MR status, state_gap,
// suggested_actions) and a hint naming the next step, so agents can self-heal or hand
// a human a precise instruction instead of digging through the web UI.
//
// Non-API errors (high-risk gate results, CLI failures) pass through untouched, and a
// failed diagnosis GET returns the original error unchanged — enrichment never masks
// the merge failure itself.
func enrichMrsMergeError(ctx context.Context, c *client.Client, repoArg, repoID, localID, mergeType string, err error) error {
	if err == nil {
		return nil
	}
	var ae *client.APIError
	if !errors.As(err, &ae) {
		return err
	}
	mr, diagPath, diagErr := fetchMergeRequestForDiag(ctx, c, repoID, localID)
	if diagErr != nil {
		return err
	}
	status := zhiyi.MRStatus(mr)
	st := zhiyi.StabilizeMergeRequest(mr)

	actions := mergeSuggestedActions(repoArg, localID, mergeType, status)
	details := map[string]any{
		"action":            "merge",
		"state_gap":         mergeStateGap(status),
		"suggested_actions": actions,
		"diagnose":          map[string]any{"source": "GET " + diagPath},
		"mr": map[string]any{
			"localId": st["localId"],
			"title":   st["title"],
			"status":  st["status"],
			"url":     st["url"],
		},
	}
	if status != "" {
		details["current_status"] = status
	}
	return &contextError{
		Context: "merge MR " + localID,
		Subtype: "merge_rejected",
		Hint:    mergeRejectionHint(status, actions),
		Details: details,
		Err:     err,
	}
}

// fetchMergeRequestForDiag GETs the MR detail for post-failure diagnosis
// (same path as `codeup mrs get`; no new endpoint invented).
func fetchMergeRequestForDiag(ctx context.Context, c *client.Client, repoID, localID string) (map[string]any, string, error) {
	path, err := c.CodeupPath(ctx, "/repositories/"+repoID+"/changeRequests/"+localID)
	if err != nil {
		return nil, "", err
	}
	var out any
	if _, err := c.Do(ctx, "GET", path, nil, nil, &out); err != nil {
		return nil, path, err
	}
	return zhiyi.UnwrapMergeRequestPayload(asStringMap(out)), path, nil
}

// mergeStateGap explains, in one sentence, why merge is blocked at the given MR status.
// UNDER_DEV evidence: #124/#127 — push-review MRs sit in 开发中(WIP) until the web UI
// 「取消 WIP」moves them to 待合并; that is the only unblock today (no OpenAPI for it).
func mergeStateGap(status string) string {
	switch mergeStatusKey(status) {
	case "UNDER_DEV":
		return "MR is UNDER_DEV (开发中/WIP): merge is blocked until WIP is cleared (web UI「取消 WIP」→ 待合并)"
	case "MERGED":
		return "MR is already MERGED: nothing left to merge"
	case "CLOSED":
		return "MR is CLOSED: reopen it before merging"
	case "UNDER_REVIEW":
		return "MR is UNDER_REVIEW: merge needs the review to pass first"
	case "":
		return "merge rejected and the MR detail carried no recognizable status"
	default:
		return "merge rejected while MR status is " + status
	}
}

// mergeSuggestedActions returns concrete next steps (CLI commands where one exists,
// otherwise the exact web UI action) for a merge rejected at the given status.
func mergeSuggestedActions(repoArg, localID, mergeType, status string) []string {
	retry := mergeRetryCommand(repoArg, localID, mergeType)
	inspect := fmt.Sprintf("yunxiao codeup mrs get --repo %s --local-id %s", shellArg(repoArg), shellArg(localID))
	switch mergeStatusKey(status) {
	case "UNDER_DEV":
		// #124: 取消 WIP has no OpenAPI yet; the web path is the confirmed workaround.
		return []string{
			"web UI: open the MR page →「…」more menu →「取消 WIP」(status becomes 待合并)",
			retry,
		}
	case "MERGED":
		return []string{inspect + " — already merged; no further action"}
	case "CLOSED":
		return []string{
			fmt.Sprintf("yunxiao codeup mrs reopen --repo %s --local-id %s --dry-run (then --yes)", shellArg(repoArg), shellArg(localID)),
			retry,
		}
	case "UNDER_REVIEW":
		return []string{
			fmt.Sprintf("yunxiao codeup mrs review --repo %s --local-id %s --opinion PASS --dry-run (then --yes) once the review should pass", shellArg(repoArg), shellArg(localID)),
			retry,
		}
	default:
		return []string{inspect, retry}
	}
}

// mergeRejectionHint renders the human/agent hint: gap first, then the next steps.
func mergeRejectionHint(status string, actions []string) string {
	hint := mergeStateGap(status)
	if len(actions) == 0 {
		return hint
	}
	return hint + "; next: " + strings.Join(actions, " | ")
}

func mergeRetryCommand(repoArg, localID, mergeType string) string {
	return fmt.Sprintf("yunxiao codeup mrs merge --repo %s --local-id %s --merge-type %s --yes",
		shellArg(repoArg), shellArg(localID), shellArg(mergeType))
}

// mergeStatusKey normalizes an MR status for table lookups (upper-case, trimmed).
func mergeStatusKey(status string) string {
	return strings.ToUpper(strings.TrimSpace(status))
}
