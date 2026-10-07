package workitemfields

import (
	"fmt"
	"strings"
)

// listFormats are field formats whose values are option ids (or lists of them).
var listFormats = map[string]bool{
	"list":      true,
	"multiList": true,
	"multilist": true,
}

// ResolveOptionID maps a user-facing value to an option id for a list-like field.
// Matching order: exact option id, then displayValue, then value (case-sensitive trim).
// Empty raw is returned unchanged. Non-list fields / fields without options pass through.
// On ambiguity or no match, returns an error listing valid display values (and ids).
func ResolveOptionID(f Field, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !listFormats[f.Format] || len(f.Options) == 0 {
		return raw, nil
	}
	for _, o := range f.Options {
		if o.ID == raw {
			return o.ID, nil
		}
	}
	var byDisplay, byValue []Option
	for _, o := range f.Options {
		if o.DisplayValue == raw {
			byDisplay = append(byDisplay, o)
		}
		if o.Value == raw && o.Value != o.DisplayValue {
			byValue = append(byValue, o)
		}
	}
	matches := byDisplay
	if len(matches) == 0 {
		matches = byValue
	}
	if len(matches) == 1 {
		return matches[0].ID, nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("field %s (%s): value %q matches %d options; use option id instead (%s)",
			f.Name, f.ID, raw, len(matches), formatOptionList(f.Options))
	}
	return "", fmt.Errorf("field %s (%s): unknown value %q; valid: %s",
		f.Name, f.ID, raw, formatOptionList(f.Options))
}

func formatOptionList(opts []Option) string {
	parts := make([]string, 0, len(opts))
	for i, o := range opts {
		if i == MaxOptions {
			parts = append(parts, fmt.Sprintf("…(+%d more)", len(opts)-MaxOptions))
			break
		}
		label := o.DisplayValue
		if label == "" {
			label = o.Value
		}
		if label == "" {
			parts = append(parts, o.ID)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", label, o.ID))
	}
	return strings.Join(parts, ", ")
}

// ResolveCustomFieldValues mutates cf in place: for each list/multiList field present
// in fields, replace display values with option ids. String values are resolved;
// []any / []string entries are resolved element-wise. Unknown field ids are left as-is.
// Returns the first resolve error (with valid options listed).
func ResolveCustomFieldValues(fields []Field, cf map[string]any) error {
	if cf == nil || len(fields) == 0 {
		return nil
	}
	byID := make(map[string]Field, len(fields))
	for _, f := range fields {
		byID[f.ID] = f
	}
	for id, v := range cf {
		f, ok := byID[id]
		if !ok || !listFormats[f.Format] || len(f.Options) == 0 {
			continue
		}
		resolved, err := resolveAny(f, v)
		if err != nil {
			return err
		}
		cf[id] = resolved
	}
	return nil
}

func resolveAny(f Field, v any) (any, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case string:
		return ResolveOptionID(f, t)
	case []string:
		out := make([]string, len(t))
		for i, s := range t {
			id, err := ResolveOptionID(f, s)
			if err != nil {
				return nil, err
			}
			out[i] = id
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, el := range t {
			s, ok := el.(string)
			if !ok {
				out[i] = el
				continue
			}
			id, err := ResolveOptionID(f, s)
			if err != nil {
				return nil, err
			}
			out[i] = id
		}
		return out, nil
	default:
		return v, nil
	}
}

// IndexByID returns fields keyed by id.
func IndexByID(fields []Field) map[string]Field {
	out := make(map[string]Field, len(fields))
	for _, f := range fields {
		out[f.ID] = f
	}
	return out
}
