package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

// withNoDiskProfileExamples simulates the npm / GitHub Release install layout
// (bin/yunxiao + skills/ only): no profiles/ directory exists on disk (#92).
func withNoDiskProfileExamples(t *testing.T) {
	t.Helper()
	empty := t.TempDir()
	prev := profileExampleSearchDirs
	profileExampleSearchDirs = func() []string {
		return []string{filepath.Join(empty, "profiles"), filepath.Join(empty, "bin", "profiles")}
	}
	t.Cleanup(func() { profileExampleSearchDirs = prev })
}

// runProfileCmd executes rootCmd with args and returns stdout, stderr and the
// processExit code (0 when the command did not exit).
func runProfileCmd(t *testing.T, dryRun bool, args ...string) (string, string, int) {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	globalDryRun = dryRun
	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })

	prevExit := processExit
	code := 0
	processExit = func(c int) {
		code = c
		panic(exitPanic{code: c})
	}
	t.Cleanup(func() { processExit = prevExit })

	resetStringFlags(t, profileInstallExampleCmd, "force")
	rootCmd.SetArgs(args)
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
				}
			}
		}()
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	}()
	return stdout.String(), stderr.String(), code
}

func repoProfileExample(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "profiles", name+".example.json"))
	if err != nil {
		t.Fatalf("read repo example: %v", err)
	}
	return b
}

// #92: npm package / release archive ship no profiles/ next to the binary;
// install-example must still work (examples are embedded in the binary).
func TestProfileInstallExampleWithoutProfilesOnDisk(t *testing.T) {
	for _, name := range []string{"zhiyi", "play"} {
		t.Run(name, func(t *testing.T) {
			xdg := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", xdg)
			withNoDiskProfileExamples(t)

			stdout, stderr, code := runProfileCmd(t, false, "profile", "install-example", name)
			if code != 0 {
				t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
			}
			var env struct {
				OK   bool `json:"ok"`
				Data struct {
					Installed string `json:"installed"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &env); err != nil {
				t.Fatalf("decode %q: %v", stdout, err)
			}
			want := filepath.Join(xdg, "yunxiao", "profiles", name+".json")
			if !env.OK || env.Data.Installed != want {
				t.Fatalf("got %+v want installed=%s", env, want)
			}
			got, err := os.ReadFile(want)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, repoProfileExample(t, name)) {
				t.Fatalf("installed %s differs from profiles/%s.example.json", want, name)
			}
			p, err := profile.Load(name)
			if err != nil {
				t.Fatalf("profile show/load: %v", err)
			}
			if p.Name == "" {
				t.Fatalf("empty profile name after install")
			}
		})
	}
}

func TestProfileInstallExampleDryRunEmbeddedSource(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	withNoDiskProfileExamples(t)

	stdout, stderr, code := runProfileCmd(t, true, "profile", "install-example", "zhiyi", "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, `"dry_run":true`) || !strings.Contains(stdout, `embedded:profiles/zhiyi.example.json`) {
		t.Fatalf("dry-run preview should name embedded source: %s", stdout)
	}
	if _, err := os.Stat(filepath.Join(xdg, "yunxiao", "profiles", "zhiyi.json")); !os.IsNotExist(err) {
		t.Fatalf("dry-run must not write profile (stat err=%v)", err)
	}
}

func TestProfileInstallExampleUnknownNameListsAvailable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	withNoDiskProfileExamples(t)

	stdout, stderr, code := runProfileCmd(t, false, "profile", "install-example", "nope")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	for _, want := range []string{"example not found", "play", "zhiyi", "raw.githubusercontent.com"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q: %s", want, stderr)
		}
	}
}

func TestProfileInstallExampleRejectsPathName(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	withNoDiskProfileExamples(t)

	_, stderr, code := runProfileCmd(t, false, "profile", "install-example", "../zhiyi")
	if code != 1 || !strings.Contains(stderr, "invalid example name") {
		t.Fatalf("expected invalid name error, code=%d stderr=%s", code, stderr)
	}
}

// Disk copies (source checkout / npm package profiles/) still win when present.
func TestProfileInstallExamplePrefersDiskCopy(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	disk := t.TempDir()
	custom := []byte("{\"name\":\"zhiyi\",\"space_id\":\"disk-copy\"}\n")
	if err := os.WriteFile(filepath.Join(disk, "zhiyi.example.json"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	prev := profileExampleSearchDirs
	profileExampleSearchDirs = func() []string { return []string{disk} }
	t.Cleanup(func() { profileExampleSearchDirs = prev })

	stdout, stderr, code := runProfileCmd(t, false, "profile", "install-example", "zhiyi")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	got, err := os.ReadFile(filepath.Join(xdg, "yunxiao", "profiles", "zhiyi.json"))
	if err != nil || !bytes.Equal(got, custom) {
		t.Fatalf("expected disk copy installed, got %q err=%v", got, err)
	}
}
