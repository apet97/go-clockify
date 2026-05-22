package mcp

import (
	"context"
	"testing"
	"time"
)

func TestTokenBucketAllowsBurstUpToCapacityThenDenies(t *testing.T) {
	b := newTokenBucket(120)
	allowed := 0
	for i := 0; i < 120; i++ {
		if b.allow() {
			allowed++
		}
	}
	if allowed != 120 {
		t.Fatalf("burst allowed %d calls, want 120", allowed)
	}
	if b.allow() {
		t.Fatal("call 121 should be denied — the bucket is empty")
	}
}

func TestTokenBucketNilAlwaysAllows(t *testing.T) {
	var b *tokenBucket // newTokenBucket(0) returns nil
	if newTokenBucket(0) != nil {
		t.Fatal("newTokenBucket(0) should return nil (disabled)")
	}
	for i := 0; i < 5; i++ {
		if !b.allow() {
			t.Fatal("a nil token bucket must always allow")
		}
	}
}

func TestFamilyLimiterCapsWritesAtTwo(t *testing.T) {
	l := newFamilyLimiter()
	ctx := context.Background()
	r1, err := l.acquire(ctx, RiskWrite)
	if err != nil {
		t.Fatalf("acquire 1: %v", err)
	}
	r2, err := l.acquire(ctx, RiskWrite)
	if err != nil {
		t.Fatalf("acquire 2: %v", err)
	}
	// Third write must block until a slot frees.
	deadlineCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := l.acquire(deadlineCtx, RiskWrite); err == nil {
		t.Fatal("third concurrent write should have blocked past the cap")
	}
	r1()
	r2()
}

func TestFamilyLimiterSerializesHighRiskWrites(t *testing.T) {
	l := newFamilyLimiter()
	ctx := context.Background()
	release, err := l.acquire(ctx, RiskDestructive)
	if err != nil {
		t.Fatalf("acquire high-risk: %v", err)
	}
	deadlineCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := l.acquire(deadlineCtx, RiskBilling); err == nil {
		t.Fatal("a second concurrent high-risk write should have blocked (cap is 1)")
	}
	release()
	// After release a high-risk slot is free again.
	release2, err := l.acquire(ctx, RiskAdmin)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	release2()
}

func TestFamilyLimiterDoesNotCapReads(t *testing.T) {
	l := newFamilyLimiter()
	ctx := context.Background()
	releases := make([]func(), 0, 10)
	for i := 0; i < 10; i++ {
		r, err := l.acquire(ctx, RiskRead)
		if err != nil {
			t.Fatalf("read acquire %d: %v", i, err)
		}
		releases = append(releases, r)
	}
	for _, r := range releases {
		r()
	}
}

func TestRateLimitedEnvelopeHasStructuredRecovery(t *testing.T) {
	env := rateLimitedEnvelope("clockify_status", RiskRead, 60)
	if env["ok"] != false {
		t.Fatalf("ok = %v, want false", env["ok"])
	}
	if env["action"] != "clockify_status" {
		t.Fatalf("action = %v, want clockify_status", env["action"])
	}
	errObj, ok := env["error"].(map[string]any)
	if !ok || errObj["code"] != "rate_limited" {
		t.Fatalf("error = %v, want code rate_limited", env["error"])
	}
	rec, ok := env["recovery"].(map[string]any)
	if !ok || rec["retryable"] != true || rec["hint"] == "" {
		t.Fatalf("recovery = %v, want a retryable hint", env["recovery"])
	}
	if rec["retryAfterSeconds"] != 60 {
		t.Fatalf("retryAfterSeconds = %v, want 60", rec["retryAfterSeconds"])
	}
}

func TestRiskRateLimiterSelectsSeparateBuckets(t *testing.T) {
	limits := RiskRateLimits{
		ReadPerMinute:         2,
		WritePerMinute:        1,
		BillingAdminPerMinute: 1,
		DestructivePerMinute:  1,
	}
	limiter := newRiskRateLimiter(limits)
	if limiter == nil {
		t.Fatal("newRiskRateLimiter returned nil")
	}
	if ok, _ := limiter.allow(RiskRead); !ok {
		t.Fatal("first read should be allowed")
	}
	if ok, _ := limiter.allow(RiskRead); !ok {
		t.Fatal("second read should be allowed from the read bucket")
	}
	if ok, _ := limiter.allow(RiskRead); ok {
		t.Fatal("third read should be rate-limited")
	}
	if ok, _ := limiter.allow(RiskWrite); !ok {
		t.Fatal("write should use its own bucket")
	}
	if ok, _ := limiter.allow(RiskWrite); ok {
		t.Fatal("second write should be rate-limited")
	}
	if ok, _ := limiter.allow(RiskBilling); !ok {
		t.Fatal("billing should use high-risk bucket")
	}
	if ok, _ := limiter.allow(RiskAdmin); ok {
		t.Fatal("admin should share the billing/admin bucket")
	}
	if ok, _ := limiter.allow(RiskDestructive); !ok {
		t.Fatal("destructive should use its own bucket")
	}
	if ok, _ := limiter.allow(RiskDestructive); ok {
		t.Fatal("second destructive call should be rate-limited")
	}
}

// TestProductWiringInstallsFamilyCapsWithRateLimitDisabled reproduces the wiring
// sequence from cmd/clockify-mcp/main.go: NewServer(...) followed by
// ConfigureToolLimits(cfg.ToolRateLimitPerMinute). CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE
// defaults to 0, so the product path routinely runs with the token-bucket rate
// limiter disabled — the per-risk-family concurrency caps must still be active.
func TestProductWiringInstallsFamilyCapsWithRateLimitDisabled(t *testing.T) {
	srv := NewServer("test", nil)
	if srv.toolFamilyCaps == nil {
		t.Fatal("NewServer must install per-risk-family concurrency caps")
	}

	srv.ConfigureToolLimits(0) // rate limiting disabled (the default)
	if srv.toolFamilyCaps == nil {
		t.Fatal("ConfigureToolLimits(0) must not clear the family caps installed by NewServer")
	}
	if srv.toolRateLimiter != nil {
		t.Fatal("ConfigureToolLimits(0) must leave the token-bucket rate limiter disabled")
	}

	// Behavioural proof: the high-risk family is serialized at 1 concurrent
	// even with rate limiting off.
	ctx := context.Background()
	release, err := srv.toolFamilyCaps.acquire(ctx, RiskDestructive)
	if err != nil {
		t.Fatalf("first high-risk acquire: %v", err)
	}
	blocked, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := srv.toolFamilyCaps.acquire(blocked, RiskBilling); err == nil {
		t.Fatal("a second concurrent high-risk acquire should block (cap is 1) with rate limiting disabled")
	}
	release()
}
