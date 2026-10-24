package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// #389: access still "fresh" by clock but API rejects it (oat-dead). Doctor must
// not leave can_refresh optimistic without probing; either force-refresh on 401
// or expose honesty fields + probe-refresh.
func TestDoctorFreshDeadTokenForcesRefreshOn401(t *testing.T) {
	srv := newOAuthRefreshStack(t, true, true)
	isolateOAuthConfig(t, srv.URL)
	writeOAuthCred(t, config.Credential{
		AccessToken: "oat-dead", RefreshToken: "ort-x", ClientID: "cid",
		TokenKind: config.TokenKindOAuth, APIBase: srv.URL,
		ExpiresAt: time.Now().Add(12 * time.Hour), // clock says fresh → silent path would be not_needed
	})
	stdout, stderr, code := runRootForOAuthExpiry(t, "doctor")
	if code != 0 {
		t.Fatalf("want healthy after 401→refresh retry; exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	env := decodeOKEnvelope(t, stdout)
	token := doctorTokenCheck(t, env)
	if got, _ := token["auth_refresh"].(string); got != authRefreshRefreshed {
		t.Fatalf("auth_refresh=%q want refreshed (401 retry); token=%#v", got, token)
	}
	if present, _ := token["refresh_token_present"].(bool); !present {
		t.Fatalf("refresh_token_present want true; token=%#v", token)
	}
	if attempted, _ := token["refresh_attempted"].(bool); !attempted {
		t.Fatalf("refresh_attempted want true; token=%#v", token)
	}
	if ok, _ := token["refresh_ok"].(bool); !ok {
		t.Fatalf("refresh_ok want true; token=%#v", token)
	}
	conn := doctorNamedCheck(t, env, "connectivity")
	if ok, _ := conn["ok"].(bool); !ok {
		t.Fatalf("connectivity.ok want true; conn=%#v", conn)
	}
	if apiOK, _ := conn["api_ok"].(bool); !apiOK {
		t.Fatalf("api_ok want true; conn=%#v", conn)
	}
	cred, _, err := config.LoadCredentials()
	if err != nil || cred.Active == nil || cred.Active.AccessToken != "oat-refreshed" {
		t.Fatalf("expected rotated access token, got %#v err=%v", cred.Active, err)
	}
}

func TestDoctorProbeRefreshFlagForcesTokenEndpoint(t *testing.T) {
	srv := newOAuthRefreshStack(t, true, true)
	isolateOAuthConfig(t, srv.URL)
	writeOAuthCred(t, config.Credential{
		AccessToken: "oat-fresh", RefreshToken: "ort-x", ClientID: "cid",
		TokenKind: config.TokenKindOAuth, APIBase: srv.URL,
		ExpiresAt: time.Now().Add(12 * time.Hour),
	})
	stdout, stderr, code := runRootForOAuthExpiry(t, "doctor", "--probe-refresh")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	env := decodeOKEnvelope(t, stdout)
	token := doctorTokenCheck(t, env)
	if got, _ := token["auth_refresh"].(string); got != authRefreshRefreshed {
		t.Fatalf("auth_refresh=%q want refreshed with --probe-refresh; token=%#v", got, token)
	}
	if attempted, _ := token["refresh_attempted"].(bool); !attempted {
		t.Fatalf("refresh_attempted want true; token=%#v", token)
	}
	cred, _, err := config.LoadCredentials()
	if err != nil || cred.Active == nil || cred.Active.AccessToken != "oat-refreshed" {
		t.Fatalf("expected force refresh wrote new token, got %#v err=%v", cred.Active, err)
	}
}

func TestDoctorHonestyFieldsWhenNotProbed(t *testing.T) {
	srv := newOAuthRefreshStack(t, true, true)
	isolateOAuthConfig(t, srv.URL)
	writeOAuthCred(t, config.Credential{
		AccessToken: "oat-fresh", RefreshToken: "ort-x", ClientID: "cid",
		TokenKind: config.TokenKindOAuth, APIBase: srv.URL,
		ExpiresAt: time.Now().Add(12 * time.Hour),
	})
	stdout, stderr, code := runRootForOAuthExpiry(t, "doctor")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	env := decodeOKEnvelope(t, stdout)
	token := doctorTokenCheck(t, env)
	if got, _ := token["auth_refresh"].(string); got != authRefreshNotNeeded {
		t.Fatalf("auth_refresh=%q want not_needed; token=%#v", got, token)
	}
	if present, _ := token["refresh_token_present"].(bool); !present {
		t.Fatalf("refresh_token_present want true")
	}
	if attempted, _ := token["refresh_attempted"].(bool); attempted {
		t.Fatalf("refresh_attempted want false when silent not_needed")
	}
	if _, has := token["refresh_ok"]; has {
		t.Fatalf("refresh_ok must be absent when refresh not attempted; token=%#v", token)
	}
	if note, _ := token["can_refresh_note"].(string); note != "not_probed" {
		t.Fatalf("can_refresh_note=%q want not_probed; token=%#v", note, token)
	}
	conn := doctorNamedCheck(t, env, "connectivity")
	if apiOK, _ := conn["api_ok"].(bool); !apiOK {
		t.Fatalf("api_ok want true; conn=%#v", conn)
	}
}

func TestAuthRefreshInvalidGrantMachineReadable(t *testing.T) {
	srv := newOAuthRefreshStack(t, false, true)
	isolateOAuthConfig(t, srv.URL)
	writeOAuthCred(t, config.Credential{
		AccessToken: "oat-old", RefreshToken: "ort-bad", ClientID: "cid",
		TokenKind: config.TokenKindOAuth, APIBase: srv.URL,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})
	stdout, stderr, code := runRootForOAuthExpiry(t, "auth", "refresh")
	if code == 0 {
		t.Fatalf("want failure exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	blob := stdout + stderr
	var env output.Envelope
	// Failure may be on stderr (Fail) or stdout Success+exit; accept either with oauth_error.
	if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &env); err != nil {
		if err2 := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &env); err2 != nil {
			t.Fatalf("parse envelopes: stderr=%v stdout=%v blob=%s", err, err2, blob)
		}
	}
	oauthErr := ""
	if env.Data != nil {
		if m, ok := env.Data.(map[string]any); ok {
			if v, _ := m["oauth_error"].(string); v != "" {
				oauthErr = v
			}
		}
	}
	if oauthErr == "" && env.Error != nil && env.Error.Details != nil {
		if v, _ := env.Error.Details["oauth_error"].(string); v != "" {
			oauthErr = v
		}
	}
	if oauthErr == "" && env.Meta != nil {
		if v, _ := env.Meta["oauth_error"].(string); v != "" {
			oauthErr = v
		}
	}
	if oauthErr != "invalid_grant" {
		t.Fatalf("oauth_error want invalid_grant, got %q env=%+v blob=%s", oauthErr, env, blob)
	}
	if !strings.Contains(blob, "invalid_grant") {
		t.Fatalf("blob must mention invalid_grant: %s", blob)
	}
}

func TestAuthStatusCanRefreshNotProbed(t *testing.T) {
	isolateOAuthConfig(t, "https://example.invalid")
	writeOAuthCred(t, config.Credential{
		AccessToken: "oat-x", RefreshToken: "ort-x", ClientID: "cid",
		TokenKind: config.TokenKindOAuth, APIBase: "https://example.invalid",
		ExpiresAt: time.Now().Add(12 * time.Hour),
	})
	stdout, stderr, code := runRootForOAuthExpiry(t, "auth", "status")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	env := decodeOKEnvelope(t, stdout)
	data, _ := env.Data.(map[string]any)
	if data["can_refresh"] != true {
		t.Fatalf("can_refresh=%v", data["can_refresh"])
	}
	if note, _ := data["can_refresh_note"].(string); note != "not_probed" {
		t.Fatalf("can_refresh_note=%q want not_probed; data=%#v", note, data)
	}
	if data["refresh_token_present"] != true {
		t.Fatalf("refresh_token_present=%v", data["refresh_token_present"])
	}
}

