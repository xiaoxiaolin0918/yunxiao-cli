package cmd

import (
	"context"
	"errors"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

const (
	ensureStepReviewersAdd = "reviewers_add"
	ensureStepReviewPass   = "review_pass"
	ensureStepMerge        = "merge"

	ensureWillRun             = "will_run"
	ensureSkipAlreadyReviewer = "skip_already_reviewer"
	ensureSkipAlreadyPass     = "skip_already_pass"
	ensureSkipNotNeeded       = "skip_not_needed"

	mergeRejectionApprovalGate   = "approval_gate"
	mergeRejectionPushPermission = "push_permission"
)

type ensureApprovedStep struct {
	Action  string   `json:"action"`
	Status  string   `json:"status"`
	Note    string   `json:"note,omitempty"`
	UserIDs []string `json:"user_ids,omitempty"`
}

type ensureApprovedPlan struct {
	ReviewerUserID string
	CurrentUserID  string
	Steps          []ensureApprovedStep
}

func (p *ensureApprovedPlan) asMap() map[string]any {
	if p == nil {
		return nil
	}
	steps := make([]map[string]any, 0, len(p.Steps))
	for _, s := range p.Steps {
		m := map[string]any{"action": s.Action, "status": s.Status}
		if s.Note != "" {
			m["note"] = s.Note
		}
		if len(s.UserIDs) > 0 {
			m["user_ids"] = s.UserIDs
		}
		steps = append(steps, m)
	}
	out := map[string]any{"steps": steps}
	if p.ReviewerUserID != "" {
		out["reviewer_user_id"] = p.ReviewerUserID
	}
	if p.CurrentUserID != "" {
		out["current_user_id"] = p.CurrentUserID
	}
	return out
}

func (p *ensureApprovedPlan) idsToAdd() []string {
	for _, s := range p.Steps {
		if s.Action == ensureStepReviewersAdd && s.Status == ensureWillRun {
			return s.UserIDs
		}
	}
	return nil
}

func (p *ensureApprovedPlan) shouldPass() bool {
	for _, s := range p.Steps {
		if s.Action == ensureStepReviewPass && s.Status == ensureWillRun {
			return true
		}
	}
	return false
}

// buildEnsureApprovedPlan decides the add → PASS → merge steps for
// `mrs merge --ensure-approved` (#408). It does not write anything.
func buildEnsureApprovedPlan(mr map[string]any, currentUserID, requestedReviewer, repoArg, localID string) (*ensureApprovedPlan, error) {
	currentUserID = strings.TrimSpace(currentUserID)
	requestedReviewer = strings.TrimSpace(requestedReviewer)
	if currentUserID == "" {
		return nil, &detailedError{
			Subtype: "ensure_approved_identity_unknown",
			Message: "mrs merge --ensure-approved: current account user id is unknown; cannot add a reviewer or submit PASS",
			Hint:    "check yunxiao auth status / whoami, then retry; or pass --reviewer <userId> together with --ensure-approved after confirming the current identity can submit PASS",
			Details: map[string]any{"local_id": localID, "action": "ensure_approved"},
		}
	}
	reviewerID := requestedReviewer
	if reviewerID == "" {
		reviewerID = currentUserID
	}
	plan := &ensureApprovedPlan{ReviewerUserID: reviewerID, CurrentUserID: currentUserID}

	needPass := !ensureAlreadyPassed(mr, currentUserID)
	authorID := mrAuthorUserID(mr)
	if needPass && authorID != "" && identitiesEqual(authorID, currentUserID) {
		return nil, reviewAsAuthorError(repoArg, localID, authorID, map[string]any{
			"action": "ensure_approved",
		})
	}

	var toAdd []string
	if !mrHasReviewer(mr, reviewerID) {
		toAdd = append(toAdd, reviewerID)
	}
	if needPass && !identitiesEqual(reviewerID, currentUserID) && !mrHasReviewer(mr, currentUserID) {
		toAdd = append(toAdd, currentUserID)
	}
	addStep := ensureApprovedStep{Action: ensureStepReviewersAdd, UserIDs: toAdd}
	switch {
	case len(toAdd) > 0:
		addStep.Status = ensureWillRun
		addStep.Note = "add reviewer(s) missing from the MR list"
	case mrHasReviewer(mr, reviewerID):
		addStep.Status = ensureSkipAlreadyReviewer
		addStep.Note = "already on the reviewer list"
	default:
		addStep.Status = ensureSkipNotNeeded
	}
	plan.Steps = append(plan.Steps, addStep)

	passStep := ensureApprovedStep{Action: ensureStepReviewPass}
	if needPass {
		passStep.Status = ensureWillRun
		passStep.Note = "submit reviewOpinion PASS as the current account"
	} else {
		passStep.Status = ensureSkipAlreadyPass
		passStep.Note = "already PASS or MR is TO_BE_MERGED"
	}
	plan.Steps = append(plan.Steps, passStep)
	plan.Steps = append(plan.Steps, ensureApprovedStep{Action: ensureStepMerge, Status: ensureWillRun, Note: "POST merge after precheck"})
	return plan, nil
}

func ensureAlreadyPassed(mr map[string]any, currentUserID string) bool {
	if strings.EqualFold(strings.TrimSpace(zhiyi.MRStatus(mr)), zhiyi.MRStatusToBeMerged) {
		return true
	}
	_, opinion := mrReviewerState(mr, currentUserID)
	if strings.EqualFold(opinion, "PASS") {
		return true
	}
	return false
}

func mrHasReviewer(mr map[string]any, userID string) bool {
	in, _ := mrReviewerState(mr, userID)
	return in
}

func mrReviewerState(mr map[string]any, userID string) (inList bool, opinion string) {
	if mr == nil || strings.TrimSpace(userID) == "" {
		return false, ""
	}
	raw, _ := mr["reviewers"].([]any)
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		id := lookupStringField(m, "userId", "user_id", "id")
		if identitiesEqual(id, userID) {
			return true, lookupStringField(m, "reviewOpinionStatus", "review_opinion_status")
		}
	}
	return false, ""
}

