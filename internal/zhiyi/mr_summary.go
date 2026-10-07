package zhiyi

// SummaryMergeRequest is the mid-tier `mrs get` view between --brief and --full
// (#130 item 3): the script-stable brief fields plus the "can it merge?" facts
// (mergeable, conflictCheckStatus, checkList with requirementRuleItems, a
// reviewers digest, and the documented gating booleans). Fields the API does
// not return are omitted; null/"" brief keys stay (same as BriefMergeRequest).
func SummaryMergeRequest(mr map[string]any) map[string]any {
	s := StabilizeMergeRequest(mr)
	out := map[string]any{
		"localId":   s["localId"],
		"title":     s["title"],
		"status":    s["status"],
		"state":     s["state"],
		"detailUrl": s["detailUrl"],
		"url":       s["url"],
	}
	// Merge readiness: copy verbatim (camelCase first, snake_case tolerated).
	copyMRFieldIfPresent(out, mr, "mergeable", "mergeable")
	copyMRFieldIfPresent(out, mr, "conflictCheckStatus", "conflictCheckStatus", "conflict_check_status")
	copyMRFieldIfPresent(out, mr, "checkList", "checkList", "check_list")
	copyMRFieldIfPresent(out, mr, "supportMergeFastForwardOnly", "supportMergeFastForwardOnly", "support_merge_fast_forward_only")
	copyMRFieldIfPresent(out, mr, "allRequirementsPass", "allRequirementsPass", "all_requirements_pass")
	copyMRFieldIfPresent(out, mr, "ahead", "ahead")
	copyMRFieldIfPresent(out, mr, "behind", "behind")
	if rs := ReviewersSummary(mr); len(rs) > 0 {
		out["reviewers"] = rs
	}
	return out
}

// copyMRFieldIfPresent copies mr[keys[0]] (first key present and non-nil) to
// out[dst]. Values are passed through unchanged.
func copyMRFieldIfPresent(out, mr map[string]any, dst string, keys ...string) {
	if mr == nil {
		return
	}
	for _, k := range keys {
		if v, ok := mr[k]; ok && v != nil {
			out[dst] = v
			return
		}
	}
}

// ReviewersSummary projects reviewers down to {name, opinion} entries (#130).
// GetChangeRequest documents reviewOpinionStatus (PASS | NOT_PASS); plain
// opinion / reviewOpinion are tolerated. Reviewers without an opinion yet are
// kept (that is the interesting case for "can it merge"); entries with neither
// a name nor an opinion are dropped.
func ReviewersSummary(mr map[string]any) []map[string]any {
	raw, _ := mr["reviewers"].([]any)
	var out []map[string]any
	for _, it := range raw {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		name := stringField(m, "name", "displayName", "username", "userName")
		op := stringField(m, "reviewOpinionStatus", "review_opinion_status", "reviewOpinion", "review_opinion", "opinion")
		if name == "" && op == "" {
			continue
		}
		e := map[string]any{}
		if name != "" {
			e["name"] = name
		}
		if op != "" {
			e["opinion"] = op
		}
		out = append(out, e)
	}
	return out
}
