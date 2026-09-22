package workflow

import (
	"fmt"
	"strings"
)

// StatusInfo is a workflow status node.
type StatusInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	NameEn      string `json:"nameEn,omitempty"`
}

// knownAliasByLabel maps common Chinese / English status labels → profile aliases.
var knownAliasByLabel = map[string]string{
	"待确认":         "confirm",
	"new":         "confirm",
	"确认":          "confirm",
	"再次打开":        "reopen",
	"reopen":      "reopen",
	"处理中":         "processing",
	"in progress": "processing",
	"修复中":         "processing",
	"待发布测试":       "deploy-test",
	"发布测试":        "deploy-test",
	"deploy-test": "deploy-test",
	"测试中":         "testing",
	"testing":     "testing",
	"待发布生产":       "deploy-prod",
	"发布生产":        "deploy-prod",
	"deploy-prod": "deploy-prod",
	"待验收":         "acceptance",
	"acceptance":  "acceptance",
	"已修复":         "fixed",
	"fixed":       "fixed",
	"回归验证":        "regression",
	"regression":  "regression",
	"延期":          "deferred",
	"deferred":    "deferred",
	"暂不修复":        "wont-fix",
	"won't fix":   "wont-fix",
	"wont fix":    "wont-fix",
	"won'tfix":    "wont-fix",
	"已关闭":         "closed-fixed",
	"closed":      "closed-fixed",
	"关闭":          "closed-fixed",
	"重新打开":        "reopen",
}

// AliasForStatus picks a best-effort profile alias for a status, or empty if ambiguous/unknown.
func AliasForStatus(s StatusInfo) string {
	candidates := []string{s.DisplayName, s.Name, s.NameEn}
	seen := map[string]string{}
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		key := strings.ToLower(c)
		if alias, ok := knownAliasByLabel[key]; ok {
			seen[alias] = c
			continue
		}
		if alias, ok := knownAliasByLabel[c]; ok {
			seen[alias] = c
		}
	}
	if len(seen) == 1 {
		for a := range seen {
			return a
		}
	}
	return ""
}

// BuildBugStatusesMap maps unambiguous aliases → status id from workflow statuses.
// When two statuses map to the same alias, that alias is omitted.
func BuildBugStatusesMap(statuses []StatusInfo) map[string]string {
	aliasToIDs := map[string][]string{}
	for _, s := range statuses {
		if s.ID == "" {
			continue
		}
		alias := AliasForStatus(s)
		if alias == "" {
			continue
		}
		aliasToIDs[alias] = append(aliasToIDs[alias], s.ID)
	}
	out := map[string]string{}
	for alias, ids := range aliasToIDs {
		uniq := uniqueStrings(ids)
		if len(uniq) == 1 {
			out[alias] = uniq[0]
		}
	}
	return out
}

