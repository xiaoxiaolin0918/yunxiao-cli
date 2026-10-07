package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// #117: gateway returns 2xx HTML (landing/auth page) for a wrong path — e.g.
// an MSYS-mangled one. The decode failure must carry the final URL, status,
// content-type and a short body preview instead of a wall of escaped HTML.
func TestDo2xxHTMLReturnsDecodeError(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><html lang=\"zh\">" + strings.Repeat("marketing ", 1000)))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL, Token: "tok", UserAgent: "t"}
	var out map[string]any
	err := c.Get(context.Background(), "/C:/Program Files/Git/oapi/v1/platform/user", nil, &out)
	de, ok := err.(*DecodeError)
	if !ok {
		t.Fatalf("want DecodeError, got %T %v", err, err)
	}
	if gotPath != "/C:/Program Files/Git/oapi/v1/platform/user" {
		t.Fatalf("server saw path %q", gotPath)
	}
	if de.Status != 200 {
		t.Fatalf("status=%d", de.Status)
	}
	if !strings.Contains(de.ContentType, "text/html") {
		t.Fatalf("content-type=%q", de.ContentType)
	}
	if !strings.HasPrefix(de.URL, srv.URL+"/C:/") {
		t.Fatalf("url=%q", de.URL)
	}
	if !strings.HasPrefix(de.Body, "<!doctype html>") {
		t.Fatalf("body preview=%q", de.Body[:40])
	}
	if len(de.Body) > decodeBodyPreview+3 { // preview + "…"
		t.Fatalf("body preview must be truncated, len=%d", len(de.Body))
	}
	msg := de.Error()
	for _, want := range []string{de.URL, "HTTP 200", "text/html", "decode response"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q: %s", want, msg)
		}
	}
}

// 2xx but not valid JSON and not HTML: still a DecodeError (same diagnostics).
func TestDo2xxNonJSONNonHTMLReturnsDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("gateway says no"))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL, Token: "tok", UserAgent: "t"}
	var out map[string]any
	err := c.Get(context.Background(), "/x", nil, &out)
	de, ok := err.(*DecodeError)
	if !ok {
		t.Fatalf("want DecodeError, got %T %v", err, err)
	}
	if de.Status != 200 || de.ContentType != "text/plain" || de.Body != "gateway says no" {
		t.Fatalf("%+v", de)
	}
}

// *string out keeps the raw passthrough (contract used by some commands).
func TestDo2xxHTMLStringOutPassthrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>raw</html>"))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL, Token: "tok", UserAgent: "t"}
	var s string
	if err := c.Get(context.Background(), "/x", nil, &s); err != nil {
		t.Fatal(err)
	}
	if s != "<html>raw</html>" {
		t.Fatalf("s=%q", s)
	}
}

// Non-2xx HTML bodies collapse to a short preview in APIError (#117).
func TestDo4xxHTMLBodyTruncatedForDisplay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("<!doctype html><html>" + strings.Repeat("x", 5000)))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL, Token: "tok", UserAgent: "t"}
	err := c.Get(context.Background(), "/oapi/v1/nowhere", nil, nil)
	ae, ok := err.(*APIError)
	if !ok || ae.Status != 404 {
		t.Fatalf("%T %v", err, err)
	}
	if len(ae.Body) > htmlBodyPreview+3 { // preview + "…"
		t.Fatalf("HTML body must be truncated, len=%d", len(ae.Body))
	}
	if !strings.HasPrefix(ae.Body, "<!doctype html>") {
		t.Fatalf("body=%q", ae.Body)
	}
	if !strings.Contains(ae.Error(), "HTTP 404") || !strings.Contains(ae.Error(), srv.URL) {
		t.Fatalf("error=%s", ae.Error())
	}
}

// JSON error bodies keep the wider 2000-char budget (yaml validation parses them).
func TestDo4xxJSONBodyNotCollapsed(t *testing.T) {
	longJSON := `{"errorMsg":"` + strings.Repeat("d", 800) + `"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(longJSON))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL, Token: "tok", UserAgent: "t"}
	err := c.Get(context.Background(), "/x", nil, nil)
	ae, ok := err.(*APIError)
	if !ok || ae.Status != 400 {
		t.Fatalf("%T %v", err, err)
	}
	if len(ae.Body) != len(longJSON) {
		t.Fatalf("JSON body must not be HTML-truncated: %d != %d", len(ae.Body), len(longJSON))
	}
}
