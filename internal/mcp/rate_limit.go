package mcp

import (
	"context"
	"sync"
	"time"
)

// tokenBucket is a simple per-process rate limiter for tool invocations.
type tokenBucket struct {
	mu         sync.Mutex
	capacity   float64
	tokens     float64
	refillRate float64 // tokens per second
	last       time.Time
}

// newTokenBucket returns a bucket allowing perMinute invocations per minute, or
// nil when perMinute <= 0 (rate limiting disabled).
func newTokenBucket(perMinute int) *tokenBucket {
	if perMinute <= 0 {
		return nil
	}
	return &tokenBucket{
		capacity:   float64(perMinute),
		tokens:     float64(perMinute),
		refillRate: float64(perMinute) / 60.0,
		last:       time.Now(),
	}
}

// allow consumes one token and reports whether the call may proceed. A nil
// bucket always allows.
func (b *tokenBucket) allow() bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.tokens += now.Sub(b.last).Seconds() * b.refillRate
	b.last = now
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// familyLimiter caps concurrent tool execution per risk family. Reads rely only
// on the global MaxInFlightToolCalls cap; writes and high-risk operations get
// tighter caps so they cannot all run at once.
type familyLimiter struct {
	write chan struct{} // ordinary writes: max 2 concurrent
	high  chan struct{} // destructive/billing/admin/permission/external: max 1
}

// newFamilyLimiter returns a limiter with the documented caps.
func newFamilyLimiter() *familyLimiter {
	return &familyLimiter{
		write: make(chan struct{}, 2),
		high:  make(chan struct{}, 1),
	}
}

// acquire blocks until a slot for the tool's risk family is free, or ctx is
// done. It returns a release func (always non-nil) and an error (non-nil only
// when ctx ended while waiting). Read-only tools acquire nothing.
func (l *familyLimiter) acquire(ctx context.Context, rc RiskClass) (func(), error) {
	if l == nil {
		return func() {}, nil
	}
	var sem chan struct{}
	switch {
	case rc.Has(RiskDestructive), rc.Has(RiskBilling), rc.Has(RiskAdmin), rc.Has(RiskPermissionChange), rc.Has(RiskExternalSideEffect):
		sem = l.high
	case rc.Has(RiskWrite):
		sem = l.write
	default:
		return func() {}, nil
	}
	select {
	case sem <- struct{}{}:
		return func() { <-sem }, nil
	case <-ctx.Done():
		return func() {}, ctx.Err()
	}
}

// rateLimitedEnvelope is the recoverable ok:false result returned when the
// per-process tool rate limit is exceeded.
func rateLimitedEnvelope(toolName string) map[string]any {
	return map[string]any{
		"ok":     false,
		"action": toolName,
		"error": map[string]any{
			"code":    "rate_limited",
			"message": "tool invocation rate limit exceeded; retry shortly",
		},
		"recovery": map[string]any{
			"hint":      "The server is rate-limiting tool calls (CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE). Wait a moment and retry.",
			"retryable": true,
		},
	}
}
