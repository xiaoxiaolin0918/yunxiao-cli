package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/output"
)

func TestPipelineRunTriggerTopLevelRuntimeVarsWrappedDryRun(t *testing.T) {
	stdout, hits := pipeTriggerSetup(t, "wrap-runtime")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "trigger",
		"--pipeline-id", "pipe-trig",
		"--branch", "test",
		"--params", `{"APP_NAME":"blade-gateway","MOUDULE_NAME":"/"}`,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeTriggerAssert(t, stdout, hits, func(t *testing.T, params any, _ map[string]any) {
		m := decodeParamsObject(t, params)
		if _, ok := m["APP_NAME"]; ok {
			t.Fatalf("APP_NAME must not stay top-level: %v", m)
		}
		envs, _ := m["envs"].(map[string]any)
		if envs["APP_NAME"] != "blade-gateway" || envs["MOUDULE_NAME"] != "/" {
			t.Fatalf("envs=%v m=%v", envs, m)
		}
		brRaw, _ := json.Marshal(m["branchModeBranchs"])
		if !strings.Contains(string(brRaw), "test") {
			t.Fatalf("branchModeBranchs=%v", m["branchModeBranchs"])
		}
	})
}

func TestPipelineRunTriggerExistingEnvsNotDoubleWrappedDryRun(t *testing.T) {
	stdout, hits := pipeTriggerSetup(t, "keep-envs")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "trigger",
		"--pipeline-id", "pipe-trig",
		"--params", `{"envs":{"APP_NAME":"blade-gateway","MOUDULE_NAME":"/"},"comment":"ok"}`,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeTriggerAssert(t, stdout, hits, func(t *testing.T, params any, _ map[string]any) {
		m := decodeParamsObject(t, params)
		envs, _ := m["envs"].(map[string]any)
		if envs["APP_NAME"] != "blade-gateway" || envs["MOUDULE_NAME"] != "/" {
			t.Fatalf("envs=%v", envs)
		}
		if nested, ok := envs["envs"]; ok {
			t.Fatalf("double-wrapped: %v", nested)
		}
		if m["comment"] != "ok" {
			t.Fatalf("comment=%v", m["comment"])
		}
	})
}

func TestPipelineRunTriggerParamsFileDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "params.json")
	// UTF-8 BOM should be stripped by --params-file.
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"APP_NAME":"blade-system","MOUDULE_NAME":"/"}`)...)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, hits := pipeTriggerSetup(t, "params-file")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "trigger",
		"--pipeline-id", "pipe-trig",
		"--branch", "test",
		"--params-file", path,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeTriggerAssert(t, stdout, hits, func(t *testing.T, params any, _ map[string]any) {
		m := decodeParamsObject(t, params)
		envs, _ := m["envs"].(map[string]any)
		if envs["APP_NAME"] != "blade-system" || envs["MOUDULE_NAME"] != "/" {
			t.Fatalf("envs=%v m=%v", envs, m)
		}
	})
}

func TestPipelineRunTriggerEnvsFlagDryRun(t *testing.T) {
	stdout, hits := pipeTriggerSetup(t, "envs-flag")
	rootCmd.SetArgs([]string{
		"pipeline", "run", "trigger",
		"--pipeline-id", "pipe-trig",
		"--branch", "test",
		"--envs", `{"APP_NAME":"blade-gateway","MOUDULE_NAME":"/"}`,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeTriggerAssert(t, stdout, hits, func(t *testing.T, params any, _ map[string]any) {
		m := decodeParamsObject(t, params)
		envs, _ := m["envs"].(map[string]any)
		if envs["APP_NAME"] != "blade-gateway" || envs["MOUDULE_NAME"] != "/" {
			t.Fatalf("envs=%v m=%v", envs, m)
		}
	})
}

func TestPipelineRunTriggerWrapHintOnStderr(t *testing.T) {
	stdout, hits := pipeTriggerSetup(t, "wrap-hint")
	var stderrBuf strings.Builder
	prevErr := output.Stderr
	output.Stderr = &stderrBuf
	t.Cleanup(func() { output.Stderr = prevErr })
	rootCmd.SetArgs([]string{
		"pipeline", "run", "trigger",
		"--pipeline-id", "pipe-trig",
		"--params", `{"APP_NAME":"blade-gateway"}`,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	pipeTriggerAssert(t, stdout, hits, func(t *testing.T, params any, _ map[string]any) {
		m := decodeParamsObject(t, params)
		envs, _ := m["envs"].(map[string]any)
		if envs["APP_NAME"] != "blade-gateway" {
			t.Fatalf("envs=%v", envs)
		}
	})
	hint := stderrBuf.String()
	if !strings.Contains(hint, "params.envs") || !strings.Contains(hint, "APP_NAME") {
		t.Fatalf("missing Chinese hint on stderr: %q", hint)
	}
}
