package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/mrlink"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

var codeupCmd = &cobra.Command{
	Use:   "codeup",
	Short: "Codeup repositories, branches, and merge requests",
	Long: `Codeup domain.

+shortcuts:
  yunxiao codeup +open-mrs [--repo <id|alias>]
  yunxiao codeup mrs +create --repo <alias|id> --source <br> --title "…" [--work-item ZYPT-…] [--wip] [--reviewer <ids>]

Typed:
  yunxiao codeup repos list|get --repo <id|alias>
  yunxiao codeup branches list|get|create|delete --repo <id|alias>
  yunxiao codeup tags list|create|delete --repo <id|alias>
  yunxiao codeup protected-branches list|get|create|delete --repo <id|alias>
  yunxiao codeup files tree|get|create|update|delete --repo <id|alias>
  yunxiao codeup commits list --repo <id|alias> --ref <branch>
  yunxiao codeup mrs list|get|update|link|unlink|diffs|comments|labels|reviewers|create|merge|close|review|reopen
  yunxiao codeup compare --repo <id|alias> --from <ref> --to <ref>

--repo accepts numeric id, profile.repositories alias, or org/repo path.

Create MR / mrs +create / tags create|delete / protected-branches create|delete are high-risk-write: use --dry-run, then --yes after user confirms.`,
}

var codeupReposCmd = &cobra.Command{Use: "repos", Short: "Repositories"}
var codeupBranchesCmd = &cobra.Command{Use: "branches", Short: "Branches"}
var codeupMrsCmd = &cobra.Command{Use: "mrs", Short: "Merge requests (changeRequests)"}

