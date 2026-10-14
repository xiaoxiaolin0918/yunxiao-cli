package zhiyi

import (
	"fmt"
	"strings"
)

// relatedWorkItemKeys are undocumented / inconsistently-populated GetChangeRequest
// fields that reviewers sometimes see as null even when the MR is linked via
// Projex extRelationRecords (#354). Official GetChangeRequest docs do not list them.
var relatedWorkItemKeys = []string{"relatedWorkItems", "workItems", "workitems", "related_work_items"}

var workItemIDKeys = []string{"workItemIds", "workitemIds", "work_item_ids"}

// StripNullRelatedWorkItems removes relatedWorkItems / workItemIds keys whose
// value is null. A literal null is misleading: GetChangeRequest often returns
// relatedWorkItems:null while Projex already has codeupMergeRequest links (#354).
// Empty slices and non-empty values are kept. Mutates mr in place; returns whether
// any null key was stripped.
func StripNullRelatedWorkItems(mr map[string]any) bool {
	if mr == nil {
		return false
	}
	stripped := false
	for _, key := range append(append([]string{}, relatedWorkItemKeys...), workItemIDKeys...) {
		if v, ok := mr[key]; ok && v == nil {
			delete(mr, key)
			stripped = true
		}
	}
	return stripped
}

// RelatedWorkItemsPresent reports whether mr carries a usable (non-nil) related
// work-item list or id field from GetChangeRequest.
func RelatedWorkItemsPresent(mr map[string]any) bool {
	if mr == nil {
		return false
	}
	for _, key := range relatedWorkItemKeys {
		if v, ok := mr[key]; ok && v != nil {
			return true
		}
	}
	for _, key := range workItemIDKeys {
		if v, ok := mr[key]; ok && v != nil {
			return true
		}
	}
	return false
}

// RelatedWorkItemsSummary projects related work items for mrs get --summary (#354).
// Prefer object entries with id / serialNumber; bare ids become {"id": "..."}.
// Returns nil when the API did not return a usable list (callers omit the key).
func RelatedWorkItemsSummary(mr map[string]any) []map[string]any {
	if mr == nil {
		return nil
	}
	var out []map[string]any
	seen := map[string]struct{}{}
	add := func(entry map[string]any) {
		if len(entry) == 0 {
			return
		}
		key := stringField(entry, "id", "workItemId", "serialNumber", "serial_number")
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, entry)
	}
	for _, key := range relatedWorkItemKeys {
		raw, ok := mr[key]
		if !ok || raw == nil {
			continue
		}
		arr, ok := raw.([]any)
		if !ok {
			continue
		}
		for _, e := range arr {
			switch item := e.(type) {
			case map[string]any:
				entry := map[string]any{}
				if id := stringField(item, "id", "workItemId", "workitemId", "identifier"); id != "" {
					entry["id"] = id
				}
				if sn := stringField(item, "serialNumber", "serial_number"); sn != "" {
					entry["serialNumber"] = sn
				}
				if subj := stringField(item, "subject", "title", "name"); subj != "" {
					entry["subject"] = subj
				}
				add(entry)
			default:
				if id := stringifyRelatedID(e); id != "" {
					add(map[string]any{"id": id})
				}
			}
		}
	}
	for _, key := range workItemIDKeys {
		raw, ok := mr[key]
		if !ok || raw == nil {
			continue
		}
		switch t := raw.(type) {
		case string:
			for _, part := range strings.Split(t, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					add(map[string]any{"id": part})
				}
			}
		case []any:
			for _, e := range t {
				if id := stringifyRelatedID(e); id != "" {
					add(map[string]any{"id": id})
				}
			}
		case []string:
			for _, id := range t {
				id = strings.TrimSpace(id)
				if id != "" {
					add(map[string]any{"id": id})
				}
			}
		default:
			if id := stringifyRelatedID(t); id != "" {
				add(map[string]any{"id": id})
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func stringifyRelatedID(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		return fmt.Sprintf("%.0f", t)
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "" || s == "<nil>" {
			return ""
		}
		return s
	}
}

// AnnotateRelatedWorkItemsMeta records that GetChangeRequest relatedWorkItems is
// not a reliable ground truth (#354). Always sets meta.related_work_items so
// scripts/agents do not treat a missing/null field as "unlinked".
func AnnotateRelatedWorkItemsMeta(meta map[string]any, mr map[string]any) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	meta["related_work_items"] = map[string]any{
		"from_get": RelatedWorkItemsPresent(mr),
		"reliable": false,
		"verify":   "yunxiao codeup mrs link --repo <id> --local-id <n> --work-item <id|serial> --dry-run",
		"note":     "GetChangeRequest relatedWorkItems is undocumented and often null/absent even when the MR is linked via Projex extRelationRecords (category=codeupMergeRequest). Do not treat missing/null as unlinked; title/branch ZYPT-xxxx is not a formal link. Verify with mrs link --dry-run or GET workitems/{id}/extRelationRecords?category=codeupMergeRequest.",
	}
	return meta
}
