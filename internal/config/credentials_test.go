package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCredentialsSaveLoadMode0600(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	os.Unsetenv(EnvAccessToken)

	p, err := SetActiveOAuth(Credential{
		AccessToken:  "oat-test-token-xxxx",
		RefreshToken: "ort-test-refresh",
		ClientID:     "cid1",
		APIBase:      "https://openapi-rdc.aliyuncs.com",
		AuthHeader:   "x-yunxiao-token",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		// Windows has no POSIX permission bits: os.Chmod only toggles the
		// read-only attribute, so a 0600 (owner-writable) file is reported
		// as 0666. Assert the owner-write bit instead — the Windows
		// observable of mode 0600 (a read-only file would clear 0200).
		if fi.Mode().Perm()&0o200 == 0 {
			t.Fatalf("mode=%o (expected owner-writable)", fi.Mode().Perm())
		}
	} else if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", fi.Mode().Perm())
	}
	f, _, err := LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if f.Active == nil || f.Active.TokenKind != TokenKindOAuth {
		t.Fatalf("%+v", f.Active)
	}
	if f.Clients["https://openapi-rdc.aliyuncs.com"] != "cid1" {
		t.Fatalf("clients=%v", f.Clients)
	}
}

func TestClientIDCachedPerAPIBase(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := SetCachedClientID("https://a.example", "cid-a"); err != nil {
		t.Fatal(err)
	}
	if err := SetCachedClientID("https://b.example", "cid-b"); err != nil {
		t.Fatal(err)
	}
	a, _ := CachedClientID("https://a.example")
	b, _ := CachedClientID("https://b.example")
	if a != "cid-a" || b != "cid-b" {
		t.Fatalf("a=%s b=%s", a, b)
	}
	_ = ClearClientIDForBase("https://a.example")
	a, _ = CachedClientID("https://a.example")
	b, _ = CachedClientID("https://b.example")
	if a != "" || b != "cid-b" {
		t.Fatalf("after clear a=%s b=%s", a, b)
	}
}

func TestResolveCredentialsBeatsProfileAndConfig(t *testing.T) {
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
	if _, err := SetActivePAT("cred-pat-token-zz", "https://openapi-rdc.aliyuncs.com"); err != nil {
		t.Fatal(err)
	}
	r, err := ResolveWithProfileToken("profile-token-yy")
	if err != nil {
		t.Fatal(err)
	}
	if r.TokenSource != "credentials" || r.AccessToken != "cred-pat-token-zz" || r.TokenKind != TokenKindPAT {
		t.Fatalf("%+v", r)
	}
	// env still wins
	t.Setenv(EnvAccessToken, "env-wins")
	r, err = ResolveWithProfileToken("profile-token-yy")
	if err != nil {
		t.Fatal(err)
	}
	if r.TokenSource != "env" {
		t.Fatalf("%+v", r)
	}
}

func TestOAuthNeedsRefresh(t *testing.T) {
	c := &Credential{TokenKind: TokenKindOAuth, RefreshToken: "ort-x", ExpiresAt: time.Now().Add(time.Minute)}
	if !OAuthNeedsRefresh(c, 5*time.Minute) {
		t.Fatal("expected needs refresh")
	}
	c.ExpiresAt = time.Now().Add(time.Hour)
	if OAuthNeedsRefresh(c, 5*time.Minute) {
		t.Fatal("expected fresh")
	}
}

func TestClearOAuthActive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	_, err := SetActiveOAuth(Credential{AccessToken: "oat-x", RefreshToken: "ort-x", ClientID: "c", APIBase: "https://openapi-rdc.aliyuncs.com"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClearOAuthActive(false); err != nil {
		t.Fatal(err)
	}
	f, _, _ := LoadCredentials()
	if f.Active != nil {
		t.Fatal("expected cleared")
	}
	if f.Clients["https://openapi-rdc.aliyuncs.com"] != "c" {
		t.Fatalf("clients should remain: %v", f.Clients)
	}
}