func classifyMergeRejection(err error) string {
	var ae *client.APIError
	if !errors.As(err, &ae) {
		return mergeRejectionApprovalGate
	}
	blob := strings.ToLower(ae.Body + " " + ae.Error())
	if strings.Contains(blob, "as the author") {
		return mergeRejectionApprovalGate
	}
	keys := []string{
		"无权限推送",
		"没有权限推送",
		"保护分支",
		"protected branch",
		"no permission to push",
		"not allowed to push",
		"cannot push",
		"you do not have permission to push",
	}
	for _, k := range keys {
		if strings.Contains(blob, strings.ToLower(k)) {
			return mergeRejectionPushPermission
		}
	}
	return mergeRejectionApprovalGate
}

func pushPermissionHint() string {
	return "合入失败是保护分支或推送权限问题，不是缺少评审 PASS。检查当前账号对该目标分支的推送权限与仓库保护规则；CLI 不会绕过保护分支。"
}

// planEnsureApprovedFromAPI loads the MR and current user (read-only) and
// builds the --ensure-approved step list. Used by --dry-run and the live path.
func planEnsureApprovedFromAPI(ctx context.Context, c *client.Client, repoArg, repositoryID, localID, requestedReviewer string) (*ensureApprovedPlan, error) {
	mr, err := fetchMrsDetail(ctx, c, repositoryID, localID)
	if err != nil {
		return nil, &contextError{Context: "ensure-approved: GET MR " + localID, Err: err, Hint: "fix the error above; add/PASS/merge were not attempted"}
	}
	user, err := fetchPlatformUser(ctx, c)
	if err != nil {
		return nil, &detailedError{
			Subtype: "ensure_approved_identity_unknown",
			Message: "mrs merge --ensure-approved: could not read GET /oapi/v1/platform/user",
			Hint:    "check auth and retry; the merge POST is not attempted",
			Details: map[string]any{"local_id": localID, "action": "ensure_approved"},
		}
	}
	return buildEnsureApprovedPlan(mr, platformUserID(user), requestedReviewer, repoArg, localID)
}

func executeEnsureApprovedPlan(ctx context.Context, c *client.Client, repoArg, repositoryID, localID string, plan *ensureApprovedPlan) error {
	repoID := client.EncodeRepoID(repositoryID)
	if ids := plan.idsToAdd(); len(ids) > 0 {
		path, err := c.CodeupPath(ctx, "/repositories/"+repoID+"/changeRequests/"+localID+"/person/REVIEWER")
		if err != nil {
			return err
		}
		var out any
		if _, err := c.Do(ctx, "POST", path, nil, map[string]any{"userIds": ids}, &out); err != nil {
			return &contextError{Context: "ensure-approved: reviewers add on MR " + localID, Err: err, Hint: "reviewer add failed; PASS and merge were not attempted"}
		}
	}
	if plan.shouldPass() {
		path, err := c.CodeupPath(ctx, "/repositories/"+repoID+"/changeRequests/"+localID+"/review")
		if err != nil {
			return err
		}
		var out any
		if _, err := c.Do(ctx, "POST", path, nil, map[string]any{"reviewOpinion": "PASS"}, &out); err != nil {
			if enriched := enrichMrsReviewError(ctx, c, repoArg, repositoryID, localID, err); enriched != err {
				return enriched
			}
			return &contextError{Context: "ensure-approved: review PASS on MR " + localID, Err: err, Hint: "PASS failed; merge was not attempted"}
		}
	}
	return nil
}
