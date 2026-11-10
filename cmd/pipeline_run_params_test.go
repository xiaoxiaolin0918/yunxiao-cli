package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizePipelineRunParamsMovesRuntimeVars(t *testing.T) {
	out, moved, err := normalizePipelineRunParams(map[string]any{
		"APP_NAME":     "blade-gateway",
		"MOUDULE_NAME": "/",
		"comment":      "ZYPT-5845",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(moved, ",") != "APP_NAME,MOUDULE_NAME" {
		t.Fatalf("moved=%v", moved)
	}
	if out["comment"] != "ZYPT-5845" {
		t.Fatalf("comment=%v", out["comment"])
	}
	if _, ok := out["APP_NAME"]; ok {
		t.Fatalf("APP_NAME should have moved into envs: %v", out)
	}
	envs, _ := out["envs"].(map[string]any)
	if envs["APP_NAME"] != "blade-gateway" || envs["MOUDULE_NAME"] != "/" {
		t.Fatalf("envs=%v", envs)
	}
}

func TestNormalizePipelineRunParamsDoesNotDoubleWrapEnvs(t *testing.T) {
	out, moved, err := normalizePipelineRunParams(map[string]any{
		"envs": map[string]any{
			"APP_NAME": "blade-auth",
		},
		"APP_NAME":     "blade-gateway",
		"MOUDULE_NAME": "/",
		"comment":      "keep",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(moved, ",") != "APP_NAME,MOUDULE_NAME" {
		t.Fatalf("moved=%v", moved)
	}
	envs, _ := out["envs"].(map[string]any)
	if envs["APP_NAME"] != "blade-auth" {
		t.Fatalf("existing envs.APP_NAME must win, got %v", envs["APP_NAME"])
	}
	if envs["MOUDULE_NAME"] != "/" {
		t.Fatalf("envs=%v", envs)
	}
	if _, ok := out["envs"].(map[string]any)["envs"]; ok {
		t.Fatalf("double-wrapped envs: %v", out)
	}
	if out["comment"] != "keep" {
		t.Fatalf("comment=%v", out["comment"])
	}
}

func TestNormalizePipelineRunParamsKeepsReservedKeys(t *testing.T) {
	out, moved, err := normalizePipelineRunParams(map[string]any{
		"branchModeBranchs":        []any{"test"},
		"comment":                  "c",
		"runningBranchs":           map[string]any{"https://example.git": "test"},
		"runningTags":              map[string]any{"https://example.git": "v1"},
		"runningPipelineArtifacts": map[string]any{"1": "2"},
		"runningAcrArtifacts":      map[string]any{"reg": "tag"},
		"runningPackagesArtifacts": map[string]any{"pkg": "v"},
		"APP_NAME":                 "blade-gateway",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(moved, ",") != "APP_NAME" {
		t.Fatalf("moved=%v", moved)
	}
	for _, k := range []string{
		"branchModeBranchs", "comment", "runningBranchs", "runningTags",
		"runningPipelineArtifacts", "runningAcrArtifacts", "runningPackagesArtifacts",
	} {
		if _, ok := out[k]; !ok {
			t.Fatalf("missing reserved %s in %v", k, out)
		}
	}
	envs, _ := out["envs"].(map[string]any)
	if envs["APP_NAME"] != "blade-gateway" {
		t.Fatalf("envs=%v", envs)
	}
}

func TestNormalizePipelineRunParamsExtraEnvsWin(t *testing.T) {
	out, _, err := normalizePipelineRunParams(map[string]any{
		"envs":     map[string]any{"APP_NAME": "blade-auth"},
		"APP_NAME": "from-top",
	}, map[string]any{"APP_NAME": "blade-gateway", "MOUDULE_NAME": "/"})
	if err != nil {
		t.Fatal(err)
	}
	envs, _ := out["envs"].(map[string]any)
	if envs["APP_NAME"] != "blade-gateway" || envs["MOUDULE_NAME"] != "/" {
		t.Fatalf("envs=%v", envs)
	}
}

func TestNormalizePipelineRunParamsRejectsNonObjectEnvs(t *testing.T) {
	_, _, err := normalizePipelineRunParams(map[string]any{"envs": "nope"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPreparePipelineRunTriggerParamsFileAndHint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "params.json")
	if err := os.WriteFile(path, []byte(`{"APP_NAME":"blade-gateway","MOUDULE_NAME":"/"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok, hint, err := preparePipelineRunTriggerParams("", path, "", "test", "ZYPT-5845")
	if err != nil || !ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatal(err)
	}
	envs, _ := m["envs"].(map[string]any)
	if envs["APP_NAME"] != "blade-gateway" || envs["MOUDULE_NAME"] != "/" {
		t.Fatalf("envs=%v body=%s", envs, got)
	}
	br, _ := json.Marshal(m["branchModeBranchs"])
	if !strings.Contains(string(br), "test") {
		t.Fatalf("branchModeBranchs=%v", m["branchModeBranchs"])
	}
	if m["comment"] != "ZYPT-5845" {
		t.Fatalf("comment=%v", m["comment"])
	}
	if !strings.Contains(hint, "params.envs") || !strings.Contains(hint, "APP_NAME") {
		t.Fatalf("hint=%q", hint)
	}
}

func TestPreparePipelineRunTriggerParamsConflict(t *testing.T) {
	_, _, _, err := preparePipelineRunTriggerParams(`{"APP_NAME":"x"}`, "params.json", "", "", "")
	if err == nil || !strings.Contains(err.Error(), "--params") || !strings.Contains(err.Error(), "--params-file") {
		t.Fatalf("err=%v", err)
	}
}

func TestPreparePipelineRunTriggerRawStringRejectsEnvs(t *testing.T) {
	_, _, _, err := preparePipelineRunTriggerParams(`"keep-me"`, "", `{"APP_NAME":"x"}`, "", "")
	if err == nil || !strings.Contains(err.Error(), "--envs") {
		t.Fatalf("err=%v", err)
	}
}

func TestPreparePipelineRunTriggerRawStringPassthrough(t *testing.T) {
	got, ok, hint, err := preparePipelineRunTriggerParams(`"keep-me"`, "", "", "ignored", "ignored")
	if err != nil || !ok || got != "keep-me" || hint != "" {
		t.Fatalf("got=%q ok=%v hint=%q err=%v", got, ok, hint, err)
	}
}

func TestPipelineParamsWrapHintEmpty(t *testing.T) {
	if pipelineParamsWrapHint(nil) != "" {
		t.Fatal("empty hint")
	}
}
