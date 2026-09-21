package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
)

var codeupMrsLinkCmd = &cobra.Command{
	Use:   "link [work-item...]",
	Short: "Link work items to an existing merge request (write)",
	Long: `Risk: write
HTTP: POST .../workitems/{id}/extRelationRecords (category=codeupMergeRequest)

Links one or more work items to an existing Codeup MR via Projex extRelationRecords
(not UpdateChangeRequest). Idempotent: already-linked items are skipped.

Prefer --dry-run first. Work item refs may be serials (ZYPT-…) or internal ids;
each is resolved via workitem get before linking.

  yunxiao codeup mrs link --repo <alias|id> --local-id 125 --work-item ZYPT-5573 --dry-run
  yunxiao codeup mrs link --repo <alias|id> --local-id 125 ZYPT-5573 ZYPT-5574`,
	Run: func(cmd *cobra.Command, args []string) {
		runMrsWorkItemLinkUnlink(cmd, args, false)
	},
}

var codeupMrsUnlinkCmd = &cobra.Command{
	Use:   "unlink [work-item...]",
	Short: "Unlink work items from an existing merge request (write)",
	Long: `Risk: write
HTTP: DELETE .../workitems/{id}/extRelationRecords/{relationRecordId}

Removes codeupMergeRequest extRelationRecords matching this MR. Idempotent:
already-unlinked items are skipped.

Prefer --dry-run first.

  yunxiao codeup mrs unlink --repo <alias|id> --local-id 125 --work-item ZYPT-5573 --dry-run`,
	Run: func(cmd *cobra.Command, args []string) {
		runMrsWorkItemLinkUnlink(cmd, args, true)
	},
}

func runMrsWorkItemLinkUnlink(cmd *cobra.Command, args []string, unlink bool) {
	flagOrg(globalOrg)
	repo, _ := cmd.Flags().GetString("repo")
	localID, _ := cmd.Flags().GetString("local-id")
	workItemCSV, _ := cmd.Flags().GetString("work-item")
	if err := requireFlags("repo", repo, "local-id", localID); err != nil {
		handleErr(err)
		return
	}
	refs := collectWorkItemRefs(workItemCSV, args)
	if len(refs) == 0 {
		handleErr(fmt.Errorf("provide --work-item and/or work item args"))
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
	resolved, err := resolveWorkItemsForMR(cmd.Context(), c, refs, "")
	if err != nil {
		handleErr(err)
		return
	}
	if unlink {
		handleErr(unlinkMRWorkItems(cmd.Context(), c, repositoryID, localID, resolved, globalDryRun))
		return
	}
	handleErr(linkMRWorkItems(cmd.Context(), c, repositoryID, localID, resolved, globalDryRun))
}

// runMrsUpdateWorkItemLinks is shared by mrs update --work-item (additive link).
func runMrsUpdateWorkItemLinks(cmd *cobra.Command, c *client.Client, repositoryID, localID string, refs []string) error {
	resolved, err := resolveWorkItemsForMR(cmd.Context(), c, refs, "")
	if err != nil {
		return err
	}
	return linkMRWorkItems(cmd.Context(), c, repositoryID, localID, resolved, globalDryRun)
}
