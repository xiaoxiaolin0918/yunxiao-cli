package zhiyi

import "strings"

// SummaryPipelineRun is the default `pipeline +status` projection (#359):
// runId (normalized from pipelineRunId / buildId / id), status fields, timing,
// url, and a compact stages/jobs digest. Omits globalParams and other bulky
// nested config that often carries plaintext credentials/paths.
func SummaryPipelineRun(run map[string]any, fallbackPipelineID string) map[string]any {
	if run == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	pid := pipelineIDForRun(run, fallbackPipelineID)
	rid := runIDForRun(run)
	if rid != "" {
		out["runId"] = rid
		// Keep the documented Flow key too so jq scripts on pipelineRunId still work.
		if _, ok := run["pipelineRunId"]; ok {
			out["pipelineRunId"] = run["pipelineRunId"]
		} else {
			out["pipelineRunId"] = rid
		}
	}
	if pid != "" {
		out["pipelineId"] = pid
	}
	copyRunFieldIfPresent(out, run, "status", "status")
	copyRunFieldIfPresent(out, run, "resultStatus", "resultStatus", "result_status")
	copyRunFieldIfPresent(out, run, "startTime", "startTime", "start_time", "createTime", "create_time")
	copyRunFieldIfPresent(out, run, "endTime", "endTime", "end_time", "finishTime", "finish_time")
	copyRunFieldIfPresent(out, run, "triggerMode", "triggerMode", "trigger_mode")
	if u := PipelineRunURL(pid, rid); u != "" {
		out["url"] = u
	} else if existing, ok := run["url"].(string); ok && strings.TrimSpace(existing) != "" {
		out["url"] = existing
	}
	if stages := StagesJobsSummary(run); len(stages) > 0 {
		out["stages"] = stages
	}
	return out
}

func copyRunFieldIfPresent(out, run map[string]any, dst string, keys ...string) {
	if run == nil {
		return
	}
	for _, k := range keys {
		if v, ok := run[k]; ok && v != nil {
			out[dst] = v
			return
		}
	}
}

// StagesJobsSummary projects run.stages down to [{name, status, jobs:[{id,name,status}]}].
// Missing/empty stages yield nil (callers omit the key).
func StagesJobsSummary(run map[string]any) []map[string]any {
	if run == nil {
		return nil
	}
	root := run
	if data, ok := run["data"].(map[string]any); ok {
		if _, has := data["stages"]; has {
			root = data
		}
	}
	raw, _ := root["stages"].([]any)
	if len(raw) == 0 {
		return nil
	}
	var out []map[string]any
	for _, st := range raw {
		sm, ok := st.(map[string]any)
		if !ok {
			continue
		}
		entry := map[string]any{}
		if name := stringField(sm, "name", "stageName"); name != "" {
			entry["name"] = name
		} else if info, ok := sm["stageInfo"].(map[string]any); ok {
			if name := stringField(info, "name", "stageName"); name != "" {
				entry["name"] = name
			}
		}
		copyRunFieldIfPresent(entry, sm, "status", "status", "resultStatus", "stageStatus")
		if jobs := jobsSummaryFromStage(sm); len(jobs) > 0 {
			entry["jobs"] = jobs
		}
		if len(entry) > 0 {
			out = append(out, entry)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func jobsSummaryFromStage(stage map[string]any) []map[string]any {
	raw := stage["jobs"]
	if raw == nil {
		if nested, ok := stage["stageInfo"].(map[string]any); ok {
			raw = nested["jobs"]
		}
	}
	arr, _ := raw.([]any)
	if len(arr) == 0 {
		return nil
	}
	var out []map[string]any
	for _, j := range arr {
		jm, ok := j.(map[string]any)
		if !ok {
			continue
		}
		entry := map[string]any{}
		if id := mapFieldID(jm, "id", "jobId", "job_id"); id != "" {
			entry["id"] = id
		}
		if name := stringField(jm, "name", "jobName"); name != "" {
			entry["name"] = name
		}
		copyRunFieldIfPresent(entry, jm, "status", "status", "resultStatus", "jobStatus")
		if len(entry) > 0 {
			out = append(out, entry)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}