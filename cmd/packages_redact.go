package cmd

import (
	"encoding/json"
	"strings"
)

// redactPackagesReposSecrets is the packages repos list after-hook (#386):
// strip plaintext credentials from repoDescriptor (JSON string) and any nested
// password/username fields before stdout / JSON envelope.
func redactPackagesReposSecrets(out any, meta map[string]any) (any, map[string]any) {
	return redactPackagesSensitive(out), meta
}

func isPackagesSensitiveKey(k string) bool {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "password", "passwd", "username", "user", "secret", "accesskey", "accesskeysecret", "access_key", "access_key_secret":
		return true
	default:
		return false
	}
}

func redactPackagesSensitive(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if isPackagesSensitiveKey(k) {
				out[k] = redactPackagesSecretValue(val)
				continue
			}
			if strings.EqualFold(k, "repoDescriptor") {
				out[k] = redactPackagesRepoDescriptor(val)
				continue
			}
			out[k] = redactPackagesSensitive(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = redactPackagesSensitive(val)
		}
		return out
	default:
		return v
	}
}

func redactPackagesSecretValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(t) == "" {
			return t
		}
		return "***"
	default:
		return "***"
	}
}

func redactPackagesRepoDescriptor(v any) any {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return t
		}
		var parsed any
		if err := json.Unmarshal([]byte(t), &parsed); err != nil {
			return t
		}
		redacted := redactPackagesSensitive(parsed)
		b, err := json.Marshal(redacted)
		if err != nil {
			return "***"
		}
		return string(b)
	default:
		return redactPackagesSensitive(v)
	}
}
