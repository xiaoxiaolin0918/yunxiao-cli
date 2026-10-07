package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// captureSkillsOutput redirects the JSON writers for the duration of one
// installSkills call and returns the fresh stdout/stderr contents.
func captureSkillsOutput(t *testing.T, srcRoot, dstRoot string, force, dryRun bool) (string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	prevOut, prevErr := output.Stdout, output.Stderr
	prevJQ, prevFmt := output.JQ, output.Format
	output.Stdout = &stdout
	output.Stderr = &stderr
	output.JQ = ""
	output.Format = "json"
	t.Cleanup(func() {
		output.Stdout = prevOut
		output.Stderr = prevErr
		output.JQ = prevJQ
		output.Format = prevFmt
	})
	if err := installSkills(srcRoot, dstRoot, nil, false, force, dryRun); err != nil {
		t.Fatalf("installSkills: %v (stdout=%s stderr=%s)", err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

type skillsInstallResult struct {
	Installed      []map[string]any `json:"installed"`
	Skipped        []map[string]any `json:"skipped"`
	InstalledCount int              `json:"installed_count"`
	SkippedCount   int              `json:"skipped_count"`
	TargetDir      string           `json:"target_dir"`
	Hint           string           `json:"hint"`
}

type skillsInstallEnvelope struct {
	OK     bool                `json:"ok"`
	DryRun bool                `json:"dry_run"`
	Data   skillsInstallResult `json:"data"`
}

// TestInstallSkillsReportsExistingSkips locks #119: rerunning skills install
// over an existing target dir must not silently report installed:0/skipped:[]
// — skipped skills are listed with reason "exists", counted in
// skipped_count, and warned on stderr with a --force hint.
func TestInstallSkillsReportsExistingSkips(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	for _, n := range []string{"yunxiao-alpha", "yunxiao-beta"} {
		d := filepath.Join(srcRoot, n)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("# "+n+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// First run: both installed, nothing skipped, no warning.
	stdout, stderr := captureSkillsOutput(t, srcRoot, dstRoot, false, false)
	var first skillsInstallEnvelope
	if err := json.Unmarshal([]byte(stdout), &first); err != nil {
		t.Fatalf("json: %v / %s", err, stdout)
	}
	if !first.OK || first.Data.InstalledCount != 2 || first.Data.SkippedCount != 0 || len(first.Data.Skipped) != 0 {
		t.Fatalf("first run: %+v", first)
	}
	if stderr != "" {
		t.Fatalf("first run must not warn: %s", stderr)
	}
	if _, err := os.Stat(filepath.Join(dstRoot, "yunxiao-alpha", "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	// Second run without --force: nothing installed, both skips reported.
	stdout, stderr = captureSkillsOutput(t, srcRoot, dstRoot, false, false)
	var second skillsInstallEnvelope
	if err := json.Unmarshal([]byte(stdout), &second); err != nil {
		t.Fatalf("json: %v / %s", err, stdout)
	}
	if !second.OK {
		t.Fatalf("rerun should still succeed: %s", stdout)
	}
	if second.Data.InstalledCount != 0 || len(second.Data.Installed) != 0 {
		t.Fatalf("rerun must install nothing: %+v", second)
	}
	if second.Data.SkippedCount != 2 || len(second.Data.Skipped) != 2 {
		t.Fatalf("rerun must report both skips: %+v", second)
	}
	skippedNames := map[string]bool{}
	for _, s := range second.Data.Skipped {
		name, _ := s["name"].(string)
		reason, _ := s["reason"].(string)
		skippedNames[name] = true
		if reason != "exists" {
			t.Fatalf("skip reason=%q, want exists (%v)", reason, s)
		}
	}
	if !skippedNames["yunxiao-alpha"] || !skippedNames["yunxiao-beta"] {
		t.Fatalf("skipped names: %v", skippedNames)
	}
	for _, want := range []string{"--force", "yunxiao-alpha", "yunxiao-beta"} {
		if !bytes.Contains([]byte(stderr), []byte(want)) {
			t.Fatalf("stderr warning missing %q: %s", want, stderr)
		}
	}
	if !bytes.Contains([]byte(second.Data.Hint), []byte("--force")) {
		t.Fatalf("data.hint should mention --force: %q", second.Data.Hint)
	}
	// Existing content untouched.
	if _, err := os.Stat(filepath.Join(dstRoot, "yunxiao-beta", "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	// Dry-run over the existing target must preview the same skips truthfully.
	// Dry-run envelopes carry the result under "request", not "data".
	stdout, stderr = captureSkillsOutput(t, srcRoot, dstRoot, false, true)
	var thirdRaw output.Envelope
	if err := json.Unmarshal([]byte(stdout), &thirdRaw); err != nil {
		t.Fatalf("json: %v / %s", err, stdout)
	}
	if !thirdRaw.OK || !thirdRaw.DryRun {
		t.Fatalf("dry-run rerun envelope: %s", stdout)
	}
	rawReq, _ := json.Marshal(thirdRaw.Request)
	var third skillsInstallResult
	if err := json.Unmarshal(rawReq, &third); err != nil {
		t.Fatalf("json: %v / %s", err, rawReq)
	}
	if third.SkippedCount != 2 || third.InstalledCount != 0 {
		t.Fatalf("dry-run rerun: %+v", third)
	}
	if !bytes.Contains([]byte(stderr), []byte("--force")) {
		t.Fatalf("dry-run rerun must warn about skips: %s", stderr)
	}
}

func TestDiscoverInstallableSkills(t *testing.T) {
	root := t.TempDir()
	mustMkSkill := func(name string, withMD bool) {
		d := filepath.Join(root, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if withMD {
			if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("# "+name+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	mustMkSkill("yunxiao-shared", true)
	mustMkSkill("yunxiao-codeup", true)
	mustMkSkill("yunxiao-empty", false)
	mustMkSkill("other-skill", true)
	if err := os.WriteFile(filepath.Join(root, "yunxiao-notadir"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	names, err := discoverInstallableSkills(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "yunxiao-codeup" || names[1] != "yunxiao-shared" {
		t.Fatalf("got %v", names)
	}

	names, err = discoverInstallableSkills(root, []string{"yunxiao-shared"})
	if err != nil || len(names) != 1 || names[0] != "yunxiao-shared" {
		t.Fatalf("filter: %v %v", names, err)
	}

	if _, err := discoverInstallableSkills(root, []string{"yunxiao-missing"}); err == nil {
		t.Fatal("expected missing skill error")
	}
}

func TestDefaultSkillsInstallDir(t *testing.T) {
	d := defaultSkillsInstallDir()
	if !filepath.IsAbs(d) && d != filepath.Join(".agents", "skills") {
		// When home is available, path should end with .agents/skills
	}
	if filepath.Base(d) != "skills" || filepath.Base(filepath.Dir(d)) != ".agents" {
		t.Fatalf("unexpected default dir %q", d)
	}
}

func TestInstallOneSkillCopyAndSkip(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	src := filepath.Join(srcRoot, "yunxiao-shared")
	if err := os.MkdirAll(filepath.Join(src, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "references", "a.md"), []byte("ref\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dstRoot, "yunxiao-shared")

	ins, sk, err := installOneSkill(src, dst, false, false, false)
	if err != nil || sk != nil || ins == nil || ins.Mode != "copy" {
		t.Fatalf("install: ins=%v sk=%v err=%v", ins, sk, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "references", "a.md")); err != nil {
		t.Fatal(err)
	}

	ins, sk, err = installOneSkill(src, dst, false, false, false)
	if err != nil || ins != nil || sk == nil {
		t.Fatalf("skip: ins=%v sk=%v err=%v", ins, sk, err)
	}

	ins, sk, err = installOneSkill(src, dst, false, true, true)
	if err != nil || sk != nil || ins == nil {
		t.Fatalf("dry-run force: ins=%v sk=%v err=%v", ins, sk, err)
	}
}
