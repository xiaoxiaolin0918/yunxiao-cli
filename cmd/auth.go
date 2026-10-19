package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/oauth"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication: browser OAuth or personal access token (PAT)",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in via browser OAuth (--browser) or store a PAT (--token)",
	Long: `Authenticate the CLI.

Browser OAuth (recommended for interactive / Agent use):
  yunxiao auth login --browser
  yunxiao auth login --browser --profile <name>   # bind OAuth to profile (#380)
  yunxiao auth login --browser --dry-run   # print auth URL + callback; do not exchange

PAT (required for CI / headless):
  yunxiao auth login --token <PAT>
  yunxiao auth login --token <PAT> --profile <name>   # bind identity to profile (#380)
  Create a PAT: https://account-devops.aliyun.com/settings/personalAccessToken

WARNING: OAuth consent grants full account API capability (platform has no module scopes).

Token lifetime: oat- tokens are short-lived (~1 day). Login success prints the
human-readable expiry (expires_at_local). Business commands and doctor/whoami
silent-refresh when a refresh_token is stored; use "yunxiao auth refresh" to
force refresh, or "yunxiao auth login --browser" when refresh is unavailable.
"auth status" / "doctor" warn when less than 24h remains.

Token precedence (#380): YUNXIAO_ACCESS_TOKEN env > credentials.profiles[<active profile>] > credentials.active (no profile) > profile JSON access_token > config.json.

Risk: write`,
	Run: func(cmd *cobra.Command, args []string) {
		handleErr(runAuthLogin(cmd))
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether a token is configured (never prints the raw token)",
	Long: `Risk: read

For browser OAuth tokens also reports expiry: expires_at / expires_at_local /
expires_in, plus a warning when less than 24h remains (or already expired). When
can_refresh, hint prefers "yunxiao auth refresh"; otherwise
"yunxiao auth login --browser" (#122).`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		r, pf, err := resolveEffectiveConfig()
		if err != nil {
			handleErr(err)
			return
		}
		out := map[string]any{
			"has_token":          r.AccessToken != "",
			"token_source":       r.TokenSource,
			"credential_profile": r.CredentialProfile,
			"token_kind":         string(r.TokenKind),
			"token_masked":       config.MaskToken(r.AccessToken),
			"api_base_url":       r.APIBaseURL,
			"organization_id":    r.OrganizationID,
			"edition":            r.Edition,
			"config_path":        r.ConfigPath,
			"credentials_path":   r.CredentialsPath,
			"auth_header":        r.AuthHeader,
		}
		if r.TokenKind == config.TokenKindOAuth {
			canRefresh := r.RefreshToken != "" && r.ClientID != ""
			out["expires_at"] = r.ExpiresAt
			out["has_refresh_token"] = r.RefreshToken != ""
			out["can_refresh"] = canRefresh
			// #122: near-expiry reminder (expires_at_local / expires_in /
			// expiring / warning) when the token cannot silently refresh.
			for k, v := range oauthExpiryFields(r.ExpiresAt, canRefresh) {
				out[k] = v
			}
		}
		if pf != nil {
			out["profile"] = pf.Name
		}
		handleErr(output.Success(out, nil))
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove local credentials (per-profile slot or global active)",
	Long: `Risk: write

With --profile / YUNXIAO_PROFILE: clears only credentials.profiles[<name>] (#380).
Without a profile name: clears the legacy global Active slot and config.json access_token.`,
	Run: func(cmd *cobra.Command, args []string) {
		handleErr(runAuthLogout())
	},
}

var authProbeCmd = &cobra.Command{
	Use:   "probe-oauth",
	Short: "O1 hard gate: probe oat- against OpenAPI (x-yunxiao-token then Bearer)",
	Long: `Calls GET /oapi/v1/platform/user with the active oauth access token using
x-yunxiao-token first, then Authorization: Bearer. Records the working header
in credentials.json. Exits non-zero if no oauth token is stored yet, or if both
headers fail (do not ship O2 in that case).

Risk: read`,
	Run: func(cmd *cobra.Command, args []string) {
		handleErr(runAuthProbe(cmd.Context()))
	},
}

