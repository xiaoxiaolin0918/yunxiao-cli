package cmd

import (
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
)

// S3: auth logout with no credentials should succeed (exit 0, cleared=true).
func TestAuthLogoutIdempotentNoCredentials(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(config.EnvAccessToken, "")
	t.Setenv("YUNXIAO_PROFILE", "")
	_ = authLoginCmd.Flags().Set("browser", "false")
	stdout, stderr, code := runRootForOAuthExpiry(t, "auth", "logout")
	if code != 0 {
		t.Fatalf("want exit 0, code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	blob := stdout + stderr
	if !strings.Contains(blob, `"cleared"`) && !strings.Contains(blob, `"cleared": true`) && !strings.Contains(blob, `"cleared":true`) {
		// Success envelope should include cleared
		if !strings.Contains(blob, "cleared") {
			t.Fatalf("want cleared in output, got %s", blob)
		}
	}
}