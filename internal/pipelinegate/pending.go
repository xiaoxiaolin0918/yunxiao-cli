// Package pipelinegate extracts ManualValidate / human-gate jobs from Flow run detail
// and classifies watch outcomes for `pipeline +pending` and `pipeline run watch`.
package pipelinegate

import (
	"fmt"
	"strings"
)

// PendingJob is one unhandled manual gate found inside a pipeline run.
type PendingJob struct {
	PipelineID  string   `json:"pipelineId"`
	RunID       string   `json:"runId"`
	JobID       string   `json:"jobId"`
	JobName     string   `json:"jobName,omitempty"`
	JobSign     string   `json:"jobSign,omitempty"`
	Status      string   `json:"status,omitempty"`
	StageName   string   `json:"stageName,omitempty"`
	Actions     []string `json:"actions,omitempty"`
	PipelineURL string   `json:"pipelineUrl,omitempty"`
	RunURL      string   `json:"runUrl,omitempty"`
	// PassCmd / RefuseCmd are ready-to-copy CLI invocations (include --yes).
	PassCmd   string `json:"passCmd,omitempty"`
	RefuseCmd string `json:"refuseCmd,omitempty"`
}

// ExtractPendingJobs walks stages/jobs in a run detail payload and returns jobs that
// look like unhandled manual gates. Prefer actions containing pass/refuse; fall back to
// ManualValidate / 人工 / Manual name signals when status is not terminal success/fail.
func ExtractPendingJobs(run map[string]any, pipelineID string) []PendingJob {
	if run == nil {
		return nil
	}
	root := run
	if data, ok := run["data"].(map[string]any); ok {
		// Prefer nested data when it carries stages; otherwise keep top-level.
		if _, has := data["stages"]; has {
			root = data
		} else if _, has := run["stages"]; !has {
			root = data
		}
	}

	runID := firstString(root, "pipelineRunId", "runId", "id", "buildId")
	if runID == "" {
		runID = firstString(run, "pipelineRunId", "runId", "id", "buildId")
	}
	pid := strings.TrimSpace(pipelineID)
	if pid == "" {
		pid = firstString(root, "pipelineId", "pipeline_id")
		if pid == "" {
			pid = firstString(run, "pipelineId", "pipeline_id")
		}
	}

	stages := stagesFrom(root)
	if stages == nil {
		stages = stagesFrom(run)
	}

	var out []PendingJob
	for _, st := range stages {
		stageName := firstString(st, "name", "stageName")
		if stageName == "" {
			if info, ok := st["stageInfo"].(map[string]any); ok {
				stageName = firstString(info, "name", "stageName")
			}
		}
		jobs := jobsFromStage(st)
		for _, job := range jobs {
			if pj, ok := pendingFromJob(job, pid, runID, stageName); ok {
				out = append(out, pj)
			}
		}
	}
	return out
}

func stagesFrom(m map[string]any) []map[string]any {
	if m == nil {
		return nil
	}
	raw, ok := m["stages"]
	if !ok {
		return nil
	}
	return asMapSlice(raw)
}

func jobsFromStage(st map[string]any) []map[string]any {
	if st == nil {
		return nil
	}
	if jobs := asMapSlice(st["jobs"]); jobs != nil {
		return jobs
	}
	if info, ok := st["stageInfo"].(map[string]any); ok {
		if jobs := asMapSlice(info["jobs"]); jobs != nil {
			return jobs
		}
	}
	return nil
}

func pendingFromJob(job map[string]any, pipelineID, runID, stageName string) (PendingJob, bool) {
	actions := stringSlice(job["actions"])
	status := firstString(job, "status", "resultStatus", "jobStatus")
	name := firstString(job, "name", "jobName")
	sign := firstString(job, "jobSign", "sign", "jobType")
	id := stringifyID(job["id"])
	if id == "" {
		id = firstString(job, "jobId", "job_id")
	}

	actionHit := hasPassOrRefuse(actions)
	nameHit := looksLikeManualGate(name, sign)
	if !actionHit && !nameHit {
		return PendingJob{}, false
	}
	if isTerminalDone(status) {
		// Terminal jobs must not enter +pending even if actions still list pass/refuse
		// (API often leaves stale actions on SUCCESS/FAIL rows).
		return PendingJob{}, false
	}
	// Name-based fallback only when status is waiting-ish / unknown / running.
	if !actionHit && nameHit && !isGateOpenStatus(status) {
		return PendingJob{}, false
	}

	pj := PendingJob{
		PipelineID:  pipelineID,
		RunID:       runID,
		JobID:       id,
		JobName:     name,
		JobSign:     sign,
		Status:      status,
		StageName:   stageName,
		Actions:     actions,
		PipelineURL: pipelineURL(pipelineID),
		RunURL:      runURL(pipelineID, runID),
	}
	if pipelineID != "" && runID != "" && id != "" {
		pj.PassCmd = fmt.Sprintf("yunxiao pipeline job pass --pipeline-id %s --run-id %s --job-id %s --yes", pipelineID, runID, id)
		pj.RefuseCmd = fmt.Sprintf("yunxiao pipeline job refuse --pipeline-id %s --run-id %s --job-id %s --yes", pipelineID, runID, id)
	}
	return pj, true
}