var codeupReposListCmd = &cobra.Command{
	Use:   "list",
	Short: "List repositories",
	Long:  "Risk: read\nHTTP: GET .../repositories",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		search, _ := cmd.Flags().GetString("search")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.CodeupPath(cmd.Context(), "/repositories")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{
			"page":    strconv.Itoa(page),
			"perPage": strconv.Itoa(perPage),
		}
		if search != "" {
			q["search"] = search
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var codeupBranchesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List branches of a repository",
	Long:  "Risk: read\nHTTP: GET .../repositories/{repo}/branches",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		search, _ := cmd.Flags().GetString("search")
		page, _ := cmd.Flags().GetInt("page")
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/branches")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"page": strconv.Itoa(page), "perPage": strconv.Itoa(perPage)}
		if search != "" {
			q["search"] = search
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var codeupMrsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List merge requests",
	Long: `Risk: read
HTTP: GET .../changeRequests

WARNING (server-ignored params): Codeup list_change_requests may silently ignore
repositoryId and status. This command sends projectIds (via --repo) and lowercase
state (via --state). Do not rely on repositoryId/status filters on the raw API.

Use --all to follow pages via client.ListAll (cap 50).
Default order: newest first by update/create time. Client-side --sort applies within
the current page (or across collected pages with --all). Use --sort asc for oldest first.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		state, _ := cmd.Flags().GetString("state")
		search, _ := cmd.Flags().GetString("search")
		projectIDs, _ := cmd.Flags().GetString("repo")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		allPages, _ := cmd.Flags().GetBool("all")
		sortFlag, _ := cmd.Flags().GetString("sort")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.CodeupPath(cmd.Context(), "/changeRequests")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"page": strconv.Itoa(page), "perPage": strconv.Itoa(perPage)}
		if state != "" {
			q["state"] = state
		}
		if search != "" {
			q["search"] = search
		}
		if projectIDs != "" {
			resolved, err := resolveCodeupRepo(projectIDs)
			if err != nil {
				handleErr(err)
				return
			}
			q["projectIds"] = resolved
		}
		after, err := afterSortByTime(sortFlag, func(out any, meta map[string]any) (any, map[string]any) {
			return zhiyi.AttachMergeRequestURLs(out), meta
		})
		if err != nil {
			handleErr(err)
			return
		}
		if allPages {
			handleErr(runReadAll(cmd.Context(), c, "GET", path, q, perPage, map[string]any{"risk": risk.Read}, after))
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, after))
	},
}
var codeupMrsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a merge request (high-risk-write)",
	Long: `Risk: high-risk-write

Requires --yes after explicit user confirmation. Prefer --dry-run first.

Success prints a brief summary (localId/title/status/url); pass --full for the raw MR object.

HTTP: POST .../repositories/{repo}/changeRequests

--reviewer accepts comma-separated userIds (OpenAPI reviewerUserIds), same as mrs +create.
--work-item accepts comma-separated ZYPT serials or internal ids; each is GETed before
create (abort if missing). Body workItemIds is a comma-separated string (OpenAPI). After create the CLI
verifies via workitem extRelationRecords, attempts repair if needed, and fails
(ok=false) if links remain missing.

  yunxiao codeup mrs create --repo <id> --source feat/x --target master \
    --title "feat: x" --reviewer <userId1,userId2> --dry-run`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		source, _ := cmd.Flags().GetString("source")
		target, _ := cmd.Flags().GetString("target")
		title, _ := cmd.Flags().GetString("title")
		desc, _ := cmd.Flags().GetString("description")
		reviewer, _ := cmd.Flags().GetString("reviewer")
		workItemCSV, _ := cmd.Flags().GetString("work-item")
		sourceProjectID, _ := cmd.Flags().GetString("source-project-id")
		targetProjectID, _ := cmd.Flags().GetString("target-project-id")
		createFrom, _ := cmd.Flags().GetString("create-from")
		if err := requireFlags("repo", repo, "source", source, "target", target, "title", title); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		// Resolve numeric project ids when repo is numeric.
		if sourceProjectID == "" {
			sourceProjectID = numericOrEmpty(repositoryID)
		}
		if targetProjectID == "" {
			targetProjectID = numericOrEmpty(repositoryID)
		}
		if sourceProjectID == "" || targetProjectID == "" {
			// fetch repository to get id
			rpath, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID)
			if err != nil {
				handleErr(err)
				return
			}
			var repoObj map[string]any
			if err := c.Get(cmd.Context(), rpath, nil, &repoObj); err != nil {
				handleErr(fmt.Errorf("need --source-project-id/--target-project-id or numeric --repo: %w", err))
				return
			}
			idStr := fmt.Sprintf("%v", repoObj["id"])
			if sourceProjectID == "" {
				sourceProjectID = idStr
			}
			if targetProjectID == "" {
				targetProjectID = idStr
			}
		}
		var resolvedWorkItems []mrlink.ResolvedWorkItem
		if refs := splitWorkItemRefs(workItemCSV); len(refs) > 0 {
			items, err := resolveWorkItemsForMR(cmd.Context(), c, refs, "")
			if err != nil {
				handleErr(err)
				return
			}
			resolvedWorkItems = items
		}
		workItemIDs := mrlink.InternalIDs(resolvedWorkItems)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{
			"title":           title,
			"sourceBranch":    source,
			"targetBranch":    target,
			"sourceProjectId": sourceProjectID,
			"targetProjectId": targetProjectID,
			"createFrom":      createFrom,
		}
		if desc != "" {
			body["description"] = desc
		}
		if ids := zhiyi.SplitUserIDs(reviewer); len(ids) > 0 {
			body["reviewerUserIds"] = ids
		}
		if csv := mrlink.WorkItemIDsCSV(workItemIDs); csv != "" {
			body["workItemIds"] = csv
		}
		if globalDryRun {
			handleErr(output.DryRunResult(string(risk.HighRiskWrite), c.Preview("POST", path, nil, body)))
			return
		}
		if err := risk.CheckHighRisk("codeup mrs create", globalYes); err != nil {
			handleErr(err)
			return
		}
		var out any
		if err := c.Post(cmd.Context(), path, body, &out); err != nil {
			handleErr(withWriteDedupeHint(err, mrsListSearchHint(repo, title)))
			return
		}
		meta := map[string]any{"risk": risk.HighRiskWrite}
		mrMap := zhiyi.StabilizeMergeRequest(zhiyi.UnwrapMergeRequestPayload(asStringMap(out)))
		zhiyi.EnrichMergeRequestMeta(meta, mrMap)
		if err := ensureMRWorkItemLinks(cmd.Context(), c, repositoryID, resolvedWorkItems, mrMap, meta); err != nil {
			handleErr(err)
			return
		}
		full, _ := cmd.Flags().GetBool("full")
		if full {
			handleErr(output.Success(mrMap, meta))
			return
		}
		handleErr(output.Success(zhiyi.BriefMergeRequest(mrMap), meta))
	},
}

var codeupOpenMrsShortcut = &cobra.Command{
	Use:   "+open-mrs",
	Short: "Shortcut: list opened merge requests",
	Long:  "Risk: read",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		search, _ := cmd.Flags().GetString("search")
		projectIDs, _ := cmd.Flags().GetString("repo")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.CodeupPath(cmd.Context(), "/changeRequests")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"page": strconv.Itoa(page), "perPage": strconv.Itoa(perPage), "state": "opened"}
		if search != "" {
			q["search"] = search
		}
		if projectIDs != "" {
			resolved, err := resolveCodeupRepo(projectIDs)
			if err != nil {
				handleErr(err)
				return
			}
			q["projectIds"] = resolved
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, func(out any, meta map[string]any) (any, map[string]any) {
			return zhiyi.AttachMergeRequestURLs(out), meta
		}))
	},
}

var codeupReposGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a repository",
	Long:  "Risk: read\nHTTP: GET .../codeup/.../repositories/{id}\nSource: getRepositoryFunc",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
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
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+client.EncodeRepoID(repositoryID))
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var codeupBranchesGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a branch",
	Long:  "Risk: read\nHTTP: GET .../repositories/{repo}/branches/{branch}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		branch, _ := cmd.Flags().GetString("branch")
		if err := requireFlags("repo", repo, "branch", branch); err != nil {
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
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+client.EncodeRepoID(repositoryID)+"/branches/"+client.EncodeFilePath(branch))
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var codeupBranchesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a branch (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: POST .../repositories/{repo}/branches?branch=&ref=\nSource: createBranchFunc (query params, not body)",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		branch, _ := cmd.Flags().GetString("branch")
		ref, _ := cmd.Flags().GetString("ref")
		if err := requireFlags("repo", repo, "branch", branch, "ref", ref); err != nil {
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
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+client.EncodeRepoID(repositoryID)+"/branches")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"branch": branch, "ref": ref}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup branches create", risk.HighRiskWrite, "POST", path, q, nil, nil))
	},
}

var codeupBranchesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a branch (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: DELETE .../repositories/{repo}/branches/{branch}\nSource: deleteBranchFunc",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		branch, _ := cmd.Flags().GetString("branch")
		if err := requireFlags("repo", repo, "branch", branch); err != nil {
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
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+client.EncodeRepoID(repositoryID)+"/branches/"+client.EncodeFilePath(branch))
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup branches delete", risk.HighRiskWrite, "DELETE", path, nil, nil, nil))
	},
}

var codeupFilesCmd = &cobra.Command{Use: "files", Short: "Repository files"}
var codeupCommitsCmd = &cobra.Command{Use: "commits", Short: "Repository commits"}

var codeupFilesTreeCmd = &cobra.Command{
	Use:   "tree",
	Short: "List repository file tree",
	Long:  "Risk: read\nHTTP: GET .../repositories/{repo}/files/tree",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		pathArg, _ := cmd.Flags().GetString("path")
		ref, _ := cmd.Flags().GetString("ref")
		typ, _ := cmd.Flags().GetString("type")
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/files/tree")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{}
		if pathArg != "" {
			q["path"] = pathArg
		}
		if ref != "" {
			q["ref"] = ref
		}
		if typ != "" {
			q["type"] = typ
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var codeupFilesGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get file blob content",
	Long:  "Risk: read\nHTTP: GET .../repositories/{repo}/files/{path}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		filePath, _ := cmd.Flags().GetString("path")
		ref, _ := cmd.Flags().GetString("ref")
		if err := requireFlags("repo", repo, "path", filePath, "ref", ref); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		encPath := client.EncodeFilePath(filePath)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/files/"+encPath)
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"ref": ref}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var codeupFilesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a file in a repository (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: POST .../repositories/{repo}/files",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		filePath, _ := cmd.Flags().GetString("path")
		branch, _ := cmd.Flags().GetString("branch")
		msg, _ := cmd.Flags().GetString("message")
		encoding, _ := cmd.Flags().GetString("encoding")
		contentFlag, _ := cmd.Flags().GetString("content")
		contentFile, _ := cmd.Flags().GetString("content-file")
		if err := requireFlags("repo", repo, "path", filePath, "branch", branch, "message", msg); err != nil {
			handleErr(err)
			return
		}
		repositoryID, err := resolveCodeupRepo(repo)
		if err != nil {
			handleErr(err)
			return
		}
		content, err := readContentInput(contentFlag, contentFile)
		if err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/files")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{
			"branch":        branch,
			"filePath":      filePath,
			"content":       content,
			"commitMessage": msg,
			"encoding":      encoding,
		}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup files create", risk.HighRiskWrite, "POST", path, nil, body, nil))
	},
}

var codeupFilesUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a file in a repository (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: PUT .../repositories/{repo}/files/{path}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		filePath, _ := cmd.Flags().GetString("path")
		branch, _ := cmd.Flags().GetString("branch")
		msg, _ := cmd.Flags().GetString("message")
		encoding, _ := cmd.Flags().GetString("encoding")
		contentFlag, _ := cmd.Flags().GetString("content")
		contentFile, _ := cmd.Flags().GetString("content-file")
		if err := requireFlags("repo", repo, "path", filePath, "branch", branch, "message", msg); err != nil {
			handleErr(err)
			return
		}
		repositoryID, err := resolveCodeupRepo(repo)
		if err != nil {
			handleErr(err)
			return
		}
		content, err := readContentInput(contentFlag, contentFile)
		if err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		repoID := client.EncodeRepoID(repositoryID)
		encPath := client.EncodeFilePath(filePath)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/files/"+encPath)
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{
			"branch":        branch,
			"content":       content,
			"commitMessage": msg,
			"encoding":      encoding,
		}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup files update", risk.HighRiskWrite, "PUT", path, nil, body, nil))
	},
}

var codeupFilesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a file in a repository (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: DELETE .../repositories/{repo}/files/{path}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		filePath, _ := cmd.Flags().GetString("path")
		branch, _ := cmd.Flags().GetString("branch")
		msg, _ := cmd.Flags().GetString("message")
		if err := requireFlags("repo", repo, "path", filePath, "branch", branch, "message", msg); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		encPath := client.EncodeFilePath(filePath)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/files/"+encPath)
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"branch": branch, "commitMessage": msg}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup files delete", risk.HighRiskWrite, "DELETE", path, q, nil, nil))
	},
}

var codeupCommitsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List commits on a ref",
	Long:  "Risk: read\nHTTP: GET .../repositories/{repo}/commits\n\nDefault order: newest first by commit time. Client-side --sort applies within the current page. Use --sort asc for oldest first.",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		ref, _ := cmd.Flags().GetString("ref")
		search, _ := cmd.Flags().GetString("search")
		pathArg, _ := cmd.Flags().GetString("path")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		sortFlag, _ := cmd.Flags().GetString("sort")
		if err := requireFlags("repo", repo, "ref", ref); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/commits")
		if err != nil {
			handleErr(err)
			return
		}
		q := client.PageQuery(page, perPage)
		q["refName"] = ref
		if search != "" {
			q["search"] = search
		}
		if pathArg != "" {
			q["path"] = pathArg
		}
		after, err := afterSortByTime(sortFlag, nil)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, after))
	},
}

var codeupMrsMergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge a merge request (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: POST .../changeRequests/{localId}/merge",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		localID, _ := cmd.Flags().GetString("local-id")
		mergeType, _ := cmd.Flags().GetString("merge-type")
		mergeMessage, _ := cmd.Flags().GetString("message")
		removeSource, _ := cmd.Flags().GetBool("remove-source-branch")
		if err := requireFlags("repo", repo, "local-id", localID, "merge-type", mergeType); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests/"+localID+"/merge")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"mergeType": mergeType}
		if mergeMessage != "" {
			body["mergeMessage"] = mergeMessage
		}
		if removeSource {
			body["removeSourceBranch"] = true
		}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup mrs merge", risk.HighRiskWrite, "POST", path, nil, body, nil))
	},
}

var codeupMrsCloseCmd = &cobra.Command{
	Use:   "close",
	Short: "Close a merge request (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: POST .../changeRequests/{localId}/close",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		localID, _ := cmd.Flags().GetString("local-id")
		if err := requireFlags("repo", repo, "local-id", localID); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests/"+localID+"/close")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup mrs close", risk.HighRiskWrite, "POST", path, nil, nil, nil))
	},
}

var codeupMrsReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Submit an MR review opinion (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: POST .../changeRequests/{localId}/review\nOpinion: PASS | NOT_PASS",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		localID, _ := cmd.Flags().GetString("local-id")
		opinion, _ := cmd.Flags().GetString("opinion")
		comment, _ := cmd.Flags().GetString("comment")
		if err := requireFlags("repo", repo, "local-id", localID, "opinion", opinion); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests/"+localID+"/review")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"reviewOpinion": opinion}
		if comment != "" {
			body["reviewComment"] = comment
		}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup mrs review", risk.HighRiskWrite, "POST", path, nil, body, nil))
	},
}

var codeupMrsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a merge request by local id",
	Long:  "Risk: read\nHTTP: GET .../changeRequests/{localId}\n\nNormalizes OpenAPI status into state (alias) for scripts. Use --brief for localId/title/status/url only.",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		localID, _ := cmd.Flags().GetString("local-id")
		if err := requireFlags("repo", repo, "local-id", localID); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests/"+localID)
		if err != nil {
			handleErr(err)
			return
		}
		brief, _ := cmd.Flags().GetBool("brief")
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, func(out any, meta map[string]any) (any, map[string]any) {
			m := zhiyi.StabilizeMergeRequest(zhiyi.UnwrapMergeRequestPayload(asStringMap(out)))
			zhiyi.EnrichMergeRequestMeta(meta, m)
			if brief {
				return zhiyi.BriefMergeRequest(m), meta
			}
			return m, meta
		}))
	},
}

var codeupMrsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update merge request title/description and/or link work items (write)",
	Long: `Risk: write
HTTP: PUT .../changeRequests/{localId} (title/description);
      POST .../workitems/{id}/extRelationRecords when --work-item is set

Prefer --dry-run first; real writes run only when not dry-run (Write risk).
At least one of --title / --description / --work-item required.
--work-item is additive (same as mrs link); does not use UpdateChangeRequest for links.

  yunxiao codeup mrs update --repo <alias|id> --local-id 125 --title "WIP: docs" --dry-run
  yunxiao codeup mrs update --repo <alias|id> --local-id 125 --work-item ZYPT-5573 --dry-run
  yunxiao codeup mrs update --repo <alias|id> --local-id 125 --title "WIP: docs" --work-item ZYPT-5573`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		localID, _ := cmd.Flags().GetString("local-id")
		title, _ := cmd.Flags().GetString("title")
		desc, _ := cmd.Flags().GetString("description")
		workItemCSV, _ := cmd.Flags().GetString("work-item")
		if err := requireFlags("repo", repo, "local-id", localID); err != nil {
			handleErr(err)
			return
		}
		title = strings.TrimSpace(title)
		desc = strings.TrimSpace(desc)
		refs := collectWorkItemRefs(workItemCSV, nil)
		if title == "" && desc == "" && len(refs) == 0 {
			handleErr(fmt.Errorf("provide --title and/or --description and/or --work-item"))
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
		full, _ := cmd.Flags().GetBool("full")
		hasTitleDesc := title != "" || desc != ""

		// Work-item only: same as mrs link (incl. dry-run preview of extRelationRecords POST).
		if !hasTitleDesc {
			handleErr(runMrsUpdateWorkItemLinks(cmd, c, repositoryID, localID, refs))
			return
		}

		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests/"+localID)
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{}
		if title != "" {
			body["title"] = title
		}
		if desc != "" {
			body["description"] = desc
		}

		// Title/description only (or dry-run of PUT when combined): use shared mutating helper.
		if len(refs) == 0 || globalDryRun {
			handleErr(runJSONMutating(cmd.Context(), c, "codeup mrs update", risk.Write, "PUT", path, nil, body, func(out any, meta map[string]any) (any, map[string]any) {
				m := zhiyi.StabilizeMergeRequest(zhiyi.UnwrapMergeRequestPayload(asStringMap(out)))
				zhiyi.EnrichMergeRequestMeta(meta, m)
				if full {
					return m, meta
				}
				return zhiyi.BriefMergeRequest(m), meta
			}))
			return
		}

		// Combined title/description + work-item (real write): one envelope after PUT + link.
		var out any
		if _, err := c.Do(cmd.Context(), "PUT", path, nil, body, &out); err != nil {
			handleErr(err)
			return
		}
		m := zhiyi.StabilizeMergeRequest(zhiyi.UnwrapMergeRequestPayload(asStringMap(out)))
		meta := map[string]any{"risk": risk.Write}
		zhiyi.EnrichMergeRequestMeta(meta, m)
		resolved, resErr := resolveWorkItemsForMR(cmd.Context(), c, refs, "")
		if resErr != nil {
			handleErr(resErr)
			return
		}
		linkOut, linkMeta, linkErr := applyMRWorkItemLinks(cmd.Context(), c, repositoryID, localID, resolved, false)
		if linkMeta != nil {
			for k, v := range linkMeta {
				if k == "risk" {
					continue
				}
				meta[k] = v
			}
		}
		if linkOut != nil {
			meta["work_item_link_result"] = linkOut
		}
		var data any = m
		if !full {
			data = zhiyi.BriefMergeRequest(m)
		}
		if linkErr != nil {
			_ = output.Success(data, meta)
			handleErr(linkErr)
			return
		}
		handleErr(output.Success(data, meta))
	},
}

var codeupMrsDiffsCmd = &cobra.Command{
	Use:   "diffs",
	Short: "List MR patch sets (diff versions)",
	Long:  "Risk: read\nHTTP: GET .../changeRequests/{localId}/diffs/patches",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		localID, _ := cmd.Flags().GetString("local-id")
		if err := requireFlags("repo", repo, "local-id", localID); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests/"+localID+"/diffs/patches")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var codeupMrsReopenCmd = &cobra.Command{
	Use:   "reopen",
	Short: "Reopen a closed merge request (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: POST .../changeRequests/{localId}/reopen",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		localID, _ := cmd.Flags().GetString("local-id")
		if err := requireFlags("repo", repo, "local-id", localID); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests/"+localID+"/reopen")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup mrs reopen", risk.HighRiskWrite, "POST", path, nil, nil, nil))
	},
}

var codeupCompareCmd = &cobra.Command{
	Use:   "compare",
	Short: "Compare two refs (branches/tags/commits)",
	Long:  "Risk: read\nHTTP: GET .../repositories/{repo}/compares",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		sourceType, _ := cmd.Flags().GetString("source-type")
		targetType, _ := cmd.Flags().GetString("target-type")
		straight, _ := cmd.Flags().GetString("straight")
		if err := requireFlags("repo", repo, "from", from, "to", to); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/compares")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"from": from, "to": to}
		if sourceType != "" {
			q["sourceType"] = sourceType
		}
		if targetType != "" {
			q["targetType"] = targetType
		}
		if straight != "" {
			q["straight"] = straight
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var codeupMrsCommentsCmd = &cobra.Command{Use: "comments", Short: "MR comments"}

var codeupMrsCommentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List MR comments",
	Long:  "Risk: read\nHTTP: POST .../changeRequests/{localId}/comments/list\n\nDefault order: newest first by create time. Use --sort asc for oldest first.",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		localID, _ := cmd.Flags().GetString("local-id")
		commentType, _ := cmd.Flags().GetString("comment-type")
		state, _ := cmd.Flags().GetString("state")
		resolved, _ := cmd.Flags().GetBool("resolved")
		filePath, _ := cmd.Flags().GetString("file-path")
		patchSets, _ := cmd.Flags().GetString("patchset-biz-ids")
		sortFlag, _ := cmd.Flags().GetString("sort")
		if err := requireFlags("repo", repo, "local-id", localID); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests/"+localID+"/comments/list")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{
			"commentType":    commentType,
			"state":          state,
			"resolved":       resolved,
			"patchSetBizIds": []string{},
		}
		if patchSets != "" {
			body["patchSetBizIds"] = splitCSV(patchSets)
		}
		if filePath != "" {
			body["filePath"] = filePath
		}
		after, err := afterSortByCreateTime(sortFlag, nil)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "POST", path, nil, body, map[string]any{"risk": risk.Read}, after))
	},
}

var codeupMrsCommentsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an MR comment (write)",
	Long: `Risk: write

HTTP: POST .../changeRequests/{localId}/comments
GLOBAL_COMMENT needs --content and --patchset-biz-id (from mrs diffs).
INLINE_COMMENT also needs --file-path --line-number --from-patchset-biz-id --to-patchset-biz-id.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		localID, _ := cmd.Flags().GetString("local-id")
		content, _ := cmd.Flags().GetString("content")
		commentType, _ := cmd.Flags().GetString("comment-type")
		patchset, _ := cmd.Flags().GetString("patchset-biz-id")
		draft, _ := cmd.Flags().GetBool("draft")
		resolved, _ := cmd.Flags().GetBool("resolved")
		filePath, _ := cmd.Flags().GetString("file-path")
		lineNumber, _ := cmd.Flags().GetInt("line-number")
		fromPS, _ := cmd.Flags().GetString("from-patchset-biz-id")
		toPS, _ := cmd.Flags().GetString("to-patchset-biz-id")
		parent, _ := cmd.Flags().GetString("parent-comment-biz-id")
		if err := requireFlags("repo", repo, "local-id", localID, "content", content, "patchset-biz-id", patchset); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests/"+localID+"/comments")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{
			"comment_type":    commentType,
			"content":         content,
			"draft":           draft,
			"resolved":        resolved,
			"patchset_biz_id": patchset,
		}
		if commentType == "INLINE_COMMENT" {
			if filePath == "" || lineNumber <= 0 || fromPS == "" || toPS == "" {
				handleErr(fmt.Errorf("INLINE_COMMENT requires --file-path --line-number --from-patchset-biz-id --to-patchset-biz-id"))
				return
			}
			body["file_path"] = filePath
			body["line_number"] = lineNumber
			body["from_patchset_biz_id"] = fromPS
			body["to_patchset_biz_id"] = toPS
		}
		if parent != "" {
			body["parent_comment_biz_id"] = parent
		}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup mrs comments create", risk.Write, "POST", path, nil, body, nil))
	},
}

