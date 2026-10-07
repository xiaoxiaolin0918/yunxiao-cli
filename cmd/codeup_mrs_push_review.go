package cmd

import (
	"context"
	"net/http"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// #132: push-review status shortcut. Push-review (推送评审) repos create/accumulate
// MRs on git push; their lifecycle status (ListMergeRequests newVersionState /
// GetChangeRequest status) is UNDER_DEV (开发中/WIP — merge blocked, #124),
// UNDER_REVIEW (评审中) or TO_BE_MERGED (待合并). This shortcut makes
// "push → check MR state → decide" one command instead of hand-parsed git output
// plus web UI visits.
var codeupMrsPushReviewStatusCmd = &cobra.Command{
	Use:   "+push-review-status",
	Short: "Shortcut: push-review status of a repository's open merge requests",
	Long: `Risk: read
HTTP: GET .../changeRequests?state=opened&projectIds=<repo>
      + one read GET .../changeRequests/{localId} per listed open MR

Per MR: localId/title/status/state/status_display/wip, source/target branches,
ahead/behind, mergeable (allRequirementsPass), todo (requirementCheckItems),
reviewers (+review_opinion_status) and url. meta carries repository_id, count and
wip_count (+ list pagination without --local-id).

Status values (push-review lifecycle): UNDER_DEV 开发中(WIP) — mrs merge is
rejected while WIP (#124, 405 SYSTEM_FORBIDDEN_ERROR); cancel it in the web UI
(MR page → 更多(…) → 取消 WIP — no OpenAPI exists for that transition), then merge.
UNDER_REVIEW 评审中. TO_BE_MERGED 待合并 — mergeable when requirements pass.

--local-id skips the list and GETs just that MR (works regardless of page order).
--all follows list pages via client.ListAll (cap 50); otherwise the first page
(--per-page, default 20) is inspected. --dry-run previews the read(s) only.

  yunxiao codeup mrs +push-review-status --repo <id|alias>
  yunxiao codeup mrs +push-review-status --repo <id> --local-id 139
  yunxiao codeup mrs +push-review-status --repo <id> --all`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		localID, _ := cmd.Flags().GetString("local-id")
		allPages, _ := cmd.Flags().GetBool("all")
		perPage, _ := cmd.Flags().GetInt("per-page")
		if err := requireFlags("repo", repo); err != nil {
			handleErr(err)
			return
		}
		repositoryID, err := resolveCodeupRepo(repo)
		if err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}

		meta := map[string]any{"risk": risk.Read, "repository_id": repositoryID}

		// Single-MR mode: one detail GET, no list call (a paginated list could miss
		// the MR on a later page; the detail GET works for merged/closed MRs too).
		if localID != "" {
			if globalDryRun {
				handleErr(mrsPushReviewDryRunPreview(cmd.Context(), c, repositoryID, localID, perPage))
				return
			}
			detail, err := fetchMrsDetail(cmd.Context(), c, repositoryID, localID)
			if err != nil {
				handleErr(&contextError{Context: "refresh MR " + localID + " push-review status", Err: err})
				return
			}
			view := zhiyi.BuildMRSummaryView(nil, detail)
			meta["local_id"] = localID
			meta["count"] = 1
			wipCount := 0
			if view.WIP != nil && *view.WIP {
				wipCount = 1
			}
			meta["wip_count"] = wipCount
			handleErr(output.Success([]zhiyi.MRSummaryView{view}, meta))
			return
		}

		if globalDryRun {
			handleErr(mrsPushReviewDryRunPreview(cmd.Context(), c, repositoryID, "", perPage))
			return
		}
		listItems, listMeta, err := fetchOpenMRsForRepo(cmd.Context(), c, repositoryID, perPage, allPages)
		if err != nil {
			handleErr(err)
			return
		}
		for k, v := range listMeta {
			if k == "risk" {
				continue
			}
			meta[k] = v
		}

		views := make([]zhiyi.MRSummaryView, 0, len(listItems))
		wipCount := 0
		for _, it := range listItems {
			item, ok := it.(map[string]any)
			if !ok {
				continue
			}
			lid := zhiyi.MergeRequestLocalID(item)
			if lid == "" {
				continue
			}
			detail, err := fetchMrsDetail(cmd.Context(), c, repositoryID, lid)
			if err != nil {
				handleErr(&contextError{Context: "refresh MR " + lid + " push-review status", Err: err})
				return
			}
			view := zhiyi.BuildMRSummaryView(item, detail)
			if view.WIP != nil && *view.WIP {
				wipCount++
			}
			views = append(views, view)
		}
		meta["count"] = len(views)
		meta["wip_count"] = wipCount
		handleErr(output.Success(views, meta))
	},
}

