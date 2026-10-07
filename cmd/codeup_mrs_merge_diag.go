package cmd

import (
	"fmt"
	"strings"
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

// mergeSuggestedActions returns concrete next steps (CLI commands where one exists,
// otherwise the exact web UI action) for a merge rejected at the given status (#127).
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

// mergeRejectionHint renders the human/agent hint: gap first, then the next steps (#127).
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
