package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultAPIBaseURL = "https://openapi-rdc.aliyuncs.com"
	EnvAccessToken    = "YUNXIAO_ACCESS_TOKEN"
	EnvAPIBaseURL     = "YUNXIAO_API_BASE_URL"
	EnvOrganizationID = "YUNXIAO_ORGANIZATION_ID"
	EnvEdition        = "YUNXIAO_EDITION" // central | region
)

type File struct {
	AccessToken    string `json:"access_token,omitempty"`
	APIBaseURL     string `json:"api_base_url,omitempty"`
	OrganizationID string `json:"organization_id,omitempty"`
	Edition        string `json:"edition,omitempty"`
	// Profile is the default tenant profile name set by `yunxiao profile use`
	// (#130): used when --profile and YUNXIAO_PROFILE are both unset.
	Profile string `json:"profile,omitempty"`
}

type Resolved struct {
	AccessToken    string
	APIBaseURL     string
	OrganizationID string
	Edition        string
	ConfigPath     string
	// CredentialsPath is ~/.config/yunxiao/credentials.json when used.
	CredentialsPath string
	// TokenSource is env | credentials | profile | config | none
	TokenSource string
	// CredentialProfile is the profile name when TokenSource is credentials
	// and the token came from credentials.profiles[<name>] (#380). Empty for
	// the legacy global Active slot, env, profile JSON, or config.
	CredentialProfile string
	// TokenKind is pat | oauth | "" (unknown/env)
	TokenKind TokenKind
	// AuthHeader is x-yunxiao-token | authorization-bearer (oauth probe result).
	AuthHeader string
	// RefreshToken / ExpiresAt / ClientID are set for oauth credentials.
	RefreshToken string
	ExpiresAt    time.Time
	ClientID     string
}

func Dir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "yunxiao"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "yunxiao"), nil
}

func Path() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

func LoadFile() (File, string, error) {
	p, err := Path()
	if err != nil {
		return File{}, "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{}, p, nil
		}
		return File{}, p, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, p, err
	}
	return f, p, nil
}

func SaveFile(f File) (string, error) {
	p, err := Path()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	b = append(b, '\n')
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return "", err
	}
	return p, nil
}

// Resolve loads config and resolves settings without a profile.
// Token precedence (#380):
//  1. YUNXIAO_ACCESS_TOKEN env (force override)
//  2. credentials.profiles[<active profile>] when a profile name is set
//  3. credentials.active (legacy global) when no profile name is set
//  4. profile JSON access_token (via ResolveForProfile / ResolveWithProfileToken)
//  5. config.json access_token (legacy)
//  6. none
//
// When a profile name is set, the legacy global Active slot is skipped so
// different profiles keep isolated identities.
func Resolve() (Resolved, error) {
	return ResolveForProfile("", "")
}

// ResolveWithProfileToken is like Resolve but considers an optional profile PAT
// from the profile JSON. Prefer ResolveForProfile when the profile name is known.
func ResolveWithProfileToken(profileToken string) (Resolved, error) {
	return ResolveForProfile("", profileToken)
}

// ResolveForProfile resolves tokens for an optional profile name (#380).
// profileName selects credentials.profiles[name] (and skips global Active).
// profileToken is the optional access_token embedded in the profile JSON.
// TokenSource reports env | credentials | profile | config | none.
func ResolveForProfile(profileName, profileToken string) (Resolved, error) {
	f, p, err := LoadFile()
	if err != nil {
		return Resolved{}, err
	}
	cred, credPath, err := LoadCredentials()
	if err != nil {
		return Resolved{}, err
	}
	r := Resolved{
		ConfigPath:      p,
		CredentialsPath: credPath,
		APIBaseURL:      DefaultAPIBaseURL,
		OrganizationID:  f.OrganizationID,
		Edition:         f.Edition,
	}
	if f.APIBaseURL != "" {
		r.APIBaseURL = strings.TrimRight(f.APIBaseURL, "/")
	}
	if v := strings.TrimSpace(os.Getenv(EnvAPIBaseURL)); v != "" {
		r.APIBaseURL = strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv(EnvOrganizationID)); v != "" {
		r.OrganizationID = v
	}
	if v := strings.TrimSpace(os.Getenv(EnvEdition)); v != "" {
		r.Edition = v
	}

	profileName = strings.TrimSpace(profileName)
	applyCred := func(a *Credential, credProfile string) {
		r.AccessToken = a.AccessToken
		r.TokenSource = "credentials"
		r.CredentialProfile = credProfile
		r.TokenKind = a.TokenKind
		r.AuthHeader = a.AuthHeader
		r.RefreshToken = a.RefreshToken
		r.ExpiresAt = a.ExpiresAt
		r.ClientID = a.ClientID
		if a.APIBase != "" && strings.TrimSpace(os.Getenv(EnvAPIBaseURL)) == "" && f.APIBaseURL == "" {
			r.APIBaseURL = strings.TrimRight(a.APIBase, "/")
		}
	}

	if t := strings.TrimSpace(os.Getenv(EnvAccessToken)); t != "" {
		r.AccessToken = t
		r.TokenSource = "env"
	} else if profileName != "" {
		if a := CredentialForProfile(cred, profileName); a != nil && strings.TrimSpace(a.AccessToken) != "" {
			applyCred(a, profileName)
		} else if t := strings.TrimSpace(profileToken); t != "" {
			r.AccessToken = t
			r.TokenSource = "profile"
			r.TokenKind = TokenKindPAT
		} else if f.AccessToken != "" {
			r.AccessToken = f.AccessToken
			r.TokenSource = "config"
			r.TokenKind = TokenKindPAT
		} else {
			r.TokenSource = "none"
		}
	} else if cred.Active != nil && strings.TrimSpace(cred.Active.AccessToken) != "" {
		applyCred(cred.Active, "")
	} else if t := strings.TrimSpace(profileToken); t != "" {
		r.AccessToken = t
		r.TokenSource = "profile"
		r.TokenKind = TokenKindPAT
	} else if f.AccessToken != "" {
		r.AccessToken = f.AccessToken
		r.TokenSource = "config"
		r.TokenKind = TokenKindPAT
	} else {
		r.TokenSource = "none"
	}

	if r.Edition == "" {
		if strings.Contains(r.APIBaseURL, "openapi-rdc.aliyuncs.com") {
			r.Edition = "central"
		} else {
			r.Edition = "region"
		}
	}
	return r, nil
}

func MaskToken(t string) string {
	if t == "" {
		return ""
	}
	if len(t) <= 8 {
		return "****"
	}
	return t[:4] + "****" + t[len(t)-4:]
}
