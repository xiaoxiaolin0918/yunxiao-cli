package cmd

import (
	"context"
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
	if cred.Active == nil || cred.Active.TokenKind != config.TokenKindOAuth {
		return res, nil
	}
	canRefresh := cred.Active.RefreshToken != "" && cred.Active.ClientID != ""
	res.CanRefresh = canRefresh
	if !cred.Active.ExpiresAt.IsZero() {
		res.AccessExpired = !time.Now().Before(cred.Active.ExpiresAt)
	}
	nearOrPast := !cred.Active.ExpiresAt.IsZero() && time.Now().Add(oauthRefreshSkew).After(cred.Active.ExpiresAt)
	res.NeedsRefresh = force || nearOrPast

	if !force && !nearOrPast {
		res.Outcome = authRefreshNotNeeded
		return res, nil
	}
	if !canRefresh {
		res.Outcome = authRefreshSkipped
		return res, nil
	}

	apiBase := cred.Active.APIBase
	if apiBase == "" && c != nil {
		apiBase = c.BaseURL
	}
	meta, err := oauth.Discover(ctx, nil, apiBase)
	if err != nil {
		res.Outcome = authRefreshFailed
		return res, clearOAuthAndErr(err)
	}
	tok, err := oauth.Refresh(ctx, nil, meta.TokenEndpoint, cred.Active.ClientID, cred.Active.RefreshToken)
	if err != nil {
		res.Outcome = authRefreshFailed
		return res, clearOAuthAndErr(err)
	}
	cred.Active.AccessToken = tok.AccessToken
	if tok.RefreshToken != "" {
		cred.Active.RefreshToken = tok.RefreshToken
	}
	cred.Active.ExpiresAt = oauth.ExpiresAt(tok.ExpiresIn, time.Now().UTC())
	cred.Active.UpdatedAt = time.Now().UTC()
	if _, err := config.SaveCredentials(cred); err != nil {
		// Token endpoint may have rotated refresh_token; keep a sibling copy so it is not lost.
		res.Outcome = authRefreshFailed
		if pending, pendErr := config.SaveCredentialsPending(cred); pendErr == nil {
			return res, fmt.Errorf("oauth refreshed but could not write credentials.json (%w); new tokens saved to %s — move over credentials.json or re-run: yunxiao auth login --browser", err, pending)
		}
		return res, fmt.Errorf("oauth refreshed but failed to save credentials (new refresh_token may be lost; re-run: yunxiao auth login --browser): %w", err)
	}
	if c != nil {
		c.Token = tok.AccessToken
	}
	res.Outcome = authRefreshRefreshed
	return res, nil
}

func clearOAuthAndErr(cause error) error {
	_, _ = config.ClearOAuthActive(false)
	return fmt.Errorf("oauth refresh failed — cleared local oauth credentials; re-run: yunxiao auth login --browser (%w)", cause)
}
