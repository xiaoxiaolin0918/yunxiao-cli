package pipelineyaml

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// YAMLValidationIssue is one structured path/message from Flow YAML validation.
type YAMLValidationIssue struct {
	Path         string `json:"path,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	Raw          string `json:"raw,omitempty"`
}

// ParseYAMLValidationError extracts issues from API error bodies like errorCode 1209300
// which often embed escaped JSON: {"errorMessage":"...","path":"..."}.
func ParseYAMLValidationError(body string) (code string, issues []YAMLValidationIssue, ok bool) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", nil, false
	}
	var top map[string]any
	if err := json.Unmarshal([]byte(body), &top); err == nil {
		code = stringify(top["errorCode"])
		msg := stringify(top["errorMessage"])
		if msg == "" {
			msg = stringify(top["errorMsg"])
		}
		issues = append(issues, parseEmbeddedIssues(msg)...)
		if path := stringify(top["path"]); path != "" {
			issues = append(issues, YAMLValidationIssue{Path: path, ErrorMessage: msg})
		}
		if len(issues) == 0 && msg != "" {
			issues = append(issues, YAMLValidationIssue{ErrorMessage: msg, Raw: msg})
		}
		if code == "1209300" || strings.Contains(body, "1209300") {
			if code == "" {
				code = "1209300"
			}
			return code, dedupeIssues(issues), true
		}
		if strings.Contains(strings.ToLower(msg), "yaml") && (stringify(top["path"]) != "" || len(issues) > 0) {
			return code, dedupeIssues(issues), true
		}
	}
	if strings.Contains(body, "1209300") {
		issues = parseEmbeddedIssues(body)
		if len(issues) == 0 {
			issues = []YAMLValidationIssue{{Raw: truncate(body, 500), ErrorMessage: "yaml validation failed"}}
		}
		return "1209300", dedupeIssues(issues), true
	}
	return "", nil, false
}

var embeddedJSON = regexp.MustCompile(`\{(?:[^{}]|\\.)*\}`)

func parseEmbeddedIssues(s string) []YAMLValidationIssue {
	if s == "" {
		return nil
	}
	candidates := []string{s, cheapUnescape(s), cheapUnescape(cheapUnescape(s))}
	var out []YAMLValidationIssue
	for _, c := range candidates {
		var obj map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &obj); err == nil {
			if em := firstNonEmpty(stringify(obj["errorMessage"]), stringify(obj["errorMsg"])); em != "" || stringify(obj["path"]) != "" {
				out = append(out, YAMLValidationIssue{
					Path:         stringify(obj["path"]),
					ErrorMessage: em,
					Raw:          truncate(c, 300),
				})
			}
		}
		for _, m := range embeddedJSON.FindAllString(c, -1) {
			for _, mm := range []string{m, cheapUnescape(m)} {
				var obj map[string]any
				if err := json.Unmarshal([]byte(mm), &obj); err != nil {
					continue
				}
				em := firstNonEmpty(stringify(obj["errorMessage"]), stringify(obj["errorMsg"]))
				path := stringify(obj["path"])
				if em == "" && path == "" {
					continue
				}
				out = append(out, YAMLValidationIssue{Path: path, ErrorMessage: em, Raw: truncate(mm, 300)})
			}
		}
	}
	return out
}

func cheapUnescape(s string) string {
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func dedupeIssues(in []YAMLValidationIssue) []YAMLValidationIssue {
	seen := map[string]struct{}{}
	var out []YAMLValidationIssue
	for _, it := range in {
		key := it.Path + "|" + it.ErrorMessage
		if key == "|" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, it)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
