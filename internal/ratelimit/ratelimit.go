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
	//
	// subjectsMu is an RWMutex so the warm-path lookup (existing subject,
	// every call after the first) only takes a read lock. The prior plain
	// Mutex serialised every AcquireForSubject through a single critical
	// section even though 99% of calls just read the map. Creation of a
	// new subject entry still requires the write lock.
	perTokenMaxConcurrent int
	perTokenMaxPerWindow  int64
	subjectsMu            sync.RWMutex
	subjects              map[string]*subjectLimiter
}

// subjectLimiter tracks one Principal.Subject's window counter and
// concurrency semaphore. Fields mirror the global RateLimiter shape.
//
// lastSeenMillis is the monotonic (wall-clock) epoch millis at which
// this subject last issued an acquire. ReapIdleSubjects uses it to
// evict entries idle past the configured TTL so the subjects map does
// not grow unbounded in long-lived multi-tenant deployments.
type subjectLimiter struct {
	semaphore      chan struct{}
	windowCount    atomic.Int64
	windowStart    atomic.Int64
	lastSeenMillis atomic.Int64
	maxPerWindow   int64
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

// SetPerTokenLimits configures the per-subject sub-layer from an
// outside-the-constructor site. FromEnv applies the same values by
// reading CLOCKIFY_PER_TOKEN_{CONCURRENCY,RATE_LIMIT}; this setter
// exists for programmatic consumers (notably the W2-09 load harness
// at tests/load/) that need to drive the rate limiter without going
// through environment variables. Call before any Acquire* so the
// config is immutable during the call path. Setting either cap to
// 0 disables that dimension of the per-subject layer.
func (rl *RateLimiter) SetPerTokenLimits(maxConcurrent int, maxPerWindow int64) {
	if rl == nil {
		return
	}
	rl.perTokenMaxConcurrent = maxConcurrent
	rl.perTokenMaxPerWindow = maxPerWindow
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
	nowMillis := time.Now().UnixMilli()
	// Warm path: RLock-protected lookup. Skips the creation branch on
	// every call after the first per subject, which is >99% of traffic
	// in steady state. Measured via BenchmarkAcquireForSubjectSteady.
	rl.subjectsMu.RLock()
	if existing, ok := rl.subjects[subject]; ok {
		rl.subjectsMu.RUnlock()
		existing.lastSeenMillis.Store(nowMillis)
		return existing
	}
	rl.subjectsMu.RUnlock()

	// Cold path: create the entry under a write lock, double-checking
	// under the lock in case another goroutine raced us between the
	// RUnlock and Lock.
	rl.subjectsMu.Lock()
	defer rl.subjectsMu.Unlock()
	if rl.subjects == nil {
		rl.subjects = map[string]*subjectLimiter{}
	}
	if existing, ok := rl.subjects[subject]; ok {
		existing.lastSeenMillis.Store(nowMillis)
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
	sub.windowStart.Store(nowMillis)
	sub.lastSeenMillis.Store(nowMillis)
	rl.subjects[subject] = sub
	return sub
}

// ReapIdleSubjects removes per-subject entries whose last acquire
// happened before now.Add(-maxIdle). Returns the number of entries
// evicted. Exposed for tests; production deployments start the
// background reaper via StartSubjectReaper.
//
// An entry that is mid-acquire (holds a semaphore slot) is skipped —
// dropping it would strand the outstanding release. Callers with a
// stuck subject should tune acquireTimeout rather than rely on reap.
func (rl *RateLimiter) ReapIdleSubjects(now time.Time, maxIdle time.Duration) int {
	if rl == nil || maxIdle <= 0 {
		return 0
	}
	cutoff := now.Add(-maxIdle).UnixMilli()
	rl.subjectsMu.Lock()
	defer rl.subjectsMu.Unlock()
	evicted := 0
	for name, sub := range rl.subjects {
		if sub.lastSeenMillis.Load() > cutoff {
			continue
		}
		if len(sub.semaphore) > 0 {
			continue
		}
		delete(rl.subjects, name)
		evicted++
	}
	return evicted
}

// StartSubjectReaper runs a background goroutine that reaps idle
// subject limiters every `interval`, evicting any entry untouched for
// longer than `maxIdle`. It returns immediately; the goroutine exits
// when ctx is done. Nil receiver is a no-op.
func (rl *RateLimiter) StartSubjectReaper(ctx context.Context, interval, maxIdle time.Duration) {
	if rl == nil || interval <= 0 || maxIdle <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				rl.ReapIdleSubjects(now, maxIdle)
			}
		}
	}()
}

// SubjectCount returns the current number of tracked per-subject
// limiters. Exposed for metrics and tests; O(1) under the read lock.
func (rl *RateLimiter) SubjectCount() int {
	if rl == nil {
		return 0
	}
	rl.subjectsMu.RLock()
	defer rl.subjectsMu.RUnlock()
	return len(rl.subjects)
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
