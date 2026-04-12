package ratelimit

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestFromEnvDefaults(t *testing.T) {
	// Clear any env that might be set.
	os.Unsetenv("CLOCKIFY_MAX_CONCURRENT")
	os.Unsetenv("CLOCKIFY_RATE_LIMIT")

	rl := FromEnv()
	if rl == nil {
		t.Fatal("expected non-nil RateLimiter with default env")
	}
	if rl.maxConcurrent != 10 {
		t.Errorf("maxConcurrent = %d; want 10", rl.maxConcurrent)
	}
	if rl.maxPerWindow != 120 {
		t.Errorf("maxPerWindow = %d; want 120", rl.maxPerWindow)
	}
	if rl.acquireTimeout != DefaultAcquireTimeout {
		t.Errorf("acquireTimeout = %v; want %v", rl.acquireTimeout, DefaultAcquireTimeout)
	}
}

func TestFromEnvWithAcquireTimeoutRespectsOverride(t *testing.T) {
	os.Unsetenv("CLOCKIFY_MAX_CONCURRENT")
	os.Unsetenv("CLOCKIFY_RATE_LIMIT")

	rl := FromEnvWithAcquireTimeout(250 * time.Millisecond)
	if rl == nil {
		t.Fatal("expected non-nil RateLimiter with default env")
	}
	if rl.acquireTimeout != 250*time.Millisecond {
		t.Fatalf("acquireTimeout = %v; want 250ms", rl.acquireTimeout)
	}
}

func TestAcquireAndRelease(t *testing.T) {
	rl := New(2, 100, 60000)

	release, err := rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if release == nil {
		t.Fatal("release function is nil")
	}
	release()
}

func TestLimitErrorHelpers(t *testing.T) {
	concurrencyErr := &ConcurrencyLimitError{
		MaxConcurrent: 1,
		Cause:         context.DeadlineExceeded,
	}
	if !errors.Is(concurrencyErr, ErrConcurrencyLimitExceeded) {
		t.Fatal("expected concurrency error to match sentinel")
	}
	if !errors.Is(concurrencyErr, context.DeadlineExceeded) {
		t.Fatal("expected concurrency error to unwrap context cause")
	}
	if concurrencyErr.Unwrap() != context.DeadlineExceeded {
		t.Fatalf("unexpected unwrap cause: %v", concurrencyErr.Unwrap())
	}
	if got := concurrencyErr.Error(); got == "" {
		t.Fatal("expected non-empty concurrency error string")
	}

	windowErr := &RateLimitError{MaxPerWindow: 5, WindowMillis: 60000}
	if !errors.Is(windowErr, ErrRateLimitExceeded) {
		t.Fatal("expected rate-limit error to match sentinel")
	}
	if got := windowErr.Error(); got != "rate limit exceeded: 5 calls in 60s window" {
		t.Fatalf("unexpected window error string: %q", got)
	}
}

func TestConcurrencyLimit(t *testing.T) {
	const cap = 2
	rl := New(cap, 1000, 60000)

	// Fill all slots.
	releases := make([]func(), cap)
	for i := 0; i < cap; i++ {
		rel, err := rl.Acquire(context.Background())
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
		releases[i] = rel
	}

	// Next acquire should timeout on the semaphore.
	_, err := rl.Acquire(context.Background())
	if err == nil {
		t.Fatal("expected concurrency-limit error, got nil")
	}
	if !errors.Is(err, ErrConcurrencyLimitExceeded) {
		t.Fatalf("expected ErrConcurrencyLimitExceeded, got %v", err)
	}
	if got := err.Error(); got != "concurrency limit exceeded: 2 concurrent calls" {
		t.Errorf("unexpected error: %s", got)
	}

	// Cleanup.
	for _, rel := range releases {
		rel()
	}
}

func TestWindowLimit(t *testing.T) {
	rl := New(10, 3, 60000) // only 3 calls per window

	for i := 0; i < 3; i++ {
		rel, err := rl.Acquire(context.Background())
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
		rel()
	}

	// 4th should fail on window limit.
	_, err := rl.Acquire(context.Background())
	if err == nil {
		t.Fatal("expected window-limit error, got nil")
	}
	if !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("expected ErrRateLimitExceeded, got %v", err)
	}
}

func TestWindowReset(t *testing.T) {
	rl := New(10, 2, 50) // 50ms window, 2 calls

	for i := 0; i < 2; i++ {
		rel, err := rl.Acquire(context.Background())
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
		rel()
	}

	// Window exhausted — wait for it to expire.
	time.Sleep(60 * time.Millisecond)

	rel, err := rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("expected window reset to allow Acquire: %v", err)
	}
	rel()
}

func TestNilRateLimiter(t *testing.T) {
	var rl *RateLimiter

	release, err := rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("nil receiver Acquire should not error: %v", err)
	}
	if release == nil {
		t.Fatal("expected non-nil no-op release")
	}
	// Should not panic.
	release()
}

