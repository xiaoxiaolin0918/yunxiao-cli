package zhiyi

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

var serialRe = regexp.MustCompile(`^[A-Z]+-\d+$`)

// LooksLikeSerial reports whether s looks like ZYPT-5768 (or PROFILE-123).
func LooksLikeSerial(s string) bool {
	s = strings.TrimSpace(s)
	return serialRe.MatchString(s)
}

// LooksLikeSerialPrefix checks optional profile prefix (e.g. ZYPT) or generic pattern.
func LooksLikeSerialPrefix(s, prefix string) bool {
	s = strings.TrimSpace(s)
	if prefix != "" {
		p := strings.ToUpper(strings.TrimSpace(prefix))
		if strings.HasPrefix(strings.ToUpper(s), p+"-") && serialRe.MatchString(s) {
			return true
		}
	}
	return LooksLikeSerial(s)
}

// ResolveBugStatusId maps alias → status id via statuses map; unknown values pass through.
func ResolveBugStatusId(aliasOrID string, statuses map[string]string) string {
	aliasOrID = strings.TrimSpace(aliasOrID)
	if statuses != nil {
		if id, ok := statuses[aliasOrID]; ok && id != "" {
			return id
		}
	}
	return aliasOrID
}

// TransitionSteps ports domain.ts transitionSteps exactly.
// edges: adjacency keyed by status id (only on-graph nodes are keys).
// allStatuses: every known bug status id (values of BUG_STATUSES).
func TransitionSteps(current, target string, edges map[string][]string, allStatuses map[string]bool) ([]string, error) {
	if !allStatuses[current] {
		return nil, fmt.Errorf("当前状态 %q 不在缺陷状态机中", current)
	}
	if !allStatuses[target] {
		return nil, fmt.Errorf("目标状态 %q 不在缺陷状态机中", target)
	}
	if current == target {
		return nil, nil
	}
	_, currentOnGraph := edges[current]
	_, targetOnGraph := edges[target]
	if currentOnGraph && targetOnGraph {
		type node struct {
			id   string
			path []string
		}
		queue := []node{{id: current, path: nil}}
		seen := map[string]bool{current: true}
		for len(queue) > 0 {
			n := queue[0]
			queue = queue[1:]
			for _, next := range edges[n.id] {
				if seen[next] {
					continue
				}
				path := append(append([]string{}, n.path...), next)
				if next == target {
					return path, nil
				}
				seen[next] = true
				queue = append(queue, node{id: next, path: path})
			}
		}
		return nil, fmt.Errorf("当前状态无法流转到目标状态：%s → %s（图内无实证边）", current, target)
	}
	// Side-branch: single hop
	return []string{target}, nil
}

// RequiredFieldIDs unions bug_transition_required for each step status id (order preserved).
func RequiredFieldIDs(steps []string, required map[string][]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range steps {
		for _, fid := range required[s] {
			if seen[fid] {
				continue
			}
			seen[fid] = true
			out = append(out, fid)
		}
	}
	return out
}

// PlanDueDateWire converts YYYY-MM-DD to ISO8601 with +08:00 midnight for date custom fields.
func PlanDueDateWire(date string) string {
	date = strings.TrimSpace(date)
	if date == "" {
		return date
	}
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, date); matched {
		return date + "T00:00:00+08:00"
	}
	return date
}

// SplitUserIDs splits comma-separated user ids (developer flag).
func SplitUserIDs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// SprintCandidate is one aggregated sprint from recent bugs.
type SprintCandidate struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// SprintCurrentResult mirrors domain.ts SprintCurrentResult.
type SprintCurrentResult struct {
	SampleSize int               `json:"sampleSize"`
	Candidates []SprintCandidate `json:"candidates"`
	Suggested  string            `json:"suggested"`
}

// ExtractSearchItems pulls work-item maps from a search response (array or {items:[]}).
func ExtractSearchItems(data any) []map[string]any {
	switch v := data.(type) {
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, it := range v {
			if m, ok := it.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]any:
		return v
	case map[string]any:
		if items, ok := v["items"]; ok {
			return ExtractSearchItems(items)
		}
	}
	return nil
}

// AggregateBugSprints ports domain.ts aggregateBugSprints.
func AggregateBugSprints(items []map[string]any) SprintCurrentResult {
	counts := map[string]int{}
	names := map[string]string{}
	for _, item := range items {
		sprint, _ := item["sprint"].(map[string]any)
		if sprint == nil {
			continue
		}
		var id string
		switch sid := sprint["id"].(type) {
		case string:
			id = sid
		case float64:
			id = fmt.Sprintf("%.0f", sid)
		default:
			if sid != nil {
				id = fmt.Sprint(sid)
			}
		}
		if id == "" || id == "<nil>" {
			continue
		}
		counts[id]++
		if _, ok := names[id]; !ok {
			name := id
			if n, ok := sprint["name"]; ok && n != nil {
				s := fmt.Sprint(n)
				if s != "" && s != "<nil>" {
					name = s
				}
			}
			names[id] = name
		}
	}
	type pair struct {
		id    string
		count int
	}
	pairs := make([]pair, 0, len(counts))
	for id, c := range counts {
		pairs = append(pairs, pair{id: id, count: c})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].id < pairs[j].id
	})
	candidates := make([]SprintCandidate, 0, len(pairs))
	for _, p := range pairs {
		candidates = append(candidates, SprintCandidate{
			ID:    p.id,
			Name:  names[p.id],
			Count: p.count,
		})
	}
	suggested := ""
	if len(candidates) > 0 {
		suggested = candidates[0].ID
	}
	return SprintCurrentResult{
		SampleSize: len(items),
		Candidates: candidates,
		Suggested:  suggested,
	}
}

