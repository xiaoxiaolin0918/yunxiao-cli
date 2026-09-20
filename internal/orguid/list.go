package orguid

// MembersFromAPI unwraps common list envelopes into a slice of items.
func MembersFromAPI(out any) []any {
	switch x := out.(type) {
	case []any:
		return x
	case map[string]any:
		for _, k := range []string{"data", "members", "result", "items"} {
			if v, ok := x[k].([]any); ok {
				return v
			}
		}
	}
	return nil
}

// ReplaceMembersInAPI writes members back into the same envelope shape as out.
// Bare slices stay slices; wrapped maps keep their original key (data|members|result|items).
func ReplaceMembersInAPI(out any, members []any) any {
	switch x := out.(type) {
	case []any:
		return members
	case map[string]any:
		cp := make(map[string]any, len(x)+1)
		for k, v := range x {
			cp[k] = v
		}
		for _, k := range []string{"data", "members", "result", "items"} {
			if _, ok := x[k].([]any); ok {
				cp[k] = members
				return cp
			}
		}
		cp["data"] = members
		return cp
	default:
		return members
	}
}
