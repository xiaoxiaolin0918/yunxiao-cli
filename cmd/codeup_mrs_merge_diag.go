package cmd

import (
	"fmt"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// mergeStateGap explains, in one sentence, why merge is blocked at the given MR status (#127).
// UNDER_DEV evidence: #124/#127 — push-review MRs sit in 开发中(WIP) until the web UI
// 「取消 WIP」moves them to 待合并; that is the only unblock today (no OpenAPI for it).
func mergeStateGap(status string) string {
	switch mergeStatusKey(status) {
	case "UNDER_DEV":
		return "MR is UNDER_DEV (开发中/WIP): merge is blocked until WIP is cleared (web UI「取消 WIP」→ 待合并; no OpenAPI to cancel WIP)"
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

// unsupportedMergeTypeGap is the state_gap wording when the API rejects the
// requested --merge-type (#130 residual: 不支持X合并方式).
func unsupportedMergeTypeGap(mergeType string) string {
	n := zhiyi.NormalizeMergeType(mergeType)
	if n == "" {
		n = mergeType
	}
	return fmt.Sprintf("repository does not support merge-type %s (server rejected the merge method)", n)
}

// mergeSuggestedActions returns concrete next steps (CLI commands where one exists,
// otherwise the exact web UI action) for a merge rejected at the given status (#127).
// When apiErr indicates an unsupported merge method (#130 residual), prefer
// alternative --merge-type commands and never echo the same failing type.
func mergeSuggestedActions(repoArg, localID, mergeType, status string, apiErr *client.APIError) []string {
	inspect := fmt.Sprintf("yunxiao codeup mrs get --repo %s --local-id %s", shellArg(repoArg), shellArg(localID))
	if isUnsupportedMergeTypeAPIError(apiErr) {
		return unsupportedMergeTypeActions(repoArg, localID, mergeType, inspect)
	}
	retry := mergeRetryCommand(repoArg, localID, mergeType)
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
			fmt.Sprintf("yunxiao codeup mrs review --repo %s --local-id %s --opinion PASS --dry-run (then --yes) once a non-author reviewer can pass it", shellArg(repoArg), shellArg(localID)),
			fmt.Sprintf("if you are the MR author, Codeup forbids self-review: yunxiao codeup mrs reviewers add --repo %s --local-id %s --reviewer <otherUserId>", shellArg(repoArg), shellArg(localID)),
			retry,
		}
	default:
		return []string{inspect, retry}
	}
}

// unsupportedMergeTypeActions lists alternative merge-type retries (excluding the
// failing one). Prefers no-fast-forward first — the common "创建合并节点" option
// when fast-forward-only is disabled on the repo.
func unsupportedMergeTypeActions(repoArg, localID, mergeType, inspect string) []string {
	failed := zhiyi.NormalizeMergeType(mergeType)
	candidates := []string{"no-fast-forward", "squash", "rebase", "ff-only"}
	var actions []string
	for _, alt := range candidates {
		if alt == failed {
			continue
		}
		actions = append(actions, mergeRetryCommand(repoArg, localID, alt))
	}
	actions = append(actions, inspect+" — confirm merge settings; or enable the method in 仓库设置 → 合并设置")
	return actions
}

// isUnsupportedMergeTypeAPIError detects Codeup rejections whose body says the
// merge method itself is forbidden (实战: HTTP 403 SYSTEM_FORBIDDEN_ERROR
// "不支持fast-forward-only合并方式"). Distinct from UNDER_DEV / WIP 405s.
func isUnsupportedMergeTypeAPIError(ae *client.APIError) bool {
	if ae == nil {
		return false
	}
	body := ae.Body
	lower := strings.ToLower(body)
	// Chinese platform wording observed on zhiyi_doc.
	if strings.Contains(body, "不支持") && strings.Contains(body, "合并方式") {
		return true
	}
	// English / mixed fallbacks.
	if strings.Contains(lower, "not support") && (strings.Contains(lower, "merge") || strings.Contains(lower, "fast-forward") || strings.Contains(lower, "rebase") || strings.Contains(lower, "squash")) {
		return true
	}
	if strings.Contains(lower, "unsupported") && strings.Contains(lower, "merge") {
		return true
	}
	return false
}

// mergeRejectionHint renders the human/agent hint: gap first, then the next steps (#127).
func mergeRejectionHint(status string, actions []string, apiErr *client.APIError, mergeType string) string {
	var hint string
	if isUnsupportedMergeTypeAPIError(apiErr) {
		hint = unsupportedMergeTypeGap(mergeType)
	} else {
		hint = mergeStateGap(status)
	}
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
