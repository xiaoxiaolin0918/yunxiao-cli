package profile

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfig writes <xdg>/yunxiao/config.json (the config default source, #130).
func writeConfig(t *testing.T, xdg, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(xdg, "yunxiao"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "yunxiao", "config.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestResolveNamePrecedence (#130): --profile explicit > YUNXIAO_PROFILE env >
// config.json "profile" default > empty. The config default must not override
// an explicit flag or env, and must be ignored when unset/unreadable.
func TestResolveNamePrecedence(t *testing.T) {
	cases := []struct {
		name     string
		explicit string
		env      string
		config   string // config.json content; "" = none
		want     string
	}{
		{name: "explicit wins over env and config", explicit: "flag-p", env: "env-p", config: `{"profile":"cfg-p"}`, want: "flag-p"},
		{name: "env wins over config default", explicit: "", env: "env-p", config: `{"profile":"cfg-p"}`, want: "env-p"},
		{name: "config default used when flag/env unset", explicit: "", env: "", config: `{"profile":"cfg-p"}`, want: "cfg-p"},
		{name: "no default anywhere", explicit: "", env: "", config: "", want: ""},
		{name: "config without profile key", explicit: "", env: "", config: `{"edition":"central"}`, want: ""},
		{name: "blank config default ignored", explicit: "", env: "", config: `{"profile":"  "}`, want: ""},
		{name: "explicit beats broken config", explicit: "flag-p", env: "", config: `{"profile":`, want: "flag-p"},
		{name: "env beats broken config", explicit: "", env: "env-p", config: `not json`, want: "env-p"},
		{name: "broken config means no default", explicit: "", env: "", config: `not json`, want: ""},
		{name: "whitespace trimmed", explicit: "  flag-p  ", env: "", config: "", want: "flag-p"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv(EnvProfile, tc.env)
			if tc.config != "" {
				xdg, _ := os.LookupEnv("XDG_CONFIG_HOME")
				writeConfig(t, xdg, tc.config)
			}
			if got := ResolveName(tc.explicit); got != tc.want {
				t.Fatalf("ResolveName(%q) = %q, want %q", tc.explicit, got, tc.want)
			}
		})
	}
}

// TestDefaultName: the config default reader is trimmed, empty-safe and
// returns "" when the config file does not exist.
func TestDefaultName(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got := DefaultName(); got != "" {
		t.Fatalf("DefaultName without config.json = %q, want empty", got)
	}
	xdg, _ := os.LookupEnv("XDG_CONFIG_HOME")
	writeConfig(t, xdg, `{"profile":"zhiyi","edition":"central"}`)
	if got := DefaultName(); got != "zhiyi" {
		t.Fatalf("DefaultName = %q, want zhiyi", got)
	}
}
