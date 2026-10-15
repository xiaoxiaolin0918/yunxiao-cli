package zhiyi

import (
	"fmt"
	"testing"
)

func TestPipelineURL(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"5272454", "https://flow.aliyun.com/pipelines/5272454"},
		{" 5272454 ", "https://flow.aliyun.com/pipelines/5272454"},
		{"", ""},
		{"<nil>", ""},
	}
	for _, tt := range tests {
		if got := PipelineURL(tt.id); got != tt.want {
			t.Fatalf("PipelineURL(%q)=%q want %q", tt.id, got, tt.want)
		}
	}
}

func TestPipelineRunURL(t *testing.T) {
	tests := []struct {
		pid, rid, want string
	}{
		{"5272454", "4", "https://flow.aliyun.com/pipelines/5272454/builds/4"},
		{"5272454", "", ""},
		{"", "4", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		if got := PipelineRunURL(tt.pid, tt.rid); got != tt.want {
			t.Fatalf("PipelineRunURL(%q,%q)=%q want %q", tt.pid, tt.rid, got, tt.want)
		}
	}
}

func TestPipelineIDFromMap_numericAndKeys(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		want string
	}{
		{"pipelineId string", map[string]any{"pipelineId": "5272454"}, "5272454"},
		{"id float64", map[string]any{"id": float64(5272454)}, "5272454"},
		{"id int", map[string]any{"id": 99}, "99"},
		{"prefer pipelineId over id", map[string]any{"pipelineId": "A", "id": "B"}, "A"},
		{"nil", nil, ""},
		{"empty", map[string]any{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PipelineIDFromMap(tt.m); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestPipelineRunIDFromMap(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		want string
	}{
		{"pipelineRunId", map[string]any{"pipelineRunId": float64(4)}, "4"},
		{"runId", map[string]any{"runId": "4"}, "4"},
		{"buildId", map[string]any{"buildId": 4}, "4"},
		{"no bare id", map[string]any{"id": "4"}, ""},
		{"prefer pipelineRunId", map[string]any{"pipelineRunId": "1", "buildId": "9"}, "1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PipelineRunIDFromMap(tt.m); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestEnrichPipelineMeta(t *testing.T) {
	meta := EnrichPipelineMeta(map[string]any{"risk": "read"}, map[string]any{"id": float64(5272454)})
	if meta["url"] != "https://flow.aliyun.com/pipelines/5272454" {
		t.Fatalf("%v", meta)
	}
	empty := EnrichPipelineMeta(nil, nil)
	if _, ok := empty["url"]; ok {
		t.Fatalf("unexpected url: %v", empty)
	}
}

func TestAttachPipelineURLs(t *testing.T) {
	arr := []any{
		map[string]any{"id": float64(5272454), "name": "p"},
		map[string]any{"pipelineId": "9"},
	}
	out := AttachPipelineURLs(arr).([]any)
	if out[0].(map[string]any)["url"] != "https://flow.aliyun.com/pipelines/5272454" {
		t.Fatalf("%v", out[0])
	}
	if out[1].(map[string]any)["url"] != "https://flow.aliyun.com/pipelines/9" {
		t.Fatalf("%v", out[1])
	}

	wrapped := map[string]any{
		"items": []any{map[string]any{"id": "1"}},
	}
	w := AttachPipelineURLs(wrapped).(map[string]any)
	item := w["items"].([]any)[0].(map[string]any)
	if item["url"] != "https://flow.aliyun.com/pipelines/1" {
		t.Fatalf("%v", item)
	}
}

func TestEnrichPipelineRunMeta(t *testing.T) {
	tests := []struct {
		name string
		run  map[string]any
		fb   string
		want string
	}{
		{
			name: "explicit ids",
			run:  map[string]any{"pipelineId": "5272454", "pipelineRunId": float64(4)},
			want: "https://flow.aliyun.com/pipelines/5272454/builds/4",
		},
		{
			name: "fallback pipeline + buildId",
			run:  map[string]any{"buildId": "4"},
			fb:   "5272454",
			want: "https://flow.aliyun.com/pipelines/5272454/builds/4",
		},
		{
			name: "fallback + bare id as run",
			run:  map[string]any{"id": float64(4)},
			fb:   "5272454",
			want: "https://flow.aliyun.com/pipelines/5272454/builds/4",
		},
		{
			name: "bare id alone is not enough",
			run:  map[string]any{"id": float64(4)},
			want: "",
		},
		{
			name: "example from brief",
			run:  map[string]any{"pipelineId": 5272454, "pipelineRunId": 4},
			want: "https://flow.aliyun.com/pipelines/5272454/builds/4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var meta map[string]any
			if tt.fb != "" {
				meta = EnrichPipelineRunMeta(map[string]any{"risk": "read"}, tt.run, tt.fb)
			} else {
				meta = EnrichPipelineRunMeta(map[string]any{"risk": "read"}, tt.run)
			}
			got := ""
			if u, ok := meta["url"]; ok {
				got = fmt.Sprint(u)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q meta=%v", got, tt.want, meta)
			}
		})
	}
}

func TestAttachPipelineRunURLs(t *testing.T) {
	arr := []any{
		map[string]any{"pipelineRunId": float64(4)},
		map[string]any{"pipelineId": "9", "runId": "2"},
	}
	out := AttachPipelineRunURLs(arr, "5272454").([]any)
	if out[0].(map[string]any)["url"] != "https://flow.aliyun.com/pipelines/5272454/builds/4" {
		t.Fatalf("%v", out[0])
	}
	if out[1].(map[string]any)["url"] != "https://flow.aliyun.com/pipelines/9/builds/2" {
		t.Fatalf("%v", out[1])
	}

	wrapped := map[string]any{
		"runs": []any{map[string]any{"buildId": "7"}},
	}
	w := AttachPipelineRunURLs(wrapped, "1").(map[string]any)
	item := w["runs"].([]any)[0].(map[string]any)
	if item["url"] != "https://flow.aliyun.com/pipelines/1/builds/7" {
		t.Fatalf("%v", item)
	}
}


func TestEnsurePipelineRunID(t *testing.T) {
	m := map[string]any{"pipelineRunId": float64(88)}
	EnsurePipelineRunID(m)
	if m["runId"] != "88" {
		t.Fatalf("%#v", m)
	}
	m2 := map[string]any{"id": float64(4)}
	EnsurePipelineRunID(m2)
	if m2["runId"] != "4" || m2["pipelineRunId"] != "4" {
		t.Fatalf("%#v", m2)
	}
	m3 := map[string]any{"runId": "9", "pipelineRunId": float64(9)}
	EnsurePipelineRunID(m3)
	if m3["runId"] != "9" {
		t.Fatalf("must not overwrite: %#v", m3)
	}
}