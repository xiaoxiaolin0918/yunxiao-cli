package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TokenKind distinguishes PAT vs OAuth access tokens in credentials.json.
type TokenKind string

const (
	TokenKindPAT   TokenKind = "pat"
	TokenKindOAuth TokenKind = "oauth"
)

// CredentialsFile is ~/.config/yunxiao/credentials.json (mode 0600).
//
// Active is the legacy global slot (login with no profile).
// Profiles holds per-profile credentials keyed by profile name (#380):
// `yunxiao auth login --profile <name>` writes Profiles[name].
type CredentialsFile struct {
	// Active is the currently effective local login when no profile is selected.
	Active *Credential `json:"active,omitempty"`
	// Profiles maps profile name → credential (#380).
	Profiles map[string]*Credential `json:"profiles,omitempty"`
	// Clients caches DCR client_id per api_base (change of base uses a separate slot).
	Clients map[string]string `json:"clients,omitempty"`
}

// Credential is one local login record.
type Credential struct {
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ClientID     string    `json:"client_id,omitempty"`
	TokenKind    TokenKind `json:"token_kind,omitempty"`
	APIBase      string    `json:"api_base,omitempty"`
	// AuthHeader records the OpenAPI header strategy proven by O1 probe:
	// "x-yunxiao-token" | "authorization-bearer". Empty means use default (x-yunxiao-token).
	AuthHeader string    `json:"auth_header,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

func CredentialsPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "credentials.json"), nil
}

// LoadCredentials reads credentials.json; missing file yields empty struct.
func LoadCredentials() (CredentialsFile, string, error) {
	p, err := CredentialsPath()
	if err != nil {
		return CredentialsFile{}, "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return CredentialsFile{}, p, nil
		}
		return CredentialsFile{}, p, err
	}
	var f CredentialsFile
	if err := json.Unmarshal(b, &f); err != nil {
		return CredentialsFile{}, p, err
	}
	if f.Clients == nil {
		f.Clients = map[string]string{}
	}
	if f.Profiles == nil {
		f.Profiles = map[string]*Credential{}
	}
	return f, p, nil
}

// SaveCredentials writes credentials.json with mode 0600.
func SaveCredentials(f CredentialsFile) (string, error) {
	p, err := CredentialsPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", err
	}
	if f.Clients == nil {
		f.Clients = map[string]string{}
	}
	if f.Profiles == nil {
		f.Profiles = map[string]*Credential{}
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	b = append(b, '\n')
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return "", err
	}
	_ = os.Chmod(p, 0o600)
	return p, nil
}

// CredentialsPendingName is the sibling file used when credentials.json cannot be written
// after an OAuth refresh that may have rotated refresh_token.
const CredentialsPendingName = "credentials.json.refresh-pending"

// SaveCredentialsPending writes f to credentials.json.refresh-pending (mode 0600).
func SaveCredentialsPending(f CredentialsFile) (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(d, CredentialsPendingName)
	if f.Clients == nil {
		f.Clients = map[string]string{}
	}
	if f.Profiles == nil {
		f.Profiles = map[string]*Credential{}
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	b = append(b, '\n')
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return "", err
	}
	_ = os.Chmod(p, 0o600)
	return p, nil
}

// ClearOAuthActive removes active credentials when they are oauth (e.g. refresh failure).
// Leaves clients cache intact unless clearClients is true.
func ClearOAuthActive(clearClients bool) (string, error) {
	return ClearOAuthSlot("", clearClients)
}

// ClearOAuthSlot clears oauth credentials in Active (empty profileName) or Profiles[profileName].
func ClearOAuthSlot(profileName string, clearClients bool) (string, error) {
	f, _, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	profileName = strings.TrimSpace(profileName)
	if profileName != "" {
		if c := f.Profiles[profileName]; c != nil && c.TokenKind == TokenKindOAuth {
			delete(f.Profiles, profileName)
		}
	} else if f.Active != nil && f.Active.TokenKind == TokenKindOAuth {
		f.Active = nil
	}
	if clearClients {
		f.Clients = map[string]string{}
	}
	return SaveCredentials(f)
}

// ClearActive removes whatever is active (logout without --profile).
func ClearActive() (string, error) {
	f, _, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	f.Active = nil
	return SaveCredentials(f)
}

// ClearProfile removes Profiles[profileName] only (#380 logout --profile).
func ClearProfile(profileName string) (string, error) {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return "", fmt.Errorf("empty profile name")
	}
	f, _, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	if f.Profiles != nil {
		delete(f.Profiles, profileName)
	}
	return SaveCredentials(f)
}

// SetActivePAT stores a PAT as the global Active credential (login with no profile).
func SetActivePAT(accessToken, apiBase string) (string, error) {
	return SetProfilePAT("", accessToken, apiBase)
}

// SetProfilePAT stores a PAT for profileName (#380). Empty profileName → Active.
func SetProfilePAT(profileName, accessToken, apiBase string) (string, error) {
	f, _, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	c := &Credential{
		AccessToken: accessToken,
		TokenKind:   TokenKindPAT,
		TokenType:   "pat",
		APIBase:     strings.TrimRight(apiBase, "/"),
		UpdatedAt:   time.Now().UTC(),
	}
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		f.Active = c
	} else {
		if f.Profiles == nil {
			f.Profiles = map[string]*Credential{}
		}
		f.Profiles[profileName] = c
	}
	return SaveCredentials(f)
}

// SetActiveOAuth stores OAuth tokens as the global Active credential.
func SetActiveOAuth(c Credential) (string, error) {
	return SetProfileOAuth("", c)
}

// SetProfileOAuth stores OAuth tokens for profileName (#380). Empty profileName → Active.
func SetProfileOAuth(profileName string, c Credential) (string, error) {
	f, _, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	c.TokenKind = TokenKindOAuth
	c.APIBase = strings.TrimRight(c.APIBase, "/")
	c.UpdatedAt = time.Now().UTC()
	cp := c
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		f.Active = &cp
	} else {
		if f.Profiles == nil {
			f.Profiles = map[string]*Credential{}
		}
		f.Profiles[profileName] = &cp
	}
	if c.ClientID != "" && c.APIBase != "" {
		if f.Clients == nil {
			f.Clients = map[string]string{}
		}
		f.Clients[c.APIBase] = c.ClientID
	}
	return SaveCredentials(f)
}

// CredentialForProfile returns Profiles[profileName] or Active when profileName is empty.
func CredentialForProfile(f CredentialsFile, profileName string) *Credential {
	profileName = strings.TrimSpace(profileName)
	if profileName != "" {
		if f.Profiles == nil {
			return nil
		}
		return f.Profiles[profileName]
	}
	return f.Active
}

// CachedClientID returns the DCR client_id for apiBase, if any.
func CachedClientID(apiBase string) (string, error) {
	f, _, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	base := strings.TrimRight(apiBase, "/")
	if f.Clients == nil {
		return "", nil
	}
	return f.Clients[base], nil
}

// SetCachedClientID stores client_id for apiBase (separate slot per base).
func SetCachedClientID(apiBase, clientID string) error {
	f, _, err := LoadCredentials()
	if err != nil {
		return err
	}
	base := strings.TrimRight(apiBase, "/")
	if f.Clients == nil {
		f.Clients = map[string]string{}
	}
	f.Clients[base] = clientID
	_, err = SaveCredentials(f)
	return err
}

// ClearClientIDForBase removes the cached client_id when api_base changes intent.
func ClearClientIDForBase(apiBase string) error {
	f, _, err := LoadCredentials()
	if err != nil {
		return err
	}
	base := strings.TrimRight(apiBase, "/")
	if f.Clients != nil {
		delete(f.Clients, base)
	}
	_, err = SaveCredentials(f)
	return err
}

// OAuthNeedsRefresh reports whether an oauth credential should be refreshed.
func OAuthNeedsRefresh(c *Credential, skew time.Duration) bool {
	if c == nil || c.TokenKind != TokenKindOAuth {
		return false
	}
	if c.RefreshToken == "" {
		return false
	}
	if c.ExpiresAt.IsZero() {
		return false
	}
	if skew <= 0 {
		skew = 5 * time.Minute
	}
	return time.Now().Add(skew).After(c.ExpiresAt)
}

// ErrNoOAuthToken is returned by probe helpers when no oauth credential is stored yet.
var ErrNoOAuthToken = fmt.Errorf("no oauth token in credentials.json — run: yunxiao auth login --browser")
