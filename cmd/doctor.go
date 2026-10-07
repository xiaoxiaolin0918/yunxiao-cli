package cmd

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
	"github.com/yunxiao-cli/yunxiao/internal/update"
	"github.com/yunxiao-cli/yunxiao/internal/version"
)

var doctorCheckUpdate bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "CLI health check: config, auth, and connectivity",
	Long: `Risk: read

Prints resolved executable path (os.Executable / argv0; Windows-friendly),
active profile summary (name, organization_id, space_id), config/token checks,
and a connectivity probe.

The token check reports browser OAuth expiry (expires_at_local / expires_in /
expiring) and warns with the renew command when less than 24h remains and no
refresh_token is stored (#122); still healthy while the token is valid.

Optional: --check-update queries GitHub Releases once (no download).
Set YUNXIAO_UPDATE_CHECK=0 to skip even when the flag is passed.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		r, pf, err := resolveEffectiveConfig()
		if err != nil {
			handleErr(err)
			return
		}
		tokenCheck := map[string]any{"name": "token", "ok": r.AccessToken != "", "source": r.TokenSource, "token_kind": string(r.TokenKind)}
		if r.AccessToken == "" {
			tokenCheck["hint"] = "Prefer: yunxiao auth login --browser — or PAT: " + patHintShort()
			tokenCheck["console"] = yunxiaoPATConsoleURL
			tokenCheck["browser"] = "yunxiao auth login --browser"
		} else if r.TokenKind == config.TokenKindOAuth {
			tokenCheck["auth_header"] = r.AuthHeader
			// #122: surface oat- expiry (expires_at_local / expires_in /
			// expiring / warning) so users renew before the token dies mid-task.
			canRefresh := r.RefreshToken != "" && r.ClientID != ""
			tokenCheck["can_refresh"] = canRefresh
			for k, v := range oauthExpiryFields(r.ExpiresAt, canRefresh) {
				tokenCheck[k] = v
			}
		} else if r.TokenKind != "" {
			tokenCheck["auth_header"] = r.AuthHeader
		}
		checks := []map[string]any{
			executableCheck(),
			activeProfileCheck(pf),
			{"name": "config_file", "ok": true, "path": r.ConfigPath},
			tokenCheck,
			{"name": "api_base_url", "ok": r.APIBaseURL != "", "value": r.APIBaseURL},
			{"name": "edition", "ok": r.Edition != "", "value": r.Edition},
		}
		connectivity := map[string]any{"name": "connectivity", "ok": false}
		if r.AccessToken != "" {
			c, err := client.New(r)
			if err == nil {
				var user map[string]any
				if err := c.Get(cmd.Context(), "/oapi/v1/platform/user", nil, &user); err != nil {
					connectivity["ok"] = false
					connectivity["error"] = err.Error()
				} else {
					connectivity["ok"] = true
					connectivity["user_id"] = user["id"]
					connectivity["user_name"] = user["name"]
					connectivity["last_organization"] = user["lastOrganization"]
				}
			} else {
				connectivity["error"] = err.Error()
			}
		} else {
			connectivity["error"] = "no token — run: yunxiao auth login --browser (or --token / YUNXIAO_ACCESS_TOKEN)"
		}
		checks = append(checks, connectivity)
		if doctorCheckUpdate && !update.UpdateCheckDisabled() {
			checks = append(checks, doctorUpdateCheck(cmd))
		}
		allOK := true
		for _, ch := range checks {
			if ok, _ := ch["ok"].(bool); !ok {
				allOK = false
			}
		}
		meta := map[string]any{"healthy": allOK, "goos": runtime.GOOS, "goarch": runtime.GOARCH}
		if err := output.Success(checks, meta); err != nil {
			handleErr(err)
			return
		}
		if !allOK {
			handleErr(output.ExitError{Code: 1, Msg: "doctor found issues"})
		}
	},
}

func executableCheck() map[string]any {
	argv0 := ""
	if len(os.Args) > 0 {
		argv0 = os.Args[0]
	}
	path, err := os.Executable()
	ch := map[string]any{
		"name":  "executable",
		"ok":    true,
		"argv0": argv0,
	}
	if err != nil {
		ch["path"] = argv0
		ch["executable_error"] = err.Error()
		return ch
	}
	if resolved, err2 := filepath.EvalSymlinks(path); err2 == nil {
		path = resolved
	}
	ch["path"] = path
	return ch
}

func activeProfileCheck(pf *profile.Profile) map[string]any {
	name := activeProfileName()
	ch := map[string]any{
		"name":         "active_profile",
		"ok":           true,
		"profile_name": name,
		"profile_set":  name != "",
	}
	if pf == nil {
		if name == "" {
			ch["hint"] = "no active profile (set YUNXIAO_PROFILE or --profile)"
		}
		return ch
	}
	ch["organization_id"] = pf.OrganizationID
	ch["space_id"] = pf.SpaceID
	return ch
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorCheckUpdate, "check-update", false, "同时查询 GitHub Releases 是否有新版本（网络；需显式开启）")
}

func doctorUpdateCheck(cmd *cobra.Command) map[string]any {
	// Informational only: never flips doctor unhealthy (GitHub blips / offline).
	ch := map[string]any{"name": "update", "ok": true}
	rel, err := update.FetchLatestRelease(cmd.Context(), update.DefaultHTTPClient(), update.GithubRepo())
	if err != nil {
		ch["skipped"] = true
		ch["error"] = err.Error()
		return ch
	}
	latest := update.NormalizeVersion(rel.TagName)
	current := update.NormalizeVersion(version.Version)
	ch["current"] = current
	ch["latest"] = latest
	ch["update_available"] = update.NewerAvailable(current, latest)
	ch["message"] = update.FormatPair(current, latest)
	if update.NewerAvailable(current, latest) {
		ch["hint"] = "run: yunxiao update --check   # or: yunxiao update --yes"
	}
	return ch
}
