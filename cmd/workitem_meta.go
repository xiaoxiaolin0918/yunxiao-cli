package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

var workitemFieldsCmd = &cobra.Command{
	Use:   "fields",
	Short: "Get work item type field config",
	Long:  "Risk: read\nHTTP: GET .../projects/{space}/workitemTypes/{typeId}/fields\nSource: getWorkItemTypeFieldConfigFunc",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		spaceID, _ := cmd.Flags().GetString("space-id")
		typeID, _ := cmd.Flags().GetString("type-id")
		if err := requireFlags("space-id", spaceID, "type-id", typeID); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := workitemFieldsPath(cmd.Context(), c, spaceID, typeID)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var workitemWorkflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Get work item type workflow",
	Long:  "Risk: read\nHTTP: GET .../projects/{space}/workitemTypes/{typeId}/workflows\nSource: getWorkItemWorkflowFunc",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		spaceID, _ := cmd.Flags().GetString("space-id")
		typeID, _ := cmd.Flags().GetString("type-id")
		if err := requireFlags("space-id", spaceID, "type-id", typeID); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.ProjexPath(cmd.Context(), "/projects/"+spaceID+"/workitemTypes/"+typeID+"/workflows")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var workitemActivitiesCmd = &cobra.Command{
	Use:   "activities",
	Short: "List work item activities",
	Long:  "Risk: read\nHTTP: GET .../workitems/{id}/activities\nSource: listWorkitemActivitiesFunc\n\nDefault order: newest first by update/create time. Use --sort asc for oldest first.",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		sortFlag, _ := cmd.Flags().GetString("sort")
		if err := requireFlags("id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.ProjexPath(cmd.Context(), "/workitems/"+id+"/activities")
		if err != nil {
			handleErr(err)
			return
		}
		after, err := afterSortByTime(sortFlag, nil)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, after))
	},
}

var workitemTypesCmd = &cobra.Command{Use: "types", Short: "Work item types"}

var workitemTypesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List work item types for a project",
	Long:  "Risk: read\nHTTP: GET .../projects/{space}/workitemTypes?category=...",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		spaceID, _ := cmd.Flags().GetString("space-id")
		category, _ := cmd.Flags().GetString("category")
		if err := requireFlags("space-id", spaceID, "category", category); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.ProjexPath(cmd.Context(), "/projects/"+spaceID+"/workitemTypes")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"category": category}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
	},
}
