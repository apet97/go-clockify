package ratelimit

import (
	"context"
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

	after := rl.windowCount.Load()
	if after != before {
		t.Errorf("windowCount changed from %d to %d on failed concurrency acquire", before, after)
	}

	rel()
}