// FormatSprintSuggestion ports domain.ts formatSprintSuggestion.
func FormatSprintSuggestion(result SprintCurrentResult) string {
	if result.Suggested == "" {
		return "未找到当前迭代，请用 --sprint 显式传入"
	}
	n := len(result.Candidates)
	if n > 3 {
		n = 3
	}
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		c := result.Candidates[i]
		parts = append(parts, fmt.Sprintf("%s(%d)", c.Name, c.Count))
	}
	return fmt.Sprintf("未指定 --sprint；候选：%s；建议 --sprint %s", strings.Join(parts, ", "), result.Suggested)
}

// CreateBugInput is the user-facing create-bug payload (before field-id mapping).
type CreateBugInput struct {
	Title              string
	Description        string
	Environment        string
	Priority           string
	SeriousLevel       string
	Module             string
	ExpectedCompletion string
	Sprint             string
	AssignedTo         string
	// Minimal skips optional profile create fields (module/environment/ExpCompletionTime)
	// even when those field ids are configured.
	Minimal bool
}

// BuildCreateBugArgs ports domain.ts buildCreateBugArgs using profile space/type/field maps.
// Module, environment, and ExpCompletionTime are included only when the profile configures
// their field ids (and Minimal is false). Priority + seriousLevel are always sent when mapped.
func BuildCreateBugArgs(input CreateBugInput, pf *profile.Profile) (map[string]any, error) {
	if pf == nil {
		return nil, fmt.Errorf("profile required to build create-bug body")
	}
	if pf.SpaceID == "" {
		return nil, fmt.Errorf("profile missing space_id")
	}
	if pf.BugTypeID == "" {
		return nil, fmt.Errorf("profile missing bug_type_id")
	}
	priority, err := pf.ResolvePriorityID(input.Priority)
	if err != nil {
		return nil, err
	}
	serious, err := pf.ResolveSeriousLevelID(input.SeriousLevel)
	if err != nil {
		return nil, err
	}
	cf := map[string]any{
		"priority":     priority,
		"seriousLevel": serious,
	}
	if !input.Minimal {
		if moduleFID := pf.ModuleFieldID(); moduleFID != "" {
			cf[moduleFID] = input.Module
		}
		if envFID := pf.EnvironmentFieldID(); envFID != "" {
			cf[envFID] = input.Environment
		}
		if expKey := pf.ExpCompletionTimeKey(); expKey != "" {
			cf[expKey] = input.ExpectedCompletion
		}
	}
	return map[string]any{
		"spaceId":           pf.SpaceID,
		"workitemTypeId":    pf.BugTypeID,
		"subject":           input.Title,
		"description":       input.Description,
		"formatType":        "MARKDOWN",
		"sprint":            input.Sprint,
		"assignedTo":        input.AssignedTo,
		"customFieldValues": cf,
	}, nil
}

// WithWipTitle ports domain.ts withWipTitle.
func WithWipTitle(title, target string, wip bool) string {
	if target == "master" && wip && !strings.HasPrefix(title, "WIP: ") {
		return "WIP: " + title
	}
	return title
}

// ResolveRepositoryID ports domain.ts resolveRepositoryId with profile.repositories.
// Accepts: numeric id, profile alias, or path-style id (org/repo or URL-encoded).
func ResolveRepositoryID(repo string, repositories map[string]int64) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", fmt.Errorf("empty repository")
	}
	if matched, _ := regexp.MatchString(`^\d+$`, repo); matched {
		return repo, nil
	}
	// Path-style repository identity (organization/repo or already URL-encoded).
	if strings.Contains(repo, "/") || strings.Contains(strings.ToLower(repo), "%2f") {
		return repo, nil
	}
	if repositories != nil {
		if id, ok := repositories[repo]; ok {
			return fmt.Sprintf("%d", id), nil
		}
	}
	return "", fmt.Errorf("unknown repository alias %q: register it under profile.repositories (alias→numeric id), or pass numeric repositoryId / org%%2Frepo path", repo)
}

// WorkItemURL builds a Projex web URL when spaceID and id/serial are known.
// Path segment follows categoryId (req/bug/task/risk/topic); unknown → req.
func WorkItemURL(item map[string]any, spaceID string) string {
	if spaceID == "" {
		return ""
	}
	nested, _ := item["data"].(map[string]any)
	internal := InternalID(item)
	if internal == "" && nested != nil {
		internal = InternalID(nested)
	}
	serial := SerialNumber(item)
	if serial == "" && nested != nil {
		serial = SerialNumber(nested)
	}
	category := "Req"
	if item != nil {
		if c, ok := item["categoryId"]; ok && c != nil {
			category = fmt.Sprint(c)
		} else if nested != nil {
			if c, ok := nested["categoryId"]; ok && c != nil {
				category = fmt.Sprint(c)
			}
		}
	}
	path := CategoryPathSegment(category)
	if internal != "" {
		return fmt.Sprintf("https://devops.aliyun.com/projex/project/%s/%s#openWorkitemIdentifier=%s", spaceID, path, internal)
	}
	if serial != "" {
		return fmt.Sprintf("https://devops.aliyun.com/projex/project/%s/%s?serialNumber=%s", spaceID, path, serial)
	}
	return ""
}
