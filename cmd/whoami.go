package cmd

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current effective identity and token status (JSON)",
	Long: `Risk: read

For OAuth credentials, silently refreshes when near/past expiry (same path as
business commands) and reports auth_refresh: not_needed | skipped | refreshed |
refresh_failed. Prefer "yunxiao auth refresh" when can_refresh; on refresh
failure credentials are cleared and hint points to "yunxiao auth login --browser".`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		r, pf, err := resolveEffectiveConfig()
		if err != nil {
			handleErr(err)
			return
		}
		result := map[string]any{
			"token_source":    r.TokenSource,
			"token_masked":    config.MaskToken(r.AccessToken),
			"api_base_url":    r.APIBaseURL,
			"organization_id": r.OrganizationID,
			"edition":         r.Edition,
			"config_path":     r.ConfigPath,
			"auth_refresh":    authRefreshSkipped,
		}
		if pf != nil {
			result["profile"] = pf.Name
		}
		if r.AccessToken == "" {
			handleErr(output.Success(result, nil))
			return
		}
		if r.TokenKind == config.TokenKindOAuth {
			canRefresh := r.RefreshToken != "" && r.ClientID != ""
			result["token_kind"] = string(r.TokenKind)
			result["can_refresh"] = canRefresh
			result["access_expired"] = !r.ExpiresAt.IsZero() && !time.Now().Before(r.ExpiresAt)
			for k, v := range oauthExpiryFields(r.ExpiresAt, canRefresh) {
				result[k] = v
			}
		} else if r.TokenKind != "" {
			result["token_kind"] = string(r.TokenKind)
		}

		c, err := client.New(r)
		if err != nil {
			result["client_error"] = err.Error()
			handleErr(output.Success(result, nil))
			return
		}
		if r.TokenKind == config.TokenKindOAuth {
			c.OnRefresh = oauthRefreshHook
			attempt, refreshErr := tryOAuthRefresh(cmd.Context(), c, false)
			result["auth_refresh"] = attempt.Outcome
			result["access_expired"] = attempt.AccessExpired
			if refreshErr != nil {
				result["user_error"] = refreshErr.Error()
				result["hint"] = oauthRenewCommand
				delete(result, "warning")
				result["can_refresh"] = false
				handleErr(output.Success(result, nil))
				return
			}
			if attempt.Outcome == authRefreshRefreshed {
				if r2, _, err2 := resolveEffectiveConfig(); err2 == nil {
					r = r2
					result["token_masked"] = config.MaskToken(r.AccessToken)
					result["token_source"] = r.TokenSource
					canRefresh := r.RefreshToken != "" && r.ClientID != ""
					result["can_refresh"] = canRefresh
					delete(result, "warning")
					delete(result, "hint")
					for k, v := range oauthExpiryFields(r.ExpiresAt, canRefresh) {
						result[k] = v
					}
				}
			}
		}
		var user map[string]any
		if err := c.Get(cmd.Context(), "/oapi/v1/platform/user", nil, &user); err != nil {
			result["user_error"] = err.Error()
		} else {
			result["user"] = user
		}
		handleErr(output.Success(result, nil))
	},
}
