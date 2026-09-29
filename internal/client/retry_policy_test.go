package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithRetryPolicyBoundsAttemptsAndDelay(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	var sleeps []time.Duration
	defer SetRetrySleepForTest(func(ctx context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil })()
	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL, UserAgent: "t"}

	ctx := WithRetryPolicy(context.Background(), 2, time.Second)
	if _, err := c.Do(ctx, "GET", "/x", nil, nil, nil); err == nil {
		t.Fatal("expected error")
	}
	if hits != 2 || len(sleeps) != 1 || sleeps[0] != time.Second {
		t.Fatalf("hits=%d sleeps=%v", hits, sleeps)
	}

	// Without a policy the default (4 attempts, Retry-After capped at 30s) applies.
	atomic.StoreInt32(&hits, 0)
	sleeps = nil
	_, _ = c.Do(context.Background(), "GET", "/x", nil, nil, nil)
	if hits != 4 || len(sleeps) != 3 || sleeps[0] != 30*time.Second {
		t.Fatalf("default hits=%d sleeps=%v", hits, sleeps)
	}
}
