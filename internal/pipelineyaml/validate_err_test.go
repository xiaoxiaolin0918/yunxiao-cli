package pipelineyaml

import "testing"

func TestParseYAMLValidationError_1209300(t *testing.T) {
	body := `{"errorCode":"1209300","errorMessage":"yaml校验失败 {\"errorMessage\":\"validators invalid\",\"path\":\"stages[0].jobs[0].steps[0].with.validators\"}"}`
	code, issues, ok := ParseYAMLValidationError(body)
	if !ok || code != "1209300" {
		t.Fatalf("%v %q %#v", ok, code, issues)
	}
	if len(issues) == 0 {
		t.Fatal("no issues")
	}
	found := false
	for _, it := range issues {
		if it.Path != "" || (it.ErrorMessage != "" && it.ErrorMessage != body) {
			found = true
		}
	}
	if !found {
		t.Fatalf("issues=%#v", issues)
	}
}

func TestParseYAMLValidationError_plain(t *testing.T) {
	_, _, ok := ParseYAMLValidationError(`{"errorCode":"Other","errorMessage":"nope"}`)
	if ok {
		t.Fatal("expected not ok")
	}
}
