package cmd

import (
	"strings"
	"testing"
)

// Baseline checklist (cli-manager docs/baseline-checklist.md §3.5/§5):
// yunxiao has no OS keychain / login.cred password store and no separate
// logout --forget; bare auth logout must document that it dual-clears
// access+refresh (forget equivalent).
func TestAuthLogoutLongDocumentsForgetEquivalent(t *testing.T) {
	long := authLogoutCmd.Long
	for _, want := range []string{
		"forget",
		"refresh_token",
		"access_token",
		"--forget",
	} {
		if !strings.Contains(long, want) {
			t.Fatalf("auth logout Long missing %q; Long=%s", want, long)
		}
	}
	if !strings.Contains(strings.ToLower(long), "keychain") && !strings.Contains(long, "login.cred") {
		t.Fatalf("auth logout Long should mention no keychain/login.cred; Long=%s", long)
	}
}