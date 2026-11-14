package cmd

import (
	"fmt"
)

// targetBranchUnsupportedError is the local failure for `mrs update --target-branch`.
// Official UpdateChangeRequest accepts only title and description; the CLI does
// not invent a targetBranch body field (#407).
func targetBranchUnsupportedError(localID, target string) error {
	return &detailedError{
		Subtype: "target_branch_unsupported",
		Message: fmt.Sprintf("OpenAPI UpdateChangeRequest does not support changing targetBranch (official body only accepts title and description); cannot retarget MR !%s to %q via the API", localID, target),
		Hint:    "请在 Codeup 网页打开该 MR，将目标分支改为 " + target + "。不要用 mrs +create 绕过已开推送评审模式的仓库规范。",
		Details: map[string]any{
			"local_id":                localID,
			"requested_target_branch": target,
			"official_update_fields":  []string{"title", "description"},
		},
	}
}
