package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/oauth"
)

// auth_refresh JSON outcomes for doctor / whoami / auth refresh.
const (
	authRefreshNotNeeded = "not_needed"     // oauth present but not near/past expiry (silent path)
	authRefreshSkipped   = "skipped"        // not oauth, no token, or cannot refresh
	authRefreshRefreshed = "refreshed"      // token endpoint succeeded
	authRefreshFailed    = "refresh_failed" // token endpoint failed; oauth creds cleared
)

// oauthRefreshSkew matches the silent mustClient / ensureFresh window.
const oauthRefreshSkew = 5 * time.Minute

// oauthRefreshAttempt is the structured result of tryOAuthRefresh.
type oauthRefreshAttempt struct {
	Outcome       string
	AccessExpired bool // access token expires_at <= now (before attempt)
	NeedsRefresh  bool // within skew or force
	CanRefresh    bool // refresh_token + client_id present
	OAuthError    string // RFC 6749 token error code when refresh failed (e.g. invalid_grant)
}

// oauthRefreshHook refreshes oat- when near expiry; on failure clears oauth creds.
// Wired as client.OnRefresh by mustClient for business commands.
func oauthRefreshHook(ctx context.Context, c *client.Client) error {
	_, err := tryOAuthRefresh(ctx, c, false)
	return err
}

// tryOAuthRefresh runs the shared OAuth refresh path.
// When force is false (silent): refreshes only if near/past the skew window.
// When force is true (auth refresh): refreshes whenever can_refresh.
// Outcomes: not_needed | skipped | refreshed | refresh_failed.
func tryOAuthRefresh(ctx context.Context, c *client.Client, force bool) (oauthRefreshAttempt, error) {
	res := oauthRefreshAttempt{Outcome: authRefreshSkipped}
	cred, _, err := config.LoadCredentials()
	if err != nil {
		return res, err
	}
	profileName := activeProfileName()
	slot := config.CredentialForProfile(cred, profileName)
	if slot == nil || slot.TokenKind != config.TokenKindOAuth {
		return res, nil
	}
	canRefresh := slot.RefreshToken != "" && slot.ClientID != ""
	res.CanRefresh = canRefresh
	if !slot.ExpiresAt.IsZero() {
		res.AccessExpired = !time.Now().Before(slot.ExpiresAt)
	}
	nearOrPast := !slot.ExpiresAt.IsZero() && time.Now().Add(oauthRefreshSkew).After(slot.ExpiresAt)
	res.NeedsRefresh = force || nearOrPast

	if !force && !nearOrPast {
		res.Outcome = authRefreshNotNeeded
		return res, nil
	}
	if !canRefresh {
		res.Outcome = authRefreshSkipped
		return res, nil
	}

	apiBase := slot.APIBase
	if apiBase == "" && c != nil {
		apiBase = c.BaseURL
	}
	meta, err := oauth.Discover(ctx, nil, apiBase)
	if err != nil {
		res.Outcome = authRefreshFailed
		return res, clearOAuthAndErr(profileName, err)
	}
	tok, err := oauth.Refresh(ctx, nil, meta.TokenEndpoint, slot.ClientID, slot.RefreshToken)
	if err != nil {
		res.Outcome = authRefreshFailed
		var te *oauth.TokenError
		if errors.As(err, &te) && te.Code != "" {
			res.OAuthError = te.Code
		}
		return res, clearOAuthAndErr(profileName, err)
	}
	slot.AccessToken = tok.AccessToken
	if tok.RefreshToken != "" {
		slot.RefreshToken = tok.RefreshToken
	}
	slot.ExpiresAt = oauth.ExpiresAt(tok.ExpiresIn, time.Now().UTC())
	slot.UpdatedAt = time.Now().UTC()
	if profileName == "" {
		cred.Active = slot
	} else {
		if cred.Profiles == nil {
			cred.Profiles = map[string]*config.Credential{}
		}
		cred.Profiles[profileName] = slot
	}
	if _, err := config.SaveCredentials(cred); err != nil {
		res.Outcome = authRefreshFailed
		if pending, pendErr := config.SaveCredentialsPending(cred); pendErr == nil {
			return res, fmt.Errorf("oauth refreshed but could not write credentials.json (%w); new tokens saved to %s; move over credentials.json or re-run: yunxiao auth login --browser", err, pending)
		}
		return res, fmt.Errorf("oauth refreshed but failed to save credentials (new refresh_token may be lost; re-run: yunxiao auth login --browser): %w", err)
	}
	if c != nil {
		c.Token = tok.AccessToken
	}
	res.Outcome = authRefreshRefreshed

	return res, nil
}

func clearOAuthAndErr(profileName string, err error) error {
	if _, cErr := config.ClearOAuthSlot(profileName, false); cErr != nil {
		return fmt.Errorf("oauth refresh failed (%v); also failed to clear credentials: %w", err, cErr)
	}
	return fmt.Errorf("oauth refresh failed (credentials cleared; re-run: yunxiao auth login --browser): %w", err)
}


// applyOAuthHonestyFields records #389 diagnostic fields on doctor/auth status maps.
// can_refresh alone means local refresh_token+client_id; can_refresh_note=not_probed
// until a token-endpoint attempt runs (silent near-expiry, --probe-refresh, or 401 retry).
func applyOAuthHonestyFields(dst map[string]any, attempt oauthRefreshAttempt, refreshTokenPresent bool) {
	dst["refresh_token_present"] = refreshTokenPresent
	attempted := attempt.Outcome == authRefreshRefreshed || attempt.Outcome == authRefreshFailed
	dst["refresh_attempted"] = attempted
	if attempted {
		dst["refresh_ok"] = attempt.Outcome == authRefreshRefreshed
		if attempt.Outcome == authRefreshRefreshed {
			dst["can_refresh_note"] = "probed_ok"
		} else {
			dst["can_refresh_note"] = "probed_fail"
		}
		if attempt.OAuthError != "" {
			dst["oauth_error"] = attempt.OAuthError
		}
	} else {
		delete(dst, "refresh_ok")
		dst["can_refresh_note"] = "not_probed"
	}
}

func isUnauthorizedAPI(err error) bool {
	var ae *client.APIError
	return errors.As(err, &ae) && ae.Status == 401
}
