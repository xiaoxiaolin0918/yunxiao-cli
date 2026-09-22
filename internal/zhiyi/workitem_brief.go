package zhiyi

import (
	"fmt"
	"strings"
)

// BriefWorkItem returns a create/get-friendly summary: keeps id, serialNumber,
// status (id + displayName), and subject; drops description and other noise
// (same direction as MR brief in #50; issue #62).
func BriefWorkItem(item map[string]any) map[string]any {
	if item == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if id := InternalID(item); id != "" {
		out["id"] = id
	}
	if sn := SerialNumber(item); sn != "" {
		out["serialNumber"] = sn
	}
	if st := StatusBrief(item); len(st) > 0 {
		out["status"] = st
	}
	if subj := Subject(item); subj != "" {
		out["subject"] = subj
	}
	return out
}

// StatusBrief keeps status.id and status.displayName (falls back to name).
func StatusBrief(item map[string]any) map[string]any {
	if item == nil {
		return nil
	}
	status, _ := item["status"].(map[string]any)
	if status == nil {
		return nil
	}
	out := map[string]any{}
	if id := statusIDString(status); id != "" {
		out["id"] = id
	}
	if dn := statusDisplayName(status); dn != "" {
		out["displayName"] = dn
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func statusIDString(status map[string]any) string {
	if status == nil {
		return ""
	}
	switch v := status["id"].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	default:
		if v != nil {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func statusDisplayName(status map[string]any) string {
	if status == nil {
		return ""
	}
	for _, key := range []string{"displayName", "name", "display_name"} {
		if v, ok := status[key]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

// WorkItemCreateIncomplete reports create payloads missing serialNumber or status.
func WorkItemCreateIncomplete(item map[string]any) bool {
	if item == nil {
		return true
	}
	if SerialNumber(item) == "" {
		return true
	}
	st, _ := item["status"].(map[string]any)
	if st == nil {
		return true
	}
	if statusDisplayName(st) == "" && statusIDString(st) == "" {
		return true
	}
	return false
}

// EnsureWorkItemCreateFields re-GETs the work item when create returned null
// serialNumber/status (common Yunxiao create response). fetch is best-effort:
// on error the original create payload is returned.
func EnsureWorkItemCreateFields(created map[string]any, fetch func(id string) (map[string]any, error)) map[string]any {
	if !WorkItemCreateIncomplete(created) {
		return created
	}
	id := InternalID(created)
	if id == "" || fetch == nil {
		return created
	}
	full, err := fetch(id)
	if err != nil || full == nil {
		return created
	}
	return full
}
