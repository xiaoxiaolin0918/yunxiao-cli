package zhiyi

import (
	"fmt"
	"strings"
)

// CategoryPathSegment maps workitem categoryId to Projex URL path segment.
// Known: Req→req, Bug→bug, Task→task, Risk→risk, Topic→topic, Request→req.
// Unknown categories fall back to "req" (openWorkitemIdentifier still works).
func CategoryPathSegment(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "bug":
		return "bug"
	case "task":
		return "task"
	case "risk":
		return "risk"
	case "topic":
		return "topic"
	case "req", "request", "":
		return "req"
	default:
		return "req"
	}
}

// SpaceIDFromItem extracts space id from item.space.id or spaceId.
func SpaceIDFromItem(item map[string]any) string {
	if item == nil {
		return ""
	}
	if v, ok := item["spaceId"]; ok && v != nil {
		s := strings.TrimSpace(fmt.Sprint(v))
		if s != "" && s != "<nil>" {
			return s
		}
	}
	if space, ok := item["space"].(map[string]any); ok && space != nil {
		if v, ok := space["id"]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	if nested, ok := item["data"].(map[string]any); ok {
		return SpaceIDFromItem(nested)
	}
	return ""
}

// ResolveSpaceID prefers profile space, then item.space.id, then flag --space-id.
func ResolveSpaceID(item map[string]any, profileSpace, flagSpace string) string {
	if s := strings.TrimSpace(profileSpace); s != "" {
		return s
	}
	if s := SpaceIDFromItem(item); s != "" {
		return s
	}
	return strings.TrimSpace(flagSpace)
}

// EnrichWorkItemMeta adds resolved_id, serial_number, and url when resolvable.
func EnrichWorkItemMeta(meta map[string]any, item map[string]any, profileSpace, flagSpace string) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	if item == nil {
		return meta
	}
	nested, _ := item["data"].(map[string]any)
	if rid := InternalID(item); rid != "" {
		meta["resolved_id"] = rid
	} else if nested != nil {
		if rid := InternalID(nested); rid != "" {
			meta["resolved_id"] = rid
		}
	}
	if sn := SerialNumber(item); sn != "" {
		meta["serial_number"] = sn
	} else if nested != nil {
		if sn := SerialNumber(nested); sn != "" {
			meta["serial_number"] = sn
		}
	}
	sid := ResolveSpaceID(item, profileSpace, flagSpace)
	if u := WorkItemURL(item, sid); u != "" {
		meta["url"] = u
	}
	return meta
}

