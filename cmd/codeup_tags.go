package cmd

import (
	"strconv"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

var codeupTagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Repository tags (Codeup)",
	Long: `Codeup tags.

  yunxiao codeup tags list --repo <id|alias>
  yunxiao codeup tags create --repo <id|alias> --tag-name v1.0 --ref master [--message …] --dry-run
  yunxiao codeup tags delete --repo <id|alias> --tag-name v1.0 --yes

--repo accepts numeric id or profile.repositories alias.
Risk: list=read; create/delete=high-risk-write.`,
}

var codeupTagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List repository tags",
	Long:  "Risk: read\nHTTP: GET .../repositories/{repo}/tags\nSource: ListTags",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		search, _ := cmd.Flags().GetString("search")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		sort, _ := cmd.Flags().GetString("sort")
		orderBy, _ := cmd.Flags().GetString("order-by")
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
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/tags")
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
		if sort != "" {
			q["sort"] = sort
		}
		if orderBy != "" {
			q["orderBy"] = orderBy
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read, "repository_id": repositoryID}, nil))
	},
}

var codeupTagsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a tag (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: POST .../repositories/{repo}/tags?tagName=&ref=&message=\nSource: CreateTag (query params)",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		tagName, _ := cmd.Flags().GetString("tag-name")
		ref, _ := cmd.Flags().GetString("ref")
		message, _ := cmd.Flags().GetString("message")
		if err := requireFlags("repo", repo, "tag-name", tagName, "ref", ref); err != nil {
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
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/tags")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"tagName": tagName, "ref": ref}
		if message != "" {
			q["message"] = message
		}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup tags create", risk.HighRiskWrite, "POST", path, q, nil, func(out any, meta map[string]any) (any, map[string]any) {
			meta["repository_id"] = repositoryID
			return out, meta
		}))
	},
}

var codeupTagsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a tag (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: DELETE .../repositories/{repo}/tags/{tagName}\nSource: DeleteTag",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		tagName, _ := cmd.Flags().GetString("tag-name")
		if err := requireFlags("repo", repo, "tag-name", tagName); err != nil {
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
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/tags/"+client.EncodeFilePath(tagName))
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup tags delete", risk.HighRiskWrite, "DELETE", path, nil, nil, func(out any, meta map[string]any) (any, map[string]any) {
			meta["repository_id"] = repositoryID
			return out, meta
		}))
	},
}

func init() {
	codeupTagsListCmd.Flags().String("repo", "", "repository id, profile alias, org/repo path, or bare name (required)")
	codeupTagsListCmd.Flags().String("search", "", "search keyword")
	codeupTagsListCmd.Flags().Int("page", 1, "page")
	codeupTagsListCmd.Flags().Int("per-page", 20, "per page")
	codeupTagsListCmd.Flags().String("sort", "", "desc|asc")
	codeupTagsListCmd.Flags().String("order-by", "", "name|create")
	codeupTagsCreateCmd.Flags().String("repo", "", "repository id, profile alias, org/repo path, or bare name (required)")
	codeupTagsCreateCmd.Flags().String("tag-name", "", "tag name (required)")
	codeupTagsCreateCmd.Flags().String("ref", "", "branch, tag, or commit SHA (required)")
	codeupTagsCreateCmd.Flags().String("message", "", "optional tag message")
	codeupTagsDeleteCmd.Flags().String("repo", "", "repository id, profile alias, org/repo path, or bare name (required)")
	codeupTagsDeleteCmd.Flags().String("tag-name", "", "tag name (required)")
	codeupTagsCmd.AddCommand(codeupTagsListCmd, codeupTagsCreateCmd, codeupTagsDeleteCmd)
	codeupCmd.AddCommand(codeupTagsCmd)
}
