// Option display-value resolution (#126): list/multiList custom field values in a
// create body may be given as the option's display value (e.g. {"priority":"高"})
// instead of the opaque option id. ResolveOptionValues maps them to option ids
// using the same field config the #95 precheck reads, so callers no longer need to
// hardcode option id tables that silently rot when ids change.
package workitemfields

import "strings"

// ResolvedValue records one display value mapped to its option id (JSON snake_case).
type ResolvedValue struct {
	FieldID   string `json:"field_id"`
	FieldName string `json:"field_name,omitempty"`
	From      string `json:"from"`
	To        string `json:"to"`
}

// InvalidValue describes one value that could not be resolved: it matches no option
// id or display value (Reason "not_found"), or its display value matches several
// options with different ids (Reason "ambiguous"). Options lists the valid choices
// for "not_found" (capped at MaxOptions; OptionsTotal keeps the real count) and only
// the colliding ones for "ambiguous".
type InvalidValue struct {
	FieldID      string          `json:"field_id"`
	Name         string          `json:"name"`
	Format       string          `json:"format,omitempty"`
	Value        string          `json:"value"`
	Reason       string          `json:"reason"` // not_found | ambiguous
	Options      []MissingOption `json:"options,omitempty"`
	OptionsTotal int             `json:"options_total,omitempty"`
}

// matchKind classifies how a string value relates to a field's options.
type matchKind int

const (
	matchNone      matchKind = iota // no option id or display value matches
	matchID                         // value is exactly an option id: keep as-is
	matchDisplay                    // value is a display value: rewrite to the option id
	matchAmbiguous                  // display value matches several options with different ids
)

// ResolveOptionValues rewrites cf (the create body's customFieldValues map) in
// place: for every list/multiList field that the config gives options for, an exact
// option id passes through unchanged, an exact display value is replaced by that
// option's id, and anything else is reported in invalid (so the caller can fail
// with the field's valid values instead of a vague server error). multiList arrays
// are resolved element by element. Fields without options, fields absent from the
// config, non-list formats, blank values and non-string values pass through
// untouched (the server validates them as before). Invalid values are reported in
// field-config order; cf is only mutated when no value of that field is invalid.
func ResolveOptionValues(fields []Field, cf map[string]any) (resolved []ResolvedValue, invalid []InvalidValue) {
	for _, f := range fields {
		if !optionField(f) {
			continue
		}
		switch v := cf[f.ID].(type) {
		case string:
			id, rec, inv, kind := resolveString(f, v)
			switch kind {
			case matchID:
				if id != "" && id != v {
					cf[f.ID] = id // exact id with surrounding blanks: send the canonical id
				}
			case matchDisplay:
				cf[f.ID] = id
				resolved = append(resolved, rec)
			case matchAmbiguous, matchNone:
				invalid = append(invalid, inv)
			}
		case []any:
			out := make([]any, len(v))
			copy(out, v)
			bad := false
			for i, el := range out {
				s, ok := el.(string)
				if !ok {
					continue
				}
				id, rec, inv, kind := resolveString(f, s)
				switch kind {
				case matchID:
					if id != "" && id != s {
						out[i] = id
					}
				case matchDisplay:
					out[i] = id
					resolved = append(resolved, rec)
				case matchAmbiguous, matchNone:
					invalid = append(invalid, inv)
					bad = true
				}
			}
			if !bad {
				cf[f.ID] = out
			}
		case []string:
			out := make([]string, len(v))
			copy(out, v)
			bad := false
			for i, s := range out {
				id, rec, inv, kind := resolveString(f, s)
				switch kind {
				case matchID:
					if id != "" && id != s {
						out[i] = id
					}
				case matchDisplay:
					out[i] = id
					resolved = append(resolved, rec)
				case matchAmbiguous, matchNone:
					invalid = append(invalid, inv)
					bad = true
				}
			}
			if !bad {
				cf[f.ID] = out
			}
		}
	}
	return resolved, invalid
}

// optionField: resolution only touches list-like fields the config gives options
// for. Other formats (user, text, date, …) pass through even if they carry an
// options array, so free-form input never fails client-side on them.
func optionField(f Field) bool {
	if len(f.Options) == 0 {
		return false
	}
	return f.Format == "list" || f.Format == "multiList"
}

// resolveString classifies one string value against the field's options. For
// matchDisplay it also returns the rewritten id and a ResolvedValue record; for
// matchNone / matchAmbiguous it returns the InvalidValue to report.
func resolveString(f Field, value string) (id string, rec ResolvedValue, inv InvalidValue, kind matchKind) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		// Blank stays blank: the precheck reports it as a missing required field.
		return "", rec, inv, matchID
	}
	for _, o := range f.Options {
		if o.ID != "" && o.ID == trimmed {
			return o.ID, rec, inv, matchID
		}
	}
	var hits []Option
	for _, o := range f.Options {
		if o.DisplayValue == trimmed || (o.Value != "" && o.Value == trimmed) {
			hits = append(hits, o)
		}
	}
	if len(hits) > 0 && sameOptionIDs(hits) {
		id = hits[0].ID
		return id, ResolvedValue{FieldID: f.ID, FieldName: f.Name, From: trimmed, To: id}, inv, matchDisplay
	}
	inv = InvalidValue{FieldID: f.ID, Name: f.Name, Format: f.Format, Value: trimmed}
	if len(hits) > 1 {
		inv.Reason = "ambiguous"
	} else {
		inv.Reason = "not_found"
	}
	opts := hits
	if len(hits) == 0 {
		opts = f.Options
	}
	inv.OptionsTotal = len(opts)
	for i, o := range opts {
		if i == MaxOptions {
			break
		}
		mo := MissingOption{ID: o.ID, DisplayValue: o.DisplayValue}
		if o.Value != o.DisplayValue {
			mo.Value = o.Value
		}
		inv.Options = append(inv.Options, mo)
	}
	return "", rec, inv, matchAmbiguous
}

// sameOptionIDs: several display matches that all point at one id still resolve.
func sameOptionIDs(opts []Option) bool {
	if len(opts) < 2 {
		return true
	}
	for _, o := range opts[1:] {
		if o.ID != opts[0].ID {
			return false
		}
	}
	return true
}
