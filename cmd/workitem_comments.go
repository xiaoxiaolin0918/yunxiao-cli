package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/orguid"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

var workitemCommentsCmd = &cobra.Command{
	Use:   "comments",
	Short: "Work item comments",
	Long: `List / delete / update work item comments.

Personal-token OAPI (this CLI): list + create only
  (create: yunxiao workitem comment). Raw DELETE .../comments/{id} → 404.

Delete / update use Aliyun OpenAPI RPC with AccessKey ACS3 signing
(same ALIBABA_CLOUD_ACCESS_KEY_* path as organization members --include-aliyun-uid):
  - delete: POST .../workitems/deleteComent (official typo; DeleteWorkitemComment)
  - update: POST .../workitems/commentUpdate (UpdateWorkitemComment)`,
}

var workitemCommentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List comments on a work item",
	Long:  "Risk: read\nHTTP: GET .../workitems/{id}/comments (OAPI personal-token)\n\nDefault order: newest first by create time (gmtCreate/createTime). Use --sort asc for oldest first.\n\nClient-side --sort applies within the current page when page/per-page are used.\n\nDelete/update are AccessKey RPC: see yunxiao workitem comments delete|update --help.",
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

var workitemCommentsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a work item comment (AccessKey RPC)",
	Long: `Risk: high-risk-write (requires --yes for real run; prefer --dry-run first)
Auth: Alibaba Cloud AccessKey (ACS3) — NOT personal-token OAPI.
HTTP: POST https://devops.{region}.aliyuncs.com/organization/{org}/workitems/deleteComent
OpenAPI: DeleteWorkitemComment (path spelling deleteComent is official).

Requires ALIBABA_CLOUD_ACCESS_KEY_ID / ALIBABA_CLOUD_ACCESS_KEY_SECRET
(same as organization members --include-aliyun-uid). Organization id comes from
YUNXIAO_ORGANIZATION_ID / profile (via personal-token client ResolveOrgID).

--id accepts work item identifier or serial (serial resolved via OAPI GET when needed).`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		commentIDStr, _ := cmd.Flags().GetString("comment-id")
		if err := requireFlags("id", id, "comment-id", commentIDStr); err != nil {
			handleErr(err)
			return
		}
		commentID, err := orguid.ParseCommentID(commentIDStr)
		if err != nil {
			handleErr(err)
			return
		}
		ak, ok := orguid.LoadAKEnv()
		if !ok {
			handleErr(fmt.Errorf("%s", orguid.MissingAKMessageComments()))
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		orgID, err := c.ResolveOrgID(cmd.Context())
		if err != nil {
			handleErr(err)
			return
		}
		identifier, err := resolveWorkitemIdentifierForRPC(cmd.Context(), c, id)
		if err != nil {
			handleErr(err)
			return
		}
		cli := &orguid.DevOpsMembersClient{AK: ak}
		preview := cli.PreviewDeleteWorkitemComment(orgID, identifier, commentID)
		handleErr(runMutating("workitem comments delete", risk.HighRiskWrite, globalDryRun, globalYes, preview, func() error {
			out, err := cli.DeleteWorkitemComment(cmd.Context(), orgID, identifier, commentID)
			if err != nil {
				return err
			}
			return output.Success(out, map[string]any{
				"risk":   risk.HighRiskWrite,
				"auth":   "alibaba_cloud_access_key",
				"action": "DeleteWorkitemComment",
				"url":    preview["url"],
			})
		}))
	},
}

var workitemCommentsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a work item comment (AccessKey RPC)",
	Long: `Risk: write
Auth: Alibaba Cloud AccessKey (ACS3) — NOT personal-token OAPI.
HTTP: POST https://devops.{region}.aliyuncs.com/organization/{org}/workitems/commentUpdate
OpenAPI: UpdateWorkitemComment

Requires ALIBABA_CLOUD_ACCESS_KEY_ID / ALIBABA_CLOUD_ACCESS_KEY_SECRET.
Use --content or --content-file (UTF-8, BOM stripped). Default --format-type MARKDOWN.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		commentIDStr, _ := cmd.Flags().GetString("comment-id")
		contentFlag, _ := cmd.Flags().GetString("content")
		contentFile, _ := cmd.Flags().GetString("content-file")
		formatType, _ := cmd.Flags().GetString("format-type")
		if err := requireFlags("id", id, "comment-id", commentIDStr); err != nil {
			handleErr(err)
			return
		}
		commentID, err := orguid.ParseCommentID(commentIDStr)
		if err != nil {
			handleErr(err)
			return
		}
		content, err := readContentInput(contentFlag, contentFile)
		if err != nil {
			handleErr(err)
			return
		}
		ak, ok := orguid.LoadAKEnv()
		if !ok {
			handleErr(fmt.Errorf("%s", orguid.MissingAKMessageComments()))
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		orgID, err := c.ResolveOrgID(cmd.Context())
		if err != nil {
			handleErr(err)
			return
		}
		identifier, err := resolveWorkitemIdentifierForRPC(cmd.Context(), c, id)
		if err != nil {
			handleErr(err)
			return
		}
		in := orguid.UpdateWorkitemCommentInput{
			Content:            content,
			FormatType:         formatType,
			WorkitemIdentifier: identifier,
			CommentID:          commentID,
		}
		cli := &orguid.DevOpsMembersClient{AK: ak}
		preview := cli.PreviewUpdateWorkitemComment(orgID, in)
		handleErr(runMutating("workitem comments update", risk.Write, globalDryRun, globalYes, preview, func() error {
			out, err := cli.UpdateWorkitemComment(cmd.Context(), orgID, in)
			if err != nil {
				return err
			}
			return output.Success(out, map[string]any{
				"risk":   risk.Write,
				"auth":   "alibaba_cloud_access_key",
				"action": "UpdateWorkitemComment",
				"url":    preview["url"],
			})
		}))
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

OAPI has no comment delete/update. Use:
  yunxiao workitem comments delete --id ... --comment-id ... --yes
  yunxiao workitem comments update --id ... --comment-id ... --content|--content-file
(AccessKey RPC; see those --help texts).`,
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

// looksLikeWorkitemSerial reports PREFIX-digits serials (e.g. ZYPT-5768).
func looksLikeWorkitemSerial(id string) bool {
	id = strings.TrimSpace(id)
	i := strings.LastIndex(id, "-")
	if i <= 0 || i == len(id)-1 {
		return false
	}
	prefix, num := id[:i], id[i+1:]
	if num == "" {
		return false
	}
	for _, r := range num {
		if r < '0' || r > '9' {
			return false
		}
	}
	for _, r := range prefix {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}
	return true
}

// resolveWorkitemIdentifierForRPC returns the work item unique id for RPC body fields.
// Serials are resolved via personal-token OAPI GET; hex identifiers pass through.
func resolveWorkitemIdentifierForRPC(ctx context.Context, c *client.Client, idOrSerial string) (string, error) {
	idOrSerial = strings.TrimSpace(idOrSerial)
	if idOrSerial == "" {
		return "", fmt.Errorf("missing work item id")
	}
	if !looksLikeWorkitemSerial(idOrSerial) {
		return idOrSerial, nil
	}
	path, err := c.ProjexPath(ctx, "/workitems/"+idOrSerial)
	if err != nil {
		return "", err
	}
	var out map[string]any
	if err := c.Get(ctx, path, nil, &out); err != nil {
		return "", fmt.Errorf("resolve serial %s to identifier: %w", idOrSerial, err)
	}
	for _, key := range []string{"id", "identifier", "workitemIdentifier"} {
		if v, ok := out[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), nil
		}
	}
	return "", fmt.Errorf("resolve serial %s: response missing id/identifier", idOrSerial)
}
