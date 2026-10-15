package zhiyi

import (
	"fmt"
	"strings"
)

// TestHub console paths under Projex (central edition).
// Verified as live routes (302→login); hash openers mirror workitem openWorkitemIdentifier.
const (
	testhubCasePath = "testcase"
	testhubPlanPath = "testplan"
)

// TestCaseURL builds a Projex TestHub case page URL when space and case id are known.
func TestCaseURL(spaceID, caseID string) string {
	spaceID = strings.TrimSpace(spaceID)
	caseID = strings.TrimSpace(caseID)
	if spaceID == "" || caseID == "" {
		return ""
	}
	return fmt.Sprintf("https://devops.aliyun.com/projex/project/%s/%s#openTestcaseIdentifier=%s",
		spaceID, testhubCasePath, caseID)
}

// TestRepoURL builds the project TestHub library page (space required).
func TestRepoURL(spaceID, repoID string) string {
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		return ""
	}
	base := fmt.Sprintf("https://devops.aliyun.com/projex/project/%s/%s", spaceID, testhubCasePath)
	if r := strings.TrimSpace(repoID); r != "" {
		return base + "?testRepoIdentifier=" + r
	}
	return base
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

// TestDirectoryURL points at the library page with directory hint (best-effort).
func TestDirectoryURL(spaceID, repoID, dirID string) string {
	u := TestRepoURL(spaceID, repoID)
	if u == "" {
		return ""
	}
	if d := strings.TrimSpace(dirID); d != "" {
		if strings.Contains(u, "?") {
			return u + "&directoryId=" + d
		}
		return u + "?directoryId=" + d
	}
	return u
}

const testhubWebPathHint = "云效项目 → 测试 → 用例库/测试计划（需项目 space_id；可设 profile.space_id）"

// EnrichTestCaseMeta sets meta.url from space+case id; otherwise webPathHint (#361).
func EnrichTestCaseMeta(meta map[string]any, spaceID, caseID string) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	if u := TestCaseURL(spaceID, caseID); u != "" {
		meta["url"] = u
		delete(meta, "webPathHint")
		return meta
	}
	if meta["url"] == nil || meta["url"] == "" {
		meta["webPathHint"] = testhubWebPathHint
	}
	return meta
}

// EnrichTestRepoMeta sets meta.url for a repo list/get when space is known.
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

// AttachTestCaseURLs adds url on each case map when spaceID is set.
func AttachTestCaseURLs(data any, spaceID string) any {
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		return data
	}
	return mapListItems(data, func(m map[string]any) {
		id := firstStringField(m, "id")
		if id == "" {
			return
		}
		if u := TestCaseURL(spaceID, id); u != "" {
			m["url"] = u
		}
	})
}

// AttachTestRepoIDsAndURLs adds id alias from testRepoIdentifier and optional url (#357/#361).
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

// AttachTestDirectoryURLs adds url on directory maps when space+repo known.
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
