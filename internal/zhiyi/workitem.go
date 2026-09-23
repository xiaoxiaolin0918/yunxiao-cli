package zhiyi

import "fmt"

// CurrentStatusID extracts status.id from a work item JSON object (map).
func CurrentStatusID(item map[string]any) string {
	if item == nil {
		return ""
	}
	status, _ := item["status"].(map[string]any)
	if status == nil {
		return ""
	}
	switch v := status["id"].(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	default:
		if v != nil {
			return fmt.Sprint(v)
		}
	}
	return ""
}

// InternalID extracts work item internal id (id or workItemId).
func InternalID(item map[string]any) string {
	if item == nil {
		return ""
	}
	for _, key := range []string{"id", "workItemId"} {
		if v, ok := item[key]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if t != "" {
					return t
				}
			case float64:
				return fmt.Sprintf("%.0f", t)
			default:
				s := fmt.Sprint(t)
				if s != "" && s != "<nil>" {
					return s
				}
			}
		}
	}
	return ""
}

// SerialNumber extracts serialNumber when present.
func SerialNumber(item map[string]any) string {
	if item == nil {
		return ""
	}
	switch v := item["serialNumber"].(type) {
	case string:
		return v
	default:
		if v != nil {
			s := fmt.Sprint(v)
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

// VerifierID extracts verifier user id from a work item (string or {id:…} object).
func VerifierID(item map[string]any) string {
	if item == nil {
		return ""
	}
	switch v := item["verifier"].(type) {
	case string:
		return v
	case map[string]any:
		switch id := v["id"].(type) {
		case string:
			return id
		case float64:
			return fmt.Sprintf("%.0f", id)
		default:
			if id != nil {
				s := fmt.Sprint(id)
				if s != "" && s != "<nil>" {
					return s
				}
			}
		}
	}
	return ""
}
