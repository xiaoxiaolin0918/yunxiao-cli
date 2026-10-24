package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TokenResponse is the OAuth token endpoint success body.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope,omitempty"`
}

// AuthURLParams builds the authorization request query.
type AuthURLParams struct {
	AuthorizationEndpoint string
	ClientID              string
	RedirectURI           string
	State                 string
	CodeChallenge         string
	CodeChallengeMethod   string // "S256"
	Scope                 string // optional metadata only
}

// AuthURL returns the browser URL for the authorization code + PKCE flow.
func AuthURL(p AuthURLParams) (string, error) {
	if p.AuthorizationEndpoint == "" || p.ClientID == "" || p.RedirectURI == "" || p.State == "" || p.CodeChallenge == "" {
		return "", fmt.Errorf("oauth AuthURL: missing required parameter")
	}
	if err := ValidateLoopbackRedirectURI(p.RedirectURI); err != nil {
		return "", err
	}
	method := p.CodeChallengeMethod
	if method == "" {
		method = "S256"
	}
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", p.RedirectURI)
	q.Set("state", p.State)
	q.Set("code_challenge", p.CodeChallenge)
	q.Set("code_challenge_method", method)
	if p.Scope != "" {
		q.Set("scope", p.Scope)
	}
	sep := "?"
	if strings.Contains(p.AuthorizationEndpoint, "?") {
		sep = "&"
	}
	return p.AuthorizationEndpoint + sep + q.Encode(), nil
}

// Exchange swaps an authorization code for tokens (public client + PKCE).
func Exchange(ctx context.Context, httpClient *http.Client, tokenEndpoint, clientID, code, redirectURI, codeVerifier string) (*TokenResponse, error) {
	if err := ValidateLoopbackRedirectURI(redirectURI); err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", codeVerifier)
	return postToken(ctx, httpClient, tokenEndpoint, form)
}

// TokenError is a non-2xx response from the OAuth token endpoint.
// Code is the RFC 6749 "error" field when present (e.g. invalid_grant).
type TokenError struct {
	Status int
	Code   string
	Body   string
}

func (e *TokenError) Error() string {
	if e == nil {
		return "oauth token: error"
	}
	if e.Code != "" {
		return fmt.Sprintf("oauth token: HTTP %d: %s: %s", e.Status, e.Code, truncate(e.Body, 400))
	}
	return fmt.Sprintf("oauth token: HTTP %d: %s", e.Status, truncate(e.Body, 400))
}

func newTokenError(status int, raw []byte) *TokenError {
	te := &TokenError{Status: status, Body: string(raw)}
	var body struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(raw, &body) == nil {
		te.Code = strings.TrimSpace(body.Error)
	}
	return te
}

// Refresh exchanges a refresh_token for a new access (and possibly refresh) token.
func Refresh(ctx context.Context, httpClient *http.Client, tokenEndpoint, clientID, refreshToken string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", clientID)
	form.Set("refresh_token", refreshToken)
	return postToken(ctx, httpClient, tokenEndpoint, form)
}

func postToken(ctx context.Context, httpClient *http.Client, tokenEndpoint string, form url.Values) (*TokenResponse, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth token: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, newTokenError(resp.StatusCode, raw)
	}
	var out TokenResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("oauth token: decode: %w", err)
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return nil, fmt.Errorf("oauth token: empty access_token")
	}
	return &out, nil
}

// ExpiresAt computes absolute expiry from expires_in seconds (now + expires_in).
func ExpiresAt(expiresIn int64, now time.Time) time.Time {
	if expiresIn <= 0 {
		// default ~24h for oat- if server omits
		expiresIn = 24 * 3600
	}
	return now.Add(time.Duration(expiresIn) * time.Second)
}
