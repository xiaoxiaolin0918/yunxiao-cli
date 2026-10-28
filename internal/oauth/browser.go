package oauth

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// BrowserLoginOptions configures the interactive authorization-code flow.
type BrowserLoginOptions struct {
	APIBase        string
	ClientName     string
	HTTPClient     *http.Client
	Timeout        time.Duration
	OpenBrowser    func(url string) error // nil → OS default
	CachedClientID string
	WarnWriter     func(string) // printed before opening browser
	DryRun         bool
	// OnDryRun is called with auth URL + redirect when DryRun is true.
	OnDryRun func(authURL, redirectURI, clientID string) error
	// OnAuthURL is called once the authorization URL is ready (before OpenBrowser
	// and before OnDryRun). WorkBuddy and other agents use this to extract a bare
	// https URL within 10s without parsing JSON.
	OnAuthURL func(authURL string)
}

// BrowserLoginResult is the outcome of a successful (non-dry-run) login.
type BrowserLoginResult struct {
	Tokens   *TokenResponse
	ClientID string
	APIBase  string
	Meta     *Metadata
	Redirect string
	AuthURL  string
}

// FullAccountCapabilityWarning is required before opening the browser.
const FullAccountCapabilityWarning = `WARNING: Authorizing grants this CLI the full Yunxiao API capability of your account.
The platform does not support module/scopes limits for OAuth — unlike fine-grained PAT checkboxes.
Only continue if you trust this machine and this CLI binary.
`

// BrowserLogin runs discover → DCR (or cached client_id) → loopback listen → open browser → exchange.
func BrowserLogin(ctx context.Context, opt BrowserLoginOptions) (*BrowserLoginResult, error) {
	if opt.Timeout <= 0 {
		opt.Timeout = 5 * time.Minute
	}
	if opt.ClientName == "" {
		opt.ClientName = "yunxiao-cli"
	}
	if opt.HTTPClient == nil {
		opt.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if opt.OpenBrowser == nil {
		opt.OpenBrowser = openSystemBrowser
	}
	meta, err := Discover(ctx, opt.HTTPClient, opt.APIBase)
	if err != nil {
		return nil, err
	}

	lctx, cancel := context.WithTimeout(ctx, opt.Timeout)
	defer cancel()

	lr, err := StartLoopbackListener(lctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if lr.Server != nil {
			_ = lr.Server.Shutdown(context.Background())
		}
	}()

	// Always DCR-register for this redirect_uri: loopback port is ephemeral, and
	// Yunxiao binds redirect_uris at registration time. The resulting client_id is
	// cached per api_base after a successful login (used for refresh_token).
	_ = opt.CachedClientID
	reg, err := Register(ctx, opt.HTTPClient, meta.RegistrationEndpoint, opt.ClientName, lr.RedirectURI)
	if err != nil {
		return nil, err
	}
	clientID := reg.ClientID

	pkce, err := NewPKCE()
	if err != nil {
		return nil, err
	}
	state, err := RandomState()
	if err != nil {
		return nil, err
	}
	authURL, err := AuthURL(AuthURLParams{
		AuthorizationEndpoint: meta.AuthorizationEndpoint,
		ClientID:              clientID,
		RedirectURI:           lr.RedirectURI,
		State:                 state,
		CodeChallenge:         pkce.Challenge,
		CodeChallengeMethod:   "S256",
	})
	if err != nil {
		return nil, err
	}

	if opt.OnAuthURL != nil {
		opt.OnAuthURL(authURL)
	}

	if opt.DryRun {
		_ = lr.Server.Shutdown(context.Background())
		if opt.OnDryRun != nil {
			if err := opt.OnDryRun(authURL, lr.RedirectURI, clientID); err != nil {
				return nil, err
			}
		}
		return &BrowserLoginResult{
			ClientID: clientID,
			APIBase:  opt.APIBase,
			Meta:     meta,
			Redirect: lr.RedirectURI,
			AuthURL:  authURL,
		}, nil
	}

	if opt.WarnWriter != nil {
		opt.WarnWriter(FullAccountCapabilityWarning)
	}

	if err := opt.OpenBrowser(authURL); err != nil {
		// Non-fatal: user can open manually.
		if opt.WarnWriter != nil {
			opt.WarnWriter(fmt.Sprintf("Could not open browser automatically (%v).\nOpen this URL manually:\n%s\n", err, authURL))
		}
	} else if opt.WarnWriter != nil {
		opt.WarnWriter(fmt.Sprintf("Opened browser for authorization.\nIf nothing opened, visit:\n%s\nWaiting for callback on %s …\n", authURL, lr.RedirectURI))
	}

	cb, err := WaitCallback(lctx, lr, state)
	if err != nil {
		return nil, err
	}
	tok, err := Exchange(ctx, opt.HTTPClient, meta.TokenEndpoint, clientID, cb.Code, lr.RedirectURI, pkce.Verifier)
	if err != nil {
		return nil, err
	}
	return &BrowserLoginResult{
		Tokens:   tok,
		ClientID: clientID,
		APIBase:  opt.APIBase,
		Meta:     meta,
		Redirect: lr.RedirectURI,
		AuthURL:  authURL,
	}, nil
}

func openSystemBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		// Prefer xdg-open; fall back to common tools.
		if _, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command("xdg-open", rawURL)
		} else if _, err := exec.LookPath("gio"); err == nil {
			cmd = exec.Command("gio", "open", rawURL)
		} else {
			return fmt.Errorf("no browser opener found (xdg-open)")
		}
	}
	return cmd.Start()
}
