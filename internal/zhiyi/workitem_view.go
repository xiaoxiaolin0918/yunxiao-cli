package zhiyi

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Views for `workitem get` (#98). The default brief view keeps what agents usually
// need and summarizes the (often multi-thousand character) description; --fields
// projects top-level keys of the raw GetWorkitem object; --full is the raw object.

// WorkItemPriorityFieldID is the --fields name for the derived priority and the
// customFieldValues fieldId Projex uses for priority in some organizations
// (GetWorkitem has no top-level priority).
const WorkItemPriorityFieldID = "priority"

// workItemPriorityFieldNames are customFieldValues fieldName values that identify the
// priority field when its fieldId is an opaque hash (compared case-insensitively).
var workItemPriorityFieldNames = []string{"优先级", "Priority"}

// workItemGetBriefKeys are copied as-is from the raw item when present and non-null.
var workItemGetBriefKeys = []string{"assignedTo", "sprint", "categoryId", "gmtModified"}

// WorkItemGetBriefFields lists the brief view's keys in documentation order.
var WorkItemGetBriefFields = []string{"id", "serialNumber", "subject", "status", "assignedTo", "sprint", "priority", "workitemType", "categoryId", "gmtModified", "description_summary"}

// workItemSchemaFields are the top-level keys of the official GetWorkitem response
// (help.aliyun.com/zh/yunxiao/developer-reference/getworkitem). --fields accepts them
// even when this work item lacks the key (e.g. no sprint) and prints null.
var workItemSchemaFields = []string{
	"assignedTo", "categoryId", "creator", "customFieldValues", "description", "formatType",
	"gmtCreate", "gmtModified", "id", "idPath", "labels", "logicalStatus", "modifier",
	"parentId", "participants", "serialNumber", "space", "sprint", "status", "statusStageId",
	"subject", "trackers", "updateStatusAt", "verifier", "versions", "workitemType",
}

// WorkItemKnownField reports whether name is a GetWorkitem schema key or the derived
// priority, i.e. a name --fields answers with null instead of unknown_fields when the
// work item lacks it.
func WorkItemKnownField(name string) bool {
	if name == WorkItemPriorityFieldID {
		return true
	}
	for _, f := range workItemSchemaFields {
		if f == name {
			return true
		}
	}
	return false
}

// WorkItemGetBrief returns id, serialNumber, subject, status (id + displayName),
// assignedTo, sprint, priority, workitemType (id + name), categoryId, gmtModified and
// a description_summary placeholder. Absent / null / empty values are omitted.
func WorkItemGetBrief(item map[string]any) map[string]any {
	out := BriefWorkItem(item)
	if item == nil {
		return out
	}
	for _, k := range workItemGetBriefKeys {
		if v, ok := item[k]; ok && !isEmptyValue(v) {
			out[k] = v
		}
	}
	if wt := idNameBrief(item["workitemType"]); wt != nil {
		out["workitemType"] = wt
	}
	if p, ok := WorkItemPriority(item); ok {
		out["priority"] = p
	}
	if s, ok := WorkItemDescriptionSummary(item); ok {
		out["description_summary"] = s
	}
	return out
}

// isEmptyValue: null or "" (brief omits both).
func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && s == ""
}