// authTargetProfile returns the profile name login/logout should bind to.
// Uses --profile / YUNXIAO_PROFILE / config default (same as ResolveName).
func authTargetProfile() string {
	return profile.ResolveName(globalProfile)
}

// authLogoutProfileTarget returns the profile slot to clear on logout.
// Only --profile flag or YUNXIAO_PROFILE (not config default) so a bare
// "auth logout" still clears the legacy global Active slot.
func authLogoutProfileTarget() string {
	if s := strings.TrimSpace(globalProfile); s != "" {
		return s
	}
	return strings.TrimSpace(os.Getenv(profile.EnvProfile))
}

func runAuthLogout() error {
	name := authLogoutProfileTarget()
	if name != "" {
		credPath, err := config.ClearProfile(name)
		if err != nil {
			return err
		}
		return output.Success(map[string]any{
			"credentials_path":   credPath,
			"cleared":            true,
			"credential_profile": name,
			"hint":               "cleared credentials.profiles[" + name + "] only; other profiles and global Active untouched",
		}, nil)
	}
	credPath, err := config.ClearActive()
	if err != nil {
		return err
	}
	f, _, err := config.LoadFile()
	if err != nil {
		return err
	}
	f.AccessToken = ""
	cfgPath, err := config.SaveFile(f)
	if err != nil {
		return err
	}
	return output.Success(map[string]any{
		"credentials_path": credPath,
		"config_path":      cfgPath,
		"cleared":          true,
		"hint":             "cleared global Active + config.json token; profile slots unchanged — use auth logout --profile <name>",
	}, nil)
}

func runAuthLogin(cmd *cobra.Command) error {
	browser, _ := cmd.Flags().GetBool("browser")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	token, _ := cmd.Flags().GetString("token")
	apiBaseFlag, _ := cmd.Flags().GetString("api-base-url")
	org, _ := cmd.Flags().GetString("organization-id")

	if dryRun && !browser {
		return output.Fail(output.ErrorBody{
			Type:    "cli",
			Message: "--dry-run requires --browser",
			Hint:    "yunxiao auth login --browser --dry-run",
		}, 1)
	}
	if browser && token != "" {
		return output.Fail(output.ErrorBody{
			Type:    "cli",
			Message: "use only one of --browser or --token",
		}, 1)
	}

	// No-arg: TTY ask; non-TTY prefer hint toward browser for agents.
	if !browser && token == "" {
		if stdinIsInteractive() {
			fmt.Fprint(os.Stderr, "Login method: [b]rowser OAuth (recommended) / [t]oken PAT / [q]uit: ")
			line, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				return err
			}
			switch strings.ToLower(strings.TrimSpace(line)) {
			case "b", "browser", "1", "":
				browser = true
			case "t", "token", "pat", "2":
				// fall through to token prompt
			default:
				return output.Fail(output.ErrorBody{Type: "cli", Message: "login cancelled"}, 1)
			}
		} else {
			return output.Fail(output.ErrorBody{
				Type:    "cli",
				Message: "specify --browser (interactive) or --token <PAT> (CI)",
				Hint:    "Agents: prefer yunxiao auth login --browser; headless CI: --token or YUNXIAO_ACCESS_TOKEN",
			}, 1)
		}
	}

	apiBase := config.DefaultAPIBaseURL
	if apiBaseFlag != "" {
		apiBase = strings.TrimRight(apiBaseFlag, "/")
	} else if v := strings.TrimSpace(os.Getenv(config.EnvAPIBaseURL)); v != "" {
		apiBase = strings.TrimRight(v, "/")
	} else {
		f, _, _ := config.LoadFile()
		if f.APIBaseURL != "" {
			apiBase = strings.TrimRight(f.APIBaseURL, "/")
		}
	}

	if browser {
		return runBrowserLogin(cmd.Context(), apiBase, org, authTargetProfile(), dryRun)
	}
	return runPATLogin(cmd, token, apiBase, org, authTargetProfile())
}

