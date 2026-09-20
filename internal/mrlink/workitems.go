package mrlink

import (
	"fmt"
	"strings"
)

// AttachedWorkItemIDs extracts work item ids already linked on an MR / changeRequest
// response. Tolerates several Codeup field shapes.
func AttachedWorkItemIDs(mr map[string]any) []string {
	if mr == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}

	for _, key := range []string{"workItemIds", "workitemIds", "work_item_ids"} {
		for _, id := range asIDList(mr[key]) {
			add(id)
		}
	}
	for _, key := range []string{"relatedWorkItems", "workItems", "workitems", "related_work_items"} {
		for _, id := range asItemIDList(mr[key]) {
			add(id)
		}
	}
	if data, ok := mr["data"].(map[string]any); ok {
		for _, id := range AttachedWorkItemIDs(data) {
			add(id)
		}
	}
	return out
}

// MissingWorkItemIDs returns wanted ids that are not present in attached (order preserved).
func MissingWorkItemIDs(wanted, attached []string) []string {
	have := map[string]struct{}{}
	for _, id := range attached {
		id = strings.TrimSpace(id)
		if id != "" {
			have[id] = struct{}{}
		}
	}
	var missing []string
	seen := map[string]struct{}{}
	for _, id := range wanted {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := have[id]; ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		missing = append(missing, id)
	}
	return missing
}

// SpaceIDFromWorkItem returns spaceId / project space when present.
func SpaceIDFromWorkItem(item map[string]any) string {
	if item == nil {
		return ""
	}
	for _, key := range []string{"spaceId", "space_id", "projectId", "project_id"} {
		if s := stringifyID(item[key]); s != "" {
			return s
		}
	}
	if space, ok := item["space"].(map[string]any); ok {
		if s := stringifyID(space["id"]); s != "" {
			return s
		}
	}
	return ""
}

// FormatMissingLinkWarning builds a user-facing warning when create succeeded but links did not stick.
func FormatMissingLinkWarning(missing []string) string {
	if len(missing) == 0 {
		return ""
	}
	return fmt.Sprintf("MR created but work item link(s) missing after create (server may have ignored workItemIds): %s; associate manually in the web UI", strings.Join(missing, ", "))
}

func asIDList(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []string:
		return t
	case []any:
		var out []string
		for _, e := range t {
			if s := stringifyID(e); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		if s := stringifyID(t); s != "" {
			return []string{s}
		}
		return nil
	}
}

func asItemIDList(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, e := range arr {
		switch item := e.(type) {
		case map[string]any:
			for _, key := range []string{"id", "workItemId", "workitemId", "identifier"} {
				if s := stringifyID(item[key]); s != "" {
					out = append(out, s)
					break
				}
			}
		default:
			if s := stringifyID(e); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func stringifyID(v any) string {
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