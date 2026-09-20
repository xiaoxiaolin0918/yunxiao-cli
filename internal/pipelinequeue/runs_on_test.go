package pipelinequeue

import "testing"

func TestExtractRunnerGroups(t *testing.T) {
	y := `
stages:
  a:
    jobs:
      j:
        runsOn:
          group: private/UY4U0xvrIoD6MHo7
          vm: true
`
	gs := ExtractRunnerGroups(y)
	if len(gs) != 1 || gs[0] != "private/UY4U0xvrIoD6MHo7" {
		t.Fatalf("%#v", gs)
	}
}

func TestExtractWaitingJobs(t *testing.T) {
	run := map[string]any{
		"stages": []any{
			map[string]any{
				"stageInfo": map[string]any{
					"jobs": []any{
						map[string]any{"id": 1.0, "name": "x", "jobSign": "s", "status": "WAITING"},
						map[string]any{"id": 2.0, "name": "y", "status": "SUCCESS"},
					},
				},
			},
		},
	}
	jobs := ExtractWaitingJobs(run)
	if len(jobs) != 1 || jobs[0].JobName != "x" {
		t.Fatalf("%#v", jobs)
	}
}
