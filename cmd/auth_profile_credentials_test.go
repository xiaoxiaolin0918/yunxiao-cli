package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

func TestAuthLoginStatusLogoutPerProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	os.Unsetenv(config.EnvAccessToken)
	os.Unsetenv(profile.EnvProfile)
	prevProfile := globalProfile
	globalProfile = ""
	t.Cleanup(func() { globalProfile = prevProfile })

	// Minimal profile JSON files (no embedded tokens).
	profDir := filepath.Join(dir, "yunxiao", "profiles")
	if err := os.MkdirAll(profDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "beta"} {
		body := []byte(`{"name":"` + name + `","organization_id":"org-` + name + `"}` + "\n")
		if err := os.WriteFile(filepath.Join(profDir, name+".json"), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	stdout, stderr, code := runRootForOAuthExpiry(t, "auth", "login", "--token", "pat-alpha-1111aaaa", "--profile", "alpha")
	if code != 0 {
		t.Fatalf("login alpha exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	env := decodeOKEnvelope(t, stdout)
	data, _ := env.Data.(map[string]any)
	if data["credential_profile"] != "alpha" {
		t.Fatalf("login alpha data=%v", data)
	}

	stdout, stderr, code = runRootForOAuthExpiry(t, "auth", "login", "--token", "pat-beta-2222bbbb", "--profile", "beta")
	if code != 0 {
		t.Fatalf("login beta exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	// Status with --profile alpha vs beta must show different masked tokens / credential_profile.
	stdout, _, code = runRootForOAuthExpiry(t, "auth", "status", "--profile", "alpha")
	if code != 0 {
		t.Fatalf("status alpha: %s", stdout)
	}
	env = decodeOKEnvelope(t, stdout)
	data, _ = env.Data.(map[string]any)
	if data["credential_profile"] != "alpha" || data["token_source"] != "credentials" {
		t.Fatalf("status alpha: %v", data)
	}
	maskedA, _ := data["token_masked"].(string)

	stdout, _, code = runRootForOAuthExpiry(t, "auth", "status", "--profile", "beta")
	if code != 0 {
		t.Fatalf("status beta: %s", stdout)
	}
	env = decodeOKEnvelope(t, stdout)
	data, _ = env.Data.(map[string]any)
	if data["credential_profile"] != "beta" {
		t.Fatalf("status beta: %v", data)
	}
	maskedB, _ := data["token_masked"].(string)
	if maskedA == "" || maskedA == maskedB {
		t.Fatalf("expected different masked tokens; a=%q b=%q", maskedA, maskedB)
	}

	// Logout --profile alpha clears only alpha.
	stdout, stderr, code = runRootForOAuthExpiry(t, "auth", "logout", "--profile", "alpha")
	if code != 0 {
		t.Fatalf("logout alpha exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	env = decodeOKEnvelope(t, stdout)
	data, _ = env.Data.(map[string]any)
	if data["credential_profile"] != "alpha" {
		t.Fatalf("logout data=%v", data)
	}

	cred, _, err := config.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if c := cred.Profiles["alpha"]; c != nil && c.AccessToken != "" {
		t.Fatalf("alpha still present: %+v", c)
	}
	if cred.Profiles["beta"] == nil || cred.Profiles["beta"].AccessToken != "pat-beta-2222bbbb" {
		t.Fatalf("beta should remain: %+v", cred.Profiles)
	}

	// After logout, alpha status should not use beta/global.
	stdout, _, code = runRootForOAuthExpiry(t, "auth", "status", "--profile", "alpha")
	if code != 0 {
		t.Fatalf("status alpha after logout: %s", stdout)
	}
	env = decodeOKEnvelope(t, stdout)
	data, _ = env.Data.(map[string]any)
	if data["has_token"] != false && data["token_source"] != "none" {
		// has_token false or source none
		if ht, _ := data["has_token"].(bool); ht {
			t.Fatalf("alpha should have no token after logout: %v", data)
		}
	}
}

func TestAuthLoginWithoutProfileWritesGlobalActive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	os.Unsetenv(config.EnvAccessToken)
	os.Unsetenv(profile.EnvProfile)
	prevProfile := globalProfile
	globalProfile = ""
	t.Cleanup(func() { globalProfile = prevProfile })

	stdout, stderr, code := runRootForOAuthExpiry(t, "auth", "login", "--token", "pat-global-zzzz9999")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	env := decodeOKEnvelope(t, stdout)
	data, _ := env.Data.(map[string]any)
	if cp, _ := data["credential_profile"].(string); cp != "" {
		t.Fatalf("expected empty credential_profile, got %v", data)
	}
	cred, _, err := config.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if cred.Active == nil || cred.Active.AccessToken != "pat-global-zzzz9999" {
		t.Fatalf("Active=%+v", cred.Active)
	}
	if len(cred.Profiles) != 0 {
		t.Fatalf("profiles should be empty: %v", cred.Profiles)
	}
}
