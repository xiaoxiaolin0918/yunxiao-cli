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
// Active credentials are always the last successful `auth login` (browser or token).
type CredentialsFile struct {
	// Active is the currently effective local login (oauth or pat).
	Active *Credential `json:"active,omitempty"`
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
	AuthHeader string `json:"auth_header,omitempty"`
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
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	b = append(b, '\n')
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return "", err
	}
	// Re-assert mode in case umask widened it.
	_ = os.Chmod(p, 0o600)
	return p, nil
}

// CredentialsPendingName is the sibling file used when credentials.json cannot be written
// after an OAuth refresh that may have rotated refresh_token.
const CredentialsPendingName = "credentials.json.refresh-pending"

// SaveCredentialsPending writes f to credentials.json.refresh-pending (mode 0600).
// Used when SaveCredentials fails after the token endpoint already rotated tokens,
// so the new refresh_token is not lost solely in process memory.
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
	f, _, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	if f.Active != nil && f.Active.TokenKind == TokenKindOAuth {
		f.Active = nil
	}
	if clearClients {
		f.Clients = map[string]string{}
	}
	return SaveCredentials(f)
}

// ClearActive removes whatever is active (logout).
func ClearActive() (string, error) {
	f, _, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	f.Active = nil
	return SaveCredentials(f)
}

// SetActivePAT stores a PAT as the active local credential (last successful auth login).
func SetActivePAT(accessToken, apiBase string) (string, error) {
	f, _, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	f.Active = &Credential{
		AccessToken: accessToken,
		TokenKind:   TokenKindPAT,
		TokenType:   "pat",
		APIBase:     strings.TrimRight(apiBase, "/"),
		UpdatedAt:   time.Now().UTC(),
	}
	return SaveCredentials(f)
}

// SetActiveOAuth stores OAuth tokens as the active local credential.
func SetActiveOAuth(c Credential) (string, error) {
	f, _, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	c.TokenKind = TokenKindOAuth
	c.APIBase = strings.TrimRight(c.APIBase, "/")
	c.UpdatedAt = time.Now().UTC()
	f.Active = &c
	if c.ClientID != "" && c.APIBase != "" {
		if f.Clients == nil {
			f.Clients = map[string]string{}
		}
		f.Clients[c.APIBase] = c.ClientID
	}
	return SaveCredentials(f)
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
	// Changing base does not clear other slots; only overwrite this base.
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
