package cmd

import (
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// mrExtIdentity is how we recognize a Codeup MR inside workitem
// extRelationRecords (category=codeupMergeRequest). Matching must not rely
// only on businessId == localId: the list API often stores the MR bizId and
// URL, and projectId may arrive as a JSON number (#402).
type mrExtIdentity struct {
	LocalID   string
	ProjectID string
	BizIDs    []string
	URL       string
}

func stringifyRelID(v any) string {
	return zhiyi.NormalizeRepositoryID(v)
}

func collectRelStrings(rel map[string]any, keys ...string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, k := range keys {
		s := stringifyRelID(rel[k])
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func mrExtIdentityFrom(mr map[string]any, repositoryID, localID string) mrExtIdentity {
	id := mrExtIdentity{LocalID: strings.TrimSpace(localID)}
	id.ProjectID = numericOrEmpty(repositoryID)
	if mr != nil {
		if id.ProjectID == "" {
			id.ProjectID = stringifyRelID(mr["projectId"])
		}
		if lid := mrLocalID(mr); lid != "" {
			id.LocalID = lid
		}
		id.URL = mrURLFrom(nil, mr)
		seen := map[string]struct{}{}
		add := func(s string) {
			s = strings.TrimSpace(s)
			if s == "" || s == id.LocalID {
				return
			}
			if _, ok := seen[s]; ok {
				return
			}
			seen[s] = struct{}{}
			id.BizIDs = append(id.BizIDs, s)
		}
		for _, k := range []string{"bizId", "mrBizId", "identifier", "changeRequestBizId", "uuid"} {
			add(stringifyRelID(mr[k]))
		}
		if s := stringifyRelID(mr["id"]); s != "" && s != id.LocalID {
			add(s)
		}
	}
	return id
}

func repoPathFromMRURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	lower := strings.ToLower(u)
	cut := -1
	for _, m := range []string{"/change/", "/merge_request/", "/merge-requests/", "/merge_requests/"} {
		if i := strings.Index(lower, m); i >= 0 && (cut < 0 || i < cut) {
			cut = i
		}
	}
	if cut < 0 {
		return ""
	}
	path := u[:cut]
	if i := strings.Index(path, "://"); i >= 0 {
		path = path[i+3:]
		if j := strings.Index(path, "/"); j >= 0 {
			path = path[j+1:]
		} else {
			return ""
		}
	}
	return strings.Trim(path, "/")
}

func hasBangLocalID(s, localID string) bool {
	if s == "" || localID == "" {
		return false
	}
	token := "!" + localID
	idx := strings.Index(s, token)
	if idx < 0 {
		return false
	}
	after := idx + len(token)
	if after < len(s) {
		c := s[after]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}

func urlsLooselyEqual(a, b string) bool {
	norm := func(s string) string {
		s = strings.TrimSpace(s)
		s = strings.TrimRight(s, "/")
		return strings.ToLower(s)
	}
	return a != "" && b != "" && norm(a) == norm(b)
}

func mrURLHasLocalID(u, localID string) bool {
	if localID == "" || u == "" {
		return false
	}
	lower := strings.ToLower(u)
	for _, marker := range []string{"/change/", "/merge_request/", "/merge-requests/", "/merge_requests/"} {
		start := 0
		for {
			i := strings.Index(lower[start:], marker)
			if i < 0 {
				break
			}
			i += start
			rest := u[i+len(marker):]
			end := len(rest)
			for j, c := range rest {
				if c == '?' || c == '#' || c == '/' {
					end = j
					break
				}
			}
			if rest[:end] == localID {
				return true
			}
			start = i + len(marker)
		}
	}
	return hasBangLocalID(u, localID)
}

func urlIdentifiesMR(u string, id mrExtIdentity) bool {
	u = strings.TrimSpace(u)
	if u == "" {
		return false
	}
	if id.URL != "" && urlsLooselyEqual(u, id.URL) {
		return true
	}
	if !mrURLHasLocalID(u, id.LocalID) {
		return false
	}
	if id.URL != "" && mrURLHasLocalID(id.URL, id.LocalID) {
		p1, p2 := repoPathFromMRURL(u), repoPathFromMRURL(id.URL)
		if p1 != "" && p2 != "" {
			return strings.EqualFold(p1, p2)
		}
	}
	return true
}

func projectIDsCompatible(a, b string) bool {
	if a == "" || b == "" {
		return true
	}
	return a == b
}

func relationMatchesMR(rel map[string]any, id mrExtIdentity) bool {
	if rel == nil {
		return false
	}
	if id.LocalID == "" && id.URL == "" && len(id.BizIDs) == 0 {
		return false
	}
	relProject := stringifyRelID(rel["projectId"])
	if relProject == "" {
		relProject = stringifyRelID(rel["repositoryId"])
	}
	relIDs := collectRelStrings(rel, "businessId", "bizId", "mergeRequestId", "localId", "mrLocalId", "changeRequestId")
	relURLs := collectRelStrings(rel, "url", "detailUrl", "webUrl", "htmlUrl", "link")

	bizSet := map[string]struct{}{}
	for _, b := range id.BizIDs {
		if b != "" {
			bizSet[b] = struct{}{}
		}
	}

	for _, u := range relURLs {
		if id.URL != "" && urlsLooselyEqual(u, id.URL) {
			return true
		}
		if urlIdentifiesMR(u, id) {
			p1, p2 := repoPathFromMRURL(u), repoPathFromMRURL(id.URL)
			if p1 != "" && p2 != "" && strings.EqualFold(p1, p2) {
				return true
			}
			if projectIDsCompatible(id.ProjectID, relProject) {
				return true
			}
		}
	}

	localHit := false
	for _, rid := range relIDs {
		if _, ok := bizSet[rid]; ok {
			return true
		}
		if id.LocalID != "" && rid == id.LocalID {
			localHit = true
			continue
		}
		if hasBangLocalID(rid, id.LocalID) {
			localHit = true
			continue
		}
		if urlIdentifiesMR(rid, id) {
			if id.URL != "" && urlsLooselyEqual(rid, id.URL) {
				return true
			}
			localHit = true
		}
	}
	if !localHit {
		return false
	}
	return projectIDsCompatible(id.ProjectID, relProject)
}
