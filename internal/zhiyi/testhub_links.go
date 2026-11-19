package zhiyi

import (
	"fmt"
	"strings"
)

// TestHub console paths (#415). Directory / repo links live under /testhub/repo/{repoId}/case
// (verified by the issue reporter). Single-case deep links use /testhub/case/{customCode}
// (console "复制链接"). The old /projex/project/{spaceId}/testcase templates white-screen
// after login and must not be emitted. Test plan links stay on Projex until a verified
// TestHub template exists.
const testhubPlanPath = "testplan"

// TestCaseURL builds a TestHub case deep link from customCode when known (#415).
// spaceID is kept for call-site compatibility and is not used in the path.
// If customCode is empty, falls back to the repo library page when repoID is set
// (there is no reliable official hash on the internal case id).
func TestCaseURL(spaceID, repoID, customCode string) string {
	if code := strings.TrimSpace(customCode); code != "" {
		return "https://devops.aliyun.com/testhub/case/" + code
	}
	return TestRepoURL(spaceID, repoID)
}

// TestRepoURL builds the TestHub library page. The path key is repoId (#415);
// spaceID is accepted for compatibility and is not used in the path.
func TestRepoURL(spaceID, repoID string) string {
	repoID = strings.TrimSpace(repoID)
	if repoID == "" {
		return ""
	}
	return "https://devops.aliyun.com/testhub/repo/" + repoID + "/case"
}

// TestPlanURL builds a Projex test plan page URL.
func TestPlanURL(spaceID, planID string) string {
	spaceID = strings.TrimSpace(spaceID)
	planID = strings.TrimSpace(planID)
	if spaceID == "" || planID == "" {
		return ""
	}
	return fmt.Sprintf("https://devops.aliyun.com/projex/project/%s/%s/%s",
		spaceID, testhubPlanPath, planID)
}

// TestDirectoryURL points at the TestHub library page with the official directory hash (#415).
func TestDirectoryURL(spaceID, repoID, dirID string) string {
	u := TestRepoURL(spaceID, repoID)
	if u == "" {
		return ""
	}
	if d := strings.TrimSpace(dirID); d != "" {
		return u + "#selectedTreeIdentifier=" + d
	}
	return u
}

const testhubWebPathHint = "云效 TestHub → 用例库（仓库页 /testhub/repo/{repoId}/case；目录加 #selectedTreeIdentifier={directoryId}；单条直链 /testhub/case/{customCode}，缺 customCode 时退到所属仓库页）"

// EnrichTestCaseMeta sets meta.url from customCode (preferred) or the repo page (#415);
// otherwise webPathHint.
func EnrichTestCaseMeta(meta map[string]any, spaceID, repoID, customCode string) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	if u := TestCaseURL(spaceID, repoID, customCode); u != "" {
		meta["url"] = u
		delete(meta, "webPathHint")
		return meta
	}
	if meta["url"] == nil || meta["url"] == "" {
		meta["webPathHint"] = testhubWebPathHint
	}
	return meta
}

// EnrichTestCaseMetaFromItem sets meta.url from item.customCode or repoID.
func EnrichTestCaseMetaFromItem(meta map[string]any, spaceID, repoID string, item map[string]any) map[string]any {
	code := ""
	if item != nil {
		code = firstStringField(item, "customCode")
	}
	return EnrichTestCaseMeta(meta, spaceID, repoID, code)
}

// EnrichTestRepoMeta sets meta.url for a repo list/get when repoID is known.
func EnrichTestRepoMeta(meta map[string]any, spaceID, repoID string) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	if u := TestRepoURL(spaceID, repoID); u != "" {
		meta["url"] = u
		delete(meta, "webPathHint")
		return meta
	}
	if meta["url"] == nil || meta["url"] == "" {
		meta["webPathHint"] = testhubWebPathHint
	}
	return meta
}

