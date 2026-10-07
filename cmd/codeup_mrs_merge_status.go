package cmd

import (
	"context"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// runMrsMerge is the mrs merge execution path (#124 + #130): same gate/dry-run
// contract as runJSONMutating (high-risk-write → --yes, --dry-run previews the POST),
// with a read-only precheck before the POST (#130: status/conflicts/merge-type vs
// repo merge settings; failure refuses the merge without POSTing, success attaches
// meta.precheck). On an API error from the merge POST it runs the #127 diagnosis
// (subtype merge_rejected: current status, state_gap, suggested_actions and
// error.details.mr with status/wip/ahead/behind/mergeable/todo; the #124 details were
// folded into that envelope, see cmd/codeup_mrs_merge_diag.go). Success output keeps
// the raw API object (meta gains url/status/precheck).
func runMrsMerge(ctx context.Context, c *client.Client, repoArg, repositoryID, localID, path string, body map[string]any, mergeType string, rp client.RequestPreview) error {
	return runMutating("codeup mrs merge", risk.HighRiskWrite, globalDryRun, globalYes, rp, func() error {
		pre, perr := precheckMrsMerge(ctx, c, client.EncodeRepoID(repositoryID), localID, mergeType)
		if perr != nil {
			return perr
		}
		var out any
		if _, err := c.Do(ctx, "POST", path, nil, body, &out); err != nil {
			// #127 merge_rejected diagnosis (superset of the former #124-only enrich).
			return enrichMrsMergeError(ctx, c, repoArg, client.EncodeRepoID(repositoryID), localID, mergeType, err)
		}
		meta := map[string]any{"risk": risk.HighRiskWrite, "precheck": pre}
		mr := zhiyi.StabilizeMergeRequest(zhiyi.UnwrapMergeRequestPayload(asStringMap(out)))
		zhiyi.EnrichMergeRequestMeta(meta, mr)
		return output.Success(out, meta)
	})
}