func runBrowserLogin(ctx context.Context, apiBase, org, profileName string, dryRun bool) error {
	cached, _ := config.CachedClientID(apiBase)
	res, err := oauth.BrowserLogin(ctx, oauth.BrowserLoginOptions{
		APIBase:        apiBase,
		CachedClientID: cached,
		DryRun:         dryRun,
		WarnWriter: func(s string) {
			fmt.Fprint(os.Stderr, s)
		},
		OnDryRun: func(authURL, redirectURI, clientID string) error {
			return output.Success(map[string]any{
				"dry_run":      true,
				"auth_url":     authURL,
				"redirect_uri": redirectURI,
				"client_id":    clientID,
				"api_base":     apiBase,
				"warning":      strings.TrimSpace(oauth.FullAccountCapabilityWarning),
				"hint":         "Re-run without --dry-run to complete login; then yunxiao auth probe-oauth",
			}, nil)
		},
	})
	if err != nil {
		return err
	}
	if dryRun {
		return nil
	}

	cred := config.Credential{
		AccessToken:  res.Tokens.AccessToken,
		RefreshToken: res.Tokens.RefreshToken,
		TokenType:    res.Tokens.TokenType,
		ClientID:     res.ClientID,
		TokenKind:    config.TokenKindOAuth,
		APIBase:      apiBase,
		ExpiresAt:    oauth.ExpiresAt(res.Tokens.ExpiresIn, time.Now().UTC()),
	}
	path, err := config.SetProfileOAuth(profileName, cred)
	if err != nil {
		return err
	}

	// O1 hard gate: probe headers immediately.
	probe, probeErr := oauth.ProbeWhoami(ctx, nil, apiBase, res.Tokens.AccessToken)
	out := map[string]any{
		"credentials_path":   path,
		"token_kind":         "oauth",
		"token_masked":       config.MaskToken(res.Tokens.AccessToken),
		"api_base":           apiBase,
		"client_id":          res.ClientID,
		"expires_at":         cred.ExpiresAt,
		"credential_profile": profileName,
	}
	// #122: human-readable expiry + renew command on login success.
	canRefresh := res.Tokens.RefreshToken != "" && res.ClientID != ""
	for k, v := range oauthExpiryFields(cred.ExpiresAt, canRefresh) {
		out[k] = v
	}
	if hint := oauthLoginHint(cred.ExpiresAt, time.Now(), canRefresh); hint != "" {
		out["hint"] = hint
	}
	if org != "" {
		f, _, _ := config.LoadFile()
		f.OrganizationID = org
		if apiBase != "" {
			f.APIBaseURL = apiBase
		}
		if _, err := config.SaveFile(f); err != nil {
			return err
		}
	}
	if probeErr != nil {
		out["probe_ok"] = false
		out["probe_error"] = probeErr.Error()
		if probe != nil {
			out["probe"] = probe
		}
		if err := output.Success(out, map[string]any{"probe_ok": false}); err != nil {
			return err
		}
		return output.ExitError{Code: 1, Msg: "O1 hard gate: OpenAPI probe failed for oat-"}
	}
	out["probe_ok"] = true
	out["probe"] = probe
	// Persist proven header strategy.
	cred.AuthHeader = string(probe.Strategy)
	if _, err := config.SetProfileOAuth(profileName, cred); err != nil {
		return err
	}
	out["auth_header"] = cred.AuthHeader
	if probe.UserName != "" {
		out["user_name"] = probe.UserName
	}
	if probe.LastOrg != "" {
		out["last_organization"] = probe.LastOrg
	}
	return output.Success(out, nil)
}

