// Package workitemfields parses Projex work item type field config
// (GET .../projects/{space}/workitemTypes/{typeId}/fields, GetWorkitemTypeFieldConfig)
// and prechecks a create body for missing required fields (#95), so all missing
// fields are reported at once instead of one server 400 per round trip.
package workitemfields

import (
	"fmt"
	"strconv"
	"strings"
)

// MaxOptions caps options listed per missing field (OptionsTotal keeps the real count).
const MaxOptions = 20

// Option is one selectable value of a list-like field.
type Option struct {
	ID           string
	Value        string
	DisplayValue string
}

// Field is the subset of a field config item used by the precheck.
type Field struct {
	ID             string
	Name           string
	Format         string // list | multiList | user | sprint | …
	Type           string // NativeField | SystemCustomField | CustomField
	Required       bool
	ShowWhenCreate *bool // nil when the API omits it
	DefaultValue   string
	Options        []Option
}

// MissingOption is an Option in precheck output (JSON snake_case).
type MissingOption struct {
	ID           string `json:"id"`
	DisplayValue string `json:"display_value,omitempty"`
	Value        string `json:"value,omitempty"`
}

// Missing describes one required field absent from the create body.
type Missing struct {
	FieldID      string          `json:"field_id"`
	Name         string          `json:"name"`
	Format       string          `json:"format,omitempty"`
	Type         string          `json:"type,omitempty"`
	PassVia      string          `json:"pass_via"` // CLI flag(s), or "customFieldValues"
	Options      []MissingOption `json:"options,omitempty"`
	OptionsTotal int             `json:"options_total,omitempty"`
}

// rootField maps a field config id to the create body root key and its CLI flag(s).
type rootField struct{ key, flag string }

var rootFields = map[string]rootField{
	"subject":      {"subject", "--subject / --subject-file"},
	"assignedTo":   {"assignedTo", "--assigned-to"},
	"description":  {"description", "--description / --description-file"},
	"sprint":       {"sprint", "--sprint"},
	"labels":       {"labels", "--labels"},
	"tag":          {"labels", "--labels"},
	"tags":         {"labels", "--labels"},
	"participants": {"participants", "--participants"},
	"participant":  {"participants", "--participants"},
	"trackers":     {"trackers", "--trackers"},
	"tracker":      {"trackers", "--trackers"},
	"verifier":     {"verifier", "--verifier"},
	"versions":     {"versions", "--versions"},
	"version":      {"versions", "--versions"},
	"parentId":     {"parentId", "--parent-id"},
	"parent":       {"parentId", "--parent-id"},
}

// serverManaged are native fields the server fills on create (never user input).
var serverManaged = map[string]bool{
	"status": true, "workitemType": true, "workitemTypeId": true, "creator": true, "modifier": true,
	"space": true, "spaceId": true, "spaceType": true, "project": true, "projectId": true,
	"gmtCreate": true, "gmtModified": true, "serialNumber": true, "identifier": true, "id": true,
	"logicalStatus": true, "finishTime": true, "updateStatusAt": true, "statusStageIdentifier": true,
	"category": true,
}

// Parse extracts field configs from a decoded response: a top-level array, or an
// object wrapping it under result / data / fields / items. Also accepts the
// ListWorkItemAllFields spelling (identifier / isRequired / isShowWhenCreate).
func Parse(raw any) ([]Field, error) {
	list, ok := raw.([]any)
	if !ok {
		m, isMap := raw.(map[string]any)
		if !isMap {
			return nil, fmt.Errorf("unexpected fields payload %T (want array)", raw)
		}
		for _, k := range []string{"result", "data", "fields", "items"} {
			if l, ok := m[k].([]any); ok {
				list = l
				break
			}
		}
		if list == nil {
			return nil, fmt.Errorf("unexpected fields payload: object without result/data/fields/items array")
		}
	}
	out := make([]Field, 0, len(list))
	for _, it := range list {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		f := Field{
			ID:           firstNonEmpty(str(m["id"]), str(m["identifier"]), str(m["fieldIdentifier"])),
			Name:         str(m["name"]),
			Format:       str(m["format"]),
			Type:         firstNonEmpty(str(m["type"]), str(m["className"])),
			Required:     boolOf(m["required"]) || boolOf(m["isRequired"]),
			DefaultValue: str(m["defaultValue"]),
		}
		if v, ok := m["showWhenCreate"].(bool); ok {
			f.ShowWhenCreate = &v
		} else if v, ok := m["isShowWhenCreate"].(bool); ok {
			f.ShowWhenCreate = &v
		}
		if opts, ok := m["options"].([]any); ok {
			for _, o := range opts {
				om, ok := o.(map[string]any)
				if !ok {
					continue
				}
				f.Options = append(f.Options, Option{
					ID:           firstNonEmpty(str(om["id"]), str(om["identifier"])),
					Value:        str(om["value"]),
					DisplayValue: firstNonEmpty(str(om["displayValue"]), str(om["value"])),
				})
			}
		}
		if f.ID != "" {
			out = append(out, f)
		}
	}
	return out, nil
}

// MissingRequired returns required fields absent from a create body, in config order,
// and how many required fields were checked. It never mutates body.
//
// Checked = required && showWhenCreate != false && not server-managed (status, creator, …)
// && no server defaultValue (see DefaultSkipped). Root fields (subject, assignedTo,
// sprint, labels, …) count as present only when set on the body root (where their CLI
// flag puts them), never via customFieldValues; other fields only when set in
// customFieldValues. Blank strings / empty lists / null are missing; numbers and bools
// are present.
func MissingRequired(fields []Field, body map[string]any) (missing []Missing, checked int) {
	cf, _ := body["customFieldValues"].(map[string]any)
	for _, f := range fields {
		if !userRequired(f) || f.DefaultValue != "" {
			continue
		}
		checked++
		passVia := "customFieldValues"
		present := isSet(cf[f.ID])
		if rf, ok := rootFields[f.ID]; ok {
			passVia = rf.flag
			present = isSet(body[rf.key])
		}
		if present {
			continue
		}
		ms := Missing{FieldID: f.ID, Name: f.Name, Format: f.Format, Type: f.Type, PassVia: passVia}
		if n := len(f.Options); n > 0 {
			ms.OptionsTotal = n
			for i, o := range f.Options {
				if i == MaxOptions {
					break
				}
				mo := MissingOption{ID: o.ID, DisplayValue: o.DisplayValue}
				if o.Value != o.DisplayValue {
					mo.Value = o.Value
				}
				ms.Options = append(ms.Options, mo)
			}
		}
		missing = append(missing, ms)
	}
	return missing, checked
}

// DefaultSkipped returns the ids of required, create-visible user fields that
// MissingRequired skips because the server config carries a defaultValue (assumed to be
// auto-filled on create; unverified), in config order.
func DefaultSkipped(fields []Field) []string {
	var out []string
	for _, f := range fields {
		if userRequired(f) && f.DefaultValue != "" {
			out = append(out, f.ID)
		}
	}
	return out
}

// userRequired: required, shown on create (or unknown) and not server-managed.
func userRequired(f Field) bool {
	if !f.Required || serverManaged[f.ID] {
		return false
	}
	return f.ShowWhenCreate == nil || *f.ShowWhenCreate
}

func isSet(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(t) != ""
	case []any:
		return len(t) > 0
	case []string:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	default:
		return true
	}
}

func boolOf(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		b, _ := strconv.ParseBool(strings.TrimSpace(t))
		return b
	}
	return false
}

func str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}