package zhiyi

import (
	"strings"
)

// Push-review MR lifecycle statuses (Codeup GetChangeRequest "status" /
// ListMergeRequests "newVersionState"; values verified against the OpenAPI docs,
// issues #124/#132). 开发中 = UNDER_DEV is the push-review WIP state: while an MR
// is UNDER_DEV the server rejects merges with 405 SYSTEM_FORBIDDEN_ERROR
// 「该状态下的评审不允许合并」.
//
// NOTE (#124): there is NO confirmed OpenAPI to transition the server-side status
// (UpdateChangeRequest only accepts title/description). "取消 WIP" exists only in
// the web UI (MR page → 更多(…) → 取消 WIP). Do not invent an endpoint for it.
const (
	MRStatusUnderDev    = "UNDER_DEV"    // 开发中 (push-review WIP; merge blocked)
	MRStatusUnderReview = "UNDER_REVIEW" // 评审中
	MRStatusToBeMerged  = "TO_BE_MERGED" // 待合并
	MRStatusClosed      = "CLOSED"       // 已关闭
	MRStatusMerged      = "MERGED"       // 已合并
)

// mrStatusDisplayCN maps push-review statuses to the Codeup web UI labels.
func mrStatusDisplayCN(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case MRStatusUnderDev:
		return "开发中(WIP)"
	case MRStatusUnderReview:
		return "评审中"
	case MRStatusToBeMerged:
		return "待合并"
	case MRStatusClosed:
		return "已关闭"
	case MRStatusMerged:
		return "已合并"
	default:
		return ""
	}
}

// MRStatusDisplay returns the Codeup web-UI label for a status ("" when unknown).
func MRStatusDisplay(status string) string { return mrStatusDisplayCN(status) }

// MergeRequestLocalID returns the MR local id as a string (localId/local_id/iid).
func MergeRequestLocalID(mr map[string]any) string { return localIDString(mr) }

// MergeRequestListStatus returns the push-review lifecycle status for a list item:
// newVersionState (UNDER_DEV/UNDER_REVIEW/TO_BE_MERGED/CLOSED/MERGED) wins, then
// top-level status (detail shape), then legacy lowercase state (opened/merged/...).
func MergeRequestListStatus(item map[string]any) string {
	if item == nil {
		return ""
	}
	if s := stringField(item, "newVersionState", "new_version_state"); s != "" {
		return s
	}
	if s := stringField(item, "status"); s != "" {
		return s
	}
	if s := stringField(item, "mergeStatus", "merge_status"); s != "" {
		return s
	}
	if s := stringField(item, "state"); s != "" {
		return s
	}
	return ""
}

// MergeRequestWIP reports the WIP signal of an MR object: true when the
// push-review status is UNDER_DEV or the list field workInProgress is true.
// ok=false when the object carries no signal at all (do not fabricate a bool).
func MergeRequestWIP(item map[string]any) (wip, ok bool) {
	if item == nil {
		return false, false
	}
	if status := MergeRequestListStatus(item); status != "" {
		return strings.EqualFold(status, MRStatusUnderDev), true
	}
	if v, present := item["workInProgress"]; present && v != nil {
		if b, isBool := v.(bool); isBool {
			return b, true
		}
	}
	return false, false
}

// attachMergeRequestStatus injects the CLI-computed status/wip keys onto one item
// (#132, same per-item injection convention as #94 "latest"): "status" is the
// normalized lifecycle status and "wip" the WIP signal; both are injected into the
// raw API object and overwrite any same-named API field. "wip" is omitted when the
// item carries no WIP signal at all. The legacy lowercase "state" field is left
// untouched (list items keep the old-version value, e.g. "opened").
func attachMergeRequestStatus(item map[string]any) {
	if item == nil {
		return
	}
	if st := MergeRequestListStatus(item); st != "" {
		item["status"] = st
		if wip, ok := MergeRequestWIP(item); ok {
			item["wip"] = wip
		}
		return
	}
	if wip, ok := MergeRequestWIP(item); ok {
		item["wip"] = wip
	}
}

// AttachMergeRequestStatuses walks list payloads (bare []any or common wrapper
// shapes, incl. the "result" array of ListMergeRequests) and injects per item the
// CLI-computed "status" (+ "wip" when a signal exists). Single map objects are
// enriched in place. Non-map items pass through unchanged (#132).
func AttachMergeRequestStatuses(data any) any {
	switch v := data.(type) {
	case []any:
		for i := range v {
			if m, ok := v[i].(map[string]any); ok {
				attachMergeRequestStatus(m)
			}
		}
		return v
	case map[string]any:
		for _, key := range []string{"items", "list", "data", "changeRequests", "result"} {
			if inner, ok := v[key]; ok {
				v[key] = AttachMergeRequestStatuses(inner)
			}
		}
		return v
	default:
		return data
	}
}

