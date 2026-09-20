package pipelineyaml

import "testing"

func TestExtractFlowYAML(t *testing.T) {
	p := map[string]any{
		"pipelineConfig": map[string]any{
			"flow": "sources: {}\nstages:\n  - name: build\n",
		},
	}
	s, err := ExtractFlowYAML(p)
	if err != nil || !stringsContains(s, "stages:") {
		t.Fatalf("%v %q", err, s)
	}
	nested := map[string]any{"data": p}
	s2, err := ExtractFlowYAML(nested)
	if err != nil || s2 == "" {
		t.Fatal(err)
	}
}

func stringsContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || stringIndex(s, sub) >= 0)
}
func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
