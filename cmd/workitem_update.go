package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

var workitemUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update work item fields (status, assignee, subject, …)",
	Long: `Risk: write
HTTP: PUT .../workitems/{id}

--labels replaces the entire label set (not append). Safe pattern: workitem get then merge ids then update --labels a,b,c. Discover ids: yunxiao project labels list (#141).

When changing status to cancelled (已取消 / Canceled / 141230), Yunxiao often requires
custom field 取消原因. Pass --cancel-reason <text> (looks up the field on the item's type)
or --custom-fields '{"<fieldId>":"…"}'.

  yunxiao workitem update --id <id> --status 141230 --cancel-reason "不再需要" --dry-run`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		subject, _ := cmd.Flags().GetString("subject")
		status, _ := cmd.Flags().GetString("status")
		assignedTo, _ := cmd.Flags().GetString("assigned-to")
		priority, _ := cmd.Flags().GetString("priority")
		description, _ := cmd.Flags().GetString("description")
		cancelReason, _ := cmd.Flags().GetString("cancel-reason")
		if err := requireFlags("id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		assignedTo, err = resolveSelfID(cmd.Context(), c, assignedTo)
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{}
		if subject != "" {
			body["subject"] = subject
		}
		if status != "" {
			body["status"] = status
		}
		if assignedTo != "" {
			body["assignedTo"] = assignedTo
		}
		if priority != "" {
			body["priority"] = priority
		}
		if description != "" {
			body["description"] = description
		}
		labels, _ := cmd.Flags().GetString("labels")
		sprint, _ := cmd.Flags().GetString("sprint")
		verifier, _ := cmd.Flags().GetString("verifier")
		participants, _ := cmd.Flags().GetString("participants")
		trackers, _ := cmd.Flags().GetString("trackers")
		versions, _ := cmd.Flags().GetString("versions")
		if labels != "" {
			body["labels"] = splitCSV(labels)
		}
		if sprint != "" {
			body["sprint"] = sprint
		}
		if verifier != "" {
			v, err := resolveSelfID(cmd.Context(), c, verifier)
			if err != nil {
				handleErr(err)
				return
			}
			body["verifier"] = v
		}
		if participants != "" {
			body["participants"] = splitCSV(participants)
		}
		if trackers != "" {
			body["trackers"] = splitCSV(trackers)
		}
		if versions != "" {
			body["versions"] = splitCSV(versions)
		}
		cfJSON, _ := cmd.Flags().GetString("custom-fields")
		if cfJSON != "" {
			cf, err := parseJSONMap(cfJSON)
			if err != nil {
				handleErr(err)
				return
			}
			// MCP merges custom field ids onto the request body for updates.
			for k, v := range cf {
				body[k] = v
			}
		}
		var item map[string]any
		needItem := strings.TrimSpace(cancelReason) != "" || zhiyi.LooksLikeCancelStatus(status)
		if needItem {
			item, err = fetchWorkItemMap(cmd.Context(), c, id)
			if err != nil {
				handleErr(fmt.Errorf("load workitem for cancel-reason/status check: %w", err))
				return
			}
			if _, err := applyCancelReasonFlag(cmd.Context(), c, item, body, cancelReason); err != nil {
				handleErr(err)
				return
			}
		}
		if len(body) == 0 {
			handleErr(fmt.Errorf("provide at least one field to update (--subject/--status/--custom-fields/--cancel-reason/…)"))
			return
		}
		warn := ""
		if item != nil {
			warn = softCancelReasonWarning(cmd.Context(), c, item, status, cancelReason)
		}
		path, err := c.ProjexPath(cmd.Context(), "/workitems/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		// Preserve cancel-reason soft-warn dry-run envelope (request + hint).
		if warn != "" && globalDryRun {
			handleErr(output.DryRunResult(string(risk.Write), map[string]any{
				"request": c.Preview("PUT", path, nil, body),
				"hint":    warn,
			}))
			return
		}
		handleErr(runJSONMutating(cmd.Context(), c, "workitem update", risk.Write, "PUT", path, nil, body, func(out any, meta map[string]any) (any, map[string]any) {
			if warn != "" {
				meta["hint"] = warn
			}
			zhiyi.EnrichWorkItemMeta(meta, asStringMap(out), profileSpaceID(), "")
			return out, meta
		}))
	},
}

var workitemDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a work item (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: DELETE .../workitems/{id}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		if err := requireFlags("id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.ProjexPath(cmd.Context(), "/workitems/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runJSONMutating(cmd.Context(), c, "workitem delete", risk.HighRiskWrite, "DELETE", path, nil, nil, nil))
	},
}
