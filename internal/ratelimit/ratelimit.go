package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const DefaultAcquireTimeout = 100 * time.Millisecond

var (
	ErrConcurrencyLimitExceeded = errors.New("concurrency limit exceeded")
	ErrRateLimitExceeded        = errors.New("rate limit exceeded")
)

// RateLimiter controls concurrent access and fixed-window call volume to the
// Clockify API. A nil *RateLimiter is safe to use — Acquire returns a no-op
// release function and nil error.
type RateLimiter struct {
	semaphore      chan struct{}
	acquireTimeout time.Duration
	maxConcurrent  int
	windowCount    atomic.Int64
	windowStart    atomic.Int64 // start of current fixed window, epoch millis
	maxPerWindow   int64
	windowMillis   int64
	windowMu       sync.Mutex // protects atomic window reset
}

type ConcurrencyLimitError struct {
	MaxConcurrent int
	Cause         error
}

func (e *ConcurrencyLimitError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("concurrency limit: context cancelled: %v", e.Cause)
	}
	return fmt.Sprintf("concurrency limit exceeded: %d concurrent calls", e.MaxConcurrent)
}

func (e *ConcurrencyLimitError) Is(target error) bool {
	if target == ErrConcurrencyLimitExceeded {
		return true
	}
	return e.Cause != nil && errors.Is(e.Cause, target)
}

func (e *ConcurrencyLimitError) Unwrap() error {
	return e.Cause
}

type RateLimitError struct {
	MaxPerWindow int64
	WindowMillis int64
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded: %d calls in %ds window", e.MaxPerWindow, e.WindowMillis/1000)
}

func (e *RateLimitError) Is(target error) bool {
	return target == ErrRateLimitExceeded
}

// FromEnv builds a RateLimiter from environment variables.
//
//	CLOCKIFY_MAX_CONCURRENT  – default 10 (0 disables concurrency limit)
//	CLOCKIFY_RATE_LIMIT      – default 120 (0 disables fixed-window limit)
//
// Returns nil if both values are 0 (rate limiting fully disabled).
func FromEnv() *RateLimiter {
	return FromEnvWithAcquireTimeout(DefaultAcquireTimeout)
}

func FromEnvWithAcquireTimeout(acquireTimeout time.Duration) *RateLimiter {
	maxConc := envInt("CLOCKIFY_MAX_CONCURRENT", 10)
	maxWin := envInt("CLOCKIFY_RATE_LIMIT", 120)
	if maxConc == 0 && maxWin == 0 {
		return nil
	}
	return NewWithAcquireTimeout(maxConc, int64(maxWin), 60000, acquireTimeout)
}

// New creates a RateLimiter with explicit parameters.
//
//	maxConcurrent – buffered-channel capacity (simultaneous in-flight calls)
//	maxPerWindow  – maximum calls allowed within each fixed window
//	windowMillis  – fixed-window length in milliseconds
func New(maxConcurrent int, maxPerWindow int64, windowMillis int64) *RateLimiter {
	return NewWithAcquireTimeout(maxConcurrent, maxPerWindow, windowMillis, DefaultAcquireTimeout)
}

func NewWithAcquireTimeout(maxConcurrent int, maxPerWindow int64, windowMillis int64, acquireTimeout time.Duration) *RateLimiter {
	var semaphore chan struct{}
	if maxConcurrent > 0 {
		semaphore = make(chan struct{}, maxConcurrent)
	}
	if acquireTimeout <= 0 {
		acquireTimeout = DefaultAcquireTimeout
	}
	rl := &RateLimiter{
		semaphore:      semaphore,
		acquireTimeout: acquireTimeout,
		maxConcurrent:  maxConcurrent,
		maxPerWindow:   maxPerWindow,
		windowMillis:   windowMillis,
	}
	rl.windowStart.Store(time.Now().UnixMilli())
	return rl
}

// Acquire reserves a slot. The returned function must be called to release
// the slot when work is done. Returns an error if the concurrency or current
// fixed-window limit would be exceeded.
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
			return nil, &RateLimitError{MaxPerWindow: rl.maxPerWindow, WindowMillis: rl.windowMillis}
		}
	}

	// Try to acquire a concurrency slot.
	if rl.maxConcurrent > 0 {
		select {
		case rl.semaphore <- struct{}{}:
			// Slot acquired.
		case <-ctx.Done():
			return nil, &ConcurrencyLimitError{MaxConcurrent: rl.maxConcurrent, Cause: ctx.Err()}
		case <-time.After(rl.acquireTimeout):
			return nil, &ConcurrencyLimitError{MaxConcurrent: rl.maxConcurrent}
		}
	}

	// Increment the window counter and verify we haven't raced past the cap.
	if rl.maxPerWindow > 0 {
		cnt := rl.windowCount.Add(1)
		if cnt > rl.maxPerWindow {
			if rl.maxConcurrent > 0 {
				<-rl.semaphore // release slot — this call won't proceed
			}
			return nil, &RateLimitError{MaxPerWindow: rl.maxPerWindow, WindowMillis: rl.windowMillis}
		}
	}

	return func() {
		if rl.maxConcurrent > 0 {
			<-rl.semaphore
		}
	}, nil
}

// Stats describes a snapshot of the rate limiter's live counters. A zero
// Stats is returned for a nil receiver so callers do not need a nil guard.
type Stats struct {
	Concurrent    int
	MaxConcurrent int
	WindowCount   int64
	MaxPerWindow  int64
	WindowMillis  int64
}

// Stats returns a snapshot of the current limiter state.
func (rl *RateLimiter) Stats() Stats {
	if rl == nil {
		return Stats{}
	}
	concurrent := 0
	if rl.semaphore != nil {
		concurrent = len(rl.semaphore)
	}
	return Stats{
		Concurrent:    concurrent,
		MaxConcurrent: rl.maxConcurrent,
		WindowCount:   rl.windowCount.Load(),
		MaxPerWindow:  rl.maxPerWindow,
		WindowMillis:  rl.windowMillis,
	}
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