var codeupMrsLabelsCmd = &cobra.Command{Use: "labels", Short: "MR labels (list/attach; no detach OpenAPI)"}

var codeupMrsLabelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List labels on an MR",
	Long:  "Risk: read\nHTTP: GET .../changeRequests/{localId}/labels",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		localID, _ := cmd.Flags().GetString("local-id")
		if err := requireFlags("repo", repo, "local-id", localID); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests/"+localID+"/labels")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var codeupMrsLabelsAttachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Attach labels to an MR (write)",
	Long:  "Risk: write\nHTTP: POST .../changeRequests/{localId}/labels\nBody: {\"label_id_list\":[...]}\nNote: no DetachLabels OpenAPI (MCP/docs only Get+Attach); detach skipped.",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		localID, _ := cmd.Flags().GetString("local-id")
		labelIDs, _ := cmd.Flags().GetString("label-ids")
		if err := requireFlags("repo", repo, "local-id", localID, "label-ids", labelIDs); err != nil {
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests/"+localID+"/labels")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"label_id_list": splitCSV(labelIDs)}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup mrs labels attach", risk.Write, "POST", path, nil, body, nil))
	},
}

var codeupMrsReviewersCmd = &cobra.Command{Use: "reviewers", Short: "MR reviewers (add via person/REVIEWER)"}

var codeupMrsReviewersAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add/update reviewers on an existing MR (write)",
	Long: `Risk: write
HTTP: POST .../changeRequests/{localId}/person/REVIEWER
Body: {"userIds":[...]}

OpenAPI UpdateChangeRequestRelatedPerson with type=REVIEWER.
--reviewer accepts comma-separated userIds (same CSV as mrs create; body field is userIds, not reviewerUserIds).

Example:
  yunxiao codeup mrs reviewers add --repo <id> --local-id 1 --reviewer <userId1,userId2> --dry-run`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		localID, _ := cmd.Flags().GetString("local-id")
		reviewer, _ := cmd.Flags().GetString("reviewer")
		if err := requireFlags("repo", repo, "local-id", localID, "reviewer", reviewer); err != nil {
			handleErr(err)
			return
		}
		ids := zhiyi.SplitUserIDs(reviewer)
		if len(ids) == 0 {
			handleErr(fmt.Errorf("--reviewer requires at least one userId"))
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
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/changeRequests/"+localID+"/person/REVIEWER")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"userIds": ids}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup mrs reviewers add", risk.Write, "POST", path, nil, body, nil))
	},
}

func numericOrEmpty(s string) string {
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		return s
	}
	return ""
}

func init() {
	codeupReposListCmd.Flags().String("search", "", "search keyword")
	codeupReposListCmd.Flags().Int("page", 1, "page")
	codeupReposListCmd.Flags().Int("per-page", 20, "per page")
	codeupBranchesListCmd.Flags().String("repo", "", "repository id, alias, or org/repo path (required)")
	codeupBranchesListCmd.Flags().String("search", "", "branch name search")
	codeupBranchesListCmd.Flags().Int("page", 1, "page")
	codeupBranchesListCmd.Flags().Int("per-page", 20, "per page")
	codeupMrsListCmd.Flags().String("state", "", "opened|merged|closed")
	codeupMrsListCmd.Flags().String("search", "", "title search")
	codeupMrsListCmd.Flags().String("repo", "", "filter by repository id or alias (projectIds)")
	codeupMrsListCmd.Flags().Int("page", 1, "page")
	codeupMrsListCmd.Flags().Int("per-page", 20, "per page")
	codeupMrsListCmd.Flags().Bool("all", false, "follow all pages (ListAll, max 50)")
	addSortFlag(codeupMrsListCmd)
	codeupMrsCreateCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsCreateCmd.Flags().String("source", "", "source branch (required)")
	codeupMrsCreateCmd.Flags().String("target", "", "target branch (required)")
	codeupMrsCreateCmd.Flags().String("title", "", "MR title (required)")
	codeupMrsCreateCmd.Flags().String("description", "", "MR description")
	codeupMrsCreateCmd.Flags().String("source-project-id", "", "numeric source project id")
	codeupMrsCreateCmd.Flags().String("target-project-id", "", "numeric target project id")
	codeupMrsCreateCmd.Flags().String("create-from", "WEB", "createFrom, default WEB")
	codeupMrsCreateCmd.Flags().String("reviewer", "", "optional reviewer userId(s), comma-separated (OpenAPI reviewerUserIds; same as mrs +create)")
	codeupMrsCreateCmd.Flags().String("work-item", "", "optional work item serial(s) or id(s), comma-separated; prechecked via workitem get")
	codeupOpenMrsShortcut.Flags().String("state", "opened", "state")
	codeupMrsCreateCmd.Flags().Bool("full", false, "print full MR JSON (default: brief localId/title/status/url)")
	codeupOpenMrsShortcut.Flags().String("search", "", "title search")
	codeupOpenMrsShortcut.Flags().String("repo", "", "filter by repository id or alias")
	codeupOpenMrsShortcut.Flags().Int("page", 1, "page")
	codeupOpenMrsShortcut.Flags().Int("per-page", 20, "per page")

	codeupFilesTreeCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupFilesTreeCmd.Flags().String("path", "", "directory path")
	codeupFilesTreeCmd.Flags().String("ref", "", "branch/tag/commit")
	codeupFilesTreeCmd.Flags().String("type", "DIRECT", "DIRECT|RECURSIVE|FLATTEN")
	codeupFilesGetCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupFilesGetCmd.Flags().String("path", "", "file path (required)")
	codeupFilesGetCmd.Flags().String("ref", "", "branch/tag/commit (required)")
	codeupCommitsListCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupCommitsListCmd.Flags().String("ref", "", "branch/tag/commit (required)")
	codeupCommitsListCmd.Flags().String("search", "", "search keyword")
	codeupCommitsListCmd.Flags().String("path", "", "file path filter")
	codeupCommitsListCmd.Flags().Int("page", 1, "page")
	codeupCommitsListCmd.Flags().Int("per-page", 20, "per page")
	addSortFlag(codeupCommitsListCmd)

	codeupReposCmd.AddCommand(codeupReposListCmd, codeupReposGetCmd)
	codeupReposGetCmd.Flags().String("repo", "", "repository id, alias, or path (required)")
	codeupBranchesGetCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupBranchesGetCmd.Flags().String("branch", "", "branch name (required)")
	codeupBranchesCreateCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupBranchesCreateCmd.Flags().String("branch", "", "new branch name (required)")
	codeupBranchesCreateCmd.Flags().String("ref", "", "source ref/commit (required)")
	codeupBranchesDeleteCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupBranchesDeleteCmd.Flags().String("branch", "", "branch name (required)")
	codeupBranchesCmd.AddCommand(codeupBranchesListCmd, codeupBranchesGetCmd, codeupBranchesCreateCmd, codeupBranchesDeleteCmd)
	codeupFilesCreateCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupFilesCreateCmd.Flags().String("path", "", "file path (required)")
	codeupFilesCreateCmd.Flags().String("branch", "", "branch (required)")
	codeupFilesCreateCmd.Flags().String("message", "", "commit message (required)")
	codeupFilesCreateCmd.Flags().String("content", "", "file content")
	codeupFilesCreateCmd.Flags().String("content-file", "", "path to content file (cwd-relative or absolute)")
	codeupFilesCreateCmd.Flags().String("encoding", "text", "text|base64")
	codeupFilesUpdateCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupFilesUpdateCmd.Flags().String("path", "", "file path (required)")
	codeupFilesUpdateCmd.Flags().String("branch", "", "branch (required)")
	codeupFilesUpdateCmd.Flags().String("message", "", "commit message (required)")
	codeupFilesUpdateCmd.Flags().String("content", "", "file content")
	codeupFilesUpdateCmd.Flags().String("content-file", "", "path to content file (cwd-relative or absolute)")
	codeupFilesUpdateCmd.Flags().String("encoding", "text", "text|base64")
	codeupFilesDeleteCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupFilesDeleteCmd.Flags().String("path", "", "file path (required)")
	codeupFilesDeleteCmd.Flags().String("branch", "", "branch (required)")
	codeupFilesDeleteCmd.Flags().String("message", "", "commit message (required)")
	codeupMrsMergeCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsMergeCmd.Flags().String("local-id", "", "MR local id (required)")
	codeupMrsMergeCmd.Flags().String("merge-type", "", "ff-only|no-fast-forward|squash|rebase (required)")
	codeupMrsMergeCmd.Flags().String("message", "", "merge commit message")
	codeupMrsMergeCmd.Flags().Bool("remove-source-branch", false, "delete source branch after merge")
	codeupMrsCloseCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsCloseCmd.Flags().String("local-id", "", "MR local id (required)")
	codeupMrsReviewCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsReviewCmd.Flags().String("local-id", "", "MR local id (required)")
	codeupMrsReviewCmd.Flags().String("opinion", "", "PASS|NOT_PASS (required)")
	codeupMrsReviewCmd.Flags().String("comment", "", "review comment")
	codeupFilesCmd.AddCommand(codeupFilesTreeCmd, codeupFilesGetCmd, codeupFilesCreateCmd, codeupFilesUpdateCmd, codeupFilesDeleteCmd)
	codeupCommitsCmd.AddCommand(codeupCommitsListCmd)
	codeupMrsGetCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsGetCmd.Flags().String("local-id", "", "MR local id (required)")
	codeupMrsGetCmd.Flags().Bool("brief", false, "only localId/title/status/state/detailUrl/url")
	codeupMrsUpdateCmd.Flags().String("repo", "", "repository id, alias, or org/repo path")
	codeupMrsUpdateCmd.Flags().String("local-id", "", "MR local id")
	codeupMrsUpdateCmd.Flags().String("title", "", "new title")
	codeupMrsUpdateCmd.Flags().String("description", "", "new description")
	codeupMrsUpdateCmd.Flags().String("work-item", "", "work item serial(s) or id(s), comma-separated; additive link via extRelationRecords")
	codeupMrsUpdateCmd.Flags().Bool("full", false, "print full MR object instead of brief summary")
	codeupMrsLinkCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsLinkCmd.Flags().String("local-id", "", "MR local id (required)")
	codeupMrsLinkCmd.Flags().String("work-item", "", "work item serial(s) or id(s), comma-separated")
	codeupMrsUnlinkCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsUnlinkCmd.Flags().String("local-id", "", "MR local id (required)")
	codeupMrsUnlinkCmd.Flags().String("work-item", "", "work item serial(s) or id(s), comma-separated")
	codeupMrsDiffsCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsDiffsCmd.Flags().String("local-id", "", "MR local id (required)")
	codeupMrsReopenCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsReopenCmd.Flags().String("local-id", "", "MR local id (required)")
	codeupCompareCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupCompareCmd.Flags().String("from", "", "from ref (required)")
	codeupCompareCmd.Flags().String("to", "", "to ref (required)")
	codeupCompareCmd.Flags().String("source-type", "", "branch|tag")
	codeupCompareCmd.Flags().String("target-type", "", "branch|tag")
	codeupCompareCmd.Flags().String("straight", "", "straight compare flag")
	codeupMrsCommentsListCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsCommentsListCmd.Flags().String("local-id", "", "MR local id (required)")
	codeupMrsCommentsListCmd.Flags().String("comment-type", "GLOBAL_COMMENT", "GLOBAL_COMMENT|INLINE_COMMENT")
	codeupMrsCommentsListCmd.Flags().String("state", "OPENED", "OPENED|DRAFT")
	codeupMrsCommentsListCmd.Flags().Bool("resolved", false, "filter resolved")
	codeupMrsCommentsListCmd.Flags().String("file-path", "", "filter by file path")
	codeupMrsCommentsListCmd.Flags().String("patchset-biz-ids", "", "comma-separated patchset biz ids")
	addSortFlag(codeupMrsCommentsListCmd)
	codeupMrsCommentsCreateCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsCommentsCreateCmd.Flags().String("local-id", "", "MR local id (required)")
	codeupMrsCommentsCreateCmd.Flags().String("content", "", "comment content (required)")
	codeupMrsCommentsCreateCmd.Flags().String("comment-type", "GLOBAL_COMMENT", "GLOBAL_COMMENT|INLINE_COMMENT")
	codeupMrsCommentsCreateCmd.Flags().String("patchset-biz-id", "", "patchset biz id (required)")
	codeupMrsCommentsCreateCmd.Flags().Bool("draft", false, "create as draft")
	codeupMrsCommentsCreateCmd.Flags().Bool("resolved", false, "mark resolved")
	codeupMrsCommentsCreateCmd.Flags().String("file-path", "", "inline: file path")
	codeupMrsCommentsCreateCmd.Flags().Int("line-number", 0, "inline: line number")
	codeupMrsCommentsCreateCmd.Flags().String("from-patchset-biz-id", "", "inline: from patchset")
	codeupMrsCommentsCreateCmd.Flags().String("to-patchset-biz-id", "", "inline: to patchset")
	codeupMrsCommentsCreateCmd.Flags().String("parent-comment-biz-id", "", "reply to comment")
	codeupMrsLabelsListCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsLabelsListCmd.Flags().String("local-id", "", "MR local id (required)")
	codeupMrsLabelsAttachCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsLabelsAttachCmd.Flags().String("local-id", "", "MR local id (required)")
	codeupMrsLabelsAttachCmd.Flags().String("label-ids", "", "comma-separated label ids (required)")
	codeupMrsReviewersAddCmd.Flags().String("repo", "", "repository id or alias (required)")
	codeupMrsReviewersAddCmd.Flags().String("local-id", "", "MR local id (required)")
	codeupMrsReviewersAddCmd.Flags().String("reviewer", "", "reviewer userId(s), comma-separated (OpenAPI person/REVIEWER body userIds)")
	codeupMrsCommentsCmd.AddCommand(codeupMrsCommentsListCmd, codeupMrsCommentsCreateCmd)
	codeupMrsLabelsCmd.AddCommand(codeupMrsLabelsListCmd, codeupMrsLabelsAttachCmd)
	codeupMrsReviewersCmd.AddCommand(codeupMrsReviewersAddCmd)
	codeupMrsCmd.AddCommand(codeupMrsListCmd, codeupMrsGetCmd, codeupMrsUpdateCmd, codeupMrsLinkCmd, codeupMrsUnlinkCmd, codeupMrsDiffsCmd, codeupMrsCommentsCmd, codeupMrsLabelsCmd, codeupMrsReviewersCmd, codeupMrsCreateCmd, codeupMrsMergeCmd, codeupMrsCloseCmd, codeupMrsReviewCmd, codeupMrsReopenCmd)
	codeupCmd.AddCommand(codeupReposCmd, codeupBranchesCmd, codeupFilesCmd, codeupCommitsCmd, codeupCompareCmd, codeupMrsCmd, codeupOpenMrsShortcut)
}
