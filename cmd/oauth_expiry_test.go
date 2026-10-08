package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// TestOAuthExpiryFields locks the #122 near-expiry evaluation (<24h warn window,
// 24h boundary inclusive, expired / can-refresh behavior, zero time).
func TestOAuthExpiryFields(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		offset     time.Duration // expires_at offset from now; 0 means zero time
		zeroAt     bool
		canRefresh bool
		wantNil    bool
		expired    bool
		expiring   bool
		wantWarn   string // required substring of warning; "" means no warning key
	}{
		{name: "far-from-expiry", offset: 72 * time.Hour, wantWarn: ""},
		{name: "just-over-24h", offset: 25 * time.Hour, wantWarn: ""},
		{name: "boundary-24h-inclusive", offset: 24 * time.Hour, expiring: true, wantWarn: "将于"},
		{name: "near-no-refresh", offset: 2 * time.Hour, expiring: true, wantWarn: "auth login --browser"},
		{name: "near-can-refresh-hint", offset: 2 * time.Hour, canRefresh: true, expiring: true, wantWarn: "auth refresh"},
		{name: "expired-no-refresh", offset: -1 * time.Hour, expired: true, wantWarn: "已于"},
		{name: "expired-can-refresh-hint", offset: -1 * time.Hour, canRefresh: true, expired: true, wantWarn: "auth refresh"},
		{name: "zero-expiry-no-fields", zeroAt: true, wantNil: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			at := now.Add(tc.offset)
			if tc.zeroAt {
				at = time.Time{}
			}
			got := oauthExpiryFieldsAt(at, now, tc.canRefresh)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("want nil fields for zero expiry, got %#v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("nil fields for non-zero expiry")
			}
			if got["expired"] != tc.expired {
				t.Fatalf("expired=%v want %v (fields=%#v)", got["expired"], tc.expired, got)
			}
			if got["expiring"] != tc.expiring {
				t.Fatalf("expiring=%v want %v (fields=%#v)", got["expiring"], tc.expiring, got)
			}
			if got["auto_refresh"] != tc.canRefresh {
				t.Fatalf("auto_refresh=%v want %v", got["auto_refresh"], tc.canRefresh)
			}
			local, _ := got["expires_at_local"].(string)
			if local == "" || !strings.Contains(local, "2026-10-") {
				t.Fatalf("expires_at_local=%q", local)
			}
			warn, hasWarn := got["warning"].(string)
			if tc.wantWarn == "" {
				if hasWarn {
					t.Fatalf("unexpected warning: %q (fields=%#v)", warn, got)
				}
				return
			}
			if !hasWarn || !strings.Contains(warn, tc.wantWarn) {
				t.Fatalf("warning=%q want substring %q (fields=%#v)", warn, tc.wantWarn, got)
			}
			wantCmd := oauthRenewCommand
			if tc.canRefresh {
				wantCmd = oauthRefreshCommand
			}
			if h, _ := got["hint"].(string); !strings.Contains(h, wantCmd) {
				t.Fatalf("hint=%q want %q", h, wantCmd)
			}
		})
	}
}

// TestOAuthLoginHint locks the login-success hint (#122 suggestion 3):
// human-readable expiry + renew command, refresh-aware wording.
func TestOAuthLoginHint(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		offset     time.Duration
		zeroAt     bool
		canRefresh bool
		want       []string
		wantEmpty  bool
	}{
		{name: "no-refresh-token", offset: 24 * time.Hour, want: []string{"2026-10-08", "auth login --browser", "refresh_token"}},
		{name: "can-refresh", offset: 24 * time.Hour, canRefresh: true, want: []string{"自动刷新", "auth refresh", "auth login --browser"}},
		{name: "zero-expiry-empty", zeroAt: true, wantEmpty: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			at := now.Add(tc.offset)
			if tc.zeroAt {
				at = time.Time{}
			}
			got := oauthLoginHint(at, now, tc.canRefresh)
			if tc.wantEmpty {
				if got != "" {
					t.Fatalf("want empty hint, got %q", got)
				}
				return
			}
			for _, sub := range tc.want {
				if !strings.Contains(got, sub) {
					t.Fatalf("hint=%q missing %q", got, sub)
				}
			}
		})
	}
}