// EnrichTestPlanMeta sets meta.url for a plan when space+plan id known.
func EnrichTestPlanMeta(meta map[string]any, spaceID, planID string) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	if u := TestPlanURL(spaceID, planID); u != "" {
		meta["url"] = u
		delete(meta, "webPathHint")
		return meta
	}
	if meta["url"] == nil || meta["url"] == "" {
		meta["webPathHint"] = testhubWebPathHint
	}
	return meta
}

// AttachTestCaseURLToMap sets m["url"] from customCode or the repo library page.
func AttachTestCaseURLToMap(m map[string]any, spaceID, repoID string) {
	if m == nil {
		return
	}
	if u := TestCaseURL(spaceID, repoID, firstStringField(m, "customCode")); u != "" {
		m["url"] = u
	}
}

// AttachTestCaseURLs adds url on each case map from customCode or repoID (#415).
func AttachTestCaseURLs(data any, spaceID, repoID string) any {
	repoID = strings.TrimSpace(repoID)
	return mapListItems(data, func(m map[string]any) {
		AttachTestCaseURLToMap(m, spaceID, repoID)
	})
}

// AttachTestRepoIDsAndURLs adds id alias from testRepoIdentifier and optional url (#357/#361/#415).
func AttachTestRepoIDsAndURLs(data any, spaceID string) any {
	return mapListItems(data, func(m map[string]any) {
		rid := firstStringField(m, "id", "testRepoIdentifier")
		if rid == "" {
			return
		}
		if _, has := m["id"]; !has || m["id"] == nil || strings.TrimSpace(fmt.Sprint(m["id"])) == "" {
			m["id"] = rid
		}
		if _, has := m["testRepoIdentifier"]; !has || m["testRepoIdentifier"] == nil {
			m["testRepoIdentifier"] = rid
		}
		if u := TestRepoURL(spaceID, rid); u != "" {
			m["url"] = u
		}
	})
}

// AttachTestPlanIDsAndURLs adds id alias from testPlanIdentifier and url (#361).
func AttachTestPlanIDsAndURLs(data any) any {
	return mapListItems(data, func(m map[string]any) {
		pid := firstStringField(m, "id", "testPlanIdentifier")
		if pid == "" {
			return
		}
		if _, has := m["id"]; !has || m["id"] == nil || strings.TrimSpace(fmt.Sprint(m["id"])) == "" {
			m["id"] = pid
		}
		sid := firstStringField(m, "spaceIdentifier", "spaceId")
		if u := TestPlanURL(sid, pid); u != "" {
			m["url"] = u
		}
	})
}

// AttachTestDirectoryURLs adds url on directory maps when repoID is known (#415).
func AttachTestDirectoryURLs(data any, spaceID, repoID string) any {
	return mapListItems(data, func(m map[string]any) {
		did := firstStringField(m, "id")
		if u := TestDirectoryURL(spaceID, repoID, did); u != "" {
			m["url"] = u
		}
	})
}

func mapListItems(data any, fn func(map[string]any)) any {
	switch v := data.(type) {
	case []any:
		for _, it := range v {
			if m, ok := it.(map[string]any); ok {
				fn(m)
			}
		}
		return v
	case map[string]any:
		for _, key := range []string{"data", "items", "list", "result"} {
			if inner, ok := v[key]; ok {
				v[key] = mapListItems(inner, fn)
			}
		}
		// single object
		if _, hasID := v["id"]; hasID || v["testRepoIdentifier"] != nil || v["testPlanIdentifier"] != nil {
			fn(v)
		}
		return v
	default:
		return data
	}
}

// AssignedToPresent reports whether body.assignedTo is a non-empty string or {id}.
func AssignedToPresent(body map[string]any) bool {
	if body == nil {
		return false
	}
	v, ok := body["assignedTo"]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t) != ""
	case map[string]any:
		return firstStringField(t, "id", "identifier", "userId") != ""
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		return s != "" && s != "<nil>"
	}
}
