package workitemfields

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Server-side transition/update 400s report status-entry required fields by Chinese
// display name only ("迭代必填;计划提测时间必填;…", #113). The type-level field config
// marks those fields required:false, so the only way back to a fieldId is comparing
// the names with the field config. Everything here is pure client-side mapping: a
// name that cannot be matched is reported verbatim (unmapped), never guessed.

var (
	// required400BracketRe captures the field name in "【字段名】必填" / "【字段名】"
	// (same shape as zhiyi's cancel-reason parser, kept independent because this one
	// extracts a list, not a single name).
	required400BracketRe = regexp.MustCompile(`【([^】]+)】`)
	// required400PrefixRe captures the run of name characters directly before 必填
	// inside a longer segment ("…：迭代必填" → 迭代).
	required400PrefixRe = regexp.MustCompile(`([^【\]\s:：,，；;]{2,48})必填`)
)

// RequiredFieldMessage returns the human-readable message of an API error body: the
// string under errorMessage / errorMsg / message when the body is a JSON object with
// one of those keys, else the raw body. Unknown shapes pass through unchanged.
func RequiredFieldMessage(body string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		return body
	}
	for _, k := range []string{"errorMessage", "errorMsg", "message"} {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return body
}

// RequiredFieldNames extracts field display-name candidates from a server 必填
// message (#113), in discovery order, deduped. Observed shapes:
//
//	迭代必填;计划提测时间必填;…;验收负责人;VP分配必填   semicolon list; an entry may
//	                                                   lack the 必填 suffix (server-side
//	                                                   inconsistency, still a field name)
//	【截止日期】必填                                     bracketed, also inside JSON bodies
//	<prose>：迭代必填                                    name captured by the prefix regex
//
// Segments without 必填 count as names only when the message contains 必填 elsewhere
// and the segment is a plausible bare field name (plausibleFieldName); anything else
// returns nil so the caller passes the original text through untouched.
func RequiredFieldNames(msg string) []string {
	if !strings.Contains(msg, "必填") {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		name = cleanRequiredName(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	for _, seg := range strings.FieldsFunc(msg, func(r rune) bool {
		return strings.ContainsRune(";；,，\n。", r)
	}) {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if m := required400BracketRe.FindStringSubmatch(seg); len(m) > 1 {
			add(m[1])
		}
		if m := required400PrefixRe.FindStringSubmatch(seg); len(m) > 1 {
			add(m[1])
		}
		if !strings.Contains(seg, "必填") && plausibleFieldName(seg) {
			add(seg) // bare list entry without the 必填 suffix (e.g. 验收负责人)
		}
	}
	// Sweep the whole message too, so 【…】 inside JSON junk is not lost when the
	// caller passes a raw body instead of the extracted message.
	for _, m := range required400BracketRe.FindAllStringSubmatch(msg, -1) {
		add(m[1])
	}
	return out
}

// cleanRequiredName normalizes one candidate: trims, drops 【】 wrapping and a
// trailing 必填, and rejects anything not name-like.
func cleanRequiredName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, "必填")
	name = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(name, "【"), "】"))
	if !plausibleFieldName(name) {
		return ""
	}
	return name
}

// plausibleFieldName keeps bare segments that can be field display names: 2–48 runes
// and no whitespace, JSON/markup punctuation or CJK delimiters. ASCII identifiers
// (ExpCompletionTime) and mixed names (VP分配) pass; URLs, status lines, JSON
// fragments and prose clauses do not.
func plausibleFieldName(name string) bool {
	if name == "" {
		return false
	}
	r := []rune(name)
	if len(r) < 2 || len(r) > 48 {
		return false
	}
	return !strings.ContainsAny(name, " \t\r\n\"'`{}[]()<>|\\/:：;；,，.。@=&%$#!?*+~^")
}

// RequiredHit is a server-reported 必填 field resolved back to its field config (#113).
type RequiredHit struct {
	Field Field
	// RootKey is the named PUT-body key for system root fields (e.g. "sprint"); empty
	// when the field travels under its fieldId (the --custom-fields channel).
	RootKey string
	// Via is the workitem update channel: a root flag ("--sprint") or "--custom-fields".
	Via string
}

// BodyKey is the key the value takes on a transition/update PUT body root: the named
// root key for system fields, else the fieldId (same wire format as
// `workitem update --custom-fields` and `+transition --fields`).
func (h RequiredHit) BodyKey() string {
	if h.RootKey != "" {
		return h.RootKey
	}
	return h.Field.ID
}

