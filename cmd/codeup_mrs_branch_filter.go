package cmd

import "strings"

// #96: `codeup mrs list --source/--target` client-side branch filters.
//
// The Codeup ListMergeRequests OpenAPI has no sourceBranch/targetBranch query
// params (its query surface is projectIds/authorIds/reviewerIds/state/search/
// orderBy/sort/createdBefore/createdAfter/page/perPage), so the CLI fetches the
// normal list and filters items locally. Commands note this via
// meta.filtered_by = "client".

// filterMrsByBranch keeps only MRs whose sourceBranch/targetBranch exactly match
// the given names (empty = no constraint on that side). Handles bare []any
// payloads and common wrapper maps (items/list/data/changeRequests), mirroring
// zhiyi.AttachMergeRequestURLs. Other shapes are returned untouched.
func filterMrsByBranch(data any, source, target string) any {
	switch v := data.(type) {
	case []any:
		return filterMrsSliceByBranch(v, source, target)
	case map[string]any:
		for _, key := range []string{"items", "list", "data", "changeRequests"} {
			if inner, ok := v[key]; ok {
				v[key] = filterMrsByBranch(inner, source, target)
				return v
			}
		}
		return v
	default:
		return data
	}
}

func filterMrsSliceByBranch(items []any, source, target string) []any {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	if source == "" && target == "" {
		return items
	}
	out := make([]any, 0, len(items))
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if source != "" && mrStringField(m, "sourceBranch") != source {
			continue
		}
		if target != "" && mrStringField(m, "targetBranch") != target {
			continue
		}
		out = append(out, it)
	}
	return out
}

// mrStringField reads a top-level string field off an MR object ("" when absent
// or non-string).
func mrStringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}