func stringField(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func localIDString(mr map[string]any) string {
	if mr == nil {
		return ""
	}
	for _, k := range []string{"localId", "local_id", "iid"} {
		if v, ok := mr[k]; ok && v != nil {
			switch t := v.(type) {
			case float64:
				return fmt.Sprintf("%.0f", t)
			case int:
				return fmt.Sprintf("%d", t)
			case int64:
				return fmt.Sprintf("%d", t)
			default:
				s := strings.TrimSpace(fmt.Sprint(t))
				if s != "" && s != "<nil>" {
					return s
				}
			}
		}
	}
	return ""
}

func looksLikeMRPageURL(u string) bool {
	return strings.Contains(u, "/change/") || strings.Contains(u, "/merge_request/")
}

// MergeRequestURL prefers API detailUrl (true MR page). Live Codeup often returns
// webUrl as the repository home only — only use webUrl when it already looks like
// an MR page; otherwise construct https://codeup.aliyun.com/{path}/change/{localId}.
func MergeRequestURL(mr map[string]any) string {
	if mr == nil {
		return ""
	}
	if u := stringField(mr, "detailUrl", "detail_url"); u != "" {
		return u
	}
	web := stringField(mr, "webUrl", "web_url", "html_url", "htmlUrl")
	if web != "" && looksLikeMRPageURL(web) {
		return web
	}
	iid := localIDString(mr)
	path := stringField(mr, "targetProjectPathWithNamespace", "sourceProjectPathWithNamespace")
	if path != "" && iid != "" {
		return "https://codeup.aliyun.com/" + strings.Trim(path, "/") + "/change/" + iid
	}
	if web != "" && iid != "" {
		return strings.TrimRight(web, "/") + "/change/" + iid
	}
	return ""
}

// UnwrapMergeRequestPayload returns the inner MR object when the API wraps it as {"data": {...}}.
func UnwrapMergeRequestPayload(out map[string]any) map[string]any {
	if out == nil {
		return nil
	}
	inner, ok := out["data"].(map[string]any)
	if !ok || inner == nil {
		return out
	}
	// Prefer inner when it looks like an MR (has status/localId/title) and outer does not.
	if MRStatus(out) == "" && localIDString(out) == "" {
		if MRStatus(inner) != "" || localIDString(inner) != "" || stringField(inner, "title") != "" {
			return inner
		}
	}
	return out
}

// MRStatus returns the merge-request lifecycle status.
// Codeup GetChangeRequest uses "status" (UNDER_REVIEW/MERGED/...); list UIs sometimes label it "state".
func MRStatus(mr map[string]any) string {
	if mr == nil {
		return ""
	}
	// Prefer top-level status (OpenAPI GetChangeRequest). Do not read author.state.
	if s := stringField(mr, "status"); s != "" {
		return s
	}
	if s := stringField(mr, "mergeStatus", "merge_status"); s != "" {
		return s
	}
	// Only use top-level "state" if it looks like an MR lifecycle value (not author.state nested).
	if v, ok := mr["state"]; ok && v != nil {
		s := strings.TrimSpace(fmt.Sprint(v))
		if s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}

// StabilizeMergeRequest copies mr and ensures script-stable keys: localId, title, status, state, detailUrl, url.
// "state" is an alias of "status" for consumers that expect GitLab-like naming (issue #46).
func StabilizeMergeRequest(mr map[string]any) map[string]any {
	if mr == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(mr)+6)
	for k, v := range mr {
		out[k] = v
	}
	if lid := localIDString(mr); lid != "" {
		out["localId"] = lid
	}
	if title := stringField(mr, "title"); title != "" {
		out["title"] = title
	}
	st := MRStatus(mr)
	if st != "" {
		out["status"] = st
		out["state"] = st
	}
	if u := MergeRequestURL(mr); u != "" {
		out["detailUrl"] = u
		out["url"] = u
	}
	return out
}

// BriefMergeRequest returns a small script-friendly view (issue #46/#50).
func BriefMergeRequest(mr map[string]any) map[string]any {
	s := StabilizeMergeRequest(mr)
	return map[string]any{
		"localId":   s["localId"],
		"title":     s["title"],
		"status":    s["status"],
		"state":     s["state"],
		"detailUrl": s["detailUrl"],
		"url":       s["url"],
	}
}

// EnrichMergeRequestMeta sets meta.url from MergeRequestURL when available.
func EnrichMergeRequestMeta(meta map[string]any, mr map[string]any) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	if u := MergeRequestURL(mr); u != "" {
		meta["url"] = u
	}
	return meta
}

// AttachMergeRequestURLs injects a clickable "url" onto each MR object in list
// payloads (bare []any or common wrapper shapes). Single objects are enriched in place.
func AttachMergeRequestURLs(data any) any {
	switch v := data.(type) {
	case []any:
		for i := range v {
			if m, ok := v[i].(map[string]any); ok {
				if u := MergeRequestURL(m); u != "" {
					m["url"] = u
				}
			}
		}
		return v
	case map[string]any:
		if u := MergeRequestURL(v); u != "" {
			v["url"] = u
			return v
		}
		for _, key := range []string{"items", "list", "data", "changeRequests"} {
			if inner, ok := v[key]; ok {
				v[key] = AttachMergeRequestURLs(inner)
			}
		}
		return v
	default:
		return data
	}
}
