package zhiyi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSummaryPipelineRun(t *testing.T) {
	run := map[string]any{
		"pipelineRunId": float64(88),
		"status":        "SUCCESS",
		"resultStatus":  "SUCCESS",
		"createTime":    float64(1710000000000),
		"endTime":       float64(1710000100000),
		"globalParams": []any{
			map[string]any{"key": "NACOS_PASSWORD", "value": "nacos"},
			map[string]any{"key": "APP_PATH", "value": "/opt/app"},
		},
		"pipelineConfig": map[string]any{"flow": "stages:\n- name: huge"},
		"stages": []any{
			map[string]any{
				"name":   "build",
				"status": "SUCCESS",
				"jobs": []any{
					map[string]any{"id": "j1", "name": "compile", "status": "SUCCESS"},
					map[string]any{"id": float64(2), "name": "test", "resultStatus": "SUCCESS"},
				},
			},
		},
	}
	got := SummaryPipelineRun(run, "4432894")
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`"runId":"88"`,
		`"pipelineRunId":88`,
		`"pipelineId":"4432894"`,
		`"status":"SUCCESS"`,
		`"startTime":1710000000000`,
		`"endTime":1710000100000`,
		`"url":"https://flow.aliyun.com/pipelines/4432894/builds/88"`,
		`"name":"build"`,
		`"name":"compile"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s: %s", want, s)
		}
	}
	if !strings.Contains(s, `"stages":[`) {
		t.Fatalf("stages missing: %s", s)
	}
	for _, no := range []string{"globalParams", "NACOS_PASSWORD", "pipelineConfig", "APP_PATH"} {
		if strings.Contains(s, no) {
			t.Fatalf("summary must not contain %s: %s", no, s)
		}
	}
}

func TestSummaryPipelineRun_runIdFromBareID(t *testing.T) {
	got := SummaryPipelineRun(map[string]any{"id": float64(4), "status": "RUNNING"}, "5272454")
	if got["runId"] != "4" {
		t.Fatalf("runId=%v want 4; %#v", got["runId"], got)
	}
	if got["url"] != "https://flow.aliyun.com/pipelines/5272454/builds/4" {
		t.Fatalf("url=%v", got["url"])
	}
}

func TestSummaryPipelineRun_nil(t *testing.T) {
	got := SummaryPipelineRun(nil, "1")
	if len(got) != 0 {
		t.Fatalf("%#v", got)
	}
}

func TestStagesJobsSummary_empty(t *testing.T) {
	if StagesJobsSummary(map[string]any{}) != nil {
		t.Fatal("want nil")
	}
}
