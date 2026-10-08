package cmd

import (
	"fmt"
	"time"
)

// oauthExpiryWarnWindow is how close to expires_at the CLI starts warning
// (#122: platform oat- tokens live ~1 day and, without a refresh_token,
// expire mid-task with no notice). 24h per the issue.
const oauthExpiryWarnWindow = 24 * time.Hour

// oauthRenewCommand is the renewal command when silent refresh is unavailable.
const oauthRenewCommand = "yunxiao auth login --browser"

// oauthRefreshCommand is the preferred renew path when refresh_token is stored.
const oauthRefreshCommand = "yunxiao auth refresh"

// oauthExpiryFieldsAt describes an oauth credential's expiry for `auth status`
// and `doctor` (#122): human-readable local time, seconds remaining, and
// expired/expiring flags.
//
// Warnings:
//   - canRefresh + (expired|expiring) → prefer `yunxiao auth refresh`
//   - !canRefresh + (expired|expiring) → `yunxiao auth login --browser`
//
// Returns nil when expiresAt is zero (no expiry recorded).
func oauthExpiryFieldsAt(expiresAt, now time.Time, canRefresh bool) map[string]any {
	if expiresAt.IsZero() {
		return nil
	}
	secs := expiresAt.Unix() - now.Unix()
	expired := secs <= 0
	expiring := !expired && secs <= int64(oauthExpiryWarnWindow/time.Second)
	fields := map[string]any{
		"expires_at_local": expiresAt.Local().Format("2006-01-02 15:04:05 MST"),
		"expires_in":       secs,
		"expired":          expired,
		"expiring":         expiring,
		"auto_refresh":     canRefresh,
	}
	switch {
	case !expired && !expiring:
		// Plenty of time left; nothing to surface.
	case canRefresh && expired:
		fields["warning"] = fmt.Sprintf("OAuth 令牌已于 %s 过期；可先运行: %s（失败再 %s）",
			fields["expires_at_local"], oauthRefreshCommand, oauthRenewCommand)
		fields["hint"] = oauthRefreshCommand
	case canRefresh && expiring:
		fields["warning"] = fmt.Sprintf("OAuth 令牌将于 %s（约 %s后）过期；建议先: %s（业务命令也会静默刷新）",
			fields["expires_at_local"], oauthHumanDuration(secs), oauthRefreshCommand)
		fields["hint"] = oauthRefreshCommand
	case expired:
		fields["warning"] = fmt.Sprintf("OAuth 令牌已于 %s 过期，请重跑: %s", fields["expires_at_local"], oauthRenewCommand)
		fields["hint"] = oauthRenewCommand
	default:
		fields["warning"] = fmt.Sprintf("OAuth 令牌将于 %s（约 %s后）过期，建议重跑: %s",
			fields["expires_at_local"], oauthHumanDuration(secs), oauthRenewCommand)
		fields["hint"] = oauthRenewCommand
	}
	return fields
}

// oauthExpiryFields is oauthExpiryFieldsAt with the current time.
func oauthExpiryFields(expiresAt time.Time, canRefresh bool) map[string]any {
	return oauthExpiryFieldsAt(expiresAt, time.Now(), canRefresh)
}

// oauthLoginHint is the `hint` string for a successful `auth login --browser`
// (#122 suggestion 3): human-readable expiry plus how to renew.
func oauthLoginHint(expiresAt, now time.Time, canRefresh bool) string {
	fields := oauthExpiryFieldsAt(expiresAt, now, canRefresh)
	local, _ := fields["expires_at_local"].(string)
	if local == "" {
		return ""
	}
	if canRefresh {
		return fmt.Sprintf("OAuth 令牌将于 %s（约 %s后）过期；临期业务命令会自动刷新，也可手动: %s；失败时重跑: %s",
			local, oauthHumanDuration(expiresAt.Unix()-now.Unix()), oauthRefreshCommand, oauthRenewCommand)
	}
	return fmt.Sprintf("OAuth 令牌将于 %s（约 %s后）过期；平台未返回 refresh_token，过期后重跑: %s",
		local, oauthHumanDuration(expiresAt.Unix()-now.Unix()), oauthRenewCommand)
}

// oauthHumanDuration renders remaining seconds as e.g. "23.5 小时" / "40 分钟".
func oauthHumanDuration(secs int64) string {
	switch {
	case secs >= 3600:
		return fmt.Sprintf("%.1f 小时", float64(secs)/3600)
	case secs >= 60:
		return fmt.Sprintf("%d 分钟", secs/60)
	default:
		return fmt.Sprintf("%d 秒", secs)
	}
}
