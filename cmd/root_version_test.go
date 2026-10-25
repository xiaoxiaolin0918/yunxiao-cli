package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/version"
)

// cli-manager contract: --version prints one-line ok-json envelope.
func TestRootVersionOKJSON(t *testing.T) {
	var stdout bytes.Buffer
	prevOut := rootCmd.OutOrStdout()
	prevErr := rootCmd.ErrOrStderr()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})
	t.Cleanup(func() {
		rootCmd.SetOut(prevOut)
		rootCmd.SetErr(prevErr)
		rootCmd.SetArgs(nil)
	})

	rootCmd.SetArgs([]string{"--version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute --version: %v\n%s", err, stdout.String())
	}

	line := strings.TrimSpace(stdout.String())
	if line == "" {
		t.Fatal("empty --version output")
	}
	if strings.Count(stdout.String(), "\n") > 1 {
		t.Fatalf("want single-line ok-json, got multiline: %q", stdout.String())
	}

	var env struct {
		OK     bool `json:"ok"`
		Result struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(line), &env); err != nil {
		t.Fatalf("not ok-json: %v / %q", err, line)
	}
	if !env.OK {
		t.Fatalf("ok=false: %q", line)
	}
	if env.Result.Name != "yunxiao" {
		t.Fatalf("name=%q want yunxiao", env.Result.Name)
	}
	if env.Result.Version != version.Version {
		t.Fatalf("version=%q want %q", env.Result.Version, version.Version)
	}
}
