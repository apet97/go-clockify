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

	// Per-subject sub-limiters, created lazily on first AcquireForSubject.
	// Each subject gets its own window counter + concurrency semaphore so a
	// noisy tenant cannot monopolise the global budget.
	perTokenMaxConcurrent int
	perTokenMaxPerWindow  int64
	subjectsMu            sync.Mutex
	subjects              map[string]*subjectLimiter
}

// subjectLimiter tracks one Principal.Subject's window counter and
// concurrency semaphore. Fields mirror the global RateLimiter shape.
type subjectLimiter struct {
	semaphore    chan struct{}
	windowCount  atomic.Int64
	windowStart  atomic.Int64
	maxPerWindow int64
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
	perTokenConc := envInt("CLOCKIFY_PER_TOKEN_CONCURRENCY", 5)
	perTokenWin := envInt("CLOCKIFY_PER_TOKEN_RATE_LIMIT", 60)
	if maxConc == 0 && maxWin == 0 && perTokenConc == 0 && perTokenWin == 0 {
		return nil
	}
	rl := NewWithAcquireTimeout(maxConc, int64(maxWin), 60000, acquireTimeout)
	if rl != nil {
		rl.perTokenMaxConcurrent = perTokenConc
		rl.perTokenMaxPerWindow = int64(perTokenWin)
	}
	return rl
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

// PerTokenScope describes the sub-layer an AcquireForSubject rejection
// originated from so callers (enforcement.Pipeline, metrics) can label
// rejections consistently without inspecting error strings.
type PerTokenScope string

const (
	ScopeGlobal     PerTokenScope = "global"
	ScopePerToken   PerTokenScope = "per_token"
	ScopeUnknown    PerTokenScope = "unknown"
	ScopeConcurrent               = "concurrency"
)

// AcquireForSubject extends Acquire with an optional per-subject layer:
// first it runs the full global acquire path (semaphore + window), then it
// also checks a subject-scoped sub-limiter so a single authenticated client
// cannot monopolise the global budget. An empty subject skips the per-token
// layer entirely (back-compat with unauthenticated paths).
//
// When the per-token layer rejects, AcquireForSubject releases the global
// slot it already acquired before returning, so the global budget is never
// stranded on a per-token failure. The returned PerTokenScope identifies
// which layer rejected.
func (rl *RateLimiter) AcquireForSubject(ctx context.Context, subject string) (func(), PerTokenScope, error) {
	release, err := rl.Acquire(ctx)
	if err != nil {
		scope := ScopeGlobal
		if errors.Is(err, ErrConcurrencyLimitExceeded) {
			scope = ScopeGlobal
		}
		return nil, scope, err
	}
	if rl == nil || subject == "" || (rl.perTokenMaxConcurrent <= 0 && rl.perTokenMaxPerWindow <= 0) {
		return release, ScopeGlobal, nil
	}

	sub := rl.subjectLimiterFor(subject)
	subRelease, err := sub.acquire(ctx, rl.windowMillis, rl.acquireTimeout)
	if err != nil {
		if release != nil {
			release()
		}
		return nil, ScopePerToken, err
	}
	combined := func() {
		subRelease()
		if release != nil {
			release()
		}
	}
	return combined, ScopePerToken, nil
}

func (rl *RateLimiter) subjectLimiterFor(subject string) *subjectLimiter {
	rl.subjectsMu.Lock()
	defer rl.subjectsMu.Unlock()
	if rl.subjects == nil {
		rl.subjects = map[string]*subjectLimiter{}
	}
	if existing, ok := rl.subjects[subject]; ok {
		return existing
	}
	var sem chan struct{}
	if rl.perTokenMaxConcurrent > 0 {
		sem = make(chan struct{}, rl.perTokenMaxConcurrent)
	}
	sub := &subjectLimiter{
		semaphore:    sem,
		maxPerWindow: rl.perTokenMaxPerWindow,
	}
	sub.windowStart.Store(time.Now().UnixMilli())
	rl.subjects[subject] = sub
	return sub
}

// acquire is the per-subject equivalent of RateLimiter.Acquire with the same
// window/concurrency contract.
func (sl *subjectLimiter) acquire(ctx context.Context, windowMillis int64, acquireTimeout time.Duration) (func(), error) {
	if sl.maxPerWindow > 0 {
		now := time.Now().UnixMilli()
		if now-sl.windowStart.Load() > windowMillis {
			sl.windowStart.Store(now)
			sl.windowCount.Store(0)
		}
		if sl.windowCount.Load() >= sl.maxPerWindow {
			return nil, &RateLimitError{MaxPerWindow: sl.maxPerWindow, WindowMillis: windowMillis}
		}
	}
	if sl.semaphore != nil {
		select {
		case sl.semaphore <- struct{}{}:
		case <-ctx.Done():
			return nil, &ConcurrencyLimitError{MaxConcurrent: cap(sl.semaphore), Cause: ctx.Err()}
		case <-time.After(acquireTimeout):
			return nil, &ConcurrencyLimitError{MaxConcurrent: cap(sl.semaphore)}
		}
	}
	if sl.maxPerWindow > 0 {
		cnt := sl.windowCount.Add(1)
		if cnt > sl.maxPerWindow {
			if sl.semaphore != nil {
				<-sl.semaphore
			}
			return nil, &RateLimitError{MaxPerWindow: sl.maxPerWindow, WindowMillis: windowMillis}
		}
	}
	return func() {
		if sl.semaphore != nil {
			<-sl.semaphore
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
