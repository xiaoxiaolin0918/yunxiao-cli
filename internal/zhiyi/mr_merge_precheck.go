package zhiyi

import (
	"fmt"
	"strings"
)

// mergeTypeAliases maps the spellings seen in the API / web UI onto the four
// MergeChangeRequest mergeType values (ff-only | no-fast-forward | squash | rebase).
// Key: lowercase with spaces/dashes/underscores removed; 中文 labels map by meaning
// (issue #130: the web UI offers "Fast-forward-only" and "创建合并节点").
var mergeTypeAliases = map[string]string{
	// ff-only
	"ffonly": "ff-only", "fastforwardonly": "ff-only", "fastforward": "ff-only",
	// no-fast-forward (创建合并节点 / merge node)
	"nofastforward": "no-fast-forward", "merge": "no-fast-forward", "mergenode": "no-fast-forward",
	"mergecommit": "no-fast-forward", "创建合并节点": "no-fast-forward", "普通合并": "no-fast-forward",
	// squash
	"squash": "squash", "squashmerge": "squash", "压缩合并": "squash",
	// rebase
	"rebase": "rebase", "rebasemerge": "rebase", "变基": "rebase",
}

// NormalizeMergeType folds a merge-method spelling onto the canonical
// ff-only | no-fast-forward | squash | rebase. Unknown values are returned
// lowercased/trimmed (so an unknown requested type can still be compared and
// reported against an availability list instead of silently passing).
func NormalizeMergeType(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	s = strings.NewReplacer(" ", "", "-", "", "_", "").Replace(s)
	if s == "" {
		return ""
	}
	if canon, ok := mergeTypeAliases[s]; ok {
		return canon
	}
	return s
}

// MergeTypesFromMR extracts the repo's enabled merge methods from an MR detail
// payload when the API exposes them: top-level mergeTypes / supportedMergeTypes
// or nested mergeSetting.mergeTypes (camelCase first, snake_case tolerated).
// Returns the normalized list and a short source label; (nil, "") when absent.
func MergeTypesFromMR(mr map[string]any) ([]string, string) {
	for _, key := range []struct{ label, camel, snake string }{
		{"mergeTypes", "mergeTypes", "merge_types"},
		{"supportedMergeTypes", "supportedMergeTypes", "supported_merge_types"},
	} {
		for _, k := range []string{key.camel, key.snake} {
			if raw, ok := mr[k].([]any); ok && len(raw) > 0 {
				return normalizeMergeTypeList(raw), key.label
			}
		}
	}
	for _, nestedKey := range []string{"mergeSetting", "merge_setting"} {
		nested, ok := mr[nestedKey].(map[string]any)
		if !ok {
			continue
		}
		for _, k := range []string{"mergeTypes", "merge_types"} {
			if raw, ok := nested[k].([]any); ok && len(raw) > 0 {
				return normalizeMergeTypeList(raw), nestedKey + "." + k
			}
		}
	}
	return nil, ""
}

func normalizeMergeTypeList(raw []any) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range raw {
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" || s == "<nil>" {
			continue
		}
		n := NormalizeMergeType(s)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

// MRConflictCheckStatus returns the conflictCheckStatus field (CHECKING |
// HAS_CONFLICT | NO_CONFLICT | FAILED; snake_case tolerated, uppercased).
func MRConflictCheckStatus(mr map[string]any) string {
	if mr == nil {
		return ""
	}
	for _, k := range []string{"conflictCheckStatus", "conflict_check_status"} {
		if v, ok := mr[k]; ok && v != nil {
			s := strings.ToUpper(strings.TrimSpace(fmt.Sprint(v)))
			if s != "" && s != "<NIL>" {
				return s
			}
		}
	}
	return ""
}

// MRBool returns a boolean-ish field (mergeable etc.); false when absent or
// not boolean-like ("true"/"false" strings tolerated).
func MRBool(mr map[string]any, keys ...string) (value bool, present bool) {
	if mr == nil {
		return false, false
	}
	for _, k := range keys {
		switch v := mr[k].(type) {
		case bool:
			return v, true
		case string:
			s := strings.ToLower(strings.TrimSpace(v))
			switch s {
			case "true":
				return true, true
			case "false":
				return false, true
			}
		}
	}
	return false, false
}