// fetchMrsDetail GETs one MR detail and returns the unwrapped object.
// Shared by +push-review-status (#132) and mrs merge error enrichment (#124).
func fetchMrsDetail(ctx context.Context, c *client.Client, repositoryID, localID string) (map[string]any, error) {
	path, err := c.CodeupPath(ctx, "/repositories/"+client.EncodeRepoID(repositoryID)+"/changeRequests/"+localID)
	if err != nil {
		return nil, err
	}
	var out any
	if _, err := c.Do(ctx, "GET", path, nil, nil, &out); err != nil {
		return nil, err
	}
	return zhiyi.UnwrapMergeRequestPayload(asStringMap(out)), nil
}

// fetchOpenMRsForRepo lists the repo's open MRs (state=opened, projectIds) and
// returns the raw items plus pagination meta (ListAll meta with --all).
func fetchOpenMRsForRepo(ctx context.Context, c *client.Client, repositoryID string, perPage int, allPages bool) ([]any, map[string]any, error) {
	listPath, err := c.CodeupPath(ctx, "/changeRequests")
	if err != nil {
		return nil, nil, err
	}
	baseQ := map[string]string{"state": "opened", "projectIds": repositoryID}
	fetch := func(ctx context.Context, q map[string]string) (any, http.Header, error) {
		var out any
		hdr, err := c.Do(ctx, "GET", listPath, q, nil, &out)
		return out, hdr, err
	}
	if allPages {
		res, err := client.ListAll(ctx, 1, perPage, client.DefaultListAllMaxPages, baseQ, fetch)
		if err != nil {
			return nil, nil, err
		}
		items := res.Items
		if items == nil {
			items = []any{}
		}
		return items, res.Meta, nil
	}
	q := map[string]string{"page": "1", "perPage": strconv.Itoa(perPage)}
	for k, v := range baseQ {
		q[k] = v
	}
	out, hdr, err := fetch(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	items := client.ExtractListItems(out)
	if items == nil {
		items = []any{}
	}
	return items, client.MetaWithPagination(map[string]any{}, hdr), nil
}

// mrsPushReviewDryRunPreview builds the +push-review-status dry-run envelope: the
// primary read preview (list GET, or the single detail GET with --local-id) plus a
// note about the per-MR detail GETs a real run performs. No network calls.
func mrsPushReviewDryRunPreview(ctx context.Context, c *client.Client, repositoryID, localID string, perPage int) error {
	var primary any
	if localID != "" {
		path, err := c.CodeupPath(ctx, "/repositories/"+client.EncodeRepoID(repositoryID)+"/changeRequests/"+localID)
		if err != nil {
			return err
		}
		primary = c.Preview("GET", path, nil, nil)
	} else {
		path, err := c.CodeupPath(ctx, "/changeRequests")
		if err != nil {
			return err
		}
		q := map[string]string{"page": "1", "perPage": strconv.Itoa(perPage), "state": "opened", "projectIds": repositoryID}
		primary = c.Preview("GET", path, q, nil)
	}
	req := map[string]any{
		"list": primary,
		"detail": map[string]any{
			"method":   "GET",
			"per_mrs":  "one read GET .../changeRequests/{localId} per listed open MR",
			"skipped":  true,
			"skip_for": "dry-run",
		},
	}
	return output.DryRunResult(string(risk.Read), req)
}

func init() {
	codeupMrsPushReviewStatusCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsPushReviewStatusCmd.Flags().String("local-id", "", "inspect only this MR (single detail GET)")
	codeupMrsPushReviewStatusCmd.Flags().Bool("all", false, "follow list pages (ListAll, max 50)")
	codeupMrsPushReviewStatusCmd.Flags().Int("per-page", 20, "list page size without --all")
	codeupMrsCmd.AddCommand(codeupMrsPushReviewStatusCmd)
}
