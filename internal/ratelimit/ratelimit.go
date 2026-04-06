package ratelimit

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RateLimiter controls concurrent access and per-window call volume to the
// Clockify API. A nil *RateLimiter is safe to use — Acquire returns a no-op
// release function and nil error.
type RateLimiter struct {
	semaphore     chan struct{}
	maxConcurrent int
	windowCount   atomic.Int64
	windowStart   atomic.Int64 // epoch millis
	maxPerWindow  int64
	windowMillis  int64
	windowMu      sync.Mutex // protects atomic window reset
}

// FromEnv builds a RateLimiter from environment variables.
//
//	CLOCKIFY_MAX_CONCURRENT  – default 10 (0 disables concurrency limit)
//	CLOCKIFY_RATE_LIMIT      – default 120 (0 disables window limit)
//
// Returns nil if both values are 0 (rate limiting fully disabled).
func FromEnv() *RateLimiter {
	maxConc := envInt("CLOCKIFY_MAX_CONCURRENT", 10)
	maxWin := envInt("CLOCKIFY_RATE_LIMIT", 120)
	if maxConc == 0 && maxWin == 0 {
		return nil
	}
	return New(maxConc, int64(maxWin), 60000)
}

// New creates a RateLimiter with explicit parameters.
//
//	maxConcurrent – buffered-channel capacity (simultaneous in-flight calls)
//	maxPerWindow  – maximum calls allowed within each rolling window
//	windowMillis  – window length in milliseconds
func New(maxConcurrent int, maxPerWindow int64, windowMillis int64) *RateLimiter {
	var semaphore chan struct{}
	if maxConcurrent > 0 {
		semaphore = make(chan struct{}, maxConcurrent)
	}
	rl := &RateLimiter{
		semaphore:     semaphore,
		maxConcurrent: maxConcurrent,
		maxPerWindow:  maxPerWindow,
		windowMillis:  windowMillis,
	}
	rl.windowStart.Store(time.Now().UnixMilli())
	return rl
}

// Acquire reserves a slot. The returned function must be called to release
// the slot when work is done. Returns an error if the concurrency or window
// limit would be exceeded.
func (rl *RateLimiter) Acquire(ctx context.Context) (func(), error) {
	if rl == nil {
		return func() {}, nil
	}

	// Atomically reset the window if it has expired.
	// Using a mutex to prevent the race where two goroutines both
	// see an expired window and both reset, losing one's increment.
	if rl.maxPerWindow > 0 {
		now := time.Now().UnixMilli()
		if now-rl.windowStart.Load() > rl.windowMillis {
			rl.windowMu.Lock()
			// Double-check after acquiring the lock.
			if time.Now().UnixMilli()-rl.windowStart.Load() > rl.windowMillis {
				rl.windowStart.Store(time.Now().UnixMilli())
				rl.windowCount.Store(0)
			}
			rl.windowMu.Unlock()
		}

		// Pre-check: bail early when the window is already exhausted.
		if rl.windowCount.Load() >= rl.maxPerWindow {
			return nil, fmt.Errorf("rate limit exceeded: %d calls in %ds window",
				rl.maxPerWindow, rl.windowMillis/1000)
		}
	}

	// Try to acquire a concurrency slot.
	if rl.maxConcurrent > 0 {
		select {
		case rl.semaphore <- struct{}{}:
			// Slot acquired.
		case <-time.After(100 * time.Millisecond):
			return nil, fmt.Errorf("concurrency limit exceeded: %d concurrent calls",
				rl.maxConcurrent)
		}
	}

	// Increment the window counter and verify we haven't raced past the cap.
	if rl.maxPerWindow > 0 {
		cnt := rl.windowCount.Add(1)
		if cnt > rl.maxPerWindow {
			if rl.maxConcurrent > 0 {
				<-rl.semaphore // release slot — this call won't proceed
			}
			return nil, fmt.Errorf("rate limit exceeded: %d calls in %ds window",
				rl.maxPerWindow, rl.windowMillis/1000)
		}
	}

	return func() {
		if rl.maxConcurrent > 0 {
			<-rl.semaphore
		}
	}, nil
}

// String returns a human-readable description of the limiter's configuration.
func (rl *RateLimiter) String() string {
	if rl == nil {
		return "RateLimiter{disabled}"
	}
	return fmt.Sprintf("RateLimiter{concurrent:%d, window:%d/%ds}",
		rl.maxConcurrent, rl.maxPerWindow, rl.windowMillis/1000)
}

// envInt reads an environment variable as an int, falling back to def.
func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