// writeOAuthCred stores an active credential in the isolated XDG config dir.
func writeOAuthCred(t *testing.T, cred config.Credential) {
	t.Helper()
	if _, err := config.SaveCredentials(config.CredentialsFile{Active: &cred}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
}

// isolateOAuthConfig points config/credentials at a temp dir and clears token env.
func isolateOAuthConfig(t *testing.T, apiBase string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(config.EnvAccessToken, "")
	t.Setenv(config.EnvAPIBaseURL, apiBase)
	t.Setenv(config.EnvOrganizationID, "org-oauth-expiry-test")
	t.Setenv("YUNXIAO_PROFILE", "")
}

// runRootForOAuthExpiry runs args through rootCmd capturing stdout/stderr/exit
// (pattern from cmd/codeup_mrs_comments_create_test.go).
func runRootForOAuthExpiry(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	// Cobra sticky bool flags: reset auth refresh --dry-run between Execute calls.
	_ = authRefreshCmd.Flags().Set("dry-run", "false")
	stdout := withCmdJSONCapture(t)
	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	prevExit := processExit
	code := 0
	processExit = func(c int) {
		code = c
		panic(exitPanic{code: c})
	}
	t.Cleanup(func() { processExit = prevExit })
	rootCmd.SetArgs(args)
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
	return stdout.String(), stderr.String(), code
}

func decodeOKEnvelope(t *testing.T, stdout string) output.Envelope {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout)
	}
	if !env.OK {
		t.Fatalf("envelope not ok: %+v / stderr", env)
	}
	return env
}

// TestAuthStatusOAuthExpiry: `auth status` surfaces near-expiry warning for oat-
// credentials that cannot silently refresh (#122), and stays quiet otherwise.
func TestAuthStatusOAuthExpiry(t *testing.T) {
	cases := []struct {
		name        string
		cred        config.Credential
		wantWarn    string
		noWarn      bool
		noExpiryKey bool
	}{
		{
			name:     "near-expiry-no-refresh",
			cred:     config.Credential{AccessToken: "oat-near", TokenKind: config.TokenKindOAuth, APIBase: "https://openapi-rdc.aliyuncs.com", ExpiresAt: time.Now().Add(2 * time.Hour)},
			wantWarn: "auth login --browser",
		},
		{
			name:     "expired-no-refresh",
			cred:     config.Credential{AccessToken: "oat-dead", TokenKind: config.TokenKindOAuth, APIBase: "https://openapi-rdc.aliyuncs.com", ExpiresAt: time.Now().Add(-1 * time.Hour)},
			wantWarn: "已于",
		},
		{
			name:     "near-expiry-can-refresh-hint",
			cred:     config.Credential{AccessToken: "oat-r", RefreshToken: "rt", ClientID: "cid", TokenKind: config.TokenKindOAuth, APIBase: "https://openapi-rdc.aliyuncs.com", ExpiresAt: time.Now().Add(2 * time.Hour)},
			wantWarn: "auth refresh",
		},
		{
			name:     "far-from-expiry",
			cred:     config.Credential{AccessToken: "oat-far", TokenKind: config.TokenKindOAuth, APIBase: "https://openapi-rdc.aliyuncs.com", ExpiresAt: time.Now().Add(72 * time.Hour)},
			noWarn:   true,
			wantWarn: "",
		},
		{
			name:        "pat-no-expiry-fields",
			cred:        config.Credential{AccessToken: "pat-token", TokenKind: config.TokenKindPAT, APIBase: "https://openapi-rdc.aliyuncs.com"},
			noExpiryKey: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateOAuthConfig(t, "")
			writeOAuthCred(t, tc.cred)
			stdout, stderr, code := runRootForOAuthExpiry(t, "auth", "status")
			if code != 0 {
				t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
			}
			env := decodeOKEnvelope(t, stdout)
			data, _ := env.Data.(map[string]any)
			if data == nil {
				t.Fatalf("data not a map: %#v", env.Data)
			}
			if data["token_kind"] != string(tc.cred.TokenKind) {
				t.Fatalf("token_kind=%v", data["token_kind"])
			}
			warn, hasWarn := data["warning"].(string)
			if tc.noExpiryKey {
				if _, ok := data["expiring"]; ok {
					t.Fatalf("pat must not carry expiry fields: %#v", data)
				}
				return
			}
			if _, ok := data["expires_at_local"].(string); !ok || data["expires_at_local"] == "" {
				t.Fatalf("expires_at_local missing: %#v", data)
			}
			if tc.noWarn {
				if hasWarn {
					t.Fatalf("unexpected warning: %q", warn)
				}
				return
			}
			if !hasWarn || !strings.Contains(warn, tc.wantWarn) {
				t.Fatalf("warning=%q want substring %q (data=%#v)", warn, tc.wantWarn, data)
			}
			wantCmd := oauthRenewCommand
			if strings.Contains(tc.wantWarn, "auth refresh") {
				wantCmd = oauthRefreshCommand
			}
			if h, _ := data["hint"].(string); !strings.Contains(h, wantCmd) {
				t.Fatalf("hint=%q want %q", h, wantCmd)
			}
		})
	}
}

