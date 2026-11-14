package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// pipelineReservedParamKeys are CreatePipelineRun params top-level keys that
// must stay at the params object root. Unknown top-level keys are treated as
// runtime / runningConfig variables and moved into envs (#404).
// See https://help.aliyun.com/zh/yunxiao/developer-reference/createpipelinerun
var pipelineReservedParamKeys = map[string]struct{}{
	"branchModeBranchs":        {},
	"envs":                     {},
	"runningBranchs":           {},
	"runningTags":              {},
	"runningPipelineArtifacts": {},
	"runningAcrArtifacts":      {},
	"runningPackagesArtifacts": {},
	"comment":                  {},
}

func isPipelineReservedParamKey(k string) bool {
	_, ok := pipelineReservedParamKeys[k]
	return ok
}

func copyStringAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// normalizePipelineRunParams moves non-reserved top-level keys into envs.
// Existing envs are never wrapped again; keys already present in envs win over
// same-named top-level keys being moved. extraEnvs (from --envs) overwrite
// both. Returns the normalized object and the sorted list of moved keys.
func normalizePipelineRunParams(paramsObj, extraEnvs map[string]any) (map[string]any, []string, error) {
	if paramsObj == nil {
		paramsObj = map[string]any{}
	}
	out := map[string]any{}
	var existingEnvs map[string]any
	if raw, ok := paramsObj["envs"]; ok && raw != nil {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("--params.envs must be a JSON object")
		}
		existingEnvs = copyStringAnyMap(m)
	} else {
		existingEnvs = map[string]any{}
	}

	var moved []string
	for k, v := range paramsObj {
		if k == "envs" {
			continue
		}
		if isPipelineReservedParamKey(k) {
			out[k] = v
			continue
		}
		moved = append(moved, k)
		if _, exists := existingEnvs[k]; !exists {
			existingEnvs[k] = v
		}
	}
	for k, v := range extraEnvs {
		existingEnvs[k] = v
	}
	if len(existingEnvs) > 0 {
		out["envs"] = existingEnvs
	}
	sort.Strings(moved)
	return out, moved, nil
}

func pipelineParamsWrapHint(moved []string) string {
	if len(moved) == 0 {
		return ""
	}
	return fmt.Sprintf("提示：检测到 --params 顶层含运行变量 [%s]，已自动放入 params.envs。自定义/运行时变量须放在 envs 下（例如 {\"envs\":{\"APP_NAME\":\"blade-gateway\",\"MOUDULE_NAME\":\"/\"}}），不要与 branchModeBranchs / comment 平级。请用 --dry-run 查看最终请求 body；Windows PowerShell 建议改用 --params-file。", strings.Join(moved, ", "))
}

// preparePipelineRunTriggerParams builds the CreatePipelineRun `params` string.
// --params / --params-file accept a JSON object or a JSON string; objects are
// normalized so runtime variables land in envs. A raw JSON string is passed
// through (existing behavior) and cannot be combined with --envs.
func preparePipelineRunTriggerParams(paramsInline, paramsFile, envsJSON, branch, comment string) (string, bool, string, error) {
	paramsText, err := readFlagOrFile(paramsInline, paramsFile, "params", false)
	if err != nil {
		return "", false, "", err
	}
	extraEnvs, err := parseJSONMap(envsJSON)
	if err != nil {
		return "", false, "", fmt.Errorf("--envs must be a JSON object: %w", err)
	}

	paramsObj := map[string]any{}
	if paramsText != "" {
		var raw any
		if err := json.Unmarshal([]byte(paramsText), &raw); err != nil {
			flag := "--params"
			if paramsFile != "" {
				flag = "--params-file"
			}
			return "", false, "", fmt.Errorf("%s must be JSON: %w", flag, err)
		}
		switch v := raw.(type) {
		case string:
			if extraEnvs != nil {
				return "", false, "", fmt.Errorf("--params JSON string cannot be combined with --envs; pass a JSON object via --params or --params-file")
			}
			return v, true, "", nil
		case map[string]any:
			paramsObj = v
		default:
			return "", false, "", fmt.Errorf("--params must be a JSON object or string")
		}
	}

	if branch != "" {
		paramsObj["branchModeBranchs"] = []string{branch}
	}
	if comment != "" {
		paramsObj["comment"] = comment
	}

	normalized, moved, err := normalizePipelineRunParams(paramsObj, extraEnvs)
	if err != nil {
		return "", false, "", err
	}
	if len(normalized) == 0 {
		return "", false, "", nil
	}
	b, err := json.Marshal(normalized)
	if err != nil {
		return "", false, "", err
	}
	return string(b), true, pipelineParamsWrapHint(moved), nil
}
