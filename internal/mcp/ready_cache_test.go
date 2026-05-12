package mcp

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestCachedReadyChecker_MemoisesInsideWindow pins the core contract:
// successive calls within `ttl` must call the inner probe at most once.
// The streamable HTTP transport's /ready handler relies on this so a
// load balancer health-checking every 5 s does not amplify into 5 s
// upstream Clockify probes; the user-facing impact is that
// MCP_HTTP_BIND replicas no longer burn ~17 280 customer-quota API
// calls per replica per day.
//
// Drift check: invert the time.Since(lastAt) < ttl guard in
// transport_streamable_http.go to time.Since(lastAt) > ttl and this
// test fails with "expected 1 inner call within ttl window, got 10".
func TestCachedReadyChecker_MemoisesInsideWindow(t *testing.T) {
	var calls atomic.Int64
	inner := func(_ context.Context) error {
		calls.Add(1)
		return nil
	}
	cached := cachedReadyChecker(inner, 100*time.Millisecond)
	for i := range 10 {
		if err := cached(context.Background()); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 inner call within ttl window, got %d", got)
	}
}

// TestCachedReadyChecker_RefreshesAfterTTL proves the cache window
// closes: once `ttl` has elapsed, the next call re-invokes the inner
// probe. Without this, a one-time upstream failure would stick
// indefinitely.
func TestCachedReadyChecker_RefreshesAfterTTL(t *testing.T) {
	var calls atomic.Int64
	inner := func(_ context.Context) error {
		calls.Add(1)
		return nil
	}
	cached := cachedReadyChecker(inner, 50*time.Millisecond)
	if err := cached(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	if err := cached(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 inner calls after ttl expiry, got %d", got)
	}
}

// TestCachedReadyChecker_CachesErrorsToo confirms an upstream failure
// is also memoised inside the window so a temporary outage does not
// turn into a quota storm against a failing upstream.
func TestCachedReadyChecker_CachesErrorsToo(t *testing.T) {
	var calls atomic.Int64
	probeErr := errors.New("upstream 503")
	inner := func(_ context.Context) error {
		calls.Add(1)
		return probeErr
	}
	cached := cachedReadyChecker(inner, 100*time.Millisecond)
	for i := range 5 {
		err := cached(context.Background())
		if !errors.Is(err, probeErr) {
			t.Fatalf("iter %d: expected upstream-error sentinel, got %v", i, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 inner call (errors cached too), got %d", got)
	}
}

// TestCachedReadyChecker_NilPassthrough preserves the existing contract
// where a nil ReadyChecker means "no readiness probe" — wrap must not
// fabricate a probe that returns nil indiscriminately.
func TestCachedReadyChecker_NilPassthrough(t *testing.T) {
	if got := cachedReadyChecker(nil, time.Second); got != nil {
		t.Fatalf("expected nil for nil checker, got non-nil function (%p)", got)
	}
}
