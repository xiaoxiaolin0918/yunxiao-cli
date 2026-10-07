package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// precheckMrsMerge (#130 item 4) reads the current MR detail (one read-only GET,
// also under --dry-run) and checks merge-method / status consistency before the
// merge POST, so an unsupported --merge-type or an unmergeable MR fails locally
// with a structured error and an actionable hint instead of an undefined server
// 405. The GET failure is fatal (fail closed: this gates a high-risk write).
// A passing check returns a meta.precheck / request.precheck map.
func precheckMrsMerge(ctx context.Context, c *client.Client, repoID, localID, mergeType string) (map[string]any, error) {
	detailPath, err := c.CodeupPath(ctx, "/repositories/"+repoID+"/changeRequests/"+localID)
	if err != nil {
		return nil, err
	}
	var out any
	if _, err := c.Do(ctx, "GET", detailPath, nil, nil, &out); err != nil {
		return nil, &contextError{
			Context: "merge precheck GET " + detailPath,
			Hint:    "the MR detail could not be read; fix the error above or rerun — the merge POST is not attempted",
			Err:     err,
		}
	}
	mr := zhiyi.UnwrapMergeRequestPayload(asStringMap(out))
	if mr == nil {
		return nil, &detailedError{
			Subtype: "mr_detail_unreadable",
			Message: fmt.Sprintf("merge precheck: MR %s detail is not a JSON object; refusing to merge blindly", localID),
			Hint:    "inspect the raw detail first: yunxiao codeup mrs get --repo <repo> --local-id " + localID + " --full",
			Details: map[string]any{"local_id": localID},
		}
	}
	pre := map[string]any{
		"status":     "ok",
		"source":     "changeRequests/" + localID,
		"merge_type": mergeType,
	}
	status := zhiyi.MRStatus(mr)
	pre["mr_status"] = status
	if ccs := zhiyi.MRConflictCheckStatus(mr); ccs != "" {
		pre["conflict_check_status"] = ccs
	}
	if mb, present := zhiyi.MRBool(mr, "mergeable"); present {
		pre["mergeable"] = mb
	}

	// 1. Status consistency: terminal states can never merge.
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "MERGED":
		return nil, &detailedError{
			Subtype: "mr_already_merged",
			Message: fmt.Sprintf("merge precheck: MR !%s is already merged (status MERGED); nothing to merge", localID),
			Hint:    "check the MR state first: yunxiao codeup mrs get --repo <repo> --local-id " + localID,
			Details: map[string]any{"local_id": localID, "mr_status": status},
		}
	case "CLOSED":
		return nil, &detailedError{
			Subtype: "mr_closed",
			Message: fmt.Sprintf("merge precheck: MR !%s is closed (status CLOSED); closed MRs cannot be merged", localID),
			Hint:    "reopen it first: yunxiao codeup mrs reopen --repo <repo> --local-id " + localID,
			Details: map[string]any{"local_id": localID, "mr_status": status},
		}
	}

	// 2. Conflict / mergeability consistency.
	switch zhiyi.MRConflictCheckStatus(mr) {
	case "HAS_CONFLICT":
		return nil, conflictPrecheckError(localID, "mr_conflict",
			"the source and target branches conflict (conflictCheckStatus HAS_CONFLICT)",
			"resolve the conflicts first (rebase or merge target into source and push), then rerun; state via: yunxiao codeup mrs get --repo <repo> --local-id "+localID, status)
	case "CHECKING":
		return nil, conflictPrecheckError(localID, "mr_conflict_checking",
			"the conflict check is still running (conflictCheckStatus CHECKING)",
			"wait a moment and rerun the merge", status)
	}
	if mb, present := zhiyi.MRBool(mr, "mergeable"); present && !mb {
		return nil, conflictPrecheckError(localID, "mr_not_mergeable",
			"the MR reports mergeable=false",
			"the server says this MR cannot be merged yet: check conflictCheckStatus, review/merge requirements via `codeup mrs get` (summary view)", status)
	}

	// 3. Merge-method availability (issue core ask: unsupported rebase reported
	// locally with the usable alternatives). Only enforced when the payload
	// exposes the configuration; otherwise noted and left to the server.
	normalized := zhiyi.NormalizeMergeType(mergeType)
	pre["merge_type_normalized"] = normalized
	if avail, src := zhiyi.MergeTypesFromMR(mr); len(avail) > 0 {
		pre["merge_types_available"] = avail
		pre["merge_types_source"] = src
		for _, a := range avail {
			if a == normalized {
				return pre, nil
			}
		}
		return nil, &detailedError{
			Subtype: "merge_type_not_supported",
			Message: fmt.Sprintf("merge precheck: this repository does not support merge-type %s (available: %s)", mergeType, strings.Join(avail, ", ")),
			Hint:    "pass one of the available types via --merge-type, or enable the method in the Codeup web UI: 仓库设置 → 合并设置 (repo settings → merge settings)",
			Details: map[string]any{"local_id": localID, "merge_type": mergeType, "merge_type_normalized": normalized, "available": avail, "available_source": src, "mr_status": status},
		}
	}
	// Documented fallback signal: fast-forward-only support flag.
	if ffOK, present := zhiyi.MRBool(mr, "supportMergeFastForwardOnly", "support_merge_fast_forward_only"); present && !ffOK && normalized == "ff-only" {
		pre["merge_types_available"] = []string{"no-fast-forward", "squash", "rebase"}
		pre["merge_types_source"] = "supportMergeFastForwardOnly=false"
		return nil, &detailedError{
			Subtype: "merge_type_not_supported",
			Message: "merge precheck: this repository does not support merge-type ff-only (supportMergeFastForwardOnly=false; available: no-fast-forward, squash, rebase)",
			Hint:    "use --merge-type no-fast-forward (创建合并节点), squash or rebase, or enable fast-forward-only in the Codeup web UI: 仓库设置 → 合并设置",
			Details: map[string]any{"local_id": localID, "merge_type": mergeType, "merge_type_normalized": normalized, "available": []string{"no-fast-forward", "squash", "rebase"}, "available_source": "supportMergeFastForwardOnly", "mr_status": status},
		}
	}
	pre["merge_types_available"] = nil
	pre["merge_types_note"] = "merge-method config not exposed by the MR detail; server will validate --merge-type"
	return pre, nil
}

func conflictPrecheckError(localID, subtype, reason, hint, status string) error {
	return &detailedError{
		Subtype: subtype,
		Message: fmt.Sprintf("merge precheck: MR !%s cannot be merged: %s", localID, reason),
		Hint:    hint,
		Details: map[string]any{"local_id": localID, "mr_status": status},
	}
}
