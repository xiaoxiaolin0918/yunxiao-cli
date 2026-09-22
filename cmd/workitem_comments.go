package cmd

import (
	"strconv"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

var workitemCommentsCmd = &cobra.Command{
	Use:   "comments",
	Short: "Work item comments",
	Long: `List work item comments (create is: yunxiao workitem comment).

Personal-token OAPI (this CLI) documents only list + create for work item comments.
Delete/update are NOT available on OAPI: raw DELETE .../comments/{id} returns 404.
Aliyun OpenAPI RPC (AccessKey SDK) has DeleteWorkitemComment / UpdateWorkitemComment
(POST .../workitems/deleteComent and POST .../workitems/commentUpdate) — different
auth surface; not wrapped here.`,
}

var workitemCommentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List comments on a work item",
	Long:  "Risk: read\nHTTP: GET .../workitems/{id}/comments\n\nDefault order: newest first by create time (gmtCreate/createTime). Use --sort asc for oldest first.\n\nClient-side --sort applies within the current page when page/per-page are used.\n\nNote: OAPI has no comment delete/update; see yunxiao workitem comments --help.",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		sortFlag, _ := cmd.Flags().GetString("sort")
		if err := requireFlags("id", id); err != nil {
			handleErr(err)
			return
		}
		after, err := afterSortByCreateTime(sortFlag, nil)
		if err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.ProjexPath(cmd.Context(), "/workitems/"+id+"/comments")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"page": strconv.Itoa(page), "perPage": strconv.Itoa(perPage)}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, after))
	},
}

var workitemCommentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Add a comment to a work item",
	Long: `Risk: write
HTTP: POST .../workitems/{id}/comments
OpenAPI: CreateWorkitemComment (OAPI personal-token surface).

Prefer --content-file for UTF-8 text (BOM stripped) when Windows PowerShell mangles
Chinese in --content. Use only one of --content or --content-file.

OAPI does not document comment delete/update; do not expect workitem comments delete.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		contentFlag, _ := cmd.Flags().GetString("content")
		contentFile, _ := cmd.Flags().GetString("content-file")
		if err := requireFlags("id", id); err != nil {
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
		path, err := c.ProjexPath(cmd.Context(), "/workitems/"+id+"/comments")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"content": content}
		handleErr(runJSONMutating(cmd.Context(), c, "workitem comment", risk.Write, "POST", path, nil, body, nil))
	},
}
