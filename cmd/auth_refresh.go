package cmd

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

var authRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh OAuth access token using stored refresh_token",
	Long: `Risk: write

Explicitly refreshes the active OAuth credential (force; does not wait for the
5-minute silent skew). Requires refresh_token + client_id from a prior
"yunxiao auth login --browser".

  yunxiao auth refresh
  yunxiao auth refresh --dry-run   # plan only: no network, no credential mutation

On success prints token_masked / expires_at_local / auth_refresh=refreshed.
On failure clears local oauth credentials and hints:
  yunxiao auth login --browser

Business commands (and doctor/whoami) also silent-refresh via the same path when
near expiry. Long-lived automation should prefer a PAT.`,
	Run: func(cmd *cobra.Command, args []string) {
		handleErr(runAuthRefresh(cmd))
	},
}

func runAuthRefresh(cmd *cobra.Command) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	cred, path, err := config.LoadCredentials()
	if err != nil {
		return err
	}
	out := map[string]any{
		"credentials_path": path,
		"dry_run":          dryRun,
	}
	if cred.Active == nil || cred.Active.AccessToken == "" {
		return output.Fail(output.ErrorBody{
			Type:    "cli",
			Message: "no active credentials",
			Hint:    "yunxiao auth login --browser  (or --token for PAT)",
		}, 1)
	}
	out["token_kind"] = string(cred.Active.TokenKind)
	out["token_masked"] = config.MaskToken(cred.Active.AccessToken)
	if cred.Active.TokenKind != config.TokenKindOAuth {
		return output.Fail(output.ErrorBody{
			Type:    "cli",
			Message: "active credential is not oauth (PAT has no refresh)",
			Hint:    "OAuth: yunxiao auth login --browser  |  PAT: no refresh needed",
		}, 1)
	}
	canRefresh := cred.Active.RefreshToken != "" && cred.Active.ClientID != ""
	nearOrPast := !cred.Active.ExpiresAt.IsZero() && time.Now().Add(oauthRefreshSkew).After(cred.Active.ExpiresAt)
	accessExpired := !cred.Active.ExpiresAt.IsZero() && !time.Now().Before(cred.Active.ExpiresAt)
	out["can_refresh"] = canRefresh
	out["has_refresh_token"] = cred.Active.RefreshToken != ""
	out["needs_refresh"] = nearOrPast
	out["access_expired"] = accessExpired
	out["would_refresh"] = canRefresh
	for k, v := range oauthExpiryFields(cred.Active.ExpiresAt, canRefresh) {
		out[k] = v
	}

	if dryRun {
		out["auth_refresh"] = authRefreshSkipped
		if canRefresh {
			out["auth_refresh"] = "would_refresh"
			out["hint"] = "Re-run without --dry-run to call the token endpoint"
		} else {
			out["hint"] = oauthRenewCommand
		}
		return output.Success(out, map[string]any{"dry_run": true})
	}

	if !canRefresh {
		return output.Fail(output.ErrorBody{
			Type:    "cli",
			Message: "cannot refresh: missing refresh_token or client_id",
			Hint:    oauthRenewCommand,
		}, 1)
	}

	attempt, err := tryOAuthRefresh(cmd.Context(), nil, true)
	out["auth_refresh"] = attempt.Outcome
	if err != nil {
		out["error"] = err.Error()
		if e := output.Success(out, map[string]any{"auth_refresh": authRefreshFailed}); e != nil {
			return e
		}
		return output.ExitError{Code: 1, Msg: "oauth refresh failed"}
	}

	// Reload after success for fresh expiry fields.
	cred2, _, err := config.LoadCredentials()
	if err != nil {
		return err
	}
	if cred2.Active != nil {
		out["token_masked"] = config.MaskToken(cred2.Active.AccessToken)
		out["expires_at"] = cred2.Active.ExpiresAt
		canRefresh2 := cred2.Active.RefreshToken != "" && cred2.Active.ClientID != ""
		out["can_refresh"] = canRefresh2
		out["has_refresh_token"] = cred2.Active.RefreshToken != ""
		delete(out, "warning")
		delete(out, "hint")
		for k, v := range oauthExpiryFields(cred2.Active.ExpiresAt, canRefresh2) {
			out[k] = v
		}
		out["access_expired"] = false
		out["needs_refresh"] = false
	}
	return output.Success(out, nil)
}

func init() {
	authRefreshCmd.Flags().Bool("dry-run", false, "preview whether refresh would run; no network / no credential mutation")
	authCmd.AddCommand(authRefreshCmd)
}
