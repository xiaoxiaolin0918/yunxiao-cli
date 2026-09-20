package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/mrlink"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

var codeupMrsPlusCreateCmd = &cobra.Command{
	Use:   "+create",
	Short: "Shortcut: create MR with repo alias, WIP title, work-item link",
	Long: `Risk: high-risk-write (requires --yes after confirmation; prefer --dry-run first)

Zhiyi-oriented wrapper around Codeup changeRequests. Does not replace typed
"codeup mrs create".

  yunxiao codeup mrs +create --profile zhiyi \
    --repo iipmes_gy --source feat/x --target master \
    --title "fix" --work-item ZYPT-5768 --wip --dry-run

  yunxiao codeup mrs +create --repo <repo-id> --source feat/x --title "fix" --yes

--repo accepts numeric id or profile.repositories alias. Profile optional when --repo is numeric.
--target defaults to master. --wip prefixes "WIP: " when target is master.
--reviewer is comma-separated userIds (OpenAPI reviewerUserIds), same as typed mrs create.
--work-item is prechecked via workitem get (abort if missing); after create, missing links warn.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)

		repo, _ := cmd.Flags().GetString("repo")
		source, _ := cmd.Flags().GetString("source")
		target, _ := cmd.Flags().GetString("target")
		title, _ := cmd.Flags().GetString("title")
		desc, _ := cmd.Flags().GetString("description")
		workItem, _ := cmd.Flags().GetString("work-item")
		reviewer, _ := cmd.Flags().GetString("reviewer")
		wip, _ := cmd.Flags().GetBool("wip")

		if err := requireFlags("repo", repo, "source", source, "title", title); err != nil {
			handleErr(err)
			return
		}
		if strings.TrimSpace(target) == "" {
			target = "master"
		}

		repositoryID, err := resolveCodeupRepo(repo)
		if err != nil {
			handleErr(err)
			return
		}
		pf, err := applyActiveProfileOrg()
		if err != nil {
			handleErr(err)
			return
		}

		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}

		expectSpace := ""
		if pf != nil {
			expectSpace = strings.TrimSpace(pf.SpaceID)
		}
		var resolvedWorkItems []mrlink.ResolvedWorkItem
		if refs := splitWorkItemRefs(workItem); len(refs) > 0 {
			items, err := resolveWorkItemsForMR(cmd.Context(), c, refs, expectSpace)
			if err != nil {
				handleErr(err)
				return
			}
			resolvedWorkItems = items
		}
		workItemIDs := mrlink.InternalIDs(resolvedWorkItems)

		title = zhiyi.WithWipTitle(title, target, wip)
		reviewerIDs := zhiyi.SplitUserIDs(reviewer)

		body := map[string]any{
			"title":           title,
			"sourceBranch":    source,
			"targetBranch":    target,
			"sourceProjectId": repositoryID,
			"targetProjectId": repositoryID,
			"createFrom":      "WEB",
			"description":     desc,
			"reviewerUserIds": reviewerIDs,
		}
		if csv := mrlink.WorkItemIDsCSV(workItemIDs); csv != "" {
			body["workItemIds"] = csv
		}

		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests")
		if err != nil {
			handleErr(err)
			return
		}

		meta := map[string]any{"risk": risk.HighRiskWrite, "repository_id": repositoryID}
		if pf != nil {
			meta["profile"] = pf.Name
		}

		if globalDryRun {
			handleErr(output.DryRunResult(string(risk.HighRiskWrite), c.Preview("POST", path, nil, body)))
			return
		}
		if err := risk.CheckHighRisk("codeup mrs +create", globalYes); err != nil {
			handleErr(err)
			return
		}
		var out any
		if err := c.Post(cmd.Context(), path, body, &out); err != nil {
			handleErr(err)
			return
		}
		mrMap := asStringMap(out)
		zhiyi.EnrichMergeRequestMeta(meta, mrMap)
		if err := ensureMRWorkItemLinks(cmd.Context(), c, repositoryID, resolvedWorkItems, mrMap, meta); err != nil {
			handleErr(err)
			return
		}
		handleErr(output.Success(out, meta))
	},
}

func init() {
	codeupMrsPlusCreateCmd.Flags().String("repo", "", "numeric repositoryId or alias (required)")
	codeupMrsPlusCreateCmd.Flags().String("source", "", "source branch (required)")
	codeupMrsPlusCreateCmd.Flags().String("target", "master", "target branch (default master)")
	codeupMrsPlusCreateCmd.Flags().String("title", "", "MR title (required)")
	codeupMrsPlusCreateCmd.Flags().String("description", "", "MR description")
	codeupMrsPlusCreateCmd.Flags().String("work-item", "", "ZYPT serial(s) or id(s), comma-separated; prechecked via workitem get")
	codeupMrsPlusCreateCmd.Flags().String("reviewer", "", "optional reviewer userId(s), comma-separated (OpenAPI reviewerUserIds; same as mrs create)")
	codeupMrsPlusCreateCmd.Flags().Bool("wip", false, "prefix WIP: when target is master")
	codeupMrsCmd.AddCommand(codeupMrsPlusCreateCmd)
}
