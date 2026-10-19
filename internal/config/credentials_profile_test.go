package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPerProfileCredentialsIsolation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	os.Unsetenv(EnvAccessToken)

	if _, err := SetProfilePAT("alpha", "pat-alpha-aaaa", "https://openapi-rdc.aliyuncs.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := SetProfilePAT("beta", "pat-beta-bbbb", "https://openapi-rdc.aliyuncs.com"); err != nil {
		t.Fatal(err)
	}
	// Global Active leftover must NOT leak into a named profile (#380).
	if _, err := SetActivePAT("pat-global-gggg", "https://openapi-rdc.aliyuncs.com"); err != nil {
		t.Fatal(err)
	}

	ra, err := ResolveForProfile("alpha", "")
	if err != nil {
		t.Fatal(err)
	}
	if ra.TokenSource != "credentials" || ra.AccessToken != "pat-alpha-aaaa" || ra.CredentialProfile != "alpha" {
		t.Fatalf("alpha: %+v", ra)
	}

	rb, err := ResolveForProfile("beta", "")
	if err != nil {
		t.Fatal(err)
	}
	if rb.TokenSource != "credentials" || rb.AccessToken != "pat-beta-bbbb" || rb.CredentialProfile != "beta" {
		t.Fatalf("beta: %+v", rb)
	}

	// No profile name → legacy global Active.
	rg, err := ResolveForProfile("", "")
	if err != nil {
		t.Fatal(err)
	}
	if rg.TokenSource != "credentials" || rg.AccessToken != "pat-global-gggg" || rg.CredentialProfile != "" {
		t.Fatalf("global: %+v", rg)
	}
}

func TestPerProfileSlotBeatsProfileJSONAndConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	os.Unsetenv(EnvAccessToken)

	cfgDir := filepath.Join(dir, "yunxiao")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"access_token":"cfg-token-xxxx"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SetProfilePAT("zhiyi", "cred-slot-zzzz", "https://openapi-rdc.aliyuncs.com"); err != nil {
		t.Fatal(err)
	}

	r, err := ResolveForProfile("zhiyi", "profile-json-yyyy")
	if err != nil {
		t.Fatal(err)
	}
	if r.TokenSource != "credentials" || r.AccessToken != "cred-slot-zzzz" || r.CredentialProfile != "zhiyi" {
		t.Fatalf("%+v", r)
	}
}

func TestNamedProfileSkipsGlobalActiveFallsToProfileJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	os.Unsetenv(EnvAccessToken)

	if _, err := SetActivePAT("pat-global-only", "https://openapi-rdc.aliyuncs.com"); err != nil {
		t.Fatal(err)
	}
	// Named profile with empty slot must NOT use global Active.
	r, err := ResolveForProfile("play", "profile-json-pat")
	if err != nil {
		t.Fatal(err)
	}
	if r.TokenSource != "profile" || r.AccessToken != "profile-json-pat" || r.CredentialProfile != "" {
		t.Fatalf("expected profile JSON, got %+v", r)
	}
}

func TestEnvStillBeatsProfileCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if _, err := SetProfilePAT("zhiyi", "slot-token", "https://openapi-rdc.aliyuncs.com"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvAccessToken, "env-force-token")
	r, err := ResolveForProfile("zhiyi", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.TokenSource != "env" || r.AccessToken != "env-force-token" {
		t.Fatalf("%+v", r)
	}
}

func TestClearProfileOnlyThatSlot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	os.Unsetenv(EnvAccessToken)

	if _, err := SetProfilePAT("alpha", "pat-a", "https://openapi-rdc.aliyuncs.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := SetProfilePAT("beta", "pat-b", "https://openapi-rdc.aliyuncs.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := ClearProfile("alpha"); err != nil {
		t.Fatal(err)
	}
	f, _, err := LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if c := f.Profiles["alpha"]; c != nil && c.AccessToken != "" {
		t.Fatalf("alpha should be cleared: %+v", c)
	}
	if f.Profiles["beta"] == nil || f.Profiles["beta"].AccessToken != "pat-b" {
		t.Fatalf("beta should remain: %+v", f.Profiles)
	}
}

func TestSetProfileOAuthAndLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	_, err := SetProfileOAuth("zhiyi", Credential{
		AccessToken:  "oat-profile-xxxx",
		RefreshToken: "ort-profile",
		ClientID:     "cid-p",
		APIBase:      "https://openapi-rdc.aliyuncs.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	f, _, err := LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	c := f.Profiles["zhiyi"]
	if c == nil || c.TokenKind != TokenKindOAuth || c.AccessToken != "oat-profile-xxxx" {
		t.Fatalf("%+v", c)
	}
	if f.Clients["https://openapi-rdc.aliyuncs.com"] != "cid-p" {
		t.Fatalf("clients=%v", f.Clients)
	}
	// Global Active untouched.
	if f.Active != nil {
		t.Fatalf("Active should be nil: %+v", f.Active)
	}
}
