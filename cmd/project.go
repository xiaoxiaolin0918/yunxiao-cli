package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

var projectCmd = &cobra.Command{
	Use:     "project",
	Aliases: []string{"projex"},
	Short:   "Projex projects and work-item shortcuts",
	Long: `Project / Projex domain.

+shortcuts:
  yunxiao project +my-open-items [--category Req|Task|Bug|Risk] [--space-id <id>]
  yunxiao project +created-by-me
  yunxiao workitem +bug-transition --id ZYPT-xxxx --to processing --dry-run  (needs --profile)

Typed (also under workitem):
  yunxiao project list
  yunxiao project labels list|create
  yunxiao workitem search|get|comment

Risk: read (writes under workitem; +bug-transition=write + --yes)`,
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List / search projects",
	Long:  "Risk: read\nHTTP: POST .../projects:search",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		name, _ := cmd.Flags().GetString("name")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.ProjexPath(cmd.Context(), "/projects:search")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{}
		if name != "" {
			conds := map[string]any{
				"conditionGroups": []any{
					[]any{
						map[string]any{
							"className":       "string",
							"fieldIdentifier": "name",
							"format":          "input",
							"operator":        "CONTAINS",
							"value":           []string{name},
						},
					},
				},
			}
			b, _ := json.Marshal(conds)
			body["conditions"] = string(b)
		}
		if page > 0 {
			body["page"] = page
		}
		if perPage > 0 {
			body["perPage"] = perPage
		}
		handleErr(runRead(cmd.Context(), c, "POST", path, nil, body, map[string]any{"risk": risk.Read}, nil))
	},
}

var projectMyOpenItemsCmd = &cobra.Command{
	Use:   "+my-open-items",
	Short: "Shortcut: list my open work items (assigned to self, not finished)",
	Long: `Risk: read

Uses workitems:search with assignedTo=current user and statusStage=1,2
(pending + in-progress). Override with --status-stage.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		category, _ := cmd.Flags().GetString("category")
		spaceID, _ := cmd.Flags().GetString("space-id")
		spaceID, err := resolveSpaceIDFlag(spaceID)
		if err != nil {
			handleErr(err)
			return
		}
		statusStage, _ := cmd.Flags().GetString("status-stage")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		var user map[string]any
		if err := c.Get(cmd.Context(), "/oapi/v1/platform/user", nil, &user); err != nil {
			handleErr(err)
			return
		}
		uid, _ := user["id"].(string)
		path, err := c.ProjexPath(cmd.Context(), "/workitems:search")
		if err != nil {
			handleErr(err)
			return
		}
		conds := map[string]any{
			"conditionGroups": []any{
				[]any{
					map[string]any{
						"className":       "user",
						"fieldIdentifier": "assignedTo",
						"format":          "list",
						"operator":        "CONTAINS",
						"value":           []string{uid},
					},
					map[string]any{
						"className":       "statusStage",
						"fieldIdentifier": "statusStage",
						"format":          "list",
						"operator":        "CONTAINS",
						"value":           splitCSV(statusStage),
					},
				},
			},
		}
		cb, _ := json.Marshal(conds)
		body := map[string]any{
			"category":   category,
			"conditions": string(cb),
			"orderBy":    "gmtCreate",
			"sort":       "desc",
			"page":       page,
			"perPage":    perPage,
		}
		body["spaceId"] = spaceID
		handleErr(runRead(cmd.Context(), c, "POST", path, nil, body, map[string]any{"risk": risk.Read, "assigned_to": uid}, nil))
	},
}

var projectCreatedByMeCmd = &cobra.Command{
	Use:   "+created-by-me",
	Short: "Shortcut: work items I created (optionally open only)",
	Long:  "Risk: read",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		category, _ := cmd.Flags().GetString("category")
		spaceID, _ := cmd.Flags().GetString("space-id")
		spaceID, err := resolveSpaceIDFlag(spaceID)
		if err != nil {
			handleErr(err)
			return
		}
		statusStage, _ := cmd.Flags().GetString("status-stage")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		var user map[string]any
		if err := c.Get(cmd.Context(), "/oapi/v1/platform/user", nil, &user); err != nil {
			handleErr(err)
			return
		}
		uid, _ := user["id"].(string)
		path, err := c.ProjexPath(cmd.Context(), "/workitems:search")
		if err != nil {
			handleErr(err)
			return
		}
		filters := []any{
			map[string]any{
				"className": "user", "fieldIdentifier": "creator", "format": "list",
				"operator": "CONTAINS", "value": []string{uid},
			},
		}
		if statusStage != "" {
			filters = append(filters, map[string]any{
				"className": "statusStage", "fieldIdentifier": "statusStage", "format": "list",
				"operator": "CONTAINS", "value": splitCSV(statusStage),
			})
		}
		conds := map[string]any{"conditionGroups": []any{filters}}
		cb, _ := json.Marshal(conds)
		body := map[string]any{
			"category": category, "conditions": string(cb),
			"orderBy": "gmtCreate", "sort": "desc", "page": page, "perPage": perPage,
		}
		body["spaceId"] = spaceID
		handleErr(runRead(cmd.Context(), c, "POST", path, nil, body, map[string]any{"risk": risk.Read, "creator": uid}, nil))
	},
}

func splitCSV(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		if r == ' ' && cur == "" {
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

var projectGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a project by id",
	Long:  "Risk: read\nHTTP: GET .../projex/.../projects/{id}\nSource: operations/projex/project.ts getProjectFunc",
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
		path, err := c.ProjexPath(cmd.Context(), "/projects/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

func init() {
	projectGetCmd.Flags().String("id", "", "project id (required)")
	projectListCmd.Flags().String("name", "", "filter by project name")
	projectListCmd.Flags().Int("page", 1, "page number")
	projectListCmd.Flags().Int("per-page", 20, "page size")
	projectMyOpenItemsCmd.Flags().String("category", "Req", "work item category: Req|Task|Bug|Risk|…")
	projectMyOpenItemsCmd.Flags().String("space-id", "", "project/space id (default: profile.space_id)")
	projectMyOpenItemsCmd.Flags().String("status-stage", "1,2", "status stage IDs (default open: 1,2)")
	projectMyOpenItemsCmd.Flags().Int("page", 1, "page")
	projectMyOpenItemsCmd.Flags().Int("per-page", 20, "per page")
	projectCreatedByMeCmd.Flags().String("category", "Req", "work item category")
	projectCreatedByMeCmd.Flags().String("space-id", "", "project/space id (default: profile.space_id)")
	projectCreatedByMeCmd.Flags().String("status-stage", "", "optional status stage filter (e.g. 1,2)")
	projectCreatedByMeCmd.Flags().Int("page", 1, "page")
	projectCreatedByMeCmd.Flags().Int("per-page", 20, "per page")
	projectCmd.AddCommand(projectListCmd, projectGetCmd, projectMyOpenItemsCmd, projectCreatedByMeCmd)
}
