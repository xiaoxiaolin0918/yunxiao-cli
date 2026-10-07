package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/workflow"
)

var workitemFieldsCmd = &cobra.Command{
	Use:   "fields",
	Short: "Get work item type field config",
	Long:  "Risk: read\nHTTP: GET .../projects/{space}/workitemTypes/{typeId}/fields\nSource: getWorkItemTypeFieldConfigFunc\n\nSee also: workitem statuses (status table), workitem types list (type discovery).",
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
	Long: `Risk: read
HTTP: GET .../projects/{space}/workitemTypes?category=...

Without --category (or with --category all) the CLI fetches every known Projex
category (Req, Bug, Task, Risk, Topic) concurrently and merges the results,
injecting each type's "category" into items that lack it (#99: Bug types used to
be invisible behind the old silent default --category Req). An explicit
--category <one> keeps the single-category query.

meta.categories lists the categories merged; meta.categories_failed carries
per-category errors (those categories are skipped with a stderr warning, the
rest still print). See also: workitem fields (field config), workitem statuses
(status table).`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		spaceID, _ := cmd.Flags().GetString("space-id")
		category, _ := cmd.Flags().GetString("category")
		if err := requireFlags("space-id", spaceID); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		if cat := strings.TrimSpace(category); cat != "" && !strings.EqualFold(cat, typesCategoryAll) {
			path, err := c.ProjexPath(cmd.Context(), "/projects/"+spaceID+"/workitemTypes")
			if err != nil {
				handleErr(err)
				return
			}
			q := map[string]string{"category": cat}
			handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
			return
		}
		handleErr(typesListAllCategories(cmd.Context(), c, spaceID))
	},
}

var workitemStatusesCmd = &cobra.Command{
	Use:   "statuses",
	Short: "List work item type statuses (from the type workflow)",
	Long: `Risk: read
HTTP: GET .../projects/{space}/workitemTypes/{typeId}/workflows (same confirmed
endpoint as workitem workflow; the statuses table is projected out of the payload)

data is the type's status list (id / name / displayName / nameEn as returned)
with a CLI-injected "default": true only on the defaultStatusId entry (any
same-named API field is overwritten, cf. mrs diffs "latest" #94);
meta.default_status_id (+ workflow_id / workflow_name when present). Use it to
configure profile workflows[<type_id>].statuses / bug_statuses aliases without
dropping to yunxiao api (#118).

See also: workitem fields (field config), workitem types list (type discovery),
workitem workflow (raw workflows payload).`,
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
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, projectStatusesFromWorkflow))
	},
}

// projectStatusesFromWorkflow is the workitem statuses after-hook (#118): parse
// the workflows payload and emit the status table with the default entry marked.
// When the payload cannot be parsed the raw response is returned with a stderr
// warning (explicit, never silent) — the API error paths stay in runRead.
func projectStatusesFromWorkflow(out any, meta map[string]any) (any, map[string]any) {
	wfID, wfName, defaultStatusID, statuses, err := workflow.ParseWorkflowResponse(out)
	if err != nil {
		fmt.Fprintf(output.Stderr, "warning: could not parse statuses from workflow payload (%v); returning raw payload\n", err)
		return out, meta
	}
	data := make([]map[string]any, 0, len(statuses))
	for _, s := range statuses {
		m := map[string]any{"id": s.ID, "default": s.ID != "" && s.ID == defaultStatusID}
		if s.Name != "" {
			m["name"] = s.Name
		}
		if s.DisplayName != "" {
			m["displayName"] = s.DisplayName
		}
		if s.NameEn != "" {
			m["nameEn"] = s.NameEn
		}
		data = append(data, m)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	meta["default_status_id"] = defaultStatusID
	if wfID != "" {
		meta["workflow_id"] = wfID
	}
	if wfName != "" {
		meta["workflow_name"] = wfName
	}
	return data, meta
}