// idNameBrief keeps id and name of an {id, name, ...} object; nil when neither is set.
func idNameBrief(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return nil
	}
	out := map[string]any{}
	for _, k := range []string{"id", "name"} {
		if x, ok := m[k]; ok && !isEmptyValue(x) {
			out[k] = x
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

var htmlTagRE = regexp.MustCompile(`</?[A-Za-z][^<>]*>`)

// WorkItemDescriptionSummary is the brief placeholder for description. The count is in
// characters (runes). For non-MARKDOWN descriptions (Projex RICHTEXT is HTML) tags are
// stripped and entities decoded before counting, and the text says so; MARKDOWN is
// counted raw. A non-string description still gets a hint naming its JSON type.
// Missing / null / "" description: no summary.
func WorkItemDescriptionSummary(item map[string]any) (string, bool) {
	raw, ok := item["description"]
	if !ok || raw == nil {
		return "", false
	}
	const tail = "use --full or --fields description"
	d, isStr := raw.(string)
	if !isStr {
		return fmt.Sprintf("(description: non-string %s value, %s)", JSONTypeName(raw), tail), true
	}
	if d == "" {
		return "", false
	}
	format, _ := item["formatType"].(string)
	if !strings.EqualFold(format, "MARKDOWN") && htmlTagRE.MatchString(d) {
		text := html.UnescapeString(htmlTagRE.ReplaceAllString(d, ""))
		return fmt.Sprintf("(description: %d chars of text excluding HTML tags, %s)", utf8.RuneCountInString(text), tail), true
	}
	return fmt.Sprintf("(description: %d chars, %s)", utf8.RuneCountInString(d), tail), true
}

// JSONTypeName names the JSON type of a decoded value (object, array, string, ...).
func JSONTypeName(v any) string {
	switch v.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case bool:
		return "boolean"
	case string:
		return "string"
	case nil:
		return "null"
	}
	return "number"
}

// WorkItemPriority returns the work item's priority:
//  1. a top-level "priority" when the API provides one that is not null / "";
//  2. else the first non-empty value among customFieldValues entries whose fieldId is
//     "priority" or whose fieldName is 优先级 / Priority (fieldIds are often hashes),
//     as {id, displayValue}. id comes from "identifier", or "id" when that is the key
//     present; keys absent from the value are omitted (never {"id": null}). Entries with
//     no values or only empty values are skipped, so duplicates don't hide a real value.
func WorkItemPriority(item map[string]any) (any, bool) {
	if item == nil {
		return nil, false
	}
	if v, ok := item["priority"]; ok && !isEmptyValue(v) {
		return v, true
	}
	cfs, _ := item["customFieldValues"].([]any)
	for _, raw := range cfs {
		cf, _ := raw.(map[string]any)
		if cf == nil || !isPriorityCustomField(cf) {
			continue
		}
		vals, _ := cf["values"].([]any)
		for _, rv := range vals {
			if p := priorityValue(rv); p != nil {
				return p, true
			}
		}
	}
	return nil, false
}

func isPriorityCustomField(cf map[string]any) bool {
	if id, _ := cf["fieldId"].(string); id == WorkItemPriorityFieldID {
		return true
	}
	name, _ := cf["fieldName"].(string)
	name = strings.TrimSpace(name)
	for _, n := range workItemPriorityFieldNames {
		if strings.EqualFold(name, n) {
			return true
		}
	}
	return false
}

func priorityValue(rv any) map[string]any {
	v, _ := rv.(map[string]any)
	if v == nil {
		return nil
	}
	out := map[string]any{}
	for _, k := range []string{"identifier", "id"} {
		if x, ok := v[k]; ok && !isEmptyValue(x) {
			out["id"] = x
			break
		}
	}
	if x, ok := v["displayValue"]; ok && !isEmptyValue(x) {
		out["displayValue"] = x
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

// WorkItemProjection is the result of ProjectWorkItem.
type WorkItemProjection struct {
	Data map[string]any
	// Absent lists known names (schema keys, priority) missing from this work item;
	// they are null in Data. Request order.
	Absent []string
	// PriorityUnresolved is set when "priority" was requested and could not be derived.
	PriorityUnresolved bool
	// Unknown lists names that are neither present nor known. Request order.
	Unknown []string
}

// ProjectWorkItem keeps only the requested top-level keys with raw values (a key
// present with null stays null). "priority" always uses WorkItemPriority, the same
// rule as the brief view, and is null when it can't be derived. Known names missing
// from this item are null (Absent); other names are Unknown.
func ProjectWorkItem(item map[string]any, fields []string) WorkItemProjection {
	res := WorkItemProjection{Data: map[string]any{}}
	for _, f := range fields {
		if f == WorkItemPriorityFieldID {
			p, ok := WorkItemPriority(item)
			res.Data[f] = p
			if !ok {
				res.PriorityUnresolved = true
			}
			continue
		}
		if v, ok := item[f]; ok {
			res.Data[f] = v
			continue
		}
		if WorkItemKnownField(f) {
			res.Data[f] = nil
			res.Absent = append(res.Absent, f)
			continue
		}
		res.Unknown = append(res.Unknown, f)
	}
	return res
}

// WorkItemAvailableFields lists the names --fields accepts for item (sorted): its
// top-level keys, the GetWorkitem schema keys and "priority".
func WorkItemAvailableFields(item map[string]any) []string {
	set := map[string]bool{WorkItemPriorityFieldID: true}
	for k := range item {
		set[k] = true
	}
	for _, k := range workItemSchemaFields {
		set[k] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
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
