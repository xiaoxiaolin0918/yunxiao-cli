package zhiyi

import (
	"fmt"
	"strings"
)

// anyIDString coerces JSON-decoded ids (string / float64 / int / int64) to a
// trimmed non-empty string. Empty, "<nil>", and blank strings return "".
func anyIDString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" || s == "<nil>" {
			return ""
		}
		return s
	case float64:
		return fmt.Sprintf("%.0f", t)
	case float32:
		return fmt.Sprintf("%.0f", t)
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case int32:
		return fmt.Sprintf("%d", t)
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "" || s == "<nil>" {
			return ""
		}
		return s
	}
}

func mapFieldID(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s := anyIDString(v); s != "" {
				return s
			}
		}
	}
	return ""
}

// PipelineURL returns the Flow console URL for a pipeline.
// Pattern: https://flow.aliyun.com/pipelines/{pipelineId}
func PipelineURL(pipelineID string) string {
	id := strings.TrimSpace(pipelineID)
	if id == "" || id == "<nil>" {
		return ""
	}
	return "https://flow.aliyun.com/pipelines/" + id
}

// PipelineRunURL returns the Flow console URL for a pipeline run/build.
// Pattern: https://flow.aliyun.com/pipelines/{pipelineId}/builds/{runId}
func PipelineRunURL(pipelineID, runID string) string {
	pid := strings.TrimSpace(pipelineID)
	rid := strings.TrimSpace(runID)
	if pid == "" || pid == "<nil>" || rid == "" || rid == "<nil>" {
		return ""
	}
	return "https://flow.aliyun.com/pipelines/" + pid + "/builds/" + rid
}

// PipelineIDFromMap reads pipelineId / id (string or numeric) from a pipeline object.
func PipelineIDFromMap(m map[string]any) string {
	return mapFieldID(m, "pipelineId", "pipeline_id", "id")
}

// PipelineRunIDFromMap reads pipelineRunId / runId / buildId (string or numeric).
// Does not fall back to bare "id" (ambiguous with pipeline objects).
func PipelineRunIDFromMap(m map[string]any) string {
	return mapFieldID(m, "pipelineRunId", "pipeline_run_id", "runId", "run_id", "buildId", "build_id")
}

// pipelineIDForRun reads explicit pipeline id keys only (never bare "id").
func pipelineIDForRun(m map[string]any, fallback string) string {
	if s := mapFieldID(m, "pipelineId", "pipeline_id"); s != "" {
		return s
	}
	return strings.TrimSpace(fallback)
}

// runIDForRun prefers dedicated run-id keys; falls back to bare "id" for APIs that
// only return id on a run object.
func runIDForRun(m map[string]any) string {
	if s := PipelineRunIDFromMap(m); s != "" {
		return s
	}
	return mapFieldID(m, "id")
}

// EnsurePipelineRunID sets runId on a run map when missing/null, using pipelineRunId /
// buildId / bare id (#359). Also backfills pipelineRunId when only runId/id exists.
// Mutates m in place.
func EnsurePipelineRunID(m map[string]any) {
	if m == nil {
		return
	}
	rid := runIDForRun(m)
	if rid == "" {
		return
	}
	if v, ok := m["runId"]; !ok || v == nil || strings.TrimSpace(fmt.Sprint(v)) == "" || fmt.Sprint(v) == "<nil>" {
		m["runId"] = rid
	}
	if v, ok := m["pipelineRunId"]; !ok || v == nil {
		m["pipelineRunId"] = rid
	}
}

// EnrichPipelineMeta sets meta.url from PipelineURL when the pipeline id is resolvable.
func EnrichPipelineMeta(meta map[string]any, pipeline map[string]any) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	if u := PipelineURL(PipelineIDFromMap(pipeline)); u != "" {
		meta["url"] = u
	}
	return meta
}

// AttachPipelineURLs injects a clickable "url" onto each pipeline object in list
// payloads (bare []any or common wrapper shapes). Single objects are enriched in place.
func AttachPipelineURLs(data any) any {
	switch v := data.(type) {
	case []any:
		for i := range v {
			if m, ok := v[i].(map[string]any); ok {
				if u := PipelineURL(PipelineIDFromMap(m)); u != "" {
					m["url"] = u
				}
			}
		}
		return v
	case map[string]any:
		if u := PipelineURL(PipelineIDFromMap(v)); u != "" {
			v["url"] = u
			return v
		}
		for _, key := range []string{"items", "list", "data", "pipelines"} {
			if inner, ok := v[key]; ok {
				v[key] = AttachPipelineURLs(inner)
			}
		}
		return v
	default:
		return data
	}
}

// EnrichPipelineRunMeta sets meta.url from PipelineRunURL when pipeline + run ids
// are resolvable. Optional fallbackPipelineID is used when the run map lacks pipelineId.
func EnrichPipelineRunMeta(meta map[string]any, run map[string]any, fallbackPipelineID ...string) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	fb := ""
	if len(fallbackPipelineID) > 0 {
		fb = fallbackPipelineID[0]
	}
	pid := pipelineIDForRun(run, fb)
	rid := runIDForRun(run)
	if u := PipelineRunURL(pid, rid); u != "" {
		meta["url"] = u
	}
	return meta
}

// AttachPipelineRunURLs injects a clickable "url" onto each run object in list
// payloads. Optional fallbackPipelineID is used when items lack pipelineId
// (e.g. list scoped under --pipeline-id).
func AttachPipelineRunURLs(data any, fallbackPipelineID ...string) any {
	fb := ""
	if len(fallbackPipelineID) > 0 {
		fb = strings.TrimSpace(fallbackPipelineID[0])
	}
	switch v := data.(type) {
	case []any:
		for i := range v {
			if m, ok := v[i].(map[string]any); ok {
				if u := PipelineRunURL(pipelineIDForRun(m, fb), runIDForRun(m)); u != "" {
					m["url"] = u
				}
			}
		}
		return v
	case map[string]any:
		if u := PipelineRunURL(pipelineIDForRun(v, fb), runIDForRun(v)); u != "" {
			v["url"] = u
			return v
		}
		for _, key := range []string{"items", "list", "data", "runs", "pipelineRuns"} {
			if inner, ok := v[key]; ok {
				v[key] = AttachPipelineRunURLs(inner, fb)
			}
		}
		return v
	default:
		return data
	}
}
