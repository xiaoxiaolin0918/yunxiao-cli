package pipelineyaml

import (
	"fmt"
	"strings"
)

// ExtractFlowYAML pulls the Flow YAML string from a pipeline GET response.
// Yunxiao typically nests it at pipelineConfig.flow (escaped string).
func ExtractFlowYAML(pipeline map[string]any) (string, error) {
	if pipeline == nil {
		return "", fmt.Errorf("empty pipeline object")
	}
	if s := asString(pipeline["content"]); s != "" && looksLikeFlowYAML(s) {
		return s, nil
	}
	cfg := asMap(pipeline["pipelineConfig"])
	if cfg == nil {
		if data := asMap(pipeline["data"]); data != nil {
			return ExtractFlowYAML(data)
		}
		return "", fmt.Errorf("pipelineConfig missing; cannot extract flow YAML")
	}
	if s := asString(cfg["flow"]); s != "" {
		return s, nil
	}
	// Some payloads store the whole config as a JSON/YAML string.
	if s := asString(cfg["content"]); s != "" && looksLikeFlowYAML(s) {
		return s, nil
	}
	return "", fmt.Errorf("pipelineConfig.flow missing or empty")
}

func looksLikeFlowYAML(s string) bool {
	t := strings.TrimSpace(s)
	return strings.Contains(t, "sources:") || strings.Contains(t, "stages:") || strings.HasPrefix(t, "sources:") || strings.HasPrefix(t, "stages:")
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}
