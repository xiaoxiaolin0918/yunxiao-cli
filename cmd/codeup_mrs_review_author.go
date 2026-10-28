package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

// runMrsReview is the mrs review execution path (#395): same gate/dry-run contract
// as runJSONMutating (high-risk-write → --yes), with a read-only author self-review
// precheck before the POST. On an API 403 "as the author" it enriches the error
// with suggested_actions (reviewers add / switch identity) instead of only the
// platform body.
func runMrsReview(ctx context.Context, c *client.Client, repoArg, repositoryID, localID, path string, body map[string]any, rp client.RequestPreview) error {
	return runMutating("codeup mrs review", risk.HighRiskWrite, globalDryRun, globalYes, rp, func() error {
		pre, perr := precheckMrsReview(ctx, c, repoArg, repositoryID, localID)
		if perr != nil {
			return perr
		}
		var out any
		if _, err := c.Do(ctx, "POST", path, nil, body, &out); err != nil {
			return enrichMrsReviewError(ctx, c, repoArg, repositoryID, localID, err)
		}
		meta := map[string]any{"risk": risk.HighRiskWrite}
		if pre != nil {
			meta["precheck"] = pre
		}
		return output.Success(out, meta)
	})
}

// precheckMrsReview (#395) GETs the MR and /platform/user; when author.userId matches
// the current user id it refuses locally with subtype review_as_author. Identity
// lookup failures fail open (status skipped) so a transient whoami outage does not
// block a legitimate non-author review — the POST 403 path still enriches.
func precheckMrsReview(ctx context.Context, c *client.Client, repoArg, repositoryID, localID string) (map[string]any, error) {
	mr, err := fetchMrsDetail(ctx, c, repositoryID, localID)
	if err != nil {
		return map[string]any{
			"status": "skipped",
			"reason": "mr_detail_unreadable",
			"note":   "author self-review precheck skipped; server will enforce",
		}, nil
	}
	authorID := mrAuthorUserID(mr)
	pre := map[string]any{
		"status": "ok",
		"source": "changeRequests/" + localID + " + platform/user",
	}
	if authorID != "" {
		pre["author_user_id"] = authorID
	}
	if authorID == "" {
		pre["status"] = "skipped"
		pre["reason"] = "author_user_id_missing"
		return pre, nil
	}
	user, err := fetchPlatformUser(ctx, c)
	if err != nil {
		pre["status"] = "skipped"
		pre["reason"] = "platform_user_unreadable"
		pre["note"] = "author self-review precheck skipped; server will enforce"
		return pre, nil
	}
	currentID := platformUserID(user)
	if currentID != "" {
		pre["current_user_id"] = currentID
	}
	if currentID == "" {
		pre["status"] = "skipped"
		pre["reason"] = "platform_user_id_missing"
		return pre, nil
	}
	same := identitiesEqual(authorID, currentID)
	pre["is_author"] = same
	if same {
		return nil, reviewAsAuthorError(repoArg, localID, authorID, nil)
	}
	return pre, nil
}

// enrichMrsReviewError (#395): when the review POST fails with the platform's
// author-self-review 403, attach subtype review_as_author + suggested_actions.
// Other API errors pass through unchanged; a failing identity refresh never masks
// the original review error.
func enrichMrsReviewError(ctx context.Context, c *client.Client, repoArg, repositoryID, localID string, reviewErr error) error {
	if !isAuthorSelfReviewForbidden(reviewErr) {
		return reviewErr
	}
	details := map[string]any{
		"action":            "review",
		"suggested_actions": reviewAuthorSuggestedActions(repoArg, localID),
		"diagnose":          map[string]any{"source": "POST changeRequests/" + localID + "/review"},
	}
	authorID := ""
	if mr, err := fetchMrsDetail(ctx, c, repositoryID, localID); err == nil {
		authorID = mrAuthorUserID(mr)
		if authorID != "" {
			details["author_user_id"] = authorID
		}
		details["mr"] = mrsMergeErrorDetail(mr)
	}
	_ = authorID
	return &contextError{
		Context: "review MR " + localID + " blocked",
		Subtype: "review_as_author",
		Hint:    reviewAsAuthorHint(repoArg, localID),
		Err:     reviewErr,
		Details: details,
	}
}

func reviewAsAuthorError(repoArg, localID, authorID string, extra map[string]any) error {
	details := map[string]any{
		"action":            "review",
		"local_id":          localID,
		"suggested_actions": reviewAuthorSuggestedActions(repoArg, localID),
	}
	if authorID != "" {
		details["author_user_id"] = authorID
		details["current_user_id"] = authorID
	}
	for k, v := range extra {
		details[k] = v
	}
	return &detailedError{
		Subtype: "review_as_author",
		Message: fmt.Sprintf("review precheck: current identity is the author of MR !%s; Codeup forbids submitting a review opinion as the author", localID),
		Hint:    reviewAsAuthorHint(repoArg, localID),
		Details: details,
	}
}

func reviewAsAuthorHint(repoArg, localID string) string {
	actions := reviewAuthorSuggestedActions(repoArg, localID)
	return "current identity is this MR's author — Codeup rejects self-review; next: " + strings.Join(actions, " | ")
}

// reviewAuthorSuggestedActions lists copy-pasteable next steps when the caller is
// the MR author (#395).
func reviewAuthorSuggestedActions(repoArg, localID string) []string {
	return []string{
		fmt.Sprintf("yunxiao codeup mrs reviewers add --repo %s --local-id %s --reviewer <otherUserId> --dry-run (then --yes); ask that reviewer to run mrs review", shellArg(repoArg), shellArg(localID)),
		"switch CLI identity (yunxiao auth login / another profile) to a non-author account, then retry mrs review",
		fmt.Sprintf("yunxiao codeup mrs get --repo %s --local-id %s", shellArg(repoArg), shellArg(localID)),
	}
}

// mrAuthorUserID extracts the Yunxiao user id from ChangeRequest.author (#395;
// OpenAPI documents author.userId). Falls back to id when userId is absent.
func mrAuthorUserID(mr map[string]any) string {
	if mr == nil {
		return ""
	}
	author, _ := mr["author"].(map[string]any)
	if author == nil {
		return ""
	}
	return firstNonBlank(
		lookupStringField(author, "userId", "user_id", "id"),
	)
}

// platformUserID extracts the id from GET /oapi/v1/platform/user.
func platformUserID(user map[string]any) string {
	return firstNonBlank(lookupStringField(user, "id", "userId", "user_id"))
}

func fetchPlatformUser(ctx context.Context, c *client.Client) (map[string]any, error) {
	var user map[string]any
	if err := c.Get(ctx, "/oapi/v1/platform/user", nil, &user); err != nil {
		return nil, err
	}
	return user, nil
}

func identitiesEqual(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	return strings.EqualFold(a, b)
}

func firstNonBlank(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// lookupStringField mirrors zhiyi.stringField multi-key lookup for cmd helpers.
func lookupStringField(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

// isAuthorSelfReviewForbidden reports whether err is the Codeup 403 that rejects
// an author submitting their own review opinion.
func isAuthorSelfReviewForbidden(err error) bool {
	var ae *client.APIError
	if !errors.As(err, &ae) {
		return false
	}
	if ae.Status != http.StatusForbidden {
		return false
	}
	body := strings.ToLower(ae.Body)
	return strings.Contains(body, "can not submit review opinion as the author") ||
		strings.Contains(body, "cannot submit review opinion as the author")
}
