package orguid

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// AccountIDString normalizes JSON number/string accountId to a decimal UID string.
func AccountIDString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return strings.TrimSpace(x.String())
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatInt(int64(x), 10)
	default:
		s := strings.TrimSpace(fmt.Sprint(x))
		if s == "<nil>" {
			return ""
		}
		return s
	}
}

// AttachAliyunUID copies oapi member maps and sets aliyunUid/accountId from legacy
// ListOrganizationMembers rows, matching by email (preferred) then name.
func AttachAliyunUID(oapi []any, legacy []map[string]any) ([]any, map[string]any) {
	byEmail := map[string]string{}
	byName := map[string]string{}
	for _, row := range legacy {
		uid := AccountIDString(row["accountId"])
		if uid == "" {
			continue
		}
		if e, ok := row["email"].(string); ok {
			e = strings.ToLower(strings.TrimSpace(e))
			if e != "" {
				byEmail[e] = uid
			}
		}
		if n, ok := row["organizationMemberName"].(string); ok {
			n = strings.TrimSpace(n)
			if n != "" {
				byName[strings.ToLower(n)] = uid
			}
		}
	}

	out := make([]any, 0, len(oapi))
	matched := 0
	for _, item := range oapi {
		m, ok := item.(map[string]any)
		if !ok {
			out = append(out, item)
			continue
		}
		cp := map[string]any{}
		for k, v := range m {
			cp[k] = v
		}
		uid := ""
		if e, ok := cp["email"].(string); ok {
			uid = byEmail[strings.ToLower(strings.TrimSpace(e))]
		}
		if uid == "" {
			if n, ok := cp["name"].(string); ok {
				uid = byName[strings.ToLower(strings.TrimSpace(n))]
			}
		}
		if uid != "" {
			cp["aliyunUid"] = uid
			cp["accountId"] = uid
			matched++
		}
		out = append(out, cp)
	}
	return out, map[string]any{
		"aliyun_uid_matched":   matched,
		"aliyun_uid_total":     len(oapi),
		"aliyun_uid_source":    "devops.ListOrganizationMembers.accountId",
		"manual_validate_note": "ManualValidate validators require aliyunUid (Aliyun numeric UID), not oapi userId hex",
	}
}

// MissingAKMessage explains how to obtain aliyun UIDs when only PAT is available.
func MissingAKMessage() string {
	return "ManualValidate (validatorType: users) requires Aliyun numeric UID in validators; " +
		"oapi/v1 platform members only returns hex userId (no accountId). " +
		"Set ALIBABA_CLOUD_ACCESS_KEY_ID and ALIBABA_CLOUD_ACCESS_KEY_SECRET " +
		"(same account that can call devops ListOrganizationMembers) and re-run " +
		"`yunxiao organization members list --include-aliyun-uid`, " +
		"or look up the UID in the Aliyun RAM console. See docs/wiki/02-domains/pipeline.md."
}
