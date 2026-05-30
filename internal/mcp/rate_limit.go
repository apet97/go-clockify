package mcp

import (
	"context"
	"math"
	"sync"
	"time"
)

// RiskRateLimits configures per-minute invocation ceilings for tools grouped
// by risk tier. A zero or negative value for any field disables the limit for
// that tier.
type RiskRateLimits struct {
	ReadPerMinute         int
	WritePerMinute        int
	BillingAdminPerMinute int
	DestructivePerMinute  int
}

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
	ok, _ := b.allowWithRetry()
	return ok
}

func (b *tokenBucket) allowWithRetry() (bool, int) {
	if b == nil {
		return true, 0
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
		if b.refillRate <= 0 {
			return false, 60
		}
		seconds := int(math.Ceil((1 - b.tokens) / b.refillRate))
		if seconds < 1 {
			seconds = 1
		}
		return false, seconds
	}
	b.tokens--
	return true, 0
}

type riskRateLimiter struct {
	read         *tokenBucket
	write        *tokenBucket
	billingAdmin *tokenBucket
	destructive  *tokenBucket
}

func newRiskRateLimiter(limits RiskRateLimits) *riskRateLimiter {
	if limits.ReadPerMinute <= 0 &&
		limits.WritePerMinute <= 0 &&
		limits.BillingAdminPerMinute <= 0 &&
		limits.DestructivePerMinute <= 0 {
		return nil
	}
	return &riskRateLimiter{
		read:         newTokenBucket(limits.ReadPerMinute),
		write:        newTokenBucket(limits.WritePerMinute),
		billingAdmin: newTokenBucket(limits.BillingAdminPerMinute),
		destructive:  newTokenBucket(limits.DestructivePerMinute),
	}
}

func (l *riskRateLimiter) allow(rc RiskClass) (bool, int) {
	if l == nil {
		return true, 0
	}
	return l.bucket(rc).allowWithRetry()
}

func (l *riskRateLimiter) bucket(rc RiskClass) *tokenBucket {
	switch {
	case rc.Has(RiskDestructive):
		return l.destructive
	case rc.Has(RiskBilling), rc.Has(RiskAdmin), rc.Has(RiskPermissionChange), rc.Has(RiskExternalSideEffect):
		return l.billingAdmin
	case rc.Has(RiskWrite):
		return l.write
	default:
		return l.read
	}
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
func rateLimitedEnvelope(toolName string, rc RiskClass, retryAfterSeconds int) map[string]any {
	if retryAfterSeconds <= 0 {
		retryAfterSeconds = 60
	}
	message := "tool invocation rate limit exceeded"
	switch {
	case rc.Has(RiskDestructive):
		message = "destructive tool rate limit exceeded"
	case rc.Has(RiskBilling), rc.Has(RiskAdmin), rc.Has(RiskPermissionChange), rc.Has(RiskExternalSideEffect):
		message = "high-risk tool rate limit exceeded"
	case rc.Has(RiskWrite):
		message = "write tool rate limit exceeded"
	case rc.Has(RiskSensitiveRead):
		message = "sensitive read tool rate limit exceeded"
	}
	return map[string]any{
		"ok":     false,
		"action": toolName,
		"error": map[string]any{
			"code":    "rate_limited",
			"message": message,
		},
		"recovery": map[string]any{
			"hint":              "The server is rate-limiting tool calls. Wait and retry.",
			"retryable":         true,
			"retryAfterSeconds": retryAfterSeconds,
		},
	}
}
