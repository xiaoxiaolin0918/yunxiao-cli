package pipelineyaml

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Change describes one structural unit that was added, removed, or modified.
type Change struct {
	Kind string `json:"kind"` // added|removed|modified
	Path string `json:"path"`
	Note string `json:"note,omitempty"`
}

// DiffResult is a normalized stage/job/step summary.
type DiffResult struct {
	Added       []Change `json:"added"`
	Removed     []Change `json:"removed"`
	Modified    []Change `json:"modified"`
	HighRisk    bool     `json:"high_risk"`
	HighRiskWhy []string `json:"high_risk_why,omitempty"`
	Summary     string   `json:"summary"`
}

// Unchanged reports whether the structural diff has no added/removed/modified units.
func (d *DiffResult) Unchanged() bool {
	if d == nil {
		return true
	}
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Modified) == 0
}

// DiffYAML compares old and new Flow YAML strings at stage/job/step granularity.
func DiffYAML(oldYAML, newYAML string) (*DiffResult, error) {
	oldUnits, err := parseUnits(oldYAML)
	if err != nil {
		return nil, fmt.Errorf("parse current YAML: %w", err)
	}
	newUnits, err := parseUnits(newYAML)
	if err != nil {
		return nil, fmt.Errorf("parse new YAML: %w", err)
	}
	res := &DiffResult{}
	all := map[string]struct{}{}
	for k := range oldUnits {
		all[k] = struct{}{}
	}
	for k := range newUnits {
		all[k] = struct{}{}
	}
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		o, okO := oldUnits[k]
		n, okN := newUnits[k]
		switch {
		case !okO && okN:
			res.Added = append(res.Added, Change{Kind: "added", Path: k})
		case okO && !okN:
			res.Removed = append(res.Removed, Change{Kind: "removed", Path: k})
			res.HighRisk = true
			res.HighRiskWhy = append(res.HighRiskWhy, "removed "+k)
		case okO && okN && o != n:
			note := ""
			if isDeployOrScriptPath(k) || containsDeployKeywords(o) || containsDeployKeywords(n) {
				res.HighRisk = true
				res.HighRiskWhy = append(res.HighRiskWhy, "modified deploy/script unit "+k)
				note = "deploy_or_script"
			}
			res.Modified = append(res.Modified, Change{Kind: "modified", Path: k, Note: note})
		}
	}
	res.Summary = fmt.Sprintf("+%d ~%d -%d", len(res.Added), len(res.Modified), len(res.Removed))
	return res, nil
}

func parseUnits(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}, nil
	}
	var doc any
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, err
	}
	doc = normalize(doc)
	out := map[string]string{}
	walkUnits("", "", doc, out)
	return out, nil
}

func normalize(v any) any {
	switch t := v.(type) {
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[fmt.Sprint(k)] = normalize(val)
		}
		return m
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = normalize(val)
		}
		return m
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = normalize(e)
		}
		return out
	default:
		return v
	}
}

func walkUnits(prefix, parentKey string, node any, out map[string]string) {
	switch t := node.(type) {
	case map[string]any:
		name := firstString(t, "name", "jobName", "sign", "id")
		kind := unitKind(parentKey, t)
		path := prefix
		if name != "" && kind != "" {
			seg := kind + "/" + name
			if path == "" {
				path = seg
			} else {
				path = path + "/" + seg
			}
			out[path] = fingerprint(t)
		}
		for _, key := range []string{"stages", "jobs", "steps", "jobsList"} {
			if v, ok := t[key]; ok {
				walkUnits(path, key, v, out)
			}
		}
		if si, ok := t["stageInfo"]; ok {
			walkUnits(path, "stageInfo", si, out)
		}
	case []any:
		for _, e := range t {
			walkUnits(prefix, parentKey, e, out)
		}
	}
}

func unitKind(parentKey string, m map[string]any) string {
	switch parentKey {
	case "stages":
		return "stage"
	case "jobs", "jobsList":
		return "job"
	case "steps":
		return "step"
	}
	if _, ok := m["jobs"]; ok {
		return "stage"
	}
	if _, ok := m["steps"]; ok {
		return "job"
	}
	if _, ok := m["stepType"]; ok {
		return "step"
	}
	if hasKey(m, "plugin") || hasKey(m, "run") || hasKey(m, "command") || hasKey(m, "script") {
		return "step"
	}
	return ""
}

func hasKey(m map[string]any, k string) bool {
	_, ok := m[k]
	return ok
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func fingerprint(m map[string]any) string {
	b, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Sprint(m)
	}
	return string(b)
}

func isDeployOrScriptPath(path string) bool {
	p := strings.ToLower(path)
	return strings.Contains(p, "deploy") || strings.Contains(p, "script") || strings.Contains(p, "kubectl") || strings.Contains(p, "helm")
}

func containsDeployKeywords(fp string) bool {
	l := strings.ToLower(fp)
	for _, k := range []string{"deploy", "kubectl", "helm", "ssh-deploy", "publish"} {
		if strings.Contains(l, k) {
			return true
		}
	}
	return false
}