func runPATLogin(cmd *cobra.Command, token, apiBase, org, profileName string) error {
	if token == "" {
		fmt.Fprint(os.Stderr, "Paste Yunxiao personal access token (input hidden not available; paste then Enter):\n> ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return err
		}
		token = strings.TrimSpace(line)
	}
	if token == "" {
		fmt.Fprintf(os.Stderr, "Create a Yunxiao PAT (console):\n  %s\nHelp: %s\n%s\n", yunxiaoPATConsoleURL, yunxiaoPATHelpURL, patPermissionsGuide())
		return output.Fail(output.ErrorBody{
			Type:    "cli",
			Message: "empty token",
			Hint:    "Prefer: yunxiao auth login --browser — or PAT: " + patHintShort(),
		}, 1)
	}
	credPath, err := config.SetProfilePAT(profileName, token, apiBase)
	if err != nil {
		return err
	}
	var cfgPath string
	if profileName == "" {
		// Keep legacy config.json in sync only for global login (older tooling).
		f, _, err := config.LoadFile()
		if err != nil {
			return err
		}
		f.AccessToken = token
		if apiBase != "" {
			f.APIBaseURL = apiBase
		}
		if org != "" {
			f.OrganizationID = org
		}
		cfgPath, err = config.SaveFile(f)
		if err != nil {
			return err
		}
	} else if org != "" {
		// Optional: persist org on config when provided with profile login.
		f, _, err := config.LoadFile()
		if err != nil {
			return err
		}
		if org != "" {
			f.OrganizationID = org
		}
		if apiBase != "" {
			f.APIBaseURL = apiBase
		}
		if _, err := config.SaveFile(f); err != nil {
			return err
		}
	}
	out := map[string]any{
		"credentials_path":   credPath,
		"token_kind":         "pat",
		"token_masked":       config.MaskToken(token),
		"credential_profile": profileName,
		"hint":               "token stored with mode 0600; prefer YUNXIAO_ACCESS_TOKEN in CI; interactive: auth login --browser",
	}
	if profileName == "" {
		out["config_path"] = cfgPath
		out["hint"] = "token stored in global Active (+ legacy config.json); use --profile <name> to bind a per-profile identity (#380)"
	} else {
		out["hint"] = "token stored in credentials.profiles[" + profileName + "]; switch with --profile / YUNXIAO_PROFILE (#380)"
	}
	return output.Success(out, nil)
}

func runAuthProbe(ctx context.Context) error {
	cred, path, err := config.LoadCredentials()
	if err != nil {
		return err
	}
	profileName := authTargetProfile()
	slot := config.CredentialForProfile(cred, profileName)
	if slot == nil || slot.TokenKind != config.TokenKindOAuth || slot.AccessToken == "" {
		hint := "Complete browser login first: yunxiao auth login --browser"
		if profileName != "" {
			hint = "Complete browser login for this profile: yunxiao auth login --browser --profile " + profileName
		}
		return output.Fail(output.ErrorBody{
			Type:    "cli",
			Message: config.ErrNoOAuthToken.Error(),
			Hint:    hint,
		}, 2)
	}
	apiBase := slot.APIBase
	if apiBase == "" {
		apiBase = config.DefaultAPIBaseURL
	}
	probe, err := oauth.ProbeWhoami(ctx, nil, apiBase, slot.AccessToken)
	out := map[string]any{
		"credentials_path":   path,
		"api_base":           apiBase,
		"token_masked":       config.MaskToken(slot.AccessToken),
		"credential_profile": profileName,
	}
	if probe != nil {
		out["probe"] = probe
	}
	if err != nil {
		out["probe_ok"] = false
		out["probe_error"] = err.Error()
		if e := output.Success(out, map[string]any{"probe_ok": false}); e != nil {
			return e
		}
		return output.ExitError{Code: 1, Msg: "O1 hard gate FAILED — oat- not usable with existing OpenAPI headers"}
	}
	slot.AuthHeader = string(probe.Strategy)
	if profileName == "" {
		cred.Active = slot
	} else {
		if cred.Profiles == nil {
			cred.Profiles = map[string]*config.Credential{}
		}
		cred.Profiles[profileName] = slot
	}
	if _, err := config.SaveCredentials(cred); err != nil {
		return err
	}
	out["probe_ok"] = true
	out["auth_header"] = slot.AuthHeader
	return output.Success(out, nil)
}

func init() {
	authLoginCmd.Flags().Bool("browser", false, "OAuth authorization-code + PKCE via local browser")
	authLoginCmd.Flags().Bool("dry-run", false, "with --browser: print auth URL and redirect_uri; do not wait/exchange")
	authLoginCmd.Flags().String("token", "", "PAT value (CI / headless; otherwise prompted)")
	authLoginCmd.Flags().String("api-base-url", "", "override API base URL (also sets oauth discovery base)")
	authLoginCmd.Flags().String("organization-id", "", "default organization ID")
	authCmd.AddCommand(authLoginCmd, authStatusCmd, authLogoutCmd, authProbeCmd)
}