// newDoctorUserServer answers GET /oapi/v1/platform/user for `doctor`.
func newDoctorUserServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/oapi/v1/platform/user") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"u-1","name":"oauth-expiry-tester","lastOrganization":"org-oauth-expiry-test"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func doctorTokenCheck(t *testing.T, env output.Envelope) map[string]any {
	t.Helper()
	checks, ok := env.Data.([]any)
	if !ok {
		t.Fatalf("doctor data not a list: %#v", env.Data)
	}
	for _, c := range checks {
		m, _ := c.(map[string]any)
		if m != nil && m["name"] == "token" {
			return m
		}
	}
	t.Fatalf("token check not found: %#v", env.Data)
	return nil
}

// TestDoctorOAuthExpiry: doctor's token check warns for near-expiry oat- without
// refresh_token (#122) while staying healthy (connectivity via httptest server).
func TestDoctorOAuthExpiry(t *testing.T) {
	srv := newDoctorUserServer(t)
	cases := []struct {
		name     string
		cred     config.Credential
		wantWarn string
		noWarn   bool
	}{
		{
			name:     "near-expiry-no-refresh",
			cred:     config.Credential{AccessToken: "oat-near", TokenKind: config.TokenKindOAuth, APIBase: srv.URL, ExpiresAt: time.Now().Add(90 * time.Minute)},
			wantWarn: "将于",
		},
		{
			name:     "expired-no-refresh",
			cred:     config.Credential{AccessToken: "oat-dead", TokenKind: config.TokenKindOAuth, APIBase: srv.URL, ExpiresAt: time.Now().Add(-30 * time.Minute)},
			wantWarn: "已于",
		},
		{
			name:   "far-from-expiry-quiet",
			cred:   config.Credential{AccessToken: "oat-far", TokenKind: config.TokenKindOAuth, APIBase: srv.URL, ExpiresAt: time.Now().Add(48 * time.Hour)},
			noWarn: true,
		},
		{
			name:   "pat-quiet",
			cred:   config.Credential{AccessToken: "pat-token", TokenKind: config.TokenKindPAT, APIBase: srv.URL},
			noWarn: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateOAuthConfig(t, "")
			writeOAuthCred(t, tc.cred)
			stdout, stderr, code := runRootForOAuthExpiry(t, "doctor")
			if code != 0 {
				t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
			}
			env := decodeOKEnvelope(t, stdout)
			if healthy, _ := env.Meta["healthy"].(bool); !healthy {
				t.Fatalf("doctor must stay healthy on a valid token: meta=%#v stdout=%s", env.Meta, stdout)
			}
			token := doctorTokenCheck(t, env)
			if token["ok"] != true {
				t.Fatalf("token check not ok: %#v", token)
			}
			warn, hasWarn := token["warning"].(string)
			if tc.noWarn {
				if hasWarn {
					t.Fatalf("unexpected warning: %q (token=%#v)", warn, token)
				}
				return
			}
			if !hasWarn || !strings.Contains(warn, tc.wantWarn) {
				t.Fatalf("warning=%q want substring %q (token=%#v)", warn, tc.wantWarn, token)
			}
			if !strings.Contains(warn, oauthRenewCommand) {
				t.Fatalf("warning must carry renew command: %q", warn)
			}
			if tc.cred.TokenKind == config.TokenKindOAuth {
				if _, ok := token["expires_at_local"].(string); !ok {
					t.Fatalf("expires_at_local missing: %#v", token)
				}
			}
		})
	}
}

// TestAuthLoginHelpDocumentsExpiry keeps the #122 behavior discoverable in help.
func TestAuthLoginHelpDocumentsExpiry(t *testing.T) {
	if !strings.Contains(authLoginCmd.Long, "expires_at_local") {
		t.Fatalf("auth login Long must document expiry output: %s", authLoginCmd.Long)
	}
	if !strings.Contains(authStatusCmd.Long, "24h") {
		t.Fatalf("auth status Long must document the 24h warning: %s", authStatusCmd.Long)
	}
	if !strings.Contains(doctorCmd.Long, "#122") {
		t.Fatalf("doctor Long must document the expiry warning: %s", doctorCmd.Long)
	}
}
