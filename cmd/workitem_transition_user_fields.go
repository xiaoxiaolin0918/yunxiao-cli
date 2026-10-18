package cmd

import (
	"fmt"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

// knownTransitionUserFieldIDs maps field id → hint label for values that must be
// userId-shaped on +bug-transition / +transition (#378). Includes profile
// bug_fields.developer / responsible_person and common native user keys.
func knownTransitionUserFieldIDs(pf *profile.Profile) map[string]string {
	out := map[string]string{
		"assignedTo":   "assignedTo",
		"verifier":     "verifier",
		"participants": "participants",
		"trackers":     "trackers",
	}
	if pf == nil {
		return out
	}
	if id := pf.FieldID("developer"); id != "" {
		out[id] = "--developer"
	}
	if id := pf.FieldID("responsible_person"); id != "" {
		out[id] = "--responsible-person"
	}
	return out
}

// requireLooksLikeUserID rejects display names / non-id shapes before dry-run or PUT (#378).
func requireLooksLikeUserID(hint, v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if looksLikeUserID(v) {
		return nil
	}
	return fmt.Errorf("%s 的用户值不合法（%q 不是 userId）。请传组织成员 userId，可用：yunxiao organization +member-resolve --name %s", hint, v, v)
}

// validateUserTypedFieldValues checks known user-typed entries in a fields map
// (string or string-list). Unknown keys are ignored.
func validateUserTypedFieldValues(known map[string]string, fields map[string]any) error {
	if len(fields) == 0 || len(known) == 0 {
		return nil
	}
	for fid, hint := range known {
		v, ok := fields[fid]
		if !ok || v == nil {
			continue
		}
		if err := assertUserIDShaped(hint, v); err != nil {
			return err
		}
	}
	return nil
}

func assertUserIDShaped(hint string, v any) error {
	switch x := v.(type) {
	case string:
		return requireLooksLikeUserID(hint, x)
	case []string:
		for _, s := range x {
			if err := requireLooksLikeUserID(hint, s); err != nil {
				return err
			}
		}
		return nil
	case []any:
		for _, el := range x {
			s, ok := el.(string)
			if !ok {
				return fmt.Errorf("%s 的用户值不合法（期望 userId 字符串，得到 %T）", hint, el)
			}
			if err := requireLooksLikeUserID(hint, s); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("%s 的用户值不合法（期望 userId 或 userId 列表，得到 %T）", hint, v)
	}
}

// validateTransitionProvidedUserFields runs the shared #378 check for a profile
// + provided/extra fields map (dry-run and real path).
func validateTransitionProvidedUserFields(pf *profile.Profile, fields map[string]any) error {
	return validateUserTypedFieldValues(knownTransitionUserFieldIDs(pf), fields)
}
