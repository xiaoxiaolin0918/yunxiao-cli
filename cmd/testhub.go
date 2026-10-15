package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

var testhubCmd = &cobra.Command{
	Use:   "testhub",
	Short: "Testhub test plans, results, and repos",
	Long: `Testhub domain.

Typed:
  yunxiao testhub plans list|progress|directories
  yunxiao testhub directories list|create --repo-id <id>
  yunxiao testhub cases search|get|create|delete
  yunxiao testhub case-comments list|create
  yunxiao testhub results list|update
  yunxiao testhub plan-comments list|create
  yunxiao testhub repos list

Risk: list/get/search=read; create/update/comments=write; cases delete=high-risk-write`,
}

var testhubPlansCmd = &cobra.Command{Use: "plans", Short: "Test plans"}
var testhubReposCmd = &cobra.Command{Use: "repos", Short: "Test case repositories"}

var testhubPlansListCmd = &cobra.Command{
	Use:   "list",
	Short: "List test plans",
	Long:  "Risk: read\nHTTP: POST .../projex/.../testPlan/list",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		projectID, _ := cmd.Flags().GetString("project-id")
		sprintID, _ := cmd.Flags().GetString("sprint-id")
		name, _ := cmd.Flags().GetString("name")
		status, _ := cmd.Flags().GetString("status")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		// MCP uses projex testPlan/list (not testhub path) for listing plans
		path, err := c.ProjexPath(cmd.Context(), "/testPlan/list")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"page": page, "perPage": perPage}
		if projectID != "" {
			body["projectIdentifier"] = projectID
		}
		if sprintID != "" {
			body["sprintIdentifier"] = sprintID
		}
		if name != "" {
			body["name"] = name
		}
		if status != "" {
			body["status"] = status
		}
		handleErr(runRead(cmd.Context(), c, "POST", path, nil, body, map[string]any{"risk": risk.Read}, func(out any, meta map[string]any) (any, map[string]any) {
			out = zhiyi.AttachTestPlanIDsAndURLs(out)
			return out, meta
		}))
	},
}

var testhubReposListCmd = &cobra.Command{
	Use:   "list",
	Short: "List test case repositories",
	Long:  "Risk: read\nHTTP: GET .../testhub/.../testRepo/list",
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
		path, err := c.TesthubPath(cmd.Context(), "/testRepo/list")
		if err != nil {
			handleErr(err)
			return
		}
		q := client.PageQuery(page, perPage)
		if name != "" {
			q["name"] = name
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, func(out any, meta map[string]any) (any, map[string]any) {
			space := profileSpaceID()
			out = zhiyi.AttachTestRepoIDsAndURLs(out, space)
			zhiyi.EnrichTestRepoMeta(meta, space, "")
			return out, meta
		}))
	},
}

var testhubResultsCmd = &cobra.Command{Use: "results", Short: "Test plan results"}

var testhubPlansProgressCmd = &cobra.Command{
	Use:   "progress",
	Short: "Get test plan progress rate",
	Long:  "Risk: read\nHTTP: GET .../testhub/.../{planId}/progressRate",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		planID, _ := cmd.Flags().GetString("plan-id")
		if err := requireFlags("plan-id", planID); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.TesthubPath(cmd.Context(), "/"+planID+"/progressRate")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var testhubPlansDirectoriesCmd = &cobra.Command{
	Use:   "directories",
	Short: "List result directories of a test plan",
	Long:  "Risk: read\nHTTP: GET .../testhub/.../{planId}/result/directory/list",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		planID, _ := cmd.Flags().GetString("plan-id")
		if err := requireFlags("plan-id", planID); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.TesthubPath(cmd.Context(), "/"+planID+"/result/directory/list")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var testhubResultsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List test results in a plan directory",
	Long:  "Risk: read\nHTTP: POST .../testhub/.../{planId}/result/list/{directoryId}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		planID, _ := cmd.Flags().GetString("plan-id")
		dirID, _ := cmd.Flags().GetString("directory-id")
		if err := requireFlags("plan-id", planID, "directory-id", dirID); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.TesthubPath(cmd.Context(), "/"+planID+"/result/list/"+dirID)
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{}
		if globalDryRun {
			handleErr(output.DryRunResult(string(risk.Read), c.Preview("POST", path, nil, body)))
			return
		}
		var out any
		if err := c.Post(cmd.Context(), path, body, &out); err != nil {
			// fallback to projex path like MCP
			alt, aerr := c.ProjexPath(cmd.Context(), "/"+planID+"/result/list/"+dirID)
			if aerr == nil {
				if err2 := c.Post(cmd.Context(), alt, body, &out); err2 == nil {
					handleErr(output.Success(out, map[string]any{"risk": risk.Read, "via": "projex"}))
					return
				}
			}
			handleErr(err)
			return
		}
		handleErr(output.Success(out, map[string]any{"risk": risk.Read}))
	},
}

var testhubResultsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a testcase result in a plan (write)",
	Long:  "Risk: write\nHTTP: PUT .../testPlans/{plan}/testcases/{testcase}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		planID, _ := cmd.Flags().GetString("plan-id")
		caseID, _ := cmd.Flags().GetString("testcase-id")
		status, _ := cmd.Flags().GetString("status")
		executor, _ := cmd.Flags().GetString("executor")
		if err := requireFlags("plan-id", planID, "testcase-id", caseID); err != nil {
			handleErr(err)
			return
		}
		if status == "" && executor == "" {
			handleErr(fmt.Errorf("provide --status and/or --executor"))
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		if executor == "self" {
			executor, err = resolveSelfID(cmd.Context(), c, "self")
			if err != nil {
				handleErr(err)
				return
			}
		}
		path, err := c.TesthubPath(cmd.Context(), "/testPlans/"+planID+"/testcases/"+caseID)
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{}
		if status != "" {
			body["status"] = status
		}
		if executor != "" {
			body["executor"] = executor
		}
		handleErr(runJSONMutating(cmd.Context(), c, "testhub results update", risk.Write, "PUT", path, nil, body, nil))
	},
}

var testhubPlanCommentsCmd = &cobra.Command{Use: "plan-comments", Short: "Comments on a testcase in a test plan"}

var testhubPlanCommentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List comments on a plan testcase",
	Long:  "Risk: read\nHTTP: GET .../testPlans/{plan}/testcases/{id}/comments\n\nDefault order: newest first by create time. Use --sort asc for oldest first.",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		planID, _ := cmd.Flags().GetString("plan-id")
		caseID, _ := cmd.Flags().GetString("testcase-id")
		if err := requireFlags("plan-id", planID, "testcase-id", caseID); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.TesthubPath(cmd.Context(), "/testPlans/"+planID+"/testcases/"+caseID+"/comments")
		if err != nil {
			handleErr(err)
			return
		}
		sortFlag, _ := cmd.Flags().GetString("sort")
		after, err := afterSortByCreateTime(sortFlag, nil)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, after))
	},
}

var testhubPlanCommentsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a comment on a plan testcase (write)",
	Long:  "Risk: write\nHTTP: POST .../testPlans/{plan}/testcases/{id}/comments",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		planID, _ := cmd.Flags().GetString("plan-id")
		caseID, _ := cmd.Flags().GetString("testcase-id")
		content, _ := cmd.Flags().GetString("content")
		parentID, _ := cmd.Flags().GetString("parent-id")
		formatType, _ := cmd.Flags().GetString("format-type")
		if err := requireFlags("plan-id", planID, "testcase-id", caseID, "content", content); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.TesthubPath(cmd.Context(), "/testPlans/"+planID+"/testcases/"+caseID+"/comments")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"content": content}
		if parentID != "" {
			body["parentId"] = parentID
		}
		if formatType != "" {
			body["formatType"] = formatType
		}
		handleErr(runJSONMutating(cmd.Context(), c, "testhub plan-comments create", risk.Write, "POST", path, nil, body, nil))
	},
}

var testhubDirectoriesCmd = &cobra.Command{Use: "directories", Short: "Testcase directories in a repo"}
var testhubCasesCmd = &cobra.Command{Use: "cases", Short: "Testcases in a repo"}
var testhubCaseCommentsCmd = &cobra.Command{Use: "case-comments", Short: "Comments on a testcase (repo-level)"}

var testhubDirectoriesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List directories in a test repo",
	Long:  "Risk: read\nHTTP: GET .../testRepos/{repo}/directories\nSource: listDirectories",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repoID, _ := cmd.Flags().GetString("repo-id")
		if err := requireFlags("repo-id", repoID); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.TesthubPath(cmd.Context(), "/testRepos/"+repoID+"/directories")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, func(out any, meta map[string]any) (any, map[string]any) {
			space := profileSpaceID()
			out = zhiyi.AttachTestDirectoryURLs(out, space, repoID)
			zhiyi.EnrichTestRepoMeta(meta, space, repoID)
			return out, meta
		}))
	},
}

var testhubDirectoriesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a directory in a test repo (write)",
	Long:  "Risk: write\nHTTP: POST .../testRepos/{repo}/directories",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repoID, _ := cmd.Flags().GetString("repo-id")
		name, _ := cmd.Flags().GetString("name")
		parent, _ := cmd.Flags().GetString("parent-id")
		if err := requireFlags("repo-id", repoID, "name", name); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.TesthubPath(cmd.Context(), "/testRepos/"+repoID+"/directories")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"name": name}
		if parent != "" {
			body["parentIdentifier"] = parent
		}
		handleErr(runJSONMutating(cmd.Context(), c, "testhub directories create", risk.Write, "POST", path, nil, body, nil))
	},
}

var testhubCasesSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search testcases in a repo",
	Long:  "Risk: read\nHTTP: POST .../testRepos/{repo}/testcases:search",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repoID, _ := cmd.Flags().GetString("repo-id")
		dirID, _ := cmd.Flags().GetString("directory-id")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		if err := requireFlags("repo-id", repoID); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.TesthubPath(cmd.Context(), "/testRepos/"+repoID+"/testcases:search")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"page": page, "perPage": perPage}
		if dirID != "" {
			body["directoryId"] = dirID
		}
		handleErr(runRead(cmd.Context(), c, "POST", path, nil, body, map[string]any{"risk": risk.Read}, func(out any, meta map[string]any) (any, map[string]any) {
			space := profileSpaceID()
			out = zhiyi.AttachTestCaseURLs(out, space)
			zhiyi.EnrichTestRepoMeta(meta, space, repoID)
			return out, meta
		}))
	},
}

var testhubCasesGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a testcase",
	Long:  "Risk: read\nHTTP: GET .../testRepos/{repo}/testcases/{id}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repoID, _ := cmd.Flags().GetString("repo-id")
		id, _ := cmd.Flags().GetString("id")
		if err := requireFlags("repo-id", repoID, "id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.TesthubPath(cmd.Context(), "/testRepos/"+repoID+"/testcases/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, func(out any, meta map[string]any) (any, map[string]any) {
			space := profileSpaceID()
			if m, ok := out.(map[string]any); ok {
				if u := zhiyi.TestCaseURL(space, id); u != "" {
					m["url"] = u
				}
			}
			zhiyi.EnrichTestCaseMeta(meta, space, id)
			return out, meta
		}))
	},
}

var testhubCasesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a testcase (write)",
	Long: `Risk: write
HTTP: POST .../testRepos/{repo}/testcases
Or --data/--data-file JSON for full body.

Required in practice (#357): assignedTo (owner). Official docs mark it optional, but the API returns HTTP 400 when omitted. Pass --assigned-to <userId> or include assignedTo in --data.

Body tips (from live GetTestCase shapes):
  - testSteps: {"contentType":"TABLE","content":[{"step":"...","expected":"..."}]}
  - preCondition: prefer RICHTEXT (article wrapper) or TEXT + preConditionFormat

Success: platform create only returns {id}; CLI refreshes with GET to include customCode/subject when possible.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repoID, _ := cmd.Flags().GetString("repo-id")
		subject, _ := cmd.Flags().GetString("subject")
		dirID, _ := cmd.Flags().GetString("directory-id")
		assignedTo, _ := cmd.Flags().GetString("assigned-to")
		dataStr, _ := cmd.Flags().GetString("data")
		dataFile, _ := cmd.Flags().GetString("data-file")
		if err := requireFlags("repo-id", repoID); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.TesthubPath(cmd.Context(), "/testRepos/"+repoID+"/testcases")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{}
		extra, err := loadJSONBodyFromFlags(dataStr, dataFile)
		if err != nil {
			handleErr(err)
			return
		}
		if extra != nil {
			m := asStringMap(extra)
			if m == nil {
				handleErr(fmt.Errorf("--data/--data-file must be a JSON object"))
				return
			}
			for k, v := range m {
				body[k] = v
			}
		}
		if subject != "" {
			body["subject"] = subject
		}
		if dirID != "" {
			body["directoryId"] = dirID
		}
		if strings.TrimSpace(assignedTo) != "" {
			body["assignedTo"] = strings.TrimSpace(assignedTo)
		}
		if len(body) == 0 {
			handleErr(fmt.Errorf("provide --subject and/or --data/--data-file"))
			return
		}
		if !zhiyi.AssignedToPresent(body) {
			handleErr(fmt.Errorf("assignedTo is required (platform returns HTTP 400 owner empty); pass --assigned-to <userId> or include assignedTo in --data/--data-file"))
			return
		}
		after := func(out any, meta map[string]any) (any, map[string]any) {
			// #357: create API only returns {id}; refresh GET for customCode/subject.
			id := ""
			if m, ok := out.(map[string]any); ok {
				id = strings.TrimSpace(fmt.Sprint(m["id"]))
				if id == "<nil>" {
					id = ""
				}
			}
			if id == "" {
				return out, meta
			}
			getPath, gerr := c.TesthubPath(cmd.Context(), "/testRepos/"+repoID+"/testcases/"+id)
			if gerr != nil {
				meta["refresh_ok"] = false
				meta["refresh_error"] = gerr.Error()
				return out, meta
			}
			var full map[string]any
			if err := c.Get(cmd.Context(), getPath, nil, &full); err != nil {
				meta["refresh_ok"] = false
				meta["refresh_error"] = err.Error()
				return out, meta
			}
			meta["refresh_ok"] = true
			space := profileSpaceID()
			if u := zhiyi.TestCaseURL(space, id); u != "" {
				full["url"] = u
			}
			zhiyi.EnrichTestCaseMeta(meta, space, id)
			return full, meta
		}
		handleErr(runJSONMutating(cmd.Context(), c, "testhub cases create", risk.Write, "POST", path, nil, body, after))
	},
}

var testhubCasesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a testcase (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: DELETE .../testRepos/{repo}/testcases/{id}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repoID, _ := cmd.Flags().GetString("repo-id")
		id, _ := cmd.Flags().GetString("id")
		if err := requireFlags("repo-id", repoID, "id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.TesthubPath(cmd.Context(), "/testRepos/"+repoID+"/testcases/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runJSONMutating(cmd.Context(), c, "testhub cases delete", risk.HighRiskWrite, "DELETE", path, nil, nil, nil))
	},
}

var testhubCaseCommentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List comments on a repo testcase",
	Long:  "Risk: read\nHTTP: GET .../testRepos/{repo}/testcases/{id}/comments\n\nDefault order: newest first by create time. Use --sort asc for oldest first.",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repoID, _ := cmd.Flags().GetString("repo-id")
		id, _ := cmd.Flags().GetString("id")
		if err := requireFlags("repo-id", repoID, "id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.TesthubPath(cmd.Context(), "/testRepos/"+repoID+"/testcases/"+id+"/comments")
		if err != nil {
			handleErr(err)
			return
		}
		sortFlag, _ := cmd.Flags().GetString("sort")
		after, err := afterSortByCreateTime(sortFlag, nil)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, after))
	},
}

var testhubCaseCommentsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a comment on a repo testcase (write)",
	Long:  "Risk: write\nHTTP: POST .../testRepos/{repo}/testcases/{id}/comments",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repoID, _ := cmd.Flags().GetString("repo-id")
		id, _ := cmd.Flags().GetString("id")
		content, _ := cmd.Flags().GetString("content")
		if err := requireFlags("repo-id", repoID, "id", id, "content", content); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.TesthubPath(cmd.Context(), "/testRepos/"+repoID+"/testcases/"+id+"/comments")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"content": content}
		handleErr(runJSONMutating(cmd.Context(), c, "testhub case-comments create", risk.Write, "POST", path, nil, body, nil))
	},
}

func init() {
	testhubPlansListCmd.Flags().String("project-id", "", "project identifier filter")
	testhubPlansListCmd.Flags().String("sprint-id", "", "sprint identifier filter")
	testhubPlansListCmd.Flags().String("name", "", "name contains")
	testhubPlansListCmd.Flags().String("status", "", "TODO|DOING|DONE (comma-separated OK)")
	testhubPlansListCmd.Flags().Int("page", 1, "page")
	testhubPlansListCmd.Flags().Int("per-page", 100, "per page")
	testhubReposListCmd.Flags().String("name", "", "name filter")
	testhubReposListCmd.Flags().Int("page", 1, "page")
	testhubReposListCmd.Flags().Int("per-page", 100, "per page")
	testhubPlansProgressCmd.Flags().String("plan-id", "", "test plan id (required)")
	testhubPlansDirectoriesCmd.Flags().String("plan-id", "", "test plan id (required)")
	testhubResultsListCmd.Flags().String("plan-id", "", "test plan id (required)")
	testhubResultsListCmd.Flags().String("directory-id", "", "directory id (required)")
	testhubResultsUpdateCmd.Flags().String("plan-id", "", "test plan id (required)")
	testhubResultsUpdateCmd.Flags().String("testcase-id", "", "testcase id (required)")
	testhubResultsUpdateCmd.Flags().String("status", "", "result status")
	testhubResultsUpdateCmd.Flags().String("executor", "", "executor user id or self")
	testhubPlanCommentsListCmd.Flags().String("plan-id", "", "test plan id (required)")
	testhubPlanCommentsListCmd.Flags().String("testcase-id", "", "testcase id (required)")
	addSortFlag(testhubPlanCommentsListCmd)
	testhubPlanCommentsCreateCmd.Flags().String("plan-id", "", "test plan id (required)")
	testhubPlanCommentsCreateCmd.Flags().String("testcase-id", "", "testcase id (required)")
	testhubPlanCommentsCreateCmd.Flags().String("content", "", "comment content (required)")
	testhubPlanCommentsCreateCmd.Flags().String("parent-id", "", "parent comment id")
	testhubPlanCommentsCreateCmd.Flags().String("format-type", "", "RICHTEXT|MARKDOWN")
	testhubPlansCmd.AddCommand(testhubPlansListCmd, testhubPlansProgressCmd, testhubPlansDirectoriesCmd)
	testhubResultsCmd.AddCommand(testhubResultsListCmd, testhubResultsUpdateCmd)
	testhubPlanCommentsCmd.AddCommand(testhubPlanCommentsListCmd, testhubPlanCommentsCreateCmd)
	testhubDirectoriesListCmd.Flags().String("repo-id", "", "test repo id (required)")
	testhubDirectoriesCreateCmd.Flags().String("repo-id", "", "test repo id (required)")
	testhubDirectoriesCreateCmd.Flags().String("name", "", "directory name (required)")
	testhubDirectoriesCreateCmd.Flags().String("parent-id", "", "parent directory id")
	testhubCasesSearchCmd.Flags().String("repo-id", "", "test repo id (required)")
	testhubCasesSearchCmd.Flags().String("directory-id", "", "directory id filter")
	testhubCasesSearchCmd.Flags().Int("page", 1, "page")
	testhubCasesSearchCmd.Flags().Int("per-page", 20, "per page")
	testhubCasesGetCmd.Flags().String("repo-id", "", "test repo id (required)")
	testhubCasesGetCmd.Flags().String("id", "", "testcase id (required)")
	testhubCasesCreateCmd.Flags().String("repo-id", "", "test repo id (required)")
	testhubCasesCreateCmd.Flags().String("subject", "", "title")
	testhubCasesCreateCmd.Flags().String("directory-id", "", "directory id")
	testhubCasesCreateCmd.Flags().String("assigned-to", "", "owner user id (required in practice; 负责人)")
	testhubCasesCreateCmd.Flags().String("data", "", "full JSON body (or @file); must include assignedTo unless --assigned-to")
	testhubCasesCreateCmd.Flags().String("data-file", "", "read JSON body from file (alternative to --data)")
	testhubCasesDeleteCmd.Flags().String("repo-id", "", "test repo id (required)")
	testhubCasesDeleteCmd.Flags().String("id", "", "testcase id (required)")
	testhubCaseCommentsListCmd.Flags().String("repo-id", "", "test repo id (required)")
	testhubCaseCommentsListCmd.Flags().String("id", "", "testcase id (required)")
	addSortFlag(testhubCaseCommentsListCmd)
	testhubCaseCommentsCreateCmd.Flags().String("repo-id", "", "test repo id (required)")
	testhubCaseCommentsCreateCmd.Flags().String("id", "", "testcase id (required)")
	testhubCaseCommentsCreateCmd.Flags().String("content", "", "comment (required)")
	testhubDirectoriesCmd.AddCommand(testhubDirectoriesListCmd, testhubDirectoriesCreateCmd)
	testhubCasesCmd.AddCommand(testhubCasesSearchCmd, testhubCasesGetCmd, testhubCasesCreateCmd, testhubCasesDeleteCmd)
	testhubCaseCommentsCmd.AddCommand(testhubCaseCommentsListCmd, testhubCaseCommentsCreateCmd)
	testhubReposCmd.AddCommand(testhubReposListCmd)
	testhubCmd.AddCommand(testhubPlansCmd, testhubDirectoriesCmd, testhubCasesCmd, testhubCaseCommentsCmd, testhubResultsCmd, testhubPlanCommentsCmd, testhubReposCmd)
}
