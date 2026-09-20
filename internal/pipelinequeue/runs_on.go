package pipelinequeue

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var groupLine = regexp.MustCompile(`(?m)^\s*group:\s*["']?([^"'\n#]+)["']?\s*$`)

// ExtractRunnerGroups returns unique runsOn.group values from Flow YAML.
func ExtractRunnerGroups(flowYAML string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(g string) {
		g = strings.TrimSpace(strings.Trim(g, `"'`))
		if g == "" {
			return
		}
		if _, ok := seen[g]; ok {
			return
		}
		seen[g] = struct{}{}
		out = append(out, g)
	}
	var doc any
	if err := yaml.Unmarshal([]byte(flowYAML), &doc); err == nil {
		walkRunsOn(normalize(doc), add)
	}
	for _, m := range groupLine.FindAllStringSubmatch(flowYAML, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	return out
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

func walkRunsOn(node any, add func(string)) {
	switch t := node.(type) {
	case map[string]any:
		for _, key := range []string{"runsOn", "runs_on"} {
			if ro, ok := t[key].(map[string]any); ok {
				if g, ok := ro["group"].(string); ok {
					add(g)
				}
			}
		}
		for _, v := range t {
			walkRunsOn(v, add)
		}
	case []any:
		for _, e := range t {
			walkRunsOn(e, add)
		}
	}
}
