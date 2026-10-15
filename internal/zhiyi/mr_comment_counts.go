package zhiyi

// StripNullCommentCounts removes totalCommentCount / unResolvedCommentCount when
// the value is null (#358). GetChangeRequest often returns these keys as null even
// when mrs comments list shows unresolved threads; a literal null is misleading.
// Non-null numbers (including 0) are kept. Mutates mr in place; returns whether
// any null key was stripped.
func StripNullCommentCounts(mr map[string]any) bool {
	if mr == nil {
		return false
	}
	stripped := false
	for _, key := range []string{
		"totalCommentCount", "total_comment_count",
		"unResolvedCommentCount", "unresolvedCommentCount", "un_resolved_comment_count",
	} {
		if v, ok := mr[key]; ok && v == nil {
			delete(mr, key)
			stripped = true
		}
	}
	return stripped
}