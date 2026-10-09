package cmd

import (
	"strings"
	"testing"
)

// auth login --dry-run without --browser must fail before any OAuth network.
func TestAuthLoginDryRunRequiresBrowser(t *testing.T) {
	_ = authLoginCmd.Flags().Set("browser", "false")
	_ = authLoginCmd.Flags().Set("dry-run", "false")
	_ = authLoginCmd.Flags().Set("token", "")
	t.Cleanup(func() {
		_ = authLoginCmd.Flags().Set("browser", "false")
		_ = authLoginCmd.Flags().Set("dry-run", "false")
		_ = authLoginCmd.Flags().Set("token", "")
	})

	stdout, stderr, code := runRootForOAuthExpiry(t, "auth", "login", "--dry-run")
	if code == 0 {
		t.Fatalf("want non-zero exit, stdout=%s stderr=%s", stdout, stderr)
	}
	blob := stdout + stderr
	if !strings.Contains(blob, "--dry-run requires --browser") {
		t.Fatalf("want dry-run/browser gate message, got %s", blob)
	}
	if !strings.Contains(blob, "auth login --browser --dry-run") {
		t.Fatalf("want hint with --browser --dry-run, got %s", blob)
	}
}