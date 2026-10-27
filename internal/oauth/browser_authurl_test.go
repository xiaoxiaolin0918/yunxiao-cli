package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// startBrowserLoginTestServer serves discover + DCR for BrowserLogin unit tests.
func startBrowserLoginTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Metadata{
			Issuer:                "https://example.test",
			AuthorizationEndpoint: "https://account.example.test/authorize",
			TokenEndpoint:         "http://" + r.Host + "/v1/oauth2/token",
			RegistrationEndpoint:  "http://" + r.Host + "/v1/oauth2/register",
		})
	})
	mux.HandleFunc("/v1/oauth2/register", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(RegisterResponse{ClientID: "cid-browser-test"})
	})
	mux.HandleFunc("/v1/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "oat-access", RefreshToken: "ort-refresh", TokenType: "Bearer", ExpiresIn: 3600,
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func assertBareHTTPSURL(t *testing.T, got string) {
	t.Helper()
	got = strings.TrimSpace(got)
	if !strings.HasPrefix(got, "https://") {
		t.Fatalf("want bare https URL, got %q", got)
	}
	if strings.ContainsAny(got, "\"'<>") {
		t.Fatalf("URL must not be quote/angle wrapped: %q", got)
	}
	if strings.Contains(got, `\u0026`) {
		t.Fatalf("URL must not JSON-escape &: %q", got)
	}
	if !strings.Contains(got, "code_challenge") {
		t.Fatalf("auth URL missing PKCE params: %q", got)
	}
}

// A1: OnAuthURL fires with a bare https URL as soon as the auth URL is ready
// (before OpenBrowser / OnDryRun).
func TestBrowserLogin_OnAuthURLBareHTTPS(t *testing.T) {
	srv := startBrowserLoginTestServer(t)

	var gotURL string
	var onAuthURLAt time.Time
	var openAt time.Time
	var dryRunAt time.Time
	var mu sync.Mutex

	_, err := BrowserLogin(context.Background(), BrowserLoginOptions{
		APIBase:    srv.URL,
		HTTPClient: srv.Client(),
		DryRun:     true,
		OpenBrowser: func(u string) error {
			mu.Lock()
			openAt = time.Now()
			mu.Unlock()
			return nil
		},
		OnAuthURL: func(u string) {
			mu.Lock()
			gotURL = u
			onAuthURLAt = time.Now()
			mu.Unlock()
		},
		OnDryRun: func(authURL, redirectURI, clientID string) error {
			mu.Lock()
			dryRunAt = time.Now()
			mu.Unlock()
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	assertBareHTTPSURL(t, gotURL)
	if onAuthURLAt.IsZero() {
		t.Fatal("OnAuthURL was not called")
	}
	if dryRunAt.IsZero() {
		t.Fatal("OnDryRun was not called")
	}
	if onAuthURLAt.After(dryRunAt) {
		t.Fatalf("OnAuthURL must run before OnDryRun")
	}
	if !openAt.IsZero() {
		t.Fatal("OpenBrowser must not run on dry-run")
	}
}

// A2: non-dry-run stays alive after OnAuthURL (reaches OpenBrowser + WaitCallback).
func TestBrowserLogin_WaitsAfterOnAuthURL(t *testing.T) {
	srv := startBrowserLoginTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var gotURL string
	opened := make(chan struct{}, 1)
	onAuthDone := make(chan struct{}, 1)

	errCh := make(chan error, 1)
	go func() {
		_, err := BrowserLogin(ctx, BrowserLoginOptions{
			APIBase:    srv.URL,
			HTTPClient: srv.Client(),
			Timeout:    30 * time.Second,
			OpenBrowser: func(u string) error {
				opened <- struct{}{}
				return nil
			},
			OnAuthURL: func(u string) {
				gotURL = u
				onAuthDone <- struct{}{}
			},
		})
		errCh <- err
	}()

	select {
	case <-onAuthDone:
	case <-time.After(5 * time.Second):
		t.Fatal("OnAuthURL not called within 5s")
	}
	assertBareHTTPSURL(t, gotURL)

	select {
	case <-opened:
	case <-time.After(5 * time.Second):
		t.Fatal("OpenBrowser not reached after OnAuthURL — process would have exited early")
	}

	// Still waiting on callback: cancel to unblock.
	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected wait/cancel error, got nil (would mean early exit after URL print)")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("BrowserLogin did not return after cancel")
	}
}