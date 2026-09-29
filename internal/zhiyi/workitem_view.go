package zhiyi

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Views for `workitem get` (#98). The default brief view keeps what agents usually
// need and summarizes the (often multi-thousand character) description; --fields
// projects top-level keys of the raw GetWorkitem object; --full is the raw object.

// WorkItemPriorityFieldID is the customFieldValues fieldId Projex uses for priority
// (GetWorkitem has no top-level priority).
const WorkItemPriorityFieldID = "priority"

// workItemGetBriefKeys are copied as-is from the raw item when present and non-null.
var workItemGetBriefKeys = []string{"assignedTo", "sprint", "gmtModified"}

// WorkItemGetBrief returns id, serialNumber, subject, status (id + displayName),
// assignedTo, sprint, priority, gmtModified and a description_summary placeholder.
// Absent / null / empty values are omitted.
func WorkItemGetBrief(item map[string]any) map[string]any {
	out := BriefWorkItem(item)
	if item == nil {
		return out
	}
	for _, k := range workItemGetBriefKeys {
		if v, ok := item[k]; ok && v != nil {
			out[k] = v
		}
	}
	if p, ok := WorkItemPriority(item); ok {
		out["priority"] = p
	}
	if d, ok := item["description"].(string); ok && d != "" {
		out["description_summary"] = fmt.Sprintf("(description: %d chars, use --full or --fields description)", utf8.RuneCountInString(d))
	}
	return out
}

// WorkItemPriority returns a top-level "priority" when the API provides one, else the
// first value of the customFieldValues entry with fieldId "priority" as
// {id, displayValue}.
func WorkItemPriority(item map[string]any) (any, bool) {
	if v, ok := item["priority"]; ok && v != nil {
		return v, true
	}
	cfs, _ := item["customFieldValues"].([]any)
	for _, raw := range cfs {
		cf, _ := raw.(map[string]any)
		if cf == nil || fmt.Sprint(cf["fieldId"]) != WorkItemPriorityFieldID {
			continue
		}
		vals, _ := cf["values"].([]any)
		if len(vals) == 0 {
			return nil, false
		}
		v, _ := vals[0].(map[string]any)
		if v == nil {
			return nil, false
		}
		return map[string]any{"id": v["identifier"], "displayValue": v["displayValue"]}, true
	}
	return nil, false
}

var workItemFieldNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ParseWorkItemFieldList parses --fields "a,b,c": trims entries, drops duplicates
// (first occurrence wins) and rejects empty entries, nested paths and invalid names.
func ParseWorkItemFieldList(s string) ([]string, error) {
	if strings.TrimSpace(s) == "" {
		return nil, fmt.Errorf("--fields needs at least one field name (e.g. --fields subject,status,assignedTo)")
	}
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		name := strings.TrimSpace(part)
		switch {
		case name == "":
			return nil, fmt.Errorf("--fields %q: empty field name (remove the extra comma)", s)
		case strings.Contains(name, "."):
			return nil, fmt.Errorf("--fields %q: nested path %q is not supported; pick the top-level key (e.g. status) or use --jq for nested values", s, name)
		case !workItemFieldNameRE.MatchString(name):
			return nil, fmt.Errorf("--fields %q: invalid field name %q (top-level keys such as subject, serialNumber, customFieldValues)", s, name)
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out, nil
}

// ProjectWorkItem keeps only the requested top-level keys (raw values; a key present
// with null stays null). "priority" falls back to WorkItemPriority when there is no
// top-level key. Names that are neither are returned as unknown, in request order.
func ProjectWorkItem(item map[string]any, fields []string) (map[string]any, []string) {
	out := map[string]any{}
	var unknown []string
	for _, f := range fields {
		if v, ok := item[f]; ok {
			out[f] = v
			continue
		}
		if f == WorkItemPriorityFieldID {
			if p, ok := WorkItemPriority(item); ok {
				out[f] = p
				continue
			}
		}
		unknown = append(unknown, f)
	}
	return out, unknown
}

// WorkItemAvailableFields lists the names --fields accepts for item (sorted).
func WorkItemAvailableFields(item map[string]any) []string {
	var out []string
	for k := range item {
		out = append(out, k)
	}
	if _, ok := item[WorkItemPriorityFieldID]; !ok {
		if _, ok := WorkItemPriority(item); ok {
			out = append(out, WorkItemPriorityFieldID)
		}
	}
	sort.Strings(out)
	return out
}

// WorkItemFieldSuggestions maps unknown names to an available name that differs only
// in case (field names are case-sensitive).
func WorkItemFieldSuggestions(unknown, available []string) map[string]string {
	out := map[string]string{}
	for _, u := range unknown {
		for _, a := range available {
			if strings.EqualFold(u, a) {
				out[u] = a
				break
			}
		}
	}
	return out
}