package zhiyi

import (
	"fmt"
	"strings"
)

// UserRef is a person field value with an optional display name (#414).
type UserRef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Empty reports whether both id and name are unset.
func (u UserRef) Empty() bool {
	return strings.TrimSpace(u.ID) == "" && strings.TrimSpace(u.Name) == ""
}

// Display formats "Name (id)", falling back to the id or "（空）".
func (u UserRef) Display() string {
	name := strings.TrimSpace(u.Name)
	id := strings.TrimSpace(u.ID)
	switch {
	case name != "" && id != "":
		return name + " (" + id + ")"
	case name != "":
		return name
	case id != "":
		return id
	default:
		return "（空）"
	}
}

// EqualUserRef compares by id when either side has one; otherwise by name.
func EqualUserRef(a, b UserRef) bool {
	aid, bid := strings.TrimSpace(a.ID), strings.TrimSpace(b.ID)
	if aid != "" || bid != "" {
		return aid == bid
	}
	return strings.TrimSpace(a.Name) == strings.TrimSpace(b.Name)
}

// PersonnelChange is one people-field mutation observed after a transition refresh.
type PersonnelChange struct {
	Field   string  `json:"field"`
	From    UserRef `json:"from"`
	To      UserRef `json:"to"`
	Warning string  `json:"warning"`
	Restore string  `json:"restore,omitempty"`
}

// PersonnelDiffOpts names the work item and optional custom field ids to compare.
type PersonnelDiffOpts struct {
	ID                 string
	DeveloperFieldID   string
	ResponsibleFieldID string
}

// UserRefFromValue normalizes a string id or {id,name}/{identifier,displayValue} object (or a list of those).
func UserRefFromValue(v any) UserRef {
	switch t := v.(type) {
	case nil:
		return UserRef{}
	case string:
		s := strings.TrimSpace(t)
		if s == "" || s == "<nil>" {
			return UserRef{}
		}
		return UserRef{ID: s}
	case map[string]any:
		return UserRef{
			ID:   firstStringField(t, "id", "identifier", "userId"),
			Name: firstStringField(t, "name", "displayValue", "displayName"),
		}
	case []any:
		if len(t) == 0 {
			return UserRef{}
		}
		return UserRefFromValue(t[0])
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "" || s == "<nil>" {
			return UserRef{}
		}
		return UserRef{ID: s}
	}
}

// UserRefFromItem reads a top-level people field (assignedTo, verifier, …).
func UserRefFromItem(item map[string]any, key string) UserRef {
	if item == nil {
		return UserRef{}
	}
	return UserRefFromValue(item[key])
}

// UserRefFromCustomField reads a user-typed custom field by fieldId.
func UserRefFromCustomField(item map[string]any, fieldID string) UserRef {
	fieldID = strings.TrimSpace(fieldID)
	if item == nil || fieldID == "" {
		return UserRef{}
	}
	switch cf := item["customFieldValues"].(type) {
	case []any:
		for _, raw := range cf {
			m, _ := raw.(map[string]any)
			if m == nil {
				continue
			}
			id := firstStringField(m, "fieldId", "id")
			if id != fieldID {
				continue
			}
			if vals, ok := m["values"]; ok {
				return UserRefFromValue(vals)
			}
			if v, ok := m["value"]; ok {
				return UserRefFromValue(v)
			}
		}
	case map[string]any:
		if v, ok := cf[fieldID]; ok {
			return UserRefFromValue(v)
		}
	}
	if v, ok := item[fieldID]; ok {
		return UserRefFromValue(v)
	}
	return UserRef{}
}

// DiffWorkItemPersonnel compares assignedTo, verifier, and optional developer /
// responsible-person custom fields between the pre-transition item and the refresh.
func DiffWorkItemPersonnel(before, after map[string]any, opts PersonnelDiffOpts) []PersonnelChange {
	if after == nil {
		return nil
	}
	type field struct {
		name string
		from UserRef
		to   UserRef
	}
	fields := []field{
		{"assignedTo", UserRefFromItem(before, "assignedTo"), UserRefFromItem(after, "assignedTo")},
		{"verifier", UserRefFromItem(before, "verifier"), UserRefFromItem(after, "verifier")},
	}
	if id := strings.TrimSpace(opts.DeveloperFieldID); id != "" {
		fields = append(fields, field{"developer", UserRefFromCustomField(before, id), UserRefFromCustomField(after, id)})
	}
	if id := strings.TrimSpace(opts.ResponsibleFieldID); id != "" {
		fields = append(fields, field{"responsiblePerson", UserRefFromCustomField(before, id), UserRefFromCustomField(after, id)})
	}
	var out []PersonnelChange
	for _, f := range fields {
		if EqualUserRef(f.from, f.to) {
			continue
		}
		msg := fmt.Sprintf("%s 已由 %s 变为 %s（平台或字段联动）", f.name, f.from.Display(), f.to.Display())
		ch := PersonnelChange{Field: f.name, From: f.from, To: f.to, Warning: msg}
		if f.name == "assignedTo" && strings.TrimSpace(f.from.ID) != "" {
			wid := strings.TrimSpace(opts.ID)
			if wid == "" {
				wid = "<id>"
			}
			ch.Restore = fmt.Sprintf("yunxiao workitem update --id %s --assigned-to %s", wid, strings.TrimSpace(f.from.ID))
			ch.Warning = msg + "；恢复: " + ch.Restore
		}
		out = append(out, ch)
	}
	return out
}

// AttachPersonnelBrief copies assignedTo and verifier onto a brief projection when
// they are present and non-empty, so brief output does not look like the verifier
// was lost (#414). Null / empty values are omitted (never "None").
func AttachPersonnelBrief(brief, src map[string]any) {
	if brief == nil || src == nil {
		return
	}
	for _, k := range []string{"assignedTo", "verifier"} {
		if v, ok := src[k]; ok && !isEmptyValue(v) {
			brief[k] = v
		}
	}
}