// BuildStatusNameMap maps displayName (and name when distinct) → status id.
func BuildStatusNameMap(statuses []StatusInfo) map[string]string {
	out := map[string]string{}
	for _, s := range statuses {
		if s.ID == "" {
			continue
		}
		for _, label := range []string{s.DisplayName, s.Name} {
			label = strings.TrimSpace(label)
			if label == "" {
				continue
			}
			out[label] = s.ID
		}
	}
	return out
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// WorkflowSnippet is the per-type_id fragment stored under profile.workflows.
type WorkflowSnippet struct {
	TypeID          string              `json:"type_id,omitempty"`
	Name            string              `json:"name,omitempty"`
	Category        string              `json:"category,omitempty"`
	WorkflowID      string              `json:"workflow_id,omitempty"`
	WorkflowName    string              `json:"workflow_name,omitempty"`
	DefaultStatusID string              `json:"default_status_id,omitempty"`
	Statuses        map[string]string   `json:"statuses,omitempty"`
	Edges           map[string][]string `json:"edges,omitempty"`        // verified only
	HintedEdges     map[string][]string `json:"hinted_edges,omitempty"` // needs_fields (issue 61)
}

// ProfileSnippet is the best-effort fragment for --write-profile / output.
type ProfileSnippet struct {
	BugStatuses map[string]string   `json:"bug_statuses,omitempty"`
	BugEdges    map[string][]string `json:"bug_edges,omitempty"`
	Workflow    WorkflowSnippet     `json:"workflow,omitempty"`
}

// SnippetInput carries explore/probe metadata for BuildProfileSnippet.
type SnippetInput struct {
	TypeID          string
	TypeName        string
	Category        string
	WorkflowID      string
	WorkflowName    string
	DefaultStatusID string
	Statuses        []StatusInfo
	Edges           map[string][]string
	HintedEdges     map[string][]string // optional; needs_fields edges
}

// BuildProfileSnippet builds a workflows[]-ready object plus legacy bug_* when category is Bug.
func BuildProfileSnippet(in SnippetInput) ProfileSnippet {
	edges := SortedCopy(in.Edges)
	hinted := SortedCopy(in.HintedEdges)
	statuses := BuildStatusNameMap(in.Statuses)
	cat := strings.TrimSpace(in.Category)
	catLower := strings.ToLower(cat)

	var bugStatuses map[string]string
	if catLower == "" || catLower == "bug" {
		bugStatuses = BuildBugStatusesMap(in.Statuses)
		for k, v := range bugStatuses {
			if k != "" && v != "" {
				if statuses == nil {
					statuses = map[string]string{}
				}
				statuses[k] = v
			}
		}
	}

	wf := WorkflowSnippet{
		TypeID:          strings.TrimSpace(in.TypeID),
		Name:            strings.TrimSpace(in.TypeName),
		Category:        cat,
		WorkflowID:      strings.TrimSpace(in.WorkflowID),
		WorkflowName:    strings.TrimSpace(in.WorkflowName),
		DefaultStatusID: strings.TrimSpace(in.DefaultStatusID),
		Statuses:        statuses,
		Edges:           edges,
		HintedEdges:     hinted,
	}

	out := ProfileSnippet{Workflow: wf}
	if catLower == "" || catLower == "bug" {
		out.BugStatuses = bugStatuses
		out.BugEdges = edges
	} else {
		// Non-Bug explores still expose edges under bug_edges historically for debugging;
		// keep empty bug_statuses. Prefer workflow for write-profile.
		out.BugEdges = edges
	}
	return out
}

// ResolveUniqueStatus maps aliasOrID to a single status id from statuses.
// Matches exact id, or unique displayName/name/nameEn (case-insensitive for English).
// Returns error when zero or ambiguous matches.
func ResolveUniqueStatus(aliasOrID string, statuses []StatusInfo) (string, error) {
	aliasOrID = strings.TrimSpace(aliasOrID)
	if aliasOrID == "" {
		return "", fmt.Errorf("empty status")
	}
	var byID string
	var nameHits []string
	lower := strings.ToLower(aliasOrID)
	for _, s := range statuses {
		if s.ID == "" {
			continue
		}
		if s.ID == aliasOrID {
			byID = s.ID
		}
		for _, label := range []string{s.DisplayName, s.Name, s.NameEn} {
			label = strings.TrimSpace(label)
			if label == "" {
				continue
			}
			if label == aliasOrID || strings.ToLower(label) == lower {
				nameHits = append(nameHits, s.ID)
				break
			}
		}
	}
	if byID != "" {
		return byID, nil
	}
	uniq := uniqueStrings(nameHits)
	if len(uniq) == 1 {
		return uniq[0], nil
	}
	if len(uniq) == 0 {
		return "", fmt.Errorf("status %q not found in type workflow statuses", aliasOrID)
	}
	return "", fmt.Errorf("status %q is ambiguous (%d matches); use a status id", aliasOrID, len(uniq))
}
