package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// setupProfileUseHome isolates XDG_CONFIG_HOME with an installed "zhiyi" profile
// and (optionally) a pre-existing config.json; returns the config path.
func setupProfileUseHome(t *testing.T, existingConfig string) string {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("YUNXIAO_PROFILE", "")
	profilesDir := filepath.Join(xdg, "yunxiao", "profiles")
	if err := os.MkdirAll(profilesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profilesDir, "zhiyi.json"), []byte(`{"name":"zhiyi","organization_id":"org-1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(xdg, "yunxiao", "config.json")
	if existingConfig != "" {
		if err := os.WriteFile(cfgPath, []byte(existingConfig), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return cfgPath
}

// runProfileUse executes `profile use ...` and returns stdout JSON, exit code.
// Globals (dry-run/yes/profile/org) are set explicitly per call and restored,
// since withCmdJSONCapture forces dry-run mode.
func runProfileUse(t *testing.T, args ...string) (string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	prevOut, prevErr, prevJQ, prevFmt := output.Stdout, output.Stderr, output.JQ, output.Format
	output.Stdout = &stdout
	output.Stderr = &stderr
	output.JQ = ""
	output.Format = "json"
	prevGlobals := []any{globalYes, globalDryRun, globalProfile, globalOrg}
	globalYes = false
	globalDryRun = strings.Contains(fmt.Sprint(args), "--dry-run")
	globalProfile = ""
	globalOrg = ""
	t.Cleanup(func() {
		output.Stdout = prevOut
		output.Stderr = prevErr
		output.JQ = prevJQ
		output.Format = prevFmt
		globalYes, globalDryRun, globalProfile, globalOrg = prevGlobals[0].(bool), prevGlobals[1].(bool), prevGlobals[2].(string), prevGlobals[3].(string)
	})
	prevExit := processExit
	code := 0
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })
	resetStringFlags(t, profileUseCmd, "unset")
	full := append([]string{"profile", "use"}, args...)
	rootCmd.SetArgs(full)
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
	if stdout.Len() == 0 && stderr.Len() > 0 {
		stdout = stderr // error envelopes live on stderr
	}
	return stdout.String(), code
}

func readConfigProfile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var f config.File
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("decode config %s: %v", string(b), err)
	}
	return f.Profile
}

// TestProfileUse (#130): default-profile command behavior, table-driven.
func TestProfileUse(t *testing.T) {
	cases := []struct {
		name       string
		existing   string   // pre-existing config.json ("" = none)
		args       []string // args after `profile use`
		wantOK     bool     // expect success envelope
		wantDry    bool     // expect dry_run envelope
		wantConfig string   // expected config.json "profile" after run
		wantSub    string   // expected error.subtype/keyword when !wantOK
		wantCode   int      // expected exit code
		check      func(t *testing.T, env output.Envelope, cfgPath string)
	}{
		{
			name:       "set default writes config and keeps other keys",
			existing:   `{"edition":"central","organization_id":"org-9"}`,
			args:       []string{"zhiyi"},
			wantOK:     true,
			wantConfig: "zhiyi",
			check: func(t *testing.T, env output.Envelope, cfgPath string) {
				data, _ := env.Data.(map[string]any)
				if data["profile"] != "zhiyi" {
					t.Fatalf("data.profile missing: %#v", env.Data)
				}
				b, _ := os.ReadFile(cfgPath)
				if !strings.Contains(string(b), `"organization_id": "org-9"`) {
					t.Fatalf("config keys clobbered: %s", string(b))
				}
			},
		},
		{
			name:     "dry-run does not write",
			existing: `{"edition":"central"}`,
			args:     []string{"--dry-run", "zhiyi"},
			wantOK:   true,
			wantDry:  true,
			check: func(t *testing.T, env output.Envelope, cfgPath string) {
				if readConfigProfile(t, cfgPath) != "" {
					t.Fatal("dry-run must not write config.json")
				}
				raw, _ := json.Marshal(env.Request)
				var req map[string]any
				_ = json.Unmarshal(raw, &req)
				if req["name"] != "zhiyi" {
					t.Fatalf("dry-run preview: %#v", req)
				}
			},
		},
		{
			name:     "missing profile errors without writing",
			existing: "",
			args:     []string{"nope"},
			wantOK:   false,
			wantSub:  "not usable",
			wantCode: 1,
			check: func(t *testing.T, env output.Envelope, cfgPath string) {
				if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
					t.Fatal("failed use must not create config.json")
				}
			},
		},
		{
			name:       "unset clears an existing default",
			existing:   `{"profile":"zhiyi"}`,
			args:       []string{"--unset"},
			wantOK:     true,
			wantConfig: "",
			check: func(t *testing.T, env output.Envelope, cfgPath string) {
				data, _ := env.Data.(map[string]any)
				if data["unset"] != true {
					t.Fatalf("data.unset missing: %#v", env.Data)
				}
			},
		},
		{
			name:     "no name and no --unset is a usage error",
			args:     nil,
			wantOK:   false,
			wantSub:  "pass a profile name",
			wantCode: 1,
		},
		{
			name:     "path-separator names rejected",
			args:     []string{"../evil"},
			wantOK:   false,
			wantSub:  "invalid profile name",
			wantCode: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfgPath := setupProfileUseHome(t, tc.existing)
			stdout, code := runProfileUse(t, tc.args...)
			if code != tc.wantCode {
				t.Fatalf("exit=%d want %d stdout=%s", code, tc.wantCode, stdout)
			}
			var env output.Envelope
			if err := json.Unmarshal([]byte(stdout), &env); err != nil || env.OK != tc.wantOK {
				t.Fatalf("stdout=%s err=%v", stdout, err)
			}
			if tc.wantOK {
				if env.DryRun != tc.wantDry {
					t.Fatalf("dry_run=%v want %v", env.DryRun, tc.wantDry)
				}
			} else if env.Error == nil || !strings.Contains(env.Error.Message, tc.wantSub) {
				t.Fatalf("error=%#v want %q in message", env.Error, tc.wantSub)
			}
			if tc.existing != "" || tc.wantConfig != "" || tc.wantOK {
				if readConfigProfile(t, cfgPath) != tc.wantConfig {
					t.Fatalf("config profile=%q want %q", readConfigProfile(t, cfgPath), tc.wantConfig)
				}
			}
			if tc.check != nil {
				tc.check(t, env, cfgPath)
			}
		})
	}
}

// TestProfileUseFeedsResolveName: after `profile use zhiyi`, activeProfileName()
// picks it up with no env/flag (the actual #130 pain point).
func TestProfileUseFeedsResolveName(t *testing.T) {
	setupProfileUseHome(t, "")
	if _, code := runProfileUse(t, "zhiyi"); code != 0 {
		t.Fatalf("profile use exit=%d", code)
	}
	prev := globalProfile
	globalProfile = ""
	t.Cleanup(func() { globalProfile = prev })
	if got := activeProfileName(); got != "zhiyi" {
		t.Fatalf("activeProfileName()=%q want zhiyi", got)
	}
}
