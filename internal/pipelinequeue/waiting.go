package pipelinequeue

import (
	"fmt"
	"strings"
	"time"
)

// QueueEntry is one RUNNING/WAITING run row for +queue / status.
type QueueEntry struct {
	PipelineID   string   `json:"pipeline_id"`
	PipelineName string   `json:"pipeline_name,omitempty"`
	RunID        string   `json:"run_id"`
	Status       string   `json:"status"`
	StartTimeMs  int64    `json:"start_time_ms,omitempty"`
	WaitSeconds  int64    `json:"wait_seconds,omitempty"`
	RunnerGroups []string `json:"runner_groups,omitempty"`
	URL          string   `json:"url,omitempty"`
}

// WaitingJob is a job still waiting inside a run detail.
type WaitingJob struct {
	JobID   string `json:"job_id,omitempty"`
	JobName string `json:"job_name,omitempty"`
	JobSign string `json:"job_sign,omitempty"`
	Status  string `json:"status,omitempty"`
}

// ExtractWaitingJobs walks stages/jobs for non-terminal waiting-ish statuses.
func ExtractWaitingJobs(run map[string]any) []WaitingJob {
	if run == nil {
		return nil
	}
	var out []WaitingJob
	stages, _ := run["stages"].([]any)
	for _, s := range stages {
		sm, _ := s.(map[string]any)
		if sm == nil {
			continue
		}
		si, _ := sm["stageInfo"].(map[string]any)
		var jobs []any
		if si != nil {
			jobs, _ = si["jobs"].([]any)
		}
		if jobs == nil {
			jobs, _ = sm["jobs"].([]any)
		}
		for _, j := range jobs {
			jm, _ := j.(map[string]any)
			if jm == nil {
				continue
			}
			st := strings.ToUpper(strings.TrimSpace(fmt.Sprint(jm["status"])))
			if st != "WAITING" && st != "PENDING" && st != "QUEUED" && st != "WAIT" {
				continue
			}
			out = append(out, WaitingJob{
				JobID:   stringify(jm["id"]),
				JobName: stringify(jm["name"]),
				JobSign: stringify(jm["jobSign"]),
				Status:  st,
			})
		}
	}
	return out
}

// WaitSecondsSince computes seconds since startTime ms (or 0).
func WaitSecondsSince(startMs int64, now time.Time) int64 {
	if startMs <= 0 {
		return 0
	}
	sec := now.Unix() - startMs/1000
	if sec < 0 {
		return 0
	}
	return sec
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return fmt.Sprintf("%.0f", t)
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "<nil>" {
			return ""
		}
		return s
	}
}

// AsInt64 coerces JSON numbers / strings to int64 ms timestamps.
func AsInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		var n int64
		fmt.Sscan(t, &n)
		return n
	default:
		return 0
	}
}
