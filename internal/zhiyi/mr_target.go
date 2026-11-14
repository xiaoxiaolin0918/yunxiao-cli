package zhiyi

import (
	"fmt"
	"strings"
	"unicode"
)

// NormalizeBranchRef trims space and a leading refs/heads/ prefix.
func NormalizeBranchRef(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "refs/heads/")
	return strings.TrimSpace(s)
}

// LooksLikeCommitSHA reports whether s looks like a git object id (7–40 hex).
func LooksLikeCommitSHA(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	for _, r := range s {
		if !unicode.Is(unicode.ASCII_Hex_Digit, r) {
			return false
		}
	}
	return true
}

func isIntegrationBranch(s string) bool {
	switch strings.ToLower(NormalizeBranchRef(s)) {
	case "master", "main", "develop", "trunk":
		return true
	default:
		return false
	}
}

// ClassifyMRTarget reports whether source and target refer to the same branch
// and whether the MR cannot be merged into master/main because of a
// self-targeted or push-review retarget anomaly (#407).
//
// reason is "target_equals_source" or "source_is_revision_target_not_integration"
// when blocked, otherwise empty. Official UpdateChangeRequest cannot change
// targetBranch (title/description only); callers should surface that limit
// rather than invent an API field.
func ClassifyMRTarget(source, target string) (equal, blocked bool, reason string) {
	src := NormalizeBranchRef(source)
	dst := NormalizeBranchRef(target)
	if src != "" && dst != "" && src == dst {
		return true, true, "target_equals_source"
	}
	if LooksLikeCommitSHA(src) && dst != "" && !isIntegrationBranch(dst) {
		return false, true, "source_is_revision_target_not_integration"
	}
	return false, false, ""
}

// MRTargetNextStep is a Chinese next-step hint for +push-review-status when
// the MR cannot be merged to master because of a #407 target anomaly.
func MRTargetNextStep(localID, source, target string) string {
	_, blocked, reason := ClassifyMRTarget(source, target)
	if !blocked {
		return ""
	}
	id := strings.TrimSpace(localID)
	if id == "" {
		id = "<local-id>"
	}
	cmd := fmt.Sprintf("yunxiao codeup mrs update --repo <repo> --local-id %s --target-branch master", id)
	dst := NormalizeBranchRef(target)
	switch reason {
	case "target_equals_source":
		return fmt.Sprintf("目标分支等于源分支，该 MR 不可直接合入 master。请先将目标分支改为 master：%s（OpenAPI UpdateChangeRequest 目前不能改目标分支，命令会说明限制并提示改用 Codeup 网页）。", cmd)
	default:
		return fmt.Sprintf("目标分支为 %s（源为 commit，不是指向 master），该 MR 不可直接合入 master。请将目标分支改为 master：%s（OpenAPI UpdateChangeRequest 目前不能改目标分支，命令会说明限制并提示改用 Codeup 网页）。", dst, cmd)
	}
}