// FilterMergeRequestListByStatus client-side filters list payloads to items whose
// normalized status equals want (case-insensitive). Bare arrays and the common
// wrapper keys are handled; other payloads pass through unchanged. The server
// /changeRequests endpoint may silently ignore status filters (wiki #24), so
// `mrs list --status` filters locally (#132): page-local unless combined with --all.
func FilterMergeRequestListByStatus(data any, want string) any {
	want = strings.ToUpper(strings.TrimSpace(want))
	if want == "" {
		return data
	}
	keep := func(items []any) []any {
		out := make([]any, 0, len(items))
		for _, it := range items {
			if m, ok := it.(map[string]any); ok {
				if !strings.EqualFold(MergeRequestListStatus(m), want) {
					continue
				}
			}
			out = append(out, it)
		}
		return out
	}
	switch v := data.(type) {
	case []any:
		return keep(v)
	case map[string]any:
		for _, key := range []string{"items", "list", "data", "changeRequests", "result"} {
			inner, ok := v[key]
			if !ok {
				continue
			}
			if slice, isSlice := inner.([]any); isSlice {
				v[key] = keep(slice)
				return v
			}
			if _, isMap := inner.(map[string]any); isMap {
				v[key] = FilterMergeRequestListByStatus(inner, want)
				return v
			}
		}
		return v
	default:
		return data
	}
}

func boolPtr(b bool) *bool { return &b }

// mrReviewersOf summarizes reviewers: name/username + reviewOpinionStatus/hasReviewed.
func mrReviewersOf(mr map[string]any) []map[string]any {
	raw, _ := mr["reviewers"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		entry := map[string]any{
			"name":     stringField(m, "name"),
			"username": stringField(m, "username"),
		}
		if op := stringField(m, "reviewOpinionStatus", "review_opinion_status"); op != "" {
			entry["review_opinion_status"] = op
		}
		if v, present := m["hasReviewed"]; present && v != nil {
			entry["has_reviewed"] = v
		}
		out = append(out, entry)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// mrTodoOf extracts todoList.requirementCheckItems from a detail object.
func mrTodoOf(mr map[string]any) []map[string]any {
	todoList, _ := mr["todoList"].(map[string]any)
	if todoList == nil {
		return nil
	}
	raw, _ := todoList["requirementCheckItems"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, it := range raw {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// mrReviewPassed reports whether every reviewer opinion was PASS
// (detail reviewers[].reviewOpinionStatus). nil when there is no opinion at all.
func mrReviewPassed(mr map[string]any) *bool {
	raw, ok := mr["reviewers"].([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	passed, opinion := true, false
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		switch strings.ToUpper(stringField(m, "reviewOpinionStatus", "review_opinion_status")) {
		case "PASS":
			opinion = true
		case "NOT_PASS":
			opinion = true
			passed = false
		}
	}
	if !opinion {
		return nil
	}
	return boolPtr(passed)
}

// MRSummaryView is the composed push-review view of one MR (#132): list fields
// plus the detail-only ahead/behind/merge-requirements. Built by
// codeup mrs +push-review-status from one detail GET per listed open MR.
type MRSummaryView struct {
	LocalID        string           `json:"localId"`
	Title          string           `json:"title"`
	Status         string           `json:"status,omitempty"`
	State          string           `json:"state,omitempty"`
	StatusDisplay  string           `json:"status_display,omitempty"`
	WIP            *bool            `json:"wip,omitempty"`
	SourceBranch   string           `json:"sourceBranch,omitempty"`
	TargetBranch   string           `json:"targetBranch,omitempty"`
	Ahead          any              `json:"ahead,omitempty"`
	Behind         any              `json:"behind,omitempty"`
	Mergeable      *bool            `json:"mergeable,omitempty"`
	Todo           []map[string]any `json:"todo,omitempty"`
	Reviewers      []map[string]any `json:"reviewers,omitempty"`
	ReviewPassed   *bool            `json:"review_passed,omitempty"`
	AuthorName     string           `json:"author_name,omitempty"`
	CreationMethod string           `json:"creation_method,omitempty"`
	DetailURL      string           `json:"url,omitempty"`
}

// BuildMRSummaryView composes the #132 push-review summary for one MR. detail is
// the detail GET object (nil → list-only fields, no ahead/behind/mergeable/todo);
// list is the list item (nil → detail-only fields).
func BuildMRSummaryView(list, detail map[string]any) MRSummaryView {
	merged := map[string]any{}
	for k, v := range list {
		merged[k] = v
	}
	for k, v := range detail {
		merged[k] = v
	}
	view := MRSummaryView{
		LocalID:      localIDString(merged),
		Title:        stringField(merged, "title"),
		Status:       MergeRequestListStatus(merged),
		SourceBranch: stringField(merged, "sourceBranch", "source_branch"),
		TargetBranch: stringField(merged, "targetBranch", "target_branch"),
		Reviewers:    mrReviewersOf(merged),
		DetailURL:    MergeRequestURL(merged),
	}
	view.State = view.Status
	view.StatusDisplay = mrStatusDisplayCN(view.Status)
	if wip, ok := MergeRequestWIP(merged); ok {
		view.WIP = boolPtr(wip)
	}
	if author, ok := merged["author"].(map[string]any); ok {
		view.AuthorName = stringField(author, "name", "username")
	}
	view.CreationMethod = stringField(merged, "creationMethod", "createFrom")
	if detail != nil {
		if v, ok := detail["ahead"]; ok && v != nil {
			view.Ahead = v
		}
		if v, ok := detail["behind"]; ok && v != nil {
			view.Behind = v
		}
		if v, ok := detail["allRequirementsPass"].(bool); ok {
			view.Mergeable = boolPtr(v)
		}
		view.Todo = mrTodoOf(detail)
		view.ReviewPassed = mrReviewPassed(detail)
	}
	return view
}
