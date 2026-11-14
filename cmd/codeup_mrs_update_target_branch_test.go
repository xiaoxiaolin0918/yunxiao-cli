package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/output"
)

func TestMrsUpdateTargetBranchFlagRegistered(t *testing.T) {
	if codeupMrsUpdateCmd.Flags().Lookup("target-branch") == nil {
		t.Fatal("--target-branch flag not registered on mrs update")
	}
}

func TestMrsUpdateTargetBranchUnsupportedNoPUT(t *testing.T) {
	s := newMrsWipServer(t, "feat: x")
	cases := []struct {
		name   string
		dryRun bool
		args   []string
	}{
		{name: "live", args: []string{"--target-branch", "master"}},
		{name: "dry-run", dryRun: true, args: []string{"--target-branch", "master", "--dry-run"}},
		{name: "combined with title must not partial-update", args: []string{"--target-branch", "master", "--title", "new title"}},
		{name: "combined with unwip", args: []string{"--target-branch", "master", "--unwip"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s.mu.Lock()
			s.puts = nil
			s.mrGETs = 0
			s.mu.Unlock()
			stdout, stderr, code := runMrsUpdateWip(t, tc.dryRun, tc.args...)
			if code != 1 {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			eb := decodeErrorBody(t, stderr)
			if eb.Type != "cli" || eb.Subtype != "target_branch_unsupported" {
				t.Fatalf("error body=%+v stderr=%s", eb, stderr)
			}
			if !strings.Contains(eb.Message, "UpdateChangeRequest") || !strings.Contains(strings.ToLower(eb.Message), "targetbranch") {
				t.Fatalf("message should name the OpenAPI limit: %q", eb.Message)
			}
			if !strings.Contains(eb.Hint, "网页") && !strings.Contains(strings.ToLower(eb.Hint), "web") {
				t.Fatalf("hint should point at the Codeup web UI: %q", eb.Hint)
			}
			if eb.Details["requested_target_branch"] != "master" {
				t.Fatalf("details=%#v", eb.Details)
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			if len(s.puts) != 0 {
				t.Fatalf("must not invent a targetBranch PUT: %#v", s.puts)
			}
		})
	}
}

func TestMrsUpdateTargetBranchCountsAsChangeFlag(t *testing.T) {
	s := newMrsWipServer(t, "feat: x")
	_, stderr, code := runMrsUpdateWip(t, true, "--target-branch", "master", "--dry-run")
	if code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	if strings.Contains(stderr, "provide --title") {
		t.Fatalf("standalone --target-branch must not be rejected as nothing-to-do: %s", stderr)
	}
	eb := decodeErrorBody(t, stderr)
	if eb.Subtype != "target_branch_unsupported" {
		t.Fatalf("subtype=%q stderr=%s", eb.Subtype, stderr)
	}
	if len(s.puts) != 0 {
		t.Fatalf("puts=%#v", s.puts)
	}
}

func TestMrsUpdateNothingToDoMentionsTargetBranch(t *testing.T) {
	_ = newMrsWipServer(t, "feat: x")
	_, stderr, code := runMrsUpdateWip(t, true)
	if code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "--target-branch") {
		t.Fatalf("nothing-to-do must mention --target-branch: %s", stderr)
	}
}

func TestMrsUpdateTargetBranchHelp(t *testing.T) {
	long := codeupMrsUpdateCmd.Long
	for _, want := range []string{"--target-branch", "UpdateChangeRequest", "title/description", "网页"} {
		if !strings.Contains(long, want) {
			t.Fatalf("mrs update help missing %q:\n%s", want, long)
		}
	}
}

func TestMrsUpdateTargetBranchJSONErrorShape(t *testing.T) {
	_ = newMrsWipServer(t, "feat: x")
	_, stderr, code := runMrsUpdateWip(t, false, "--target-branch", "develop")
	if code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(stderr), &env); err != nil {
		t.Fatalf("stderr JSON: %v / %s", err, stderr)
	}
	if env.OK || env.Error == nil {
		t.Fatalf("env=%+v", env)
	}
	if env.Error.Details["local_id"] != "125" || env.Error.Details["requested_target_branch"] != "develop" {
		t.Fatalf("details=%#v", env.Error.Details)
	}
}
