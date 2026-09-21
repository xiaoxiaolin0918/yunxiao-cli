package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsTransientNetworkError(t *testing.T) {
	if !IsTransientNetworkError(fmt.Errorf("request failed: Post x: wsarecv: An existing connection was forcibly closed by the remote host.")) {
		t.Fatal("wsarecv")
	}
	if !IsTransientNetworkError(&net.OpError{Op: "read", Err: errors.New("connection reset by peer")}) {
		t.Fatal("OpError")
	}
	if IsTransientNetworkError(&APIError{Status: 400, Body: "no"}) {
		t.Fatal("APIError must not count as network")
	}
	if IsTransientNetworkError(nil) {
		t.Fatal("nil")
	}
}

func TestAnnotateWriteNetworkError(t *testing.T) {
	base := fmt.Errorf("request failed: connection reset")
	got := AnnotateWriteNetworkError(base, "POST", "yunxiao codeup mrs list --search t")
	if got == nil || !strings.Contains(got.Error(), "check before retry:") || !strings.Contains(got.Error(), "mrs list") {
		t.Fatalf("%v", got)
	}
	if AnnotateWriteNetworkError(base, "GET", "x") != base {
		t.Fatal("GET should not annotate")
	}
}

type failThenOKTransport struct {
	failsLeft atomic.Int32
	ok        http.RoundTripper
}

func (f *failThenOKTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if f.failsLeft.Add(-1) >= 0 {
		return nil, &net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset by peer")}
	}
	return f.ok.RoundTrip(r)
}

func TestGetRetriesTransientNetworkThenSucceeds(t *testing.T) {
	restore := SetRetrySleepForTest(func(ctx context.Context, d time.Duration) error { return nil })
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	tr := &failThenOKTransport{ok: srv.Client().Transport}
	tr.failsLeft.Store(2) // fail twice, then succeed
	c := &Client{
		HTTP:      &http.Client{Transport: tr, Timeout: 5 * time.Second},
		BaseURL:   srv.URL,
		Token:     "t",
		UserAgent: "test",
	}
	var out map[string]any
	if err := c.Get(context.Background(), "/ok", nil, &out); err != nil {
		t.Fatal(err)
	}
	if out["ok"] != true {
		t.Fatalf("%v", out)
	}
}

func TestPostNetworkErrorAnnotated(t *testing.T) {
	c := &Client{
		HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, &net.OpError{Op: "write", Net: "tcp", Err: errors.New("connection reset by peer")}
		}), Timeout: 5 * time.Second},
		BaseURL:   "http://127.0.0.1:1",
		Token:     "t",
		UserAgent: "test",
	}
	err := c.Post(context.Background(), "/x", map[string]any{"a": 1}, nil)
	if err == nil || !strings.Contains(err.Error(), "check before retry:") {
		t.Fatalf("want annotated network error, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAnnotateWriteNetworkError_ReplacesPriorHint(t *testing.T) {
	base := fmt.Errorf("request failed: connection reset")
	generic := AnnotateWriteNetworkError(base, "POST", "")
	specific := AnnotateWriteNetworkError(generic, "POST", `yunxiao codeup mrs list --repo r --search "t"`)
	msg := specific.Error()
	if strings.Count(msg, writeNetworkHintPrefix) != 1 {
		t.Fatalf("want one hint line, got %q", msg)
	}
	if !strings.Contains(msg, "mrs list") || strings.Contains(msg, "search existing resources") {
		t.Fatalf("want specific hint only: %q", msg)
	}
}
