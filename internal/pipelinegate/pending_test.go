package pipelinegate

import (
	"encoding/json"
	"testing"
)

const fixtureManualWaiting = `{
  "pipelineId": 5272454,
  "pipelineRunId": 88,
  "status": "WAITING",
  "stages": [
    {
      "name": "Build",
      "stageInfo": {
        "jobs": [
          {
            "id": 1001,
            "name": "compile",
            "jobSign": "Build",
            "status": "SUCCESS",
            "actions": []
          }
        ]
      }
    },
    {
      "name": "Gate",
      "stageInfo": {
        "jobs": [
          {
            "id": 123456789012345,
            "name": "人工确认",
            "jobSign": "ManualValidate",
            "status": "WAITING",
            "actions": ["pass", "refuse"]
          }
        ]
      }
    }
  ]
}`

const fixtureNestedData = `{
  "data": {
    "pipelineId": "42",
    "id": 7,
    "status": "RUNNING",
    "stages": [
      {
        "stageName": "Approve",
        "jobs": [
          {
            "id": 55.0,
            "jobName": "Manual gate",
            "jobSign": "ManualValidate",
            "status": "RUNNING",
            "actions": ["PASS", "Refuse"]
          }
        ]
      }
    ]
  }
}`

const fixtureNameOnly = `{
  "pipelineId": "1",
  "pipelineRunId": "2",
  "status": "WAITING",
  "stages": [
    {
      "name": "S",
      "jobs": [
        {
          "id": "99",
          "name": "人工卡点",
          "jobSign": "Other",
          "status": "WAITING",
          "actions": []
        },
        {
          "id": "100",
          "name": "人工卡点-done",
          "jobSign": "ManualValidate",
          "status": "SUCCESS",
          "actions": []
        }
      ]
    }
  ]
}`

const fixtureSuccessWithActions = `{
  "pipelineId": "9",
  "pipelineRunId": "10",
  "status": "RUNNING",
  "stages": [
    {
      "name": "Gate",
      "jobs": [
        {
          "id": "200",
          "name": "人工确认",
          "jobSign": "ManualValidate",
          "status": "SUCCESS",
          "actions": ["pass", "refuse"]
        },
        {
          "id": "201",
          "name": "still-waiting",
          "jobSign": "ManualValidate",
          "status": "WAITING",
          "actions": ["pass", "refuse"]
        }
      ]
    }
  ]
}`

func mustParse(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("json: %v", err)
	}
	return m
}

func TestExtractPendingJobs_actionBased(t *testing.T) {
	jobs := ExtractPendingJobs(mustParse(t, fixtureManualWaiting), "5272454")
	if len(jobs) != 1 {
		t.Fatalf("len=%d want 1: %+v", len(jobs), jobs)
	}
	j := jobs[0]
	if j.PipelineID != "5272454" || j.RunID != "88" {
		t.Fatalf("ids: %+v", j)
	}
	if j.JobID != "123456789012345" {
		t.Fatalf("jobId=%q (need non-scientific)", j.JobID)
	}
	if j.JobSign != "ManualValidate" || j.Status != "WAITING" {
		t.Fatalf("sign/status: %+v", j)
	}
	if j.StageName != "Gate" {
		t.Fatalf("stage=%q", j.StageName)
	}
	if j.RunURL == "" || j.PassCmd == "" || j.RefuseCmd == "" {
		t.Fatalf("urls/cmds empty: %+v", j)
	}
}

func TestExtractPendingJobs_nestedData(t *testing.T) {
	jobs := ExtractPendingJobs(mustParse(t, fixtureNestedData), "")
	if len(jobs) != 1 {
		t.Fatalf("len=%d want 1: %+v", len(jobs), jobs)
	}
	j := jobs[0]
	if j.PipelineID != "42" || j.RunID != "7" || j.JobID != "55" {
		t.Fatalf("ids: %+v", j)
	}
	if j.StageName != "Approve" {
		t.Fatalf("stage=%q", j.StageName)
	}
}

func TestExtractPendingJobs_nameFallback(t *testing.T) {
	jobs := ExtractPendingJobs(mustParse(t, fixtureNameOnly), "1")
	if len(jobs) != 1 {
		t.Fatalf("len=%d want 1 (SUCCESS manual skipped): %+v", len(jobs), jobs)
	}
	if jobs[0].JobID != "99" {
		t.Fatalf("jobId=%q", jobs[0].JobID)
	}
}

func TestExtractPendingJobs_terminalWithStaleActions(t *testing.T) {
	jobs := ExtractPendingJobs(mustParse(t, fixtureSuccessWithActions), "9")
	if len(jobs) != 1 {
		t.Fatalf("len=%d want 1 (SUCCESS+actions must be skipped): %+v", len(jobs), jobs)
	}
	if jobs[0].JobID != "201" {
		t.Fatalf("jobId=%q want 201", jobs[0].JobID)
	}
}

func TestClassifyWatchOutcome(t *testing.T) {
	pending := []PendingJob{{JobID: "1"}}
	type tc struct {
		status string
		pend   []PendingJob
		out    string
		code   int
	}
	cases := []tc{
		{"SUCCESS", nil, "success", 0},
		{"FAIL", nil, "fail", 1},
		{"FAILED", nil, "fail", 1},
		{"CANCELED", nil, "canceled", 2},
		{"CANCELLED", nil, "canceled", 2},
		{"WAITING", pending, "gate_paused", 3},
		{"RUNNING", pending, "gate_paused", 3},
		{"WAITING", nil, "still_running", -1},
		{"RUNNING", nil, "still_running", -1},
		{"WEIRD", nil, "still_running", -1},
		{"SKIPPED", nil, "fail", 1},
	}
	for _, c := range cases {
		out, code := ClassifyWatchOutcome(c.status, c.pend)
		if out != c.out || code != c.code {
			t.Fatalf("status=%s pend=%d → (%s,%d) want (%s,%d)", c.status, len(c.pend), out, code, c.out, c.code)
		}
	}
}