func hasPassOrRefuse(actions []string) bool {
	for _, a := range actions {
		switch strings.ToLower(strings.TrimSpace(a)) {
		case "pass", "refuse":
			return true
		}
	}
	return false
}

func looksLikeManualGate(name, sign string) bool {
	s := strings.ToLower(name + " " + sign)
	if strings.Contains(s, "manualvalidate") || strings.Contains(s, "manual_validate") {
		return true
	}
	if strings.Contains(s, "manual") {
		return true
	}
	if strings.Contains(name, "人工") || strings.Contains(sign, "人工") {
		return true
	}
	return false
}

func isTerminalDone(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "SUCCESS", "SUCCEED", "FAIL", "FAILED", "ERROR", "CANCELED", "CANCELLED", "SKIP", "SKIPPED":
		return true
	default:
		return false
	}
}

func isGateOpenStatus(status string) bool {
	s := strings.ToUpper(strings.TrimSpace(status))
	if s == "" {
		return true
	}
	switch s {
	case "WAITING", "PENDING", "RUNNING", "WAIT", "MANUAL", "BLOCKED":
		return true
	default:
		return !isTerminalDone(s)
	}
}

// ClassifyWatchOutcome maps run status + pending gates to (outcome, exitCode).
// exitCode < 0 means keep polling (still_running).
func ClassifyWatchOutcome(runStatus string, pending []PendingJob) (outcome string, exitCode int) {
	s := strings.ToUpper(strings.TrimSpace(runStatus))
	switch s {
	case "SUCCESS", "SUCCEED":
		return "success", 0
	case "FAIL", "FAILED", "ERROR":
		return "fail", 1
	case "CANCELED", "CANCELLED":
		return "canceled", 2
	case "WAITING", "RUNNING", "PENDING", "WAIT":
		if len(pending) > 0 {
			return "gate_paused", 3
		}
		return "still_running", -1
	default:
		if s == "" {
			if len(pending) > 0 {
				return "gate_paused", 3
			}
			return "still_running", -1
		}
		// Unknown terminal-ish → fail
		if isTerminalDone(s) {
			return "fail", 1
		}
		if len(pending) > 0 {
			return "gate_paused", 3
		}
		return "still_running", -1
	}
}

func pipelineURL(pipelineID string) string {
	id := strings.TrimSpace(pipelineID)
	if id == "" {
		return ""
	}
	return "https://flow.aliyun.com/pipelines/" + id
}

func runURL(pipelineID, runID string) string {
	pid := strings.TrimSpace(pipelineID)
	rid := strings.TrimSpace(runID)
	if pid == "" || rid == "" {
		return ""
	}
	return "https://flow.aliyun.com/pipelines/" + pid + "/builds/" + rid
}

func firstString(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s := stringifyID(v); s != "" {
				return s
			}
		}
	}
	return ""
}

// stringifyID formats JSON numbers without scientific notation.
func stringifyID(v any) string {
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
	case jsonNumber:
		return t.String()
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "" || s == "<nil>" {
			return ""
		}
		return s
	}
}

// jsonNumber avoids importing encoding/json just for json.Number in tests that
// may pass plain floats; kept as a local interface for fmt.Stringer numbers.
type jsonNumber interface {
	String() string
}

func stringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return append([]string(nil), t...)
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			s := strings.TrimSpace(fmt.Sprint(x))
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func asMapSlice(v any) []map[string]any {
	switch t := v.(type) {
	case []map[string]any:
		return t
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, x := range t {
			if m, ok := x.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}
