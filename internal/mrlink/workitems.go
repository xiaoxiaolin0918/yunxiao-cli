package mrlink

import (
	"fmt"
	"strings"
)

// ResolvedWorkItem is a prechecked work item ready for MR create.
type ResolvedWorkItem struct {
	InternalID string
	// MatchKeys are ids/serials/refs that should count as "linked" when scanning the MR response.
	MatchKeys []string
}

// InternalIDs returns body workItemIds from resolved items.
func InternalIDs(items []ResolvedWorkItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if it.InternalID != "" {
			out = append(out, it.InternalID)
		}
	}
	return out
}

// WorkItemIDsCSV formats internal ids for Codeup CreateChangeRequest body field
// workItemIds, which OpenAPI documents as a comma-separated string (not an array).
func WorkItemIDsCSV(ids []string) string {
	var parts []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			parts = append(parts, id)
		}
	}
	return strings.Join(parts, ",")
}


// AttachedWorkItemIDs extracts work item ids already linked on an MR / changeRequest
// response. Tolerates several Codeup field shapes (id, workItemId, identifier, serialNumber).
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

// MissingWorkItemIDs returns wanted internal ids that are not present in attached
// when matching against any of the resolved MatchKeys (id, serial, original ref).
func MissingWorkItemIDs(wanted []ResolvedWorkItem, attached []string) []string {
	have := map[string]struct{}{}
	for _, id := range attached {
		id = strings.TrimSpace(id)
		if id != "" {
			have[id] = struct{}{}
		}
	}
	var missing []string
	seen := map[string]struct{}{}
	for _, w := range wanted {
		if w.InternalID == "" {
			continue
		}
		if _, ok := seen[w.InternalID]; ok {
			continue
		}
		matched := false
		keys := w.MatchKeys
		if len(keys) == 0 {
			keys = []string{w.InternalID}
		}
		for _, k := range keys {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			if _, ok := have[k]; ok {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		seen[w.InternalID] = struct{}{}
		missing = append(missing, w.InternalID)
	}
	return missing
}

// SpaceIDFromWorkItem returns spaceId / project space when present.
func SpaceIDFromWorkItem(item map[string]any) string {
	if item == nil {
		return ""
	}
	for _, key := range []string{"spaceId", "space_id"} {
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

// FormatMissingLinkError is used when --work-item was required and links did not stick.
func FormatMissingLinkError(missing []string, mrURL string) string {
	base := fmt.Sprintf("work item link(s) missing after create (sent workItemIds as comma-separated string per OpenAPI): %s", strings.Join(missing, ", "))
	linkHint := "yunxiao codeup mrs link --repo <id> --local-id <n> --work-item <id|serial>"
	if mrURL != "" {
		return base + "; MR exists at " + mrURL + " — link with `" + linkHint + "`, or close/recreate / web UI"
	}
	return base + "; link with `" + linkHint + "`, or close/recreate / web UI"
}

// RelationRecordID extracts the extRelationRecords id used by DeleteWorkitemExtRelationRecord.
// OpenAPI documents relationRecordId; some payloads may only expose id.
func RelationRecordID(rel map[string]any) string {
	if rel == nil {
		return ""
	}
	for _, key := range []string{"relationRecordId", "id"} {
		if s := stringifyID(rel[key]); s != "" {
			return s
		}
	}
	return ""
}

// MatchKeysFromWorkItem collects ids/serials from a work item JSON object.
func MatchKeysFromWorkItem(item map[string]any, originalRef string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(originalRef)
	if item == nil {
		return out
	}
	for _, key := range []string{"id", "workItemId", "workitemId", "identifier", "serialNumber", "serial_number"} {
		add(stringifyID(item[key]))
	}
	return out
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
			for _, key := range []string{"id", "workItemId", "workitemId", "identifier", "serialNumber", "serial_number"} {
				if s := stringifyID(item[key]); s != "" {
					out = append(out, s)
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