func TestFailedConcurrencyDoesntBurnWindow(t *testing.T) {
	rl := New(1, 100, 60000) // 1 concurrent slot, generous window

	// Occupy the single slot.
	rel, err := rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}

	before := rl.windowCount.Load()

	// This should fail on concurrency, not touch the window counter.
	_, err = rl.Acquire(context.Background())
	if err == nil {
		t.Fatal("expected concurrency-limit error")
	}
	if !errors.Is(err, ErrConcurrencyLimitExceeded) {
		t.Fatalf("expected ErrConcurrencyLimitExceeded, got %v", err)
	}

	after := rl.windowCount.Load()
	if after != before {
		t.Errorf("windowCount changed from %d to %d on failed concurrency acquire", before, after)
	}

	rel()
}

func TestZeroConcurrentDisablesConcurrencyLayer(t *testing.T) {
	rl := New(0, 3, 60000)

	release1, err := rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	release2, err := rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}

	release1()
	release2()
}

func TestZeroWindowDisablesWindowLayer(t *testing.T) {
	rl := New(1, 0, 60000)

	release, err := rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	release()

	release, err = rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("second acquire failed with window disabled: %v", err)
	}
	release()
}

func TestConcurrentStress(t *testing.T) {
	const (
		maxConcurrent = 5
		goroutines    = 50
		perWindow     = 200
	)
	rl := New(maxConcurrent, int64(perWindow), 60000)

	errCh := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			rel, err := rl.Acquire(context.Background())
			if err != nil {
				errCh <- err
				return
			}
			time.Sleep(1 * time.Millisecond) // brief hold
			rel()
			errCh <- nil
		}()
	}

	var succeeded, rateLimited int
	for i := 0; i < goroutines; i++ {
		err := <-errCh
		if err == nil {
			succeeded++
		} else {
			rateLimited++
		}
	}

	// At least maxConcurrent should succeed; some may be rate-limited due to semaphore timeout
	if succeeded < maxConcurrent {
		t.Errorf("expected at least %d successes, got %d", maxConcurrent, succeeded)
	}
	t.Logf("stress: %d succeeded, %d rate-limited", succeeded, rateLimited)
}

func TestAcquireRespectsContextCancellation(t *testing.T) {
	rl := New(1, 100, 60000) // 1 concurrent slot

	// Occupy the single slot.
	rel, err := rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	defer rel()

	// Create a context that cancels after 20ms.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = rl.Acquire(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, ErrConcurrencyLimitExceeded) {
		t.Fatalf("expected ErrConcurrencyLimitExceeded, got %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected wrapped context deadline exceeded, got %v", err)
	}
	// Should return before the 100ms semaphore timeout.
	if elapsed >= 80*time.Millisecond {
		t.Errorf("expected cancellation before 80ms, took %v", elapsed)
	}
}

func TestAcquireHonorsConfiguredTimeout(t *testing.T) {
	rl := NewWithAcquireTimeout(1, 100, 60000, 20*time.Millisecond)

	rel, err := rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	defer rel()

	start := time.Now()
	_, err = rl.Acquire(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, ErrConcurrencyLimitExceeded) {
		t.Fatalf("expected ErrConcurrencyLimitExceeded, got %v", err)
	}
	if elapsed < 15*time.Millisecond || elapsed > 100*time.Millisecond {
		t.Fatalf("expected configured timeout around 20ms, got %v", elapsed)
	}
}

func TestNewWithAcquireTimeoutDefaultsZero(t *testing.T) {
	rl := NewWithAcquireTimeout(1, 1, 60000, 0)
	if rl.acquireTimeout != DefaultAcquireTimeout {
		t.Fatalf("expected default acquire timeout, got %v", rl.acquireTimeout)
	}
}

func TestStatsAndString(t *testing.T) {
	rl := NewWithAcquireTimeout(2, 3, 60000, 250*time.Millisecond)

	rel, err := rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer rel()

	stats := rl.Stats()
	if stats.Concurrent != 1 {
		t.Fatalf("Concurrent = %d; want 1", stats.Concurrent)
	}
	if stats.MaxConcurrent != 2 || stats.MaxPerWindow != 3 || stats.WindowMillis != 60000 {
		t.Fatalf("unexpected stats snapshot: %+v", stats)
	}

	if got := rl.String(); got != "RateLimiter{concurrent:2, window:3/60s}" {
		t.Fatalf("unexpected string: %q", got)
	}
	if got := ((*RateLimiter)(nil)).String(); got != "RateLimiter{disabled}" {
		t.Fatalf("unexpected nil string: %q", got)
	}
	if stats := ((*RateLimiter)(nil)).Stats(); stats != (Stats{}) {
		t.Fatalf("expected zero stats for nil limiter, got %+v", stats)
	}
}

func TestEnvInt(t *testing.T) {
	t.Setenv("CLOCKIFY_RATE_LIMIT", "250")
	if got := envInt("CLOCKIFY_RATE_LIMIT", 120); got != 250 {
		t.Fatalf("envInt parsed value = %d; want 250", got)
	}

	t.Setenv("CLOCKIFY_RATE_LIMIT", "bad")
	if got := envInt("CLOCKIFY_RATE_LIMIT", 120); got != 120 {
		t.Fatalf("envInt fallback = %d; want 120", got)
	}
}