// MapRequiredFields matches server field display names against the type's field
// config (name or displayName, trimmed exact; first config entry wins). Unmatched
// names are returned verbatim in unmapped — never guessed.
func MapRequiredFields(fields []Field, names []string) (hits []RequiredHit, unmapped []string) {
	for _, name := range names {
		matched := false
		for _, f := range fields {
			if f.ID == "" {
				continue
			}
			if strings.TrimSpace(f.Name) != name && strings.TrimSpace(f.DisplayName) != name {
				continue
			}
			h := RequiredHit{Field: f}
			if rf, ok := rootFields[f.ID]; ok {
				h.RootKey = rf.key
				h.Via = rootFlagOnly(rf.flag)
			} else {
				h.Via = "--custom-fields"
			}
			hits = append(hits, h)
			matched = true
			break
		}
		if !matched {
			unmapped = append(unmapped, name)
		}
	}
	return hits, unmapped
}

// rootFlagOnly trims create-oriented flag strings ("--subject / --subject-file") to
// the first flag, which is also the update-command flag.
func rootFlagOnly(flag string) string {
	if i := strings.Index(flag, " / "); i > 0 {
		return flag[:i]
	}
	return flag
}

// CurrentValue reports the field's current value on a GetWorkitem payload: the root
// key for root fields (e.g. sprint), else the customFieldValues entry whose fieldId
// matches. nil when absent (the server just rejected the transition for emptiness).
func (h RequiredHit) CurrentValue(item map[string]any) any {
	if item == nil {
		return nil
	}
	if h.RootKey != "" {
		return item[h.RootKey]
	}
	cfs, _ := item["customFieldValues"].([]any)
	for _, raw := range cfs {
		cf, _ := raw.(map[string]any)
		if cf == nil {
			continue
		}
		if id, _ := cf["fieldId"].(string); strings.TrimSpace(id) == h.Field.ID {
			if v, ok := cf["values"]; ok {
				return v
			}
			return cf["value"]
		}
	}
	return nil
}

// RequiredDetail is one mapped 必填 field in error.details.fields (#113).
type RequiredDetail struct {
	FieldID      string          `json:"field_id"`
	Name         string          `json:"name"`
	Format       string          `json:"format,omitempty"`
	Type         string          `json:"type,omitempty"`
	PassVia      string          `json:"pass_via"`
	CurrentValue any             `json:"current_value"`
	Options      []MissingOption `json:"options,omitempty"`
	OptionsTotal int             `json:"options_total,omitempty"`
	Draft        string          `json:"draft"`
}

// RequiredDetails builds the error.details.fields entries from hits; item supplies
// current values (may be nil). Options are capped at MaxOptions like the create
// precheck, with OptionsTotal keeping the real count.
func RequiredDetails(hits []RequiredHit, item map[string]any) []RequiredDetail {
	out := make([]RequiredDetail, 0, len(hits))
	for _, h := range hits {
		name := h.Field.Name
		if name == "" {
			name = h.Field.DisplayName
		}
		d := RequiredDetail{
			FieldID:      h.Field.ID,
			Name:         name,
			Format:       h.Field.Format,
			Type:         h.Field.Type,
			PassVia:      h.Via,
			CurrentValue: h.CurrentValue(item),
			Draft:        h.Draft(),
		}
		if n := len(h.Field.Options); n > 0 {
			d.OptionsTotal = n
			for i, o := range h.Field.Options {
				if i == MaxOptions {
					break
				}
				mo := MissingOption{ID: o.ID, DisplayValue: o.DisplayValue}
				if o.Value != o.DisplayValue {
					mo.Value = o.Value
				}
				d.Options = append(d.Options, mo)
			}
		}
		out = append(out, d)
	}
	return out
}

// Draft is the copy-paste fill hint for this field on `workitem update`: --sprint for
// the sprint root field, the root flag for other system fields, else a
// --custom-fields JSON skeleton (option ids for enums, a date for date formats).
// `+transition` takes the same keys via --fields (FieldsDraft).
func (h RequiredHit) Draft() string {
	if h.Field.ID == "sprint" || h.RootKey == "sprint" {
		return "--sprint <sprintId>"
	}
	if h.RootKey != "" {
		return h.Via + " <value>"
	}
	return fmt.Sprintf("--custom-fields '{\"%s\":\"%s\"}'", h.Field.ID, h.valuePlaceholder())
}

func (h RequiredHit) valuePlaceholder() string {
	switch {
	case len(h.Field.Options) > 0:
		return "<option id>"
	case strings.EqualFold(h.Field.Format, "dateTime") || strings.EqualFold(h.Field.Format, "date"):
		return "<date>"
	default:
		return "<value>"
	}
}

// FieldsDraft is the --fields skeleton for retrying the failed transition: PUT-body
// root key → value placeholder. Root fields keep their named key so values reach the
// right channel (sprint ≠ customFields, #113).
func FieldsDraft(hits []RequiredHit) map[string]any {
	out := make(map[string]any, len(hits))
	for _, h := range hits {
		ph := h.valuePlaceholder()
		if h.Field.ID == "sprint" || h.RootKey == "sprint" {
			ph = "<sprintId>"
		}
		out[h.BodyKey()] = ph
	}
	return out
}
