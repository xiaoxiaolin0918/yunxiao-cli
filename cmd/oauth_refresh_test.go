package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// newOAuthRefreshStack serves discovery + token + /platform/user for P0/P1 tests.
// tokenOK controls whether POST token succeeds; userOK controls whoami probe.
func newOAuthRefreshStack(t *testing.T, tokenOK, userOK bool) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/.well-known/oauth-authorization-server"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{
				"issuer":"`+srv.URL+`",
				"authorization_endpoint":"`+srv.URL+`/oauth/authorize",
				"token_endpoint":"`+srv.URL+`/oauth/token",
				"registration_endpoint":"`+srv.URL+`/oauth/register"
			}`)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/oauth/token"):
			if !tokenOK {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
				return
			}
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "refresh_token" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":"unsupported_grant_type"}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"oat-refreshed","refresh_token":"ort-rotated","token_type":"Bearer","expires_in":86400}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/oapi/v1/platform/user"):
			tok := r.Header.Get("x-yunxiao-token")
			if tok == "" {
				if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
					tok = strings.TrimPrefix(a, "Bearer ")
				}
			}
			if !userOK || tok == "" || tok == "oat-dead" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"errorCode":"ExpiredToken","errorMsg":"token expired"}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"u-refresh","name":"refresh-tester","lastOrganization":"org-refresh-test"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDoctorOAuthAutoRefresh(t *testing.T) {
	cases := []struct {
		name           string
		cred           config.Credential
		tokenOK        bool
		wantRefresh    string
		wantHealthy    bool
		wantConnOK     bool
		forbidBrowser  bool // refreshed path must not push login --browser as required hint
		wantHintSub    string
	}{
		{
			name: "expired-can-refresh-refreshed",
			cred: config.Credential{
				AccessToken: "oat-dead", RefreshToken: "ort-x", ClientID: "cid",
				TokenKind: config.TokenKindOAuth, ExpiresAt: time.Now().Add(-30 * time.Minute),
			},
			tokenOK: true, wantRefresh: authRefreshRefreshed, wantHealthy: true, wantConnOK: true, forbidBrowser: true,
		},
		{
			name: "expired-no-refresh-skipped",
			cred: config.Credential{
				AccessToken: "oat-dead", TokenKind: config.TokenKindOAuth,
				ExpiresAt: time.Now().Add(-30 * time.Minute),
			},
			tokenOK: true, wantRefresh: authRefreshSkipped, wantHealthy: false, wantConnOK: false,
			wantHintSub: oauthRenewCommand,
		},
		{
			name: "expired-refresh-failed",
			cred: config.Credential{
				AccessToken: "oat-dead", RefreshToken: "ort-bad", ClientID: "cid",
				TokenKind: config.TokenKindOAuth, ExpiresAt: time.Now().Add(-10 * time.Minute),
			},
			tokenOK: false, wantRefresh: authRefreshFailed, wantHealthy: false, wantConnOK: false,
			wantHintSub: oauthRenewCommand,
		},
		{
			name: "fresh-not-needed",
			cred: config.Credential{
				AccessToken: "oat-fresh", RefreshToken: "ort-x", ClientID: "cid",
				TokenKind: config.TokenKindOAuth, ExpiresAt: time.Now().Add(12 * time.Hour),
			},
			tokenOK: true, wantRefresh: authRefreshNotNeeded, wantHealthy: true, wantConnOK: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newOAuthRefreshStack(t, tc.tokenOK, true)
			isolateOAuthConfig(t, srv.URL)
			tc.cred.APIBase = srv.URL
			writeOAuthCred(t, tc.cred)
			stdout, stderr, code := runRootForOAuthExpiry(t, "doctor")
			if tc.wantHealthy && code != 0 {
				t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
			}
			if !tc.wantHealthy && code == 0 {
				t.Fatalf("want non-zero exit, stdout=%s", stdout)
			}
			env := decodeOKEnvelope(t, stdout)
			token := doctorTokenCheck(t, env)
			if got, _ := token["auth_refresh"].(string); got != tc.wantRefresh {
				t.Fatalf("auth_refresh=%q want %q (token=%#v)", got, tc.wantRefresh, token)
			}
			conn := doctorNamedCheck(t, env, "connectivity")
			if ok, _ := conn["ok"].(bool); ok != tc.wantConnOK {
				t.Fatalf("connectivity.ok=%v want %v (conn=%#v)", ok, tc.wantConnOK, conn)
			}
			if tc.forbidBrowser {
				if w, _ := token["warning"].(string); strings.Contains(w, oauthRenewCommand) && !strings.Contains(w, oauthRefreshCommand) {
					t.Fatalf("refreshed path must not force browser re-login: warning=%q", w)
				}
				if h, _ := token["hint"].(string); h == oauthRenewCommand {
					t.Fatalf("refreshed path must not hint browser-only: hint=%q", h)
				}
			}
			if tc.wantHintSub != "" {
				blob := ""
				if w, _ := token["warning"].(string); w != "" {
					blob += w
				}
				if h, _ := token["hint"].(string); h != "" {
					blob += h
				}
				if e, _ := conn["error"].(string); e != "" {
					blob += e
				}
				if !strings.Contains(blob, tc.wantHintSub) {
					t.Fatalf("expected hint substring %q in token/conn, got token=%#v conn=%#v", tc.wantHintSub, token, conn)
				}
			}
		})
	}
}

func doctorNamedCheck(t *testing.T, env output.Envelope, name string) map[string]any {
	t.Helper()
	checks, ok := env.Data.([]any)
	if !ok {
		t.Fatalf("doctor data not a list: %#v", env.Data)
	}
	for _, c := range checks {
		m, _ := c.(map[string]any)
		if m != nil && m["name"] == name {
			return m
		}
	}
	t.Fatalf("%s check not found: %#v", name, env.Data)
	return nil
}

func TestWhoamiOAuthAutoRefresh(t *testing.T) {
	srv := newOAuthRefreshStack(t, true, true)
	isolateOAuthConfig(t, srv.URL)
	writeOAuthCred(t, config.Credential{
		AccessToken: "oat-dead", RefreshToken: "ort-x", ClientID: "cid",
		TokenKind: config.TokenKindOAuth, APIBase: srv.URL,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})
	stdout, stderr, code := runRootForOAuthExpiry(t, "whoami")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	env := decodeOKEnvelope(t, stdout)
	data, _ := env.Data.(map[string]any)
	if data["auth_refresh"] != authRefreshRefreshed {
		t.Fatalf("auth_refresh=%v data=%#v", data["auth_refresh"], data)
	}
	user, _ := data["user"].(map[string]any)
	if user == nil || user["name"] != "refresh-tester" {
		t.Fatalf("user=%#v", data["user"])
	}
	if _, err := data["user_error"]; err {
		// user_error must be absent on success
		t.Fatalf("unexpected user_error: %#v", data)
	}
}

func TestAuthRefreshDryRunNoNetwork(t *testing.T) {
	// No httptest server: dry-run must not dial.
	isolateOAuthConfig(t, "http://127.0.0.1:1")
	writeOAuthCred(t, config.Credential{
		AccessToken: "oat-x", RefreshToken: "ort-x", ClientID: "cid",
		TokenKind: config.TokenKindOAuth, APIBase: "http://127.0.0.1:1",
		ExpiresAt: time.Now().Add(-5 * time.Minute),
	})
	stdout, stderr, code := runRootForOAuthExpiry(t, "auth", "refresh", "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	env := decodeOKEnvelope(t, stdout)
	data, _ := env.Data.(map[string]any)
	if data["dry_run"] != true {
		t.Fatalf("dry_run=%v", data["dry_run"])
	}
	if data["would_refresh"] != true {
		t.Fatalf("would_refresh=%v data=%#v", data["would_refresh"], data)
	}
	if data["auth_refresh"] != "would_refresh" {
		t.Fatalf("auth_refresh=%v", data["auth_refresh"])
	}
	// Credentials must remain.
	cred, _, err := config.LoadCredentials()
	if err != nil || cred.Active == nil || cred.Active.AccessToken != "oat-x" {
		t.Fatalf("dry-run mutated credentials: %#v err=%v", cred, err)
	}
}

func TestAuthRefreshForceSuccess(t *testing.T) {
	srv := newOAuthRefreshStack(t, true, true)
	isolateOAuthConfig(t, srv.URL)
	writeOAuthCred(t, config.Credential{
		AccessToken: "oat-old", RefreshToken: "ort-x", ClientID: "cid",
		TokenKind: config.TokenKindOAuth, APIBase: srv.URL,
		ExpiresAt: time.Now().Add(12 * time.Hour), // still fresh; force refresh anyway
	})
	stdout, stderr, code := runRootForOAuthExpiry(t, "auth", "refresh")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	env := decodeOKEnvelope(t, stdout)
	data, _ := env.Data.(map[string]any)
	if data["auth_refresh"] != authRefreshRefreshed {
		t.Fatalf("auth_refresh=%v data=%#v", data["auth_refresh"], data)
	}
	cred, _, err := config.LoadCredentials()
	if err != nil || cred.Active == nil || cred.Active.AccessToken != "oat-refreshed" {
		t.Fatalf("expected rotated access token, got %#v err=%v", cred.Active, err)
	}
}

func TestAuthRefreshNoRefreshToken(t *testing.T) {
	isolateOAuthConfig(t, "")
	writeOAuthCred(t, config.Credential{
		AccessToken: "oat-x", TokenKind: config.TokenKindOAuth,
		APIBase: "https://example.invalid", ExpiresAt: time.Now().Add(-1 * time.Hour),
	})
	stdout, stderr, code := runRootForOAuthExpiry(t, "auth", "refresh")
	if code == 0 {
		t.Fatalf("want failure, stdout=%s stderr=%s", stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, oauthRenewCommand) && !strings.Contains(stdout+stderr, "refresh_token") {
		t.Fatalf("want refresh_token / login hint, stdout=%s stderr=%s", stdout, stderr)
	}
}

func TestAuthRefreshHelpDocuments(t *testing.T) {
	if !strings.Contains(authRefreshCmd.Long, "--dry-run") {
		t.Fatalf("auth refresh Long must document --dry-run: %s", authRefreshCmd.Long)
	}
	if !strings.Contains(doctorCmd.Long, "auth_refresh") {
		t.Fatalf("doctor Long must document auth_refresh: %s", doctorCmd.Long)
	}
	if !strings.Contains(whoamiCmd.Long, "auth_refresh") {
		t.Fatalf("whoami Long must document auth_refresh: %s", whoamiCmd.Long)
	}
}

// silence unused import if json only needed transitively
var _ = json.Marshal


func TestTryOAuthRefreshSaveFailKeepsPending(t *testing.T) {
	srv := newOAuthRefreshStack(t, true, true)
	isolateOAuthConfig(t, srv.URL)
	writeOAuthCred(t, config.Credential{
		AccessToken: "oat-dead", RefreshToken: "ort-old", ClientID: "cid",
		TokenKind: config.TokenKindOAuth, APIBase: srv.URL,
		ExpiresAt: time.Now().Add(-time.Hour),
	})
	// Make credentials.json unwritable (directory where a file is expected).
	p, err := config.CredentialsPath()
	if err != nil {
		t.Fatal(err)
	}
	// Make credentials.json read-only so SaveCredentials fails; LoadCredentials still works.
	if err := os.Chmod(p, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o600) })

	res, err := tryOAuthRefresh(t.Context(), nil, true)
	if err == nil {
		t.Fatal("want save error")
	}
	if res.Outcome != authRefreshFailed {
		t.Fatalf("outcome=%q", res.Outcome)
	}
	if !strings.Contains(err.Error(), config.CredentialsPendingName) {
		t.Fatalf("error should point at pending file: %v", err)
	}
	pending := filepath.Join(filepath.Dir(p), config.CredentialsPendingName)
	b, readErr := os.ReadFile(pending)
	if readErr != nil {
		t.Fatalf("pending missing: %v", readErr)
	}
	var f config.CredentialsFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if f.Active == nil || f.Active.RefreshToken != "ort-rotated" || f.Active.AccessToken != "oat-refreshed" {
		t.Fatalf("pending creds = %#v", f.Active)
	}
}
