package profile

import (
	"fmt"
	"sort"
	"strings"
)

// CategoryTypeCandidate is one workitem type id usable for a Projex category
// (Req|Bug|Task|Risk|Topic|…). Source is one of:
//
//	"profile"          explicit profile key (bug_type_id / risk_type_id / req_type_id)
//	"workitem_defaults" workitem_defaults[<type_id>].category matched
//	"workflows"         workflows[<type_id>].category matched
type CategoryTypeCandidate struct {
	TypeID string `json:"type_id"`
	Name   string `json:"name,omitempty"`
	Source string `json:"source"`
}

// explicitCategoryTypeID returns the profile key for a category, if any.
// Unknown categories have no explicit key ("").
func (p *Profile) explicitCategoryTypeID(category string) string {
	if p == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "bug":
		return strings.TrimSpace(p.BugTypeID)
	case "risk":
		return strings.TrimSpace(p.RiskTypeID)
	case "req":
		return strings.TrimSpace(p.ReqTypeID)
	default:
		return ""
	}
}

// CategoryTypeCandidates lists candidate type ids for a Projex category, explicit
// profile key first, then workitem_defaults and workflows entries whose category
// matches (case-insensitive), deduped by type id (defaults source wins name/source).
func (p *Profile) CategoryTypeCandidates(category string) []CategoryTypeCandidate {
	if p == nil {
		return nil
	}
	cat := strings.ToLower(strings.TrimSpace(category))
	order := []string{}
	seen := map[string]CategoryTypeCandidate{}
	add := func(typeID, name, source string) {
		typeID = strings.TrimSpace(typeID)
		if typeID == "" {
			return
		}
		if cur, ok := seen[typeID]; ok {
			// Keep the richer entry; only fill missing name.
			if cur.Name == "" && name != "" {
				cur.Name = name
				seen[typeID] = cur
			}
			return
		}
		seen[typeID] = CategoryTypeCandidate{TypeID: typeID, Name: strings.TrimSpace(name), Source: source}
		order = append(order, typeID)
	}
	if id := p.explicitCategoryTypeID(cat); id != "" {
		add(id, "", "profile")
	}
	for typeID, d := range p.WorkitemDefaults {
		if strings.EqualFold(strings.TrimSpace(d.Category), cat) {
			add(typeID, d.Name, "workitem_defaults")
		}
	}
	for typeID, wf := range p.Workflows {
		if strings.EqualFold(strings.TrimSpace(wf.Category), cat) {
			add(typeID, wf.Name, "workflows")
		}
	}
	out := make([]CategoryTypeCandidate, 0, len(order))
	for _, id := range order {
		out = append(out, seen[id])
	}
	return out
}

// ResolveCategoryTypeID picks the type id to use for +risk-create / +req-create
// (and any future category shortcut): the explicit profile key wins; otherwise a
// single discovered candidate is used; zero or several discovered candidates error
// with the candidate list (never guess — different type ids mean different workflows
// and required fields). Callers may override with an explicit --type-id flag.
func (p *Profile) ResolveCategoryTypeID(category string) (string, error) {
	if p == nil {
		return "", fmt.Errorf("profile required to resolve %s type id", category)
	}
	cat := strings.TrimSpace(category)
	if id := p.explicitCategoryTypeID(cat); id != "" {
		return id, nil
	}
	cands := p.CategoryTypeCandidates(cat)
	if len(cands) == 0 {
		key := strings.ToLower(cat) + "_type_id"
		return "", fmt.Errorf("profile %s has no %s workitem type: set profile %s (or add workitem_defaults/workflows entries with category %q); find ids with: yunxiao workitem types list --space-id %s --category %s", p.Name, cat, key, cat, p.SpaceID, cat)
	}
	if len(cands) > 1 {
		parts := make([]string, 0, len(cands))
		for _, c := range cands {
			if c.Name != "" {
				parts = append(parts, fmt.Sprintf("%s (%s, %s)", c.TypeID, c.Name, c.Source))
			} else {
				parts = append(parts, fmt.Sprintf("%s (%s)", c.TypeID, c.Source))
			}
		}
		sort.Strings(parts)
		return "", fmt.Errorf("profile %s has multiple %s workitem types: %s — set profile %s_type_id (or pass --type-id) to disambiguate", p.Name, cat, strings.Join(parts, ", "), strings.ToLower(cat))
	}
	return cands[0].TypeID, nil
}

// typedPriorityDefault returns workitem_defaults[typeID].fields["priority"] when set.
func (p *Profile) typedPriorityDefault(typeID string) (WorkitemDefaultField, bool) {
	if p == nil || p.WorkitemDefaults == nil {
		return WorkitemDefaultField{}, false
	}
	typeID = strings.TrimSpace(typeID)
	if typeID == "" {
		return WorkitemDefaultField{}, false
	}
	defs, ok := p.WorkitemDefaults[typeID]
	if !ok || defs.Fields == nil {
		return WorkitemDefaultField{}, false
	}
	f, ok := defs.Fields["priority"]
	if !ok || isEmptyDefault(f.Value) {
		return WorkitemDefaultField{}, false
	}
	return f, true
}

func defaultFieldValueString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return fmt.Sprintf("%.0f", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case int:
		return fmt.Sprintf("%d", t)
	default:
		return ""
	}
}

// ResolveTypedCreatePriority resolves a --priority value for typed (non-bug) creates
// (+risk-create / +req-create). Resolution order:
//
//  1. exact match of workitem_defaults[typeID].fields.priority.display (显示值, e.g. 高/中)
//  2. bug_create_fields.priority alias map (priority option ids are space-scoped, so the
//     +bug-create map is reused; same known-alias rules)
//  3. the "medium" default falls back to workitem_defaults[typeID].fields.priority.value
//  4. known alias (urgent/high/medium/low) with no mapping → error (avoids API 400
//     "字段【优先级】所填值无效")
//  5. anything else passes through as a raw option id
//
// Empty input returns "" (priority omitted; workitem_defaults may still fill it).
func (p *Profile) ResolveTypedCreatePriority(typeID, aliasOrID string) (string, error) {
	aliasOrID = strings.TrimSpace(aliasOrID)
	if aliasOrID == "" {
		return "", nil
	}
	if f, ok := p.typedPriorityDefault(typeID); ok {
		if display, ok := f.Display.(string); ok && strings.TrimSpace(display) == aliasOrID {
			return defaultFieldValueString(f.Value), nil
		}
	}
	if id, err := p.ResolvePriorityID(aliasOrID); err == nil && strings.TrimSpace(id) != "" {
		return id, nil
	}
	if strings.EqualFold(aliasOrID, "medium") {
		if f, ok := p.typedPriorityDefault(typeID); ok {
			return defaultFieldValueString(f.Value), nil
		}
	}
	if isKnownPriorityAlias(aliasOrID) {
		return "", fmt.Errorf("priority alias %q is not mapped to an option id; set profile bug_create_fields.priority[%q] or workitem_defaults[%s].fields.priority.display, or pass the option id. Hint: yunxiao profile doctor", aliasOrID, strings.ToLower(aliasOrID), strings.TrimSpace(typeID))
	}
	return aliasOrID, nil
}
