package zhiyi

import (
	"fmt"
	"strings"
)

// WorkItemStatusDisplayable reports whether item has a status agents can show:
// status.name or status.displayName (issue #369).
func WorkItemStatusDisplayable(item map[string]any) bool {
	NormalizeWorkItemStatus(item)
	st, _ := item["status"].(map[string]any)
	return statusDisplayName(st) != ""
}

// NormalizeWorkItemStatus coerces common GetWorkitem / SearchWorkitems status
// shapes in place: a bare string/number id becomes {"id": "..."}; maps are left
// as-is. No-op when status is null or already a map.
func NormalizeWorkItemStatus(item map[string]any) {
	if item == nil {
		return
	}
	raw, ok := item["status"]
	if !ok || raw == nil {
		return
	}
	if _, isMap := raw.(map[string]any); isMap {
		return
	}
	id := ""
	switch v := raw.(type) {
	case string:
		id = strings.TrimSpace(v)
	case float64:
		id = fmt.Sprintf("%.0f", v)
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s != "" && s != "<nil>" {
			id = s
		}
	}
	if id == "" {
		return
	}
	item["status"] = map[string]any{"id": id}
}

// MergeWorkItemStatus copies a displayable status from src onto dst.
// Returns true when dst ends with a displayable status taken from src.
func MergeWorkItemStatus(dst, src map[string]any) bool {
	if dst == nil || src == nil {
		return false
	}
	NormalizeWorkItemStatus(src)
	if !WorkItemStatusDisplayable(src) {
		return false
	}
	st, _ := src["status"].(map[string]any)
	if st == nil {
		return false
	}
	// Shallow copy so later mutations of src don't affect dst.
	cp := make(map[string]any, len(st))
	for k, v := range st {
		cp[k] = v
	}
	dst["status"] = cp
	return true
}

// EnsureWorkItemGetStatus fills a missing/null/non-displayable status for
// workitem get brief (#369). refresh is typically a SearchWorkitems lookup for
// the same serial (search usually returns status when GetWorkitem omits it).
//
// Returns:
//   - refreshed=true when status was merged from refresh
//   - failed=true when status was not displayable and refresh did not help
//
// When status is already displayable, refresh is not called.
func EnsureWorkItemGetStatus(item map[string]any, refresh func() (map[string]any, error)) (out map[string]any, refreshed, failed bool) {
	if item == nil {
		return nil, false, true
	}
	NormalizeWorkItemStatus(item)
	if WorkItemStatusDisplayable(item) {
		return item, false, false
	}
	if refresh == nil {
		return item, false, true
	}
	src, err := refresh()
	if err != nil || src == nil {
		return item, false, true
	}
	NormalizeWorkItemStatus(src)
	if MergeWorkItemStatus(item, src) {
		return item, true, false
	}
	return item, false, true
}