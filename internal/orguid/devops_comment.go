package orguid

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Official path typo: deleteComent (missing 'm'). Do not "fix" the spelling.
const pathDeleteWorkitemComment = "/workitems/deleteComent"
const pathUpdateWorkitemComment = "/workitems/commentUpdate"

// DeleteWorkitemComment calls POST /organization/{org}/workitems/deleteComent
// (DeleteWorkitemComment). identifier is the work item unique id (not serial).
func (c *DevOpsMembersClient) DeleteWorkitemComment(ctx context.Context, orgID, identifier string, commentID int64) (map[string]any, error) {
	if orgID == "" {
		return nil, fmt.Errorf("organizationId required")
	}
	if strings.TrimSpace(identifier) == "" {
		return nil, fmt.Errorf("identifier required")
	}
	if commentID <= 0 {
		return nil, fmt.Errorf("commentId required")
	}
	path := "/organization/" + orgID + pathDeleteWorkitemComment
	body := map[string]any{
		"identifier": identifier,
		"commentId":  commentID,
	}
	return c.DoROA(ctx, http.MethodPost, path, "DeleteWorkitemComment", nil, body)
}

// UpdateWorkitemCommentInput is the body for UpdateWorkitemComment.
type UpdateWorkitemCommentInput struct {
	Content            string
	FormatType         string // MARKDOWN or RICHTEXT
	WorkitemIdentifier string
	CommentID          int64
}

// UpdateWorkitemComment calls POST /organization/{org}/workitems/commentUpdate.
func (c *DevOpsMembersClient) UpdateWorkitemComment(ctx context.Context, orgID string, in UpdateWorkitemCommentInput) (map[string]any, error) {
	if orgID == "" {
		return nil, fmt.Errorf("organizationId required")
	}
	if strings.TrimSpace(in.WorkitemIdentifier) == "" {
		return nil, fmt.Errorf("workitemIdentifier required")
	}
	if strings.TrimSpace(in.Content) == "" {
		return nil, fmt.Errorf("content required")
	}
	if in.CommentID <= 0 {
		return nil, fmt.Errorf("commentId required")
	}
	ft := strings.TrimSpace(in.FormatType)
	if ft == "" {
		ft = "MARKDOWN"
	}
	path := "/organization/" + orgID + pathUpdateWorkitemComment
	body := map[string]any{
		"content":            in.Content,
		"formatType":         ft,
		"workitemIdentifier": in.WorkitemIdentifier,
		"commentId":          in.CommentID,
	}
	return c.DoROA(ctx, http.MethodPost, path, "UpdateWorkitemComment", nil, body)
}

// ParseCommentID parses a CLI --comment-id value (decimal string).
func ParseCommentID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("missing --comment-id")
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid --comment-id %q (expect positive integer)", s)
	}
	return id, nil
}

// PreviewDeleteWorkitemComment builds a dry-run request map (no network).
func (c *DevOpsMembersClient) PreviewDeleteWorkitemComment(orgID, identifier string, commentID int64) map[string]any {
	path := "/organization/" + orgID + pathDeleteWorkitemComment
	return map[string]any{
		"method": http.MethodPost,
		"url":    c.EndpointURL(path, nil),
		"action": "DeleteWorkitemComment",
		"auth":   "alibaba_cloud_access_key (ACS3-HMAC-SHA256)",
		"body": map[string]any{
			"identifier": identifier,
			"commentId":  commentID,
		},
	}
}

// PreviewUpdateWorkitemComment builds a dry-run request map (no network).
func (c *DevOpsMembersClient) PreviewUpdateWorkitemComment(orgID string, in UpdateWorkitemCommentInput) map[string]any {
	ft := strings.TrimSpace(in.FormatType)
	if ft == "" {
		ft = "MARKDOWN"
	}
	path := "/organization/" + orgID + pathUpdateWorkitemComment
	return map[string]any{
		"method": http.MethodPost,
		"url":    c.EndpointURL(path, nil),
		"action": "UpdateWorkitemComment",
		"auth":   "alibaba_cloud_access_key (ACS3-HMAC-SHA256)",
		"body": map[string]any{
			"content":            in.Content,
			"formatType":         ft,
			"workitemIdentifier": in.WorkitemIdentifier,
			"commentId":          in.CommentID,
		},
	}
}
